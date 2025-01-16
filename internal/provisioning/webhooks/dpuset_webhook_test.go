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

package webhooks

import (
	"context"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
)

var _ = Describe("DPUSet", func() {

	var getObjKey = func(obj *provisioningv1.DPUSet) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.DPUSet {
		return &provisioningv1.DPUSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec:   provisioningv1.DPUSetSpec{},
			Status: provisioningv1.DPUSetStatus{},
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

			objFetched := &provisioningv1.DPUSet{}
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

			objFetched := &provisioningv1.DPUSet{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
		})

		It("spec.dpuTemplate.spec.cluster.nodeSelector is mutable", func() {
			refValue := map[string]string{"k1": "v1"}
			newValue := map[string]string{"k1": "v11", "k2": "v2"}

			obj := createObj("obj-4")
			obj.Spec.DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
				NodeLabels: refValue,
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.DPUTemplate.Spec.Cluster.NodeLabels = newValue
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUSet{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.DPUTemplate.Spec.Cluster.NodeLabels).To(Equal(newValue))
		})

		It("nodeEffect is updated", func() {
			refValue := map[string]string{"k1": "v1"}
			newValue := map[string]string{"k1": "v11", "k2": "v2"}

			obj := createObj("node-effect-object")
			obj.Spec.DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
				NodeLabels: refValue,
			}
			obj.Spec.DPUTemplate.Spec.NodeEffect = &provisioningv1.NodeEffect{
				Taint: &corev1.Taint{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			}

			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.DPUTemplate.Spec.Cluster.NodeLabels = newValue
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUSet{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.DPUTemplate.Spec.Cluster.NodeLabels).To(Equal(newValue))
		})

		It("only one field may be set in spec.nodeEffect", func() {
			obj := createObj("checking-node-effect")
			// Error when creating a DPUSet with a nodeEffect setting taint and customLabel.
			obj.Spec.DPUTemplate.Spec.NodeEffect = &provisioningv1.NodeEffect{
				Taint: &corev1.Taint{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
				CustomLabel: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Create(ctx, obj)).NotTo(Succeed())

			// Error when creating a DPUSet with a nodeEffect setting taint and drain.
			obj.Spec.DPUTemplate.Spec.NodeEffect = &provisioningv1.NodeEffect{
				Taint: &corev1.Taint{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
				Drain: &provisioningv1.Drain{
					AutomaticNodeReboot: true,
				},
			}
			Expect(k8sClient.Create(ctx, obj)).NotTo(Succeed())

			// Error when creating a DPUSet with a nodeeffect setting Drain and NoEffect
			obj.Spec.DPUTemplate.Spec.NodeEffect = &provisioningv1.NodeEffect{
				Drain: &provisioningv1.Drain{},
				CustomLabel: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Create(ctx, obj)).NotTo(Succeed())

			// Error when creating a DPUSet with a nodeeffect setting Drain and NoEffect
			obj.Spec.DPUTemplate.Spec.NodeEffect = &provisioningv1.NodeEffect{
				NoEffect: ptr.To(true),
				CustomLabel: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Create(ctx, obj)).NotTo(Succeed())
			// Accepted when creating a DPUSet with an empty nodeEffect only
			obj.Spec.DPUTemplate.Spec.NodeEffect = nil
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			Expect(k8sClient.Get(ctx, getObjKey(obj), obj)).To(Succeed())
			Expect(obj.Spec.DPUTemplate.Spec.NodeEffect).To(Equal(&provisioningv1.NodeEffect{Drain: &provisioningv1.Drain{AutomaticNodeReboot: true}}))
		})

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUSet
metadata:
  name: obj-5
  namespace: default
spec:
  nodeSelector:
  strategy:
    rollingUpdate:
      maxUnavailable: 10%
    type: RollingUpdate
  dpuTemplate:
    annotations:
      nvidia.com/dpuOperator-override-powercycle-command: "cycle"
    spec:
      dpuFlavor: "hbn"
      bfb:
        name: "doca-24.04"
      nodeEffect:
        taint:
          key: "dpu"
          value: "provisioning"
          effect: NoSchedule
      Cluster:
        nodeLabels:
          "dpf.node.dpu/role": "worker"
`)
			obj := &provisioningv1.DPUSet{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create from yaml minimal", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUSet
metadata:
  name: obj-6
  namespace: default
`)
			obj := &provisioningv1.DPUSet{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
