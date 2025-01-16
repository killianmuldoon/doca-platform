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

package webhooks

import (
	"context"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("DPUNode", func() {

	var getObjKey = func(obj *provisioningv1.DPUNode) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.DPUNode {
		return &provisioningv1.DPUNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec:   provisioningv1.DPUNodeSpec{NodeRebootMethod: "external"},
			Status: provisioningv1.DPUNodeStatus{},
		}
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("create and get object", func() {
			obj := createObj("obj-1")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
		})

		It("delete object", func() {
			obj := createObj("obj-2")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, getObjKey(obj), obj)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("update object", func() {
			obj := createObj("obj-3")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
		})

		It("create from yaml", func() {
			dpudeviceYml := []byte(`
            apiVersion: provisioning.dpu.nvidia.com/v1alpha1
            kind: DPUDevice
            metadata:
              name: dpu-device-1
              namespace: default
            spec:
              bmcIp: 3.3.3.3
              pciAddress: 0000-04-00
              psid: MT_0000000034
              opn: 900-9D3B4-00SV-EA0
            `)
			dpudeviceObj := &provisioningv1.DPUDevice{}
			err := yaml.UnmarshalStrict(dpudeviceYml, dpudeviceObj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, dpudeviceObj)
			Expect(err).NotTo(HaveOccurred())

			dpunodeYml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUNode
metadata:
  name: obj-4
  namespace: default
spec:
  kubeNodeRef: node-1
  nodeRebootMethod: external
  nodeDMSAddress: 
    ip: 4.4.4.4
    port: 50
  dpus:
    dpu-device-1: true
`)
			obj := &provisioningv1.DPUNode{}
			err = yaml.UnmarshalStrict(dpunodeYml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create from yaml minimal", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUNode
metadata:
  name: obj-5
  namespace: default
spec:
  nodeRebootMethod: external
`)
			obj := &provisioningv1.DPUNode{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
		Context("create from yaml with invalid NodeRebootMethod", func() {
			It("should not create a DPUNode with an invalid NodeRebootMethod value", func() {
				dpunodeYml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUNode
metadata:
  name: obj-6
  namespace: default
spec:
  kubeNodeRef: node-1
  nodeRebootMethod: invalid
  nodeDMSAddress:
    ip: 4.4.4.4
    port: 50
  dpus:
    dpu-device-1: true
`)
				obj := &provisioningv1.DPUNode{}
				err := yaml.UnmarshalStrict(dpunodeYml, obj)
				Expect(err).To(Succeed())
				err = k8sClient.Create(ctx, obj)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("create from yaml with invalid NodeDMSAddress", func() {
			It("should not create a DPUNode with an invalid NodeDMSAddress value", func() {
				dpunodeYml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUNode
metadata:
  name: obj-7
  namespace: default
spec:
  kubeNodeRef: node-1
  nodeRebootMethod: external
  nodeDMSAddress:
    ip: invalid-ip
    port: 50
  dpus:
    dpu-device-1: true
`)
				obj := &provisioningv1.DPUNode{}
				err := yaml.UnmarshalStrict(dpunodeYml, obj)
				Expect(err).To(Succeed())
				err = k8sClient.Create(ctx, obj)
				Expect(err).To(HaveOccurred())
			})
		})

		It("update object - check immutability of kubeNodeRef", func() {
			obj := createObj("obj-8")
			node2Ref := "node-2"
			obj.Spec.KubeNodeRef = &node2Ref
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			node3Ref := "node-3"
			obj.Spec.KubeNodeRef = &node3Ref
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("create from yaml without NodeRebootMethod - should fail", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUNode
metadata:
  name: obj-9
  namespace: default
`)
			obj := &provisioningv1.DPUNode{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("create from yaml", func() {
			dpudeviceYml := []byte(`
            apiVersion: provisioning.dpu.nvidia.com/v1alpha1
            kind: DPUDevice
            metadata:
              name: dpu-device-2
              namespace: default
            spec:
              bmcIp: 3.3.3.4
              pciAddress: 0000-02-00
              psid: MT_0000000034
              opn: 900-9D3B4-00SV-EA0
            `)
			dpudeviceObj := &provisioningv1.DPUDevice{}
			err := yaml.UnmarshalStrict(dpudeviceYml, dpudeviceObj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, dpudeviceObj)
			Expect(err).NotTo(HaveOccurred())

			dpunodeYml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUNode
metadata:
  name: obj-10
  namespace: default
spec:
  kubeNodeRef: node-1
  nodeRebootMethod: external
  nodeDMSAddress:
    ip: 4.4.4.4
    port: 50
  dpus:
    dpu-device-1: true
    dpu-device-2: true
`)
			obj := &provisioningv1.DPUNode{}
			err = yaml.UnmarshalStrict(dpunodeYml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})
		It("create DPUNode with missing NodeRebootMethod", func() {
			obj := createObj("obj-13")
			obj.Spec.NodeRebootMethod = ""
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("create DPUNode with invalid NodeRebootMethod", func() {
			obj := createObj("obj-14")
			obj.Spec.NodeRebootMethod = "invalid-method"
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("update DPUNode with new NodeRebootMethod", func() {
			obj := createObj("obj-15")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			obj.Spec.NodeRebootMethod = "custom_script"
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.NodeRebootMethod).To(Equal(obj.Spec.NodeRebootMethod))
		})
		It("create and update status with condition", func() {
			obj := createObj("obj-16")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			condition := metav1.Condition{
				Type:               string(provisioningv1.DPUNodeConditionReady),
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "Initialized",
				Message:            "DPUNode is ready",
			}
			obj.Status.Conditions = append(obj.Status.Conditions, condition)
			obj.Status.Phase = provisioningv1.DPUNodeReady
			obj.Status.DPUInstallInterface = string(provisioningv1.DPUNodeInstallInterfaceGNOI)
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Status.Conditions).To(HaveLen(1))
			Expect(objFetched.Status.Conditions[0].Type).To(Equal(string(provisioningv1.DPUNodeConditionReady)))
			Expect(objFetched.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		})
		It("create and update status with multiple conditions", func() {
			obj := createObj("obj-17")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			condition1 := metav1.Condition{
				Type:               string(provisioningv1.DPUNodeConditionReady),
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "Initialized",
				Message:            "DPUNode is ready",
			}
			condition2 := metav1.Condition{
				Type:               string(provisioningv1.DPUNodeConditionNotReady),
				Status:             metav1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "RebootNotStarted",
				Message:            "DPUNode reboot has not started",
			}
			obj.Status.Conditions = append(obj.Status.Conditions, condition1, condition2)
			obj.Status.Phase = provisioningv1.DPUNodeRebootInProgress
			obj.Status.DPUInstallInterface = string(provisioningv1.DPUNodeInstallInterfaceGNOI)
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Status.Conditions).To(HaveLen(2))
			Expect(objFetched.Status.Conditions[0].Type).To(Equal(string(provisioningv1.DPUNodeConditionReady)))
			Expect(objFetched.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(objFetched.Status.Conditions[1].Type).To(Equal(string(provisioningv1.DPUNodeConditionNotReady)))
			Expect(objFetched.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
		})
		It("update DPUNode status to Ready", func() {
			obj := createObj("obj-18")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			obj.Status.Phase = provisioningv1.DPUNodeReady
			obj.Status.DPUInstallInterface = string(provisioningv1.DPUNodeInstallInterfaceGNOI)
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Status.Phase).To(Equal(provisioningv1.DPUNodeReady))
		})
		It("update DPUNode status to InvalidDPUDetails", func() {
			obj := createObj("obj-19")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			obj.Status.Phase = provisioningv1.DPUNodeInvalidDPUDetails
			obj.Status.DPUInstallInterface = string(provisioningv1.DPUNodeInstallIntrefaceRedfish)
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Status.Phase).To(Equal(provisioningv1.DPUNodeInvalidDPUDetails))
		})
		It("update DPUNode status to invalid condition", func() {
			obj := createObj("obj-20")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			obj.Status.Phase = "Invalid Phase"
			obj.Status.DPUInstallInterface = string(provisioningv1.DPUNodeInstallIntrefaceRedfish)
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("create DPUNode with DPUInstallInterface", func() {
			obj := createObj("obj-21")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			obj.Status.DPUInstallInterface = string(provisioningv1.DPUNodeInstallInterfaceGNOI)
			obj.Status.Phase = provisioningv1.DPUNodeInvalidDPUDetails
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Status.DPUInstallInterface).To(Equal(string(provisioningv1.DPUNodeInstallInterfaceGNOI)))
		})

		It("update DPUNode DPUInstallInterface", func() {
			obj := createObj("obj-22")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			obj.Status.DPUInstallInterface = string(provisioningv1.DPUNodeInstallIntrefaceRedfish)
			obj.Status.Phase = provisioningv1.DPUNodeReady
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			objFetched := &provisioningv1.DPUNode{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Status.DPUInstallInterface).To(Equal(string(provisioningv1.DPUNodeInstallIntrefaceRedfish)))
		})

		It("update DPUNode DPUInstallInterface to invalid value", func() {
			obj := createObj("obj-23")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			obj.Status.DPUInstallInterface = "InvalidInterface"
			obj.Status.Phase = provisioningv1.DPUNodeReady
			err = k8sClient.Status().Update(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
	})
})
