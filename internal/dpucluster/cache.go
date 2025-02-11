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
	"errors"
	"fmt"
	"sync"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// This package is based on https://github.com/kubernetes-sigs/cluster-api/blob/v1.9.4/controllers/clustercache/cluster_cache.go
// It is a re-implementation of the clusterCache for the specific usage in DPF for dpu clusters.

const (
	// defaultRequeueAfter is used as a fallback if no other duration should be used.
	defaultRequeueAfter = 10 * time.Second
)

// ErrDPUClusterNotConnected is returned by the dpuClusterCacheReconciler when e.g. a Client cannot be returned
// because there is no connection to the dpu cluster.
var ErrDPUClusterNotConnected = errors.New("connection to the dpu cluster is down")

// CacheOptionsIndex is a index that is added to the cache.
type CacheOptionsIndex struct {
	// Object is the object for which the index is created.
	Object client.Object

	// Field is the field of the index that can later be used when selecting
	// objects with a field selector.
	Field string

	// ExtractValue is a func that extracts the index value from an object.
	ExtractValue client.IndexerFunc
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

// WatcherOptions specifies the parameters used to establish a new watch for a workload cluster.
// A source.TypedKind source (configured with Kind, TypedEventHandler and Predicates) will be added to the Watcher.
// To watch for events, the source.TypedKind will create an informer on the Cache that we have created and cached
// for the given Cluster.
type WatcherOptions = TypedWatcherOptions[client.Object, ctrl.Request]

// TypedWatcherOptions specifies the parameters used to establish a new watch for a workload cluster.
// A source.TypedKind source (configured with Kind, TypedEventHandler and Predicates) will be added to the Watcher.
// To watch for events, the source.TypedKind will create an informer on the Cache that we have created and cached
// for the given Cluster.
type TypedWatcherOptions[object client.Object, request comparable] struct {
	// Name represents a unique Watch request for the specified Cluster.
	// The name is used to track that a specific watch is only added once to a cache.
	// After a connection (and thus also the cache) has been re-created, watches have to be added
	// again by calling the Watch method again.
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

// NewWatcher creates a Watcher for the workload cluster.
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
func SetupWithManager(ctx context.Context, mgr ctrl.Manager, controllerOptions controller.Options) (*RemoteCache, error) {
	cc := &RemoteCache{
		client:         mgr.GetAPIReader(),
		accessorConfig: buildClusterAccessorConfig(mgr.GetScheme()),
		accessors:      make(map[client.ObjectKey]*accessor),
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&provisioningv1.DPUCluster{}).
		WithOptions(controllerOptions).
		Complete(cc)
	if err != nil {
		return nil, fmt.Errorf("failed setting up clusterCacheReconciler with a controller manager: %w", err)
	}

	return cc, nil
}

// RemoteCache reconcile dpuClusters CRs and manges caches for those dpuClusters.
// It provides a way to watch dpuClusters and get cachedClients for dpuClusters.
// Specific objects can be watched in the dpu cluster.
type RemoteCache struct {
	client client.Reader

	// accessorConfig is the config for accessors.
	accessorConfig *accessorConfig

	// accessorsLock is used to synchronize access to accessors.
	accessorsLock sync.RWMutex

	// accessors is the map of accessors by dpu cluster.
	accessors map[client.ObjectKey]*accessor

	// dpuClusterSourcesLock is used to synchronize access to dpuClusterSources.
	dpuClusterSourcesLock sync.RWMutex

	// dpuClusterSources is used to store information about cluster sources.
	// This information is necessary so we can enqueue reconcile.Requests for reconcilers that
	// got a cluster source via GetDPUClusterSource.
	dpuClusterSources []clusterSource
}

// clusterSource stores the necessary information so we can enqueue reconcile.Requests for reconcilers that
// got a cluster source via GetClusterSource.
type clusterSource struct {
	// controllerName is the name of the controller that will watch this source.
	controllerName string

	// ch is the channel on which to send events.
	ch chan event.TypedGenericEvent[provisioningv1.DPUCluster]
}

// Reconcile reconciles Clusters and manages corresponding accessors.
func (cc *RemoteCache) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")

	cluster := &provisioningv1.DPUCluster{}
	if err := cc.client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("dpuCluster has been deleted, disconnecting")
			return ctrl.Result{}, nil
		}

		// Requeue, error getting the object.
		log.Error(err, fmt.Sprintf("Requeuing after %s (error getting dpuCluster object)", defaultRequeueAfter))
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	return cc.reconcile(ctx, cluster)
}

// reconcile handles the main reconciliation loop
//
//nolint:unparam
func (cc *RemoteCache) reconcile(ctx context.Context, dpuCluster *provisioningv1.DPUCluster) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func buildClusterAccessorConfig(scheme *runtime.Scheme) *accessorConfig {
	return &accessorConfig{
		scheme:                          scheme,
		connectionCreationRetryInterval: 30 * time.Second,
		cache: &accessorCacheConfig{
			initialSyncTimeout: 5 * time.Minute,
			syncPeriod:         &[]time.Duration{5 * time.Minute}[0],
			byObject:           make(map[client.Object]cache.ByObject),
			indexes:            []CacheOptionsIndex{},
		},
		client: &accessorClientConfig{
			timeout:   10 * time.Second,
			qps:       100,
			burst:     100,
			userAgent: fmt.Sprintf("doca-platform/%s", "0.0.1"),
		},
	}
}

// GetNewDPUClusterSource returns a Source of dpu cluster events.
// The mapFunc will be used to map from dpu cluster to reconcile.Request.
// reconcile.Requests will always be enqueued on connect and disconnect.
// Reconciler can use the Source to watch for dpu cluster events.
// e.g.:
//
//	ctrl.NewControllerManagedBy(mgr).
//	  Named("dpuServiceChain-controller").
//	  For(&dpuServicev1.DPUServiceChain{}).
//	  WatchesRawSources(dpuClusterCache.GetNewDPUClusterSource("dpuServiceChain", dpuClusterToDPUServiceChains).
//	  Complete(&DPUServiceChainReconciler{Client: mgr.GetClient()})
func (cc *RemoteCache) GetNewDPUClusterSource(controllerName string, mapFunc func(ctx context.Context, dpuCluster provisioningv1.DPUCluster) []ctrl.Request) source.Source {
	cc.dpuClusterSourcesLock.Lock()
	defer cc.dpuClusterSourcesLock.Unlock()

	cs := clusterSource{
		controllerName: controllerName,
		ch:             make(chan event.TypedGenericEvent[provisioningv1.DPUCluster]),
	}
	cc.dpuClusterSources = append(cc.dpuClusterSources, cs)

	return source.Channel(cs.ch, handler.TypedEnqueueRequestsFromMapFunc(mapFunc))
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
func (cc *RemoteCache) Watch(ctx context.Context, cluster client.ObjectKey, watcher Watcher) error {
	accessor := cc.getaccessor(cluster)
	if accessor == nil {
		return fmt.Errorf("could not create watch %s for %T: %w", watcher.Name(), watcher.Object(), ErrDPUClusterNotConnected)
	}
	return accessor.Watch(ctx, watcher)
}

// getaccessor returns a accessor if it exists, otherwise nil.
func (cc *RemoteCache) getaccessor(cluster client.ObjectKey) *accessor {
	cc.accessorsLock.RLock()
	defer cc.accessorsLock.RUnlock()

	if accessor, ok := cc.accessors[cluster]; ok {
		return accessor
	}

	accessor := newAccessor(cluster, cc.accessorConfig)
	cc.accessorsLock.Lock()
	defer cc.accessorsLock.Unlock()
	cc.accessors[cluster] = accessor
	return accessor
}

// GetClient returns a client for the given dpu cluster.
// It is a cached client that read from the given dpu cluster cache.
func (cc *RemoteCache) GetClient(ctx context.Context, cluster client.ObjectKey) (client.Client, error) {
	accessor := cc.getaccessor(cluster)
	if accessor == nil {
		return nil, fmt.Errorf("could not get client for %T: %w", cluster, ErrDPUClusterNotConnected)
	}
	return nil, nil
}
