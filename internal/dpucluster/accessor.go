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

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This package is based on https://github.com/kubernetes-sigs/cluster-api/blob/v1.9.4/controllers/clustercache/
// It is a re-implementation of the clusterCache for the specific usage in DPF for dpu clusters.

// accessor is the object used to create and manage connections to a specific dpu cluster.
type accessor struct {
	cluster client.ObjectKey

	// The methods lock, or rLock should always be used to access this field.
	state accessorState

	sync.RWMutex
}

// accessorState is the state of the accessor.
type accessorState struct {
	// connection holds the connection state (e.g. client, cache) of the accessor.
	connection *accessorConnectionState
}

// accessorConnectionState holds the connection state (e.g. client, cache) of the accessor.
type accessorConnectionState struct {
	// cachedClient to communicate with the dpu cluster.
	// It uses cache for Get & List calls for all Unstructured objects and
	// all typed objects.
	cachedClient client.Client

	// cache is the cache used by the client.
	// It manages informers that have been created e.g. by adding indexes to the cache,
	// Get & List calls from the client or via the Watch method of the accessor.
	cache *cacheWithCancel

	// watches is used to track the watches that have been added through the Watch method
	// of the accessor. This is important to avoid adding duplicate watches.
	watches sets.Set[string]
}

// newAccessor creates a new accessor.
func newAccessor(dpuCluster client.ObjectKey) *accessor {
	return &accessor{
		cluster: dpuCluster,
	}
}

// connect creates a connection to the dpu cluster, i.e. it creates a client, cache, etc.
func (a *accessor) connect(ctx context.Context, opts *Options) (retErr error) {
	if a.isConnected() {
		return nil
	}

	// Creating clients, cache etc. is intentionally done without a lock to avoid blocking other reconcilers.
	// Controller-runtime guarantees that the DPUClusterCache reconciler will never reconcile the same DPUCluster concurrently.
	// Thus, we can rely on this method not being called concurrently for the same DPUCluster / accessor.
	// This means we don't have to hold the lock when we create the client, cache, etc.
	cachedClient, cache, err := a.createConnection(ctx, opts)
	if err != nil {
		return err
	}

	a.Lock()
	defer a.Unlock()

	a.state.connection = &accessorConnectionState{
		cachedClient: cachedClient,
		cache:        cache,
		watches:      sets.Set[string]{},
	}

	return nil

}

func (a *accessor) isConnected() bool {
	a.RLock()
	defer a.RUnlock()

	return a.state.connection != nil
}

func (a *accessor) getClient() (client.Client, error) {
	a.RLock()
	defer a.RUnlock()

	if a.state.connection == nil {
		return nil, fmt.Errorf("no connection available")
	}

	return a.state.connection.cachedClient, nil
}

// Watch watches a dpu cluster for events.
func (a *accessor) watch(ctx context.Context, watcher Watcher) error {
	// not implemented
	return nil
}
