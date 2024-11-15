/*
Copyright 2024 NVIDIA.

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

package ovshelper

import (
	"context"
	"time"

	"github.com/nvidia/doca-platform/internal/utils/ovsclient"

	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
)

// OVSHelper is responsible for finding and resolving known OVS bugs that the system hits in runtime
type OVSHelper struct {
	// runTicker is the ticker that orchestrates when the job should run
	runTicker clock.Ticker
	ovsClient ovsclient.OVSClient
}

// New creates a OVSHelper
func New(clock clock.WithTicker, ovsClient ovsclient.OVSClient) *OVSHelper {
	return &OVSHelper{
		runTicker: clock.NewTicker(30 * time.Second),
		ovsClient: ovsClient,
	}
}

// RunOnce runs the find and fix job once
func (o *OVSHelper) RunOnce() error {
	return nil
}

// Start starts the OVSHelper find and fix loop. This is a blocking function.
func (o *OVSHelper) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.runTicker.C():
			if err := o.RunOnce(); err != nil {
				klog.Errorf("error while running the job: %s", err.Error())
			}
		}
	}
}
