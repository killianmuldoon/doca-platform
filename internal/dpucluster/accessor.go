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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// accessor is the object used to create and manage connections to a specific dpu cluster.
type accessor struct {
	dpuCluster client.ObjectKey

	// config is the config of the accessor.
	config *accessorConfig
}

// accessorConfig is the config of the accessor.
type accessorConfig struct {
	// scheme is the scheme used for the client and the cache.
	scheme *runtime.Scheme

	// connectionCreationRetryInterval is the interval after which to retry to create a
	// connection after creating a connection failed.
	connectionCreationRetryInterval time.Duration

	// cache is the config used for the cache that the accessor creates.
	cache *accessorCacheConfig

	// client is the config used for the client that the accessor creates.
	client *accessorClientConfig
}

// accessorCacheConfig is the config used for the cache that the accessor creates.
type accessorCacheConfig struct {
	// initialSyncTimeout is the timeout used when waiting for the cache to sync after cache start.
	initialSyncTimeout time.Duration

	// syncPeriod is the sync period of the cache.
	syncPeriod *time.Duration

	// byObject restricts the cache's ListWatch to the desired fields per GVK at the specified object.
	byObject map[client.Object]cache.ByObject

	// indexes are the indexes added to the cache.
	indexes []CacheOptionsIndex
}

// accessorClientConfig is the config used for the client that the accessor creates.
type accessorClientConfig struct {
	// timeout is the timeout used for the rest.Config.
	// The rest.Config is also used to create the client and the cache.
	timeout time.Duration

	// qps is the qps used for the rest.Config.
	// The rest.Config is also used to create the client and the cache.
	qps float32

	// burst is the burst used for the rest.Config.
	// The rest.Config is also used to create the client and the cache.
	burst int

	// userAgent is the user agent used for the rest.Config.
	// The rest.Config is also used to create the client and the cache.
	userAgent string
}

// newAccessor creates a new accessor.
func newAccessor(dpuCluster client.ObjectKey, clusterAccessorConfig *accessorConfig) *accessor {
	return &accessor{
		dpuCluster: dpuCluster,
		config:     clusterAccessorConfig,
	}
}

// Watch watches a dpu cluster for events.
// Each unique watch (by watcher.Name()) is only added once after a Connect (otherwise we return early).
// During a disconnect existing watches (i.e. informers) are shutdown when stopping the cache.
// After a re-connect watches will be re-added (assuming the Watch method is called again).
func (a *accessor) Watch(ctx context.Context, watcher Watcher) error {
	// not implemented
	return nil
}
