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

package inventory

import (
	"fmt"
	"testing"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDPFProvisioningControllerObjects_Parse(t *testing.T) {
	g := NewGomegaWithT(t)
	originalObjects, err := utils.BytesToUnstructured(dpfProvisioningControllerData)
	g.Expect(err).NotTo(HaveOccurred())

	iterate := func(op func(*unstructured.Unstructured) bool) []byte {
		ret := []*unstructured.Unstructured{}
		for _, obj := range originalObjects {
			cpy := obj.DeepCopy()
			include := op(cpy)
			if include {
				ret = append(ret, cpy)
			}
		}
		b, err := utils.UnstructuredToBytes(ret)
		g.Expect(err).NotTo(HaveOccurred())
		return b
	}

	correct := iterate(func(u *unstructured.Unstructured) bool { return true })
	missingDeployment := iterate(func(u *unstructured.Unstructured) bool {
		return u.GetKind() != utils.Deployment
	})
	wrongName := iterate(func(u *unstructured.Unstructured) bool {
		if u.GetKind() == utils.Deployment {
			u.SetName("wrong-name")
		}
		return true
	})
	volumeMissing := iterate(func(u *unstructured.Unstructured) bool {
		if u.GetKind() == utils.Deployment {
			deploy := &appsv1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), deploy)
			g.Expect(err).NotTo(HaveOccurred())
			deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "some-other-volume",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			}
			un, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
			g.Expect(err).NotTo(HaveOccurred())
			*u = unstructured.Unstructured{Object: un}
		}
		return true
	})

	volumeWrongName := iterate(func(u *unstructured.Unstructured) bool {
		if u.GetKind() == utils.Deployment {
			deploy := &appsv1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), deploy)
			g.Expect(err).NotTo(HaveOccurred())
			deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "some-other-volume",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
				{
					Name: "wrong-name",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: bfbVolumeName,
						},
					},
				},
			}
			un, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
			g.Expect(err).NotTo(HaveOccurred())
			*u = unstructured.Unstructured{Object: un}
		}
		return true
	})

	tests := []struct {
		name      string
		data      []byte
		expectErr bool
	}{
		{
			name:      "should succeed",
			data:      correct,
			expectErr: false,
		},
		{
			name:      "fail if no Deployment in manifests",
			data:      missingDeployment,
			expectErr: true,
		},
		{
			name:      "fail if wrong Deployment name in manifests",
			data:      wrongName,
			expectErr: true,
		},
		{
			name:      "fail if PVC volume is missing",
			data:      volumeMissing,
			expectErr: true,
		},
		{
			name:      "fail if PVC volume has different name",
			data:      volumeWrongName,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := dpfProvisioningControllerObjects{
				data: dpfProvisioningControllerData,
			}
			p.data = tc.data
			if tc.expectErr {
				NewGomegaWithT(t).Expect(p.Parse()).To(HaveOccurred())
			} else {
				NewGomegaWithT(t).Expect(p.Parse()).NotTo(HaveOccurred())
			}
		})
	}
}

func TestProvisioningControllerObjects_GenerateManifests(t *testing.T) {
	originalObjs, err := utils.BytesToUnstructured(dpfProvisioningControllerData)
	NewGomegaWithT(t).Expect(err).NotTo(HaveOccurred())
	provCtrl := dpfProvisioningControllerObjects{
		data: dpfProvisioningControllerData,
	}
	NewGomegaWithT(t).Expect(provCtrl.Parse()).NotTo(HaveOccurred())

	t.Run("no objects if disable is set", func(t *testing.T) {
		vars := Variables{
			DisableSystemComponents: map[string]bool{
				provCtrl.Name(): true,
			},
		}
		objs, err := provCtrl.GenerateManifests(vars)
		if err != nil {
			t.Fatalf("failed to generate manifests: %v", err)
		}
		if len(objs) != 0 {
			t.Fatalf("manifests should not be generated when disabled: %v", objs)
		}
	})

	t.Run("fail if empty pvc", func(t *testing.T) {
		vars := Variables{
			DPFProvisioningController: DPFProvisioningVariables{
				BFBPersistentVolumeClaimName: " ",
				ImagePullSecret:              "secret",
			},
		}
		_, err := provCtrl.GenerateManifests(vars)
		NewGomegaWithT(t).Expect(err).To(HaveOccurred())
	})

	t.Run("fail if empty imagePullSecret", func(t *testing.T) {
		vars := Variables{
			DPFProvisioningController: DPFProvisioningVariables{
				BFBPersistentVolumeClaimName: "pvc",
				ImagePullSecret:              " ",
			},
		}
		_, err := provCtrl.GenerateManifests(vars)
		NewGomegaWithT(t).Expect(err).To(HaveOccurred())
	})

	// This test is customized for the current Provisioning manifest, internal/operator/inventory/manifests/provisioningctrl.yaml.
	// These tests should be reviewed every time the manifest is updated
	t.Run("test field modification", func(t *testing.T) {
		g := NewGomegaWithT(t)
		expectedPVC := "foo-test-pvc"
		expectedImagePullSecret := "foo-test-image-pull-secret"
		vars := Variables{
			Namespace: "foo",
			DPFProvisioningController: DPFProvisioningVariables{
				BFBPersistentVolumeClaimName: expectedPVC,
				ImagePullSecret:              expectedImagePullSecret,
			},
		}
		generatedObjs, err := provCtrl.GenerateManifests(vars)
		g.Expect(err).NotTo(HaveOccurred())

		// Expect the CRD and Namespace to have been removed. There are 3 CRDs and 1 Namespace in the manifest file.
		g.Expect(generatedObjs).To(HaveLen(len(originalObjs) - 4))

		for _, obj := range generatedObjs {
			g.Expect(obj.GetNamespace()).To(Equal("foo"))
		}
		var gotDeployment *appsv1.Deployment
		for i, obj := range generatedObjs {
			if obj.GetObjectKind().GroupVersionKind().Kind == utils.Deployment {
				deployment, ok := generatedObjs[i].(*appsv1.Deployment)
				g.Expect(ok).To(BeTrue())
				gotDeployment = deployment
				continue
			}
		}
		// * ensure that the expected modifications have been made to the deployment.
		g.Expect(gotDeployment).NotTo(BeNil())
		g.Expect(gotDeployment.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
		g.Expect(gotDeployment.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal(expectedImagePullSecret))
		// * check bfb pvc
		g.Expect(gotDeployment.Spec.Template.Spec.Volumes).To(HaveLen(2))
		g.Expect(gotDeployment.Spec.Template.Spec.Volumes[1].PersistentVolumeClaim).NotTo(BeNil())
		g.Expect(gotDeployment.Spec.Template.Spec.Volumes[1].PersistentVolumeClaim.ClaimName).To(Equal(expectedPVC))
		// * check args of the manager container
		var container *corev1.Container
		for _, c := range gotDeployment.Spec.Template.Spec.Containers {
			if c.Name == "manager" {
				container = c.DeepCopy()
				break
			}
		}
		g.Expect(container).NotTo(BeNil())
		expectedArgs := []string{
			"--health-probe-bind-address=:8081",
			"--metrics-bind-address=127.0.0.1:8080",
			"--leader-elect",
			"--zap-log-level=3",
			"--dms-image=gitlab-master.nvidia.com:5005/doca-platform-foundation/dpf-provisioning-controller/dms-server:v2.7",
			"--hostnetwork-image=gitlab-master.nvidia.com:5005/doca-platform-foundation/dpf-provisioning-controller/hostnetworksetup:v0.1",
			"--dhcrelay-image=gitlab-master.nvidia.com:5005/doca-platform-foundation/dpf-provisioning-controller/dhcrelay:v0.1",
			"--parprouterd-image=gitlab-master.nvidia.com:5005/doca-platform-foundation/dpf-provisioning-controller/parprouterd:v0.1",
			fmt.Sprintf("--image-pull-secret=%s", expectedImagePullSecret),
			fmt.Sprintf("--bfb-pvc=%s", expectedPVC),
			"--dhcp=10.211.0.124",
		}
		g.Expect(gotDeployment.Spec.Template.Spec.Containers).To(HaveLen(1))
		for i, ea := range expectedArgs {
			g.Expect(container.Args[i]).To(Equal(ea))
		}
	})

}
