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

package bfbdownloader

import (
	"context"
	"fmt"
	"os"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/bfb/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MockBFBDownloader is a mock implementation of the BFBDownloader interface for testing purposes.
type MockBFBDownloader struct{}

func (m *MockBFBDownloader) CreateBFBDownloadJob(ctx context.Context, client client.Client, bfb *provisioningv1.BFB, option util.BFBOptions) error {
	logger := log.FromContext(ctx)
	jobName := cutil.GenerateBFBJobName(*bfb)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: bfb.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bfb, provisioningv1.BFBGroupVersionKind),
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "minimal-job",
							Image:   "busybox",
							Command: []string{"sh", "-c", "exit 0"},
						},
					},
				},
			},
		},
	}

	err := client.Create(ctx, job)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create %s  job", jobName))
		return err
	}
	logger.V(3).Info(fmt.Sprintf("%s job created", jobName))
	if err = createFakeBFBFile(cutil.GenerateBFBFilePath(bfb.Status.FileName)); err != nil {
		return err
	}
	return nil
}

func (m *MockBFBDownloader) ProcessJobConditions(_ *batchv1.Job, _ time.Duration) (bool, error) {
	return true, nil
}

func (m *MockBFBDownloader) GetBFBVersion(_ string) (provisioningv1.BFBVersions, error) {
	return provisioningv1.BFBVersions{DOCA: "2.8.0", BSP: "4.8.0.13249", ATF: "v2.2(release)", UEFI: "4.8.0-36-gf01f42f"}, nil
}

func createFakeBFBFile(filePath string) error {
	_, err := os.Create(filePath)
	if err != nil {
		return err
	}
	err = os.Chmod(filePath, 0755)
	if err != nil {
		return err
	}

	return nil
}
