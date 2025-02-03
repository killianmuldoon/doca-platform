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
	"strconv"
	"strings"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/bfb/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	"github.com/Masterminds/semver/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BFBDownloader is an interface for creating and managing BFB download jobs.
type BFBDownloader interface {
	CreateBFBDownloadJob(ctx context.Context, client client.Client, bfb *provisioningv1.BFB, option util.BFBOptions) error
	ProcessJobConditions(job *batchv1.Job, timeout time.Duration) (bool, error)
	GetBFBVersion(filePath string) (provisioningv1.BFBVersions, error)
}

// RealBFBDownloader is a real implementation of the BFBDownloader interface.
type RealBFBDownloader struct{}

func (r *RealBFBDownloader) CreateBFBDownloadJob(ctx context.Context, client client.Client, bfb *provisioningv1.BFB, option util.BFBOptions) error {
	logger := log.FromContext(ctx)
	jobName := cutil.GenerateBFBJobName(*bfb)
	bfbDownloaderCommand := fmt.Sprintf(
		"%s --url=%s --file=%s --uid=%s --base-dir=%s --versions-output=%s",
		cutil.BFBDownloaderScript,
		bfb.Spec.URL,
		bfb.Status.FileName,
		string(bfb.UID),
		cutil.BFBBaseDirPath,
		cutil.GenerateBFBVersionFilePath(bfb.Status.FileName),
	)
	backOffLimit := int32(6) // Specifies the number of retries before marking this job failed.

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: bfb.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bfb, provisioningv1.BFBGroupVersionKind),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: bfb.Namespace,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            cutil.BFBDownloader,
							Image:           option.BFBDownloaderImageWithTag,
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "bfb",
									MountPath: "/bfb",
								},
							},
							Command: []string{"/bin/bash", "-c", "--"},
							Args: []string{
								bfbDownloaderCommand,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("4"),
									corev1.ResourceMemory: resource.MustParse("8Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/control-plane",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "node-role.kubernetes.io/control-plane",
												Operator: corev1.NodeSelectorOpExists,
											},
										},
									},
								},
							},
						},
					},
					ImagePullSecrets: option.ImagePullSecrets,
					Volumes: []corev1.Volume{
						{
							Name: "bfb",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: option.BFBPVC,
								},
							},
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
	return nil
}

func (r *RealBFBDownloader) ProcessJobConditions(job *batchv1.Job, timeout time.Duration) (bool, error) {
	for _, condition := range job.Status.Conditions {
		switch {
		case condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue:
			return true, nil

		case condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue:
			return false, fmt.Errorf("job %s/%s failed", job.Namespace, job.Name)

		default:
			if job.Status.StartTime != nil {
				startTime := job.Status.StartTime.Time
				elapsedTime := time.Since(startTime)
				if elapsedTime > timeout {
					return false, fmt.Errorf("job %s/%s timed out", job.Namespace, job.Name)
				}
			}
		}
	}
	return false, nil
}

func (r *RealBFBDownloader) GetBFBVersion(filePath string) (provisioningv1.BFBVersions, error) {
	bfbVer := provisioningv1.BFBVersions{}
	// Check if the file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return bfbVer, fmt.Errorf("file does not exist: %s", filePath)
		}
		return bfbVer, fmt.Errorf("error checking file: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return bfbVer, fmt.Errorf("failed to read file: %s, error: %v", filePath, err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ATF version") {
			bfbVer.ATF = strings.TrimSpace(strings.Join(strings.Split(line, ":")[1:], ":"))
		} else if strings.Contains(line, "UEFI version") {
			bfbVer.UEFI = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.Contains(line, "BSP version") {
			bfbVer.BSP = strings.TrimSpace(strings.Split(line, ":")[1])

			docaVersion, err := parseDOCAVersionFromBSP(bfbVer.BSP)
			if err != nil {
				return bfbVer, fmt.Errorf("error parsing DOCA version from BSP: %w", err)
			}
			bfbVer.DOCA = docaVersion
		}
	}
	return bfbVer, nil
}

// parseDOCAVersionFromBSP parses the DOCA version from BSP version
// This is temporary solution until the actual DOCA version is populated in the BFB.
func parseDOCAVersionFromBSP(bspVersion string) (string, error) {
	// Compute docaVersion by adjusting the major version of BSP
	bspParts := strings.Split(bspVersion, ".")
	if len(bspParts) == 0 {
		return "", fmt.Errorf("invalid BSP version")
	}
	majorVersion, err := strconv.Atoi(bspParts[0])
	if err != nil {
		return "", fmt.Errorf("failed to parse BSP major version: %v", err)
	}
	if majorVersion < 2 {
		return "", fmt.Errorf("BSP major version is less than 2")
	}
	// We keep only the first 3 parts of the version to create a valid semver. In the future, we won't
	// need to do that as the DOCA team is going to push the actual DOCA version in bfver. DOCA version
	// is compliant with semver
	if len(bspParts) > 3 {
		bspParts = bspParts[:3]
	}
	docaVersion := fmt.Sprintf("%d.%s", majorVersion-2, strings.Join(bspParts[1:], "."))
	_, err = semver.NewVersion(docaVersion)
	if err != nil {
		return "", fmt.Errorf("DOCA version error: %w", err)
	}
	return docaVersion, nil
}
