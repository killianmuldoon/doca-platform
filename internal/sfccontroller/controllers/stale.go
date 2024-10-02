/*
COPYRIGHT 2024 NVIDIA

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

package controller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type StaleFlowsRemover struct {
	duration time.Duration
	client   client.Client
}

func NewStaleFlowsRemover(duration time.Duration, client client.Client) *StaleFlowsRemover {
	return &StaleFlowsRemover{duration: duration, client: client}
}

func (r *StaleFlowsRemover) Start(ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	log.Info("setup stale flows cleaner")
	ticker := time.NewTicker(r.duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := r.removeStaleFlows(ctx)
			if err != nil {
				log.Error(err, "failed to remove stale flows")
			}
		}
	}
}

// removeStaleFlows compares the derived OpenFlow cookies from the current CRs
// and compares them against actual OpenFlow cookies in the OVS flows.
// The difference is treated as stale flows and removed.
func (r *StaleFlowsRemover) removeStaleFlows(ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	log.Info("running stale flows cleaner")
	currentCookiesSet, err := getFlowCookies()
	if err != nil {
		return fmt.Errorf("failed to get flow cookies: %w", err)
	}
	desiredCookiesSet := sets.New[string]()
	serviceChainList := &dpuservicev1.ServiceChainList{}
	if err = r.client.List(ctx, serviceChainList); err != nil {
		return fmt.Errorf("failed to list service chains: %w", err)
	}

	for _, serviceChain := range serviceChainList.Items {
		serviceChainNamespacedName := serviceChain.GetNamespace() + "/" + serviceChain.Name
		desiredCookiesSet.Insert("0x" + strconv.FormatUint(hash(serviceChainNamespacedName), 16))
	}

	msg := fmt.Sprintf("lenCurrentCookies: %d lenDesiredCookies: %d currentCookies: %s desiredCookies: %s",
		len(currentCookiesSet), len(desiredCookiesSet),
		currentCookiesSet.UnsortedList(), desiredCookiesSet.UnsortedList())
	// TODO: Use a logger which has debug level. eg: logrus
	log.V(5).Info(msg) // debug level, info is level 0.

	unwantedCookiesSet := currentCookiesSet.Difference(desiredCookiesSet)

	for flowCookie := range unwantedCookiesSet {
		log.V(5).Info(fmt.Sprintf("remove cookie=%s/-1", flowCookie))
		flowErrors := delFlows(fmt.Sprintf("cookie=%s/-1", flowCookie))
		if flowErrors != nil {
			return fmt.Errorf("failed to delete flow cookie %s : %w", flowCookie, flowErrors)
		}
	}
	return nil
}
