/*
Copyright 2025 NVIDIA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dpucluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// This package is based on https://github.com/kubernetes-sigs/cluster-api/blob/v1.9.4/controllers/clustercache/
// It is a re-implementation of the clusterCache for the specific usage in DPF for dpu clusters.

const (
	// defaultRequeueAfter is used as a fallback if no other duration should be used.
	defaultRequeueAfter = 10 * time.Second
)

// ErrDPUClusterNotConnected is returned when a dpu cluster that is not connected.
var ErrDPUClusterNotConnected = fmt.Errorf("dpu cluster is not connected")

type Options struct {
	// scheme is the scheme used for the client and the cache.
	scheme *runtime.Scheme

	// hostClient is the client for the host cluster. It is used to fetch the
	// kubeconfig secret.
	hostClient client.Reader

	// initialSyncTimeout is the timeout used when waiting for the cache to sync after cache start.
	initialSyncTimeout time.Duration

	// syncPeriod is the sync period of the cache.
	syncPeriod *time.Duration

	ClientOptions
}

type Option interface {
	Apply(options *Options)
}

// OptionScheme is the scheme used for the client and the cache.
type OptionScheme struct {
	Scheme *runtime.Scheme
}

func (o OptionScheme) Apply(options *Options) {
	options.scheme = o.Scheme
}

// OptionHostClient is the client for the host cluster. It is used to fetch the kubeconfig secret.
type OptionHostClient struct {
	Client client.Reader
}

func (o OptionHostClient) Apply(options *Options) {
	options.hostClient = o.Client
}

// OptionTimeout is the timeout used for the REST config, client and cache.
type OptionTimeout time.Duration

func (o OptionTimeout) Apply(options *Options) {
	options.timeout = time.Duration(o)
}

// OptionUserAgent is the user agent used for the REST config, client and cache.
type OptionUserAgent string

func (o OptionUserAgent) Apply(options *Options) {
	options.userAgent = string(o)
}

// OptionInitialSyncTimeout is the timeout used when waiting for the cache to sync after cache start.
type OptionInitialSyncTimeout time.Duration

func (o OptionInitialSyncTimeout) Apply(options *Options) {
	options.initialSyncTimeout = time.Duration(o)
}

// OptionSyncPeriod is the sync period of the cache.
type OptionSyncPeriod time.Duration

func (o OptionSyncPeriod) Apply(options *Options) {
	options.syncPeriod = ptr.To(time.Duration(o))
}

// Watcher is an interface that can start a Watch.
type Watcher interface {
	Name() string
	Object() client.Object
	Watch(cache cache.Cache) error
}

// SourceWatcher is a scoped-down interface from Controller that only has the Watch func.
type SourceWatcher[request comparable] interface {
	Watch(src source.TypedSource[request]) error
}

// WatcherOptions specifies the parameters used to establish a new watch for a dpu cluster.
// A source.TypedKind source (configured with Kind, TypedEventHandler and Predicates) will be added to the Watcher.
// To watch for events, the source.TypedKind will create an informer on the Cache that we have created and cached
// for the given Cluster.
type WatcherOptions = TypedWatcherOptions[client.Object, ctrl.Request]

// TypedWatcherOptions specifies the parameters used to establish a new watch for a dpu cluster.
// A source.TypedKind source (configured with Kind, TypedEventHandler and Predicates) will be added to the Watcher.
// To watch for events, the source.TypedKind will create an informer on the Cache that we have created and cached
// for the given Cluster.
type TypedWatcherOptions[object client.Object, request comparable] struct {
	// Name represents a unique Watch request for the specified Cluster.
	Name string

	// Watcher is the watcher (controller) whose Reconcile() function will be called for events.
	Watcher SourceWatcher[request]

	// Kind is the type of resource to watch.
	Kind object

	// EventHandler contains the event handlers to invoke for resource events.
	EventHandler handler.TypedEventHandler[object, request]

	// Predicates is used to filter resource events.
	Predicates []predicate.TypedPredicate[object]
}

// NewWatcher creates a Watcher for the dpu cluster.
// A source.TypedKind source (configured with Kind, TypedEventHandler and Predicates) will be added to the SourceWatcher.
// To watch for events, the source.TypedKind will create an informer on the Cache that we have created and cached
// for the given Cluster.
func NewWatcher[object client.Object, request comparable](options TypedWatcherOptions[object, request]) Watcher {
	return &watcher[object, request]{
		name:         options.Name,
		kind:         options.Kind,
		eventHandler: options.EventHandler,
		predicates:   options.Predicates,
		watcher:      options.Watcher,
	}
}

type watcher[object client.Object, request comparable] struct {
	name         string
	kind         object
	eventHandler handler.TypedEventHandler[object, request]
	predicates   []predicate.TypedPredicate[object]
	watcher      SourceWatcher[request]
}

func (tw *watcher[object, request]) Name() string          { return tw.name }
func (tw *watcher[object, request]) Object() client.Object { return tw.kind }
func (tw *watcher[object, request]) Watch(cache cache.Cache) error {
	return tw.watcher.Watch(source.TypedKind[object, request](cache, tw.kind, tw.eventHandler, tw.predicates...))
}

// SetupWithManager sets up a clusterCacheReconciler with the given Manager and Options.
// This will add a reconciler to the Manager and returns a clusterCacheReconciler which can be used
// to retrieve e.g. Clients for a given Cluster.
func SetupWithManager(ctx context.Context, mgr ctrl.Manager, opts ...Option) (*RemoteCache, error) {
	rc := &RemoteCache{
		client:    mgr.GetClient(),
		options:   makeRemoteCacheOptions(opts...),
		accessors: make(map[client.ObjectKey]*accessor),
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&provisioningv1.DPUCluster{}).
		Complete(rc)
	if err != nil {
		return nil, fmt.Errorf("failed setting up clusterCacheReconciler with a controller manager: %w", err)
	}

	return rc, nil
}

// RemoteCache reconcile dpuClusters CRs and manges caches for those dpuClusters.
// It provides a way to watch dpuClusters and get cachedClients for dpuClusters.
// Specific objects can be watched in the dpu cluster.
type RemoteCache struct {
	client client.Reader

	// accessors is the map of accessors by dpu cluster.
	accessors map[client.ObjectKey]*accessor

	options *Options

	// accessorsLock is used to synchronize arcess to accessors.
	sync.RWMutex
}

// Reconcile reconciles Clusters and manages corresponding accessors.
func (rc *RemoteCache) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")

	cluster := &provisioningv1.DPUCluster{}
	if err := rc.client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("dpuCluster has been deleted, disconnecting")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	return rc.reconcile(ctx, cluster)
}

// reconcile handles the main reconciliation loop
//
//nolint:unparam
func (rc *RemoteCache) reconcile(ctx context.Context, cluster *provisioningv1.DPUCluster) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// wait until the cluster is ready
	if cluster.Status.Phase != provisioningv1.PhaseReady {
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	clusterKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}
	accessor := rc.getAccessor(clusterKey)
	if accessor == nil {
		accessor = rc.createAccessor(clusterKey)
	}

	// Try to connect, if not connected.
	connected := accessor.isConnected()
	if !connected {
		if err := accessor.connect(ctx, rc.options); err != nil {
			log.Info("Requeuing after %s (error connecting to dpuCluster)", defaultRequeueAfter)
			return ctrl.Result{RequeueAfter: defaultRequeueAfter}, fmt.Errorf("error connecting to dpuCluster: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

// createAccessor creates a new accessor for the given dpu cluster.
// It overwrites an existing accessor.  Use getAccessor to check if an accessor exists.
func (rc *RemoteCache) createAccessor(cluster client.ObjectKey) *accessor {
	rc.Lock()
	defer rc.Unlock()

	accessor := newAccessor(cluster)
	rc.accessors[cluster] = accessor
	return accessor
}

// getaccessor returns a accessor if it exists, otherwise nil.
func (rc *RemoteCache) getAccessor(cluster client.ObjectKey) *accessor {
	rc.RLock()
	defer rc.RUnlock()

	if accessor, ok := rc.accessors[cluster]; ok {
		return accessor
	}

	return nil
}

// Watch can be used to watch specific resources in the dpu cluster.
// e.g.:
//
//	err := RemoteCache.Watch(ctx, util.ObjectKey(cluster), clustercache.NewWatcher(clustercache.WatcherOptions{
//		Name:         "watch-DPUNodes",
//		Watcher:      r.controller,
//		Kind:         &provisionningv1aplha1.DPUNode{},
//		EventHandler: handler.EnqueueRequestsFromMapFunc(r.dpuNodeToVPC),
//	})
func (rc *RemoteCache) Watch(ctx context.Context, cluster client.ObjectKey, watcher Watcher) error {
	accessor := rc.getAccessor(cluster)
	if accessor == nil {
		return fmt.Errorf("could not create watch %s for %T: %w", watcher.Name(), watcher.Object(), ErrDPUClusterNotConnected)
	}
	return accessor.watch(ctx, watcher)
}

// GetClient returns a client for the given dpu cluster.
// It is a cached client that read from the given dpu cluster cache.
func (rc *RemoteCache) GetClient(cluster client.ObjectKey) (client.Client, error) {
	accessor := rc.getAccessor(cluster)
	if accessor == nil {
		return nil, fmt.Errorf("could not get client for %T: %w", cluster, ErrDPUClusterNotConnected)
	}
	return accessor.getClient()
}

func makeRemoteCacheOptions(opts ...Option) *Options {
	options := &Options{}
	for _, o := range opts {
		o.Apply(options)
	}

	if options.initialSyncTimeout == 0 {
		options.initialSyncTimeout = 5 * time.Minute
	}
	return options
}
