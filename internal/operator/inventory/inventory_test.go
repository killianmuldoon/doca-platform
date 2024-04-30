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

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestManifests_Parse(t *testing.T) {
	g := NewGomegaWithT(t)

	tests := []struct {
		name      string
		inventory *Manifests
		wantErr   bool
	}{
		{
			name:      "parse objects from release directory",
			inventory: New(),
		},
		// DPUServiceControllerObjects
		{
			name:      "fail if DPUService controller data is nil",
			inventory: New().setDPUService(DPUServiceControllerObjects{data: nil}),
			wantErr:   true,
		},
		{
			name: "fail if an unexpected DPUService controller object is present",
			inventory: New().setDPUService(DPUServiceControllerObjects{
				data: addUnexpectedKindToObjects(g, dpuServiceData),
			}),
			wantErr: true,
		},
		{
			name: "fail if any DPUService controller object is missing",
			inventory: New().setDPUService(DPUServiceControllerObjects{
				data: removeKindFromObjects(g, "Deployment", dpuServiceData),
			}),
			wantErr: true,
		},
		// ServiceFunctionChainSetObjects
		{
			name: "fail if ServiceFunctionChainSet data is nil",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: "serviceFunctionChainSet",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if ServiceFunctionChainSet data has an unexpected object",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: "serviceFunctionChainSet",
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if ServiceFunctionChainSet is missing the DPUService",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: "serviceFunctionChainSet",
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// multus
		{
			name: "fail if Multus data is nil",
			inventory: New().setMultus(fromDPUService{
				name: "multus",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if Multus data has an unexpected object",
			inventory: New().setMultus(fromDPUService{
				name: "multus",
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if Multus is missing the DPUService",
			inventory: New().setMultus(fromDPUService{
				name: "multus",
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// sriovDevicePlugin
		{
			name: "fail if sriovDevicePlugin data is nil",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: "sriovDevicePlugin",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if SRIOVDevicePlugin data has an unexpected object",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: "sriovDevicePlugin",
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if SRIOVDevicePlugin is missing the DPUService",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: "sriovDevicePlugin",
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// flannel
		{
			name: "fail if flannel data is nil",
			inventory: New().setFlannel(fromDPUService{
				name: "flannel",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if flannel data has an unexpected object",
			inventory: New().setFlannel(fromDPUService{
				name: "flannel",
				data: addUnexpectedKindToObjects(g, flannelData),
			}),
			wantErr: true,
		},
		{
			name: "fail if flannel is missing the DPUService",
			inventory: New().setFlannel(fromDPUService{
				name: "flannel",
				data: removeKindFromObjects(g, "DPUService", flannelData),
			}),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.inventory.Parse()
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			for _, obj := range tt.inventory.Objects() {
				g.Expect(obj).ToNot(BeNil())
			}
		})
	}
}

func TestProvCtrlObjects_Parse(t *testing.T) {
	g := NewGomegaWithT(t)
	originalObjects, err := utils.BytesToUnstructured(provisioningCtrlData)
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
							ClaimName: BFBVolumeName,
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
			p := NewProvisionCtrlObjects()
			p.data = tc.data
			if tc.expectErr {
				NewGomegaWithT(t).Expect(p.Parse()).To(HaveOccurred())
			} else {
				NewGomegaWithT(t).Expect(p.Parse()).NotTo(HaveOccurred())
			}
		})
	}
}

func TestProvCtrlObjects_GenerateManifests(t *testing.T) {
	originalObjs, err := utils.BytesToUnstructured(provisioningCtrlData)
	NewGomegaWithT(t).Expect(err).NotTo(HaveOccurred())
	provCtrl := NewProvisionCtrlObjects()
	NewGomegaWithT(t).Expect(provCtrl.Parse()).NotTo(HaveOccurred())

	t.Run("fail if empty pvc", func(t *testing.T) {
		cfg := &operatorv1.DPFOperatorConfig{
			Spec: operatorv1.DPFOperatorConfigSpec{
				ProvisioningConfiguration: operatorv1.ProvisioningConfiguration{
					BFBPVCName:      " ",
					ImagePullSecret: "secret",
				},
			},
		}
		_, err := provCtrl.GenerateManifests(cfg)
		NewGomegaWithT(t).Expect(err).To(HaveOccurred())
	})

	t.Run("fail if empty imagePullSecret", func(t *testing.T) {
		cfg := &operatorv1.DPFOperatorConfig{
			Spec: operatorv1.DPFOperatorConfigSpec{
				ProvisioningConfiguration: operatorv1.ProvisioningConfiguration{
					BFBPVCName:      "pvc",
					ImagePullSecret: " ",
				},
			},
		}
		_, err := provCtrl.GenerateManifests(cfg)
		NewGomegaWithT(t).Expect(err).To(HaveOccurred())
	})

	// This test is customized for the current Provisioning manifest, internal/operator/inventory/manifests/provisioningctrl.yaml.
	// These tests should be reviewed every time the manifest is updated
	t.Run("test field modification", func(t *testing.T) {
		g := NewGomegaWithT(t)
		expectedPVC := "foo-test-pvc"
		expectedImagePullSecret := "foo-test-image-pull-secret"
		cfg := &operatorv1.DPFOperatorConfig{
			Spec: operatorv1.DPFOperatorConfigSpec{
				ProvisioningConfiguration: operatorv1.ProvisioningConfiguration{
					BFBPVCName:      expectedPVC,
					ImagePullSecret: expectedImagePullSecret,
				},
			},
		}
		generatedObjs, err := provCtrl.GenerateManifests(cfg)
		g.Expect(err).NotTo(HaveOccurred())

		// * ensure nothing except the manager Deployment is changed
		var deploy *appsv1.Deployment
		g.Expect(originalObjs).To(HaveLen(len(generatedObjs)))
		for i, originalObj := range originalObjs {
			if originalObj.GetKind() == utils.Deployment {
				// at the time of writing, we have only one Deployment in the manifest
				g.Expect(deploy).To(BeNil())
				deploy = &appsv1.Deployment{}
				g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(generatedObjs[i].UnstructuredContent(), deploy)).NotTo(HaveOccurred())
				continue
			}
			g.Expect(equality.Semantic.DeepEqual(originalObj, generatedObjs[i])).To(BeTrue())
		}
		g.Expect(deploy).NotTo(BeNil())
		g.Expect(deploy.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
		g.Expect(deploy.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal(expectedImagePullSecret))
		// * check bfb pvc
		g.Expect(deploy.Spec.Template.Spec.Volumes).To(HaveLen(2))
		g.Expect(deploy.Spec.Template.Spec.Volumes[1].PersistentVolumeClaim).NotTo(BeNil())
		g.Expect(deploy.Spec.Template.Spec.Volumes[1].PersistentVolumeClaim.ClaimName).To(Equal(expectedPVC))
		// * check args of the manager container
		var container *corev1.Container
		for _, c := range deploy.Spec.Template.Spec.Containers {
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
			"--dms-image-name=gitlab-master.nvidia.com:5005/doca-platform-foundation/dpf-provisioning-controller/dms-server",
			"--dms-image-tag=v2",
			fmt.Sprintf("--dms-image-pull-secret=%s", expectedImagePullSecret),
			fmt.Sprintf("--bfb-pvc=%s", expectedPVC),
		}
		g.Expect(deploy.Spec.Template.Spec.Containers).To(HaveLen(2))
		for i, ea := range expectedArgs {
			g.Expect(container.Args[i]).To(Equal(ea))
		}
	})
}

func removeKindFromObjects(g Gomega, kindToRemove string, data []byte) []byte {
	// DPUService objects which is missing one of the expected Kinds.
	objsWithMissingKind, err := utils.BytesToUnstructured(data)
	g.Expect(err).NotTo(HaveOccurred())
	for i, obj := range objsWithMissingKind {
		if obj.GetObjectKind().GroupVersionKind().Kind == kindToRemove {
			objsWithMissingKind = append(objsWithMissingKind[:i], objsWithMissingKind[i+1:]...)
		}
	}
	dataWithMissingKind, err := utils.UnstructuredToBytes(objsWithMissingKind)
	g.Expect(err).NotTo(HaveOccurred())
	return dataWithMissingKind
}

func addUnexpectedKindToObjects(g Gomega, data []byte) []byte {
	objsWithUnexpectedKind, err := utils.BytesToUnstructured(data)
	g.Expect(err).NotTo(HaveOccurred())
	objsWithUnexpectedKind[0].SetKind("FakeKind")
	dataWithUnexpectedKind, err := utils.UnstructuredToBytes(objsWithUnexpectedKind)
	g.Expect(err).NotTo(HaveOccurred())
	return dataWithUnexpectedKind
}
