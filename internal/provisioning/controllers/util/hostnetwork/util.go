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

package hostnetwork

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	NumofVFDefaultValue        = "16"
	ParprouterdRefreshInterval = 30
)

func getNumOfVFsFromFlavor(flavor *provisioningdpfv1alpha1.DPUFlavor) (string, bool) {
	regex := regexp.MustCompile(`^NUM_OF_VFS=([0-9]+)`)
	for _, nvconfig := range flavor.Spec.NVConfig {
		for _, parmeter := range nvconfig.Parameters {
			matches := regex.FindStringSubmatch(parmeter)
			if len(matches) == 2 {
				return matches[1], true
			}
		}
	}
	return "", false
}

func CreateHostNetworkSetupPod(ctx context.Context, client client.Client, dpu *provisioningdpfv1alpha1.Dpu, option dutil.DPUOptions) error {
	logger := log.FromContext(ctx)
	hostnetworkPodName := cutil.GenerateHostnetworkPodName(dpu.Name)

	num_of_vfs := NumofVFDefaultValue
	flavor := &provisioningdpfv1alpha1.DPUFlavor{}
	if err := client.Get(ctx, types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      dpu.Spec.DPUFlavor,
	}, flavor); err != nil {
		return err
	}

	if num, ok := getNumOfVFsFromFlavor(flavor); ok {
		num_of_vfs = num
	}

	var pci_address string
	if pci, ok := dpu.Labels[cutil.DpuPCIAddressLabel]; ok {
		// replace - to :
		// 0000:4b:00
		pci_address = strings.ReplaceAll(pci, "-", ":")
	} else {
		return fmt.Errorf("no PCI address on DPU %s/%s label", dpu.Namespace, dpu.Name)
	}

	hostCIDR, cidrerr := cutil.GetHostCIDRFromOpenShiftClusterConfig(ctx, client)
	if cidrerr != nil {
		return fmt.Errorf("error while getting Host CIDR from OpenShift cluster config: %w", cidrerr)
	}

	hostPathType := corev1.HostPathDirectory
	dhcpCommand := fmt.Sprintf("sleep 5 && dhcrelay --no-pid -d %s -i br-dpu -i br-ex -a -m forward", option.DHCP)
	parprouterdCommand := fmt.Sprintf("/parprouted -d --cidr %s --refresh %d br-ex br-dpu", hostCIDR.String(), ParprouterdRefreshInterval)
	owner := metav1.NewControllerRef(dpu,
		provisioningdpfv1alpha1.GroupVersion.WithKind("Dpu"))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            hostnetworkPodName,
			Namespace:       dpu.Namespace,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			InitContainers: []corev1.Container{
				{
					Name:            "hostnetwork",
					Image:           option.HostnetworkImageWithTag,
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "dev",
							MountPath: "/dev",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &[]bool{true}[0],
					},
					Env: []corev1.EnvVar{
						{
							Name:  "device_pci_address",
							Value: pci_address,
						},
						{
							Name:  "num_of_vfs",
							Value: num_of_vfs,
						},
					},

					Command: []string{"/bin/bash", "-c", "--"},
					Args: []string{
						"hostnetwork.sh",
					},
				},
			},
			// we need parprouted to start before dhcrelay
			// TODO: Perhaps we should merge dhcrelay and parprouted into one image
			Containers: []corev1.Container{
				{
					Name:            "parprouterd",
					Image:           option.PrarprouterdImageWithTag,
					ImagePullPolicy: corev1.PullAlways,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &[]bool{true}[0],
					},
					Command: []string{"/bin/bash", "-c", "--"},
					Args: []string{
						parprouterdCommand,
					},
				},
				{
					Name:            "dhcrelay",
					Image:           option.DHCRelayImageWithTag,
					ImagePullPolicy: corev1.PullAlways,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &[]bool{true}[0],
					},
					Command: []string{"/bin/sh", "-c", "--"},
					Args: []string{
						dhcpCommand,
					},
				},
			},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: option.ImagePullSecret}},
			Volumes: []corev1.Volume{
				{
					Name: "dev",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/dev",
							Type: &hostPathType,
						},
					},
				},
			},
		},
	}
	pod.Spec.Affinity = cutil.ReplaceDaemonSetPodNodeNameNodeAffinity(
		pod.Spec.Affinity, dpu.Spec.NodeName)
	pod.Spec.Tolerations = cutil.GeneratePodToleration(*dpu.Spec.NodeEffect)
	err := client.Create(ctx, pod)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create %s hostnetwork pod", hostnetworkPodName))
		return err
	}
	logger.V(3).Info(fmt.Sprintf("%s Hostnetwork pod created", hostnetworkPodName))
	return nil
}
