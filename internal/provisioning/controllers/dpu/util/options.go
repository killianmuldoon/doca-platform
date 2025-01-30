/*
Copyright 2024 NVIDIA

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

package util

import (
	"sync"
	"time"

	"github.com/nvidia/doca-platform/internal/provisioning/controllers/allocator"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/future"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/reboot"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MaxRetryCount = 10
)

var OsInstallTaskMap sync.Map
var RebootTaskMap sync.Map

type TaskWithRetry struct {
	Task       *future.Future
	RetryCount int
}

type DPUOptions struct {
	DMSImageWithTag          string
	HostnetworkImageWithTag  string
	PrarprouterdImageWithTag string
	ImagePullSecrets         []corev1.LocalObjectReference
	BFBPVC                   string
	DMSTimeout               int
	DMSPodTimeout            time.Duration
	DPUInstallInterface      string
	BFCFGTemplateFile        string
}

type ControllerContext struct {
	client.Client
	Scheme               *runtime.Scheme
	Options              DPUOptions
	Recorder             record.EventRecorder
	ClusterAllocator     allocator.Allocator
	JoinCommandGenerator NodeJoinCommandGenerator
	HostUptimeChecker    reboot.HostUptimeChecker
}
