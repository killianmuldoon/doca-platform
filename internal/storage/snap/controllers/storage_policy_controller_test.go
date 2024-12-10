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

package controllers

import (
	"context"
	"time"

	snapstoragev1 "github.com/nvidia/doca-platform/api/storage/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Storage_Policy", func() {
	const (
		DefaultNS   = "nvidia-storage"
		Provisioner = "csi.snap.nvidia.com"
	)

	var (
		testNS *corev1.Namespace
	)

	var getObjKey = func(obj *snapstoragev1.StoragePolicy) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	ReclaimPolicy := corev1.PersistentVolumeReclaimRetain
	VolumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
	var createStorageClassObj = func(name string) *storagev1.StorageClass {
		return &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Provisioner:          Provisioner,
			ReclaimPolicy:        &ReclaimPolicy,
			AllowVolumeExpansion: ptr.To(false),
			VolumeBindingMode:    &VolumeBindingMode,
		}
	}

	var createStorageVendorObj = func(name string, storageClassName string, pluginName string) *snapstoragev1.StorageVendor {
		return &snapstoragev1.StorageVendor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec: snapstoragev1.StorageVendorSpec{
				StorageClassName: storageClassName,
				PluginName:       pluginName,
			},
		}
	}

	var createStoragePolicyObj = func(name string, storageVendorList []string) *snapstoragev1.StoragePolicy {
		return &snapstoragev1.StoragePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec: snapstoragev1.StoragePolicySpec{
				StorageVendors:      storageVendorList,
				StorageSelectionAlg: snapstoragev1.Random,
			},
		}
	}

	BeforeEach(func() {
		By("creating the namespaces")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: DefaultNS}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())
	})

	AfterEach(func() {
		By("deleting the namespace")
		Expect(k8sClient.Delete(ctx, testNS)).To(Succeed())
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("check valid status and destroy", func() {
			By("creating the obj")
			storageClass := createStorageClassObj("low-latency")
			Expect(k8sClient.Create(ctx, storageClass)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageClass)

			storageVendor := createStorageVendorObj("excelero", "low-latency", "excelero-plugin")
			Expect(k8sClient.Create(ctx, storageVendor)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageVendor)

			storagePolicy := createStoragePolicyObj("excelero", []string{"excelero"})
			Expect(k8sClient.Create(ctx, storagePolicy)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storagePolicy)

			objFetched := &snapstoragev1.StoragePolicy{}

			By("expecting the Status (Valid)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateValid))
		})

		It("check invalid status (missing storage class) and destroy", func() {
			By("creating the obj")

			storageVendor := createStorageVendorObj("excelero", "low-latency", "excelero-plugin")
			Expect(k8sClient.Create(ctx, storageVendor)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageVendor)

			storagePolicy := createStoragePolicyObj("excelero", []string{"excelero"})
			Expect(k8sClient.Create(ctx, storagePolicy)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storagePolicy)

			objFetched := &snapstoragev1.StoragePolicy{}

			By("expecting the Status (Invalid)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateInvalid))
		})

		It("check invalid status (missing storage vendor) and destroy", func() {
			By("creating the obj")
			storageClass := createStorageClassObj("low-latency")
			Expect(k8sClient.Create(ctx, storageClass)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageClass)

			storagePolicy := createStoragePolicyObj("excelero", []string{"excelero"})
			Expect(k8sClient.Create(ctx, storagePolicy)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storagePolicy)

			objFetched := &snapstoragev1.StoragePolicy{}

			By("expecting the Status (Invalid)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateInvalid))
		})

		It("check status change from valid to invaild(delete storage class)", func() {
			By("creating the obj")
			storageClass := createStorageClassObj("low-latency")
			Expect(k8sClient.Create(ctx, storageClass)).To(Succeed())

			storageVendor := createStorageVendorObj("excelero", "low-latency", "excelero-plugin")
			Expect(k8sClient.Create(ctx, storageVendor)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageVendor)

			storagePolicy := createStoragePolicyObj("excelero", []string{"excelero"})
			Expect(k8sClient.Create(ctx, storagePolicy)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storagePolicy)

			objFetched := &snapstoragev1.StoragePolicy{}

			By("expecting the Status (Vaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateValid))

			By("deleting storage class")
			Expect(k8sClient.Delete(ctx, storageClass)).To(Succeed())

			By("expecting the Status (Invaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateInvalid))
		})

		It("check status change from valid to invaild(delete storage vendor)", func() {
			By("creating the obj")
			storageClass := createStorageClassObj("low-latency")
			Expect(k8sClient.Create(ctx, storageClass)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageClass)

			storageVendor := createStorageVendorObj("excelero", "low-latency", "excelero-plugin")
			Expect(k8sClient.Create(ctx, storageVendor)).To(Succeed())

			storagePolicy := createStoragePolicyObj("excelero", []string{"excelero"})
			Expect(k8sClient.Create(ctx, storagePolicy)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storagePolicy)

			objFetched := &snapstoragev1.StoragePolicy{}

			By("expecting the Status (Vaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateValid))

			By("deleting storage vendor")
			Expect(k8sClient.Delete(ctx, storageVendor)).To(Succeed())

			By("expecting the Status (Invaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateInvalid))
		})

		It("check status change from invaild to vaild(create storage class)", func() {
			By("creating the obj")
			storageVendor := createStorageVendorObj("excelero", "low-latency", "excelero-plugin")
			Expect(k8sClient.Create(ctx, storageVendor)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageVendor)

			storagePolicy := createStoragePolicyObj("excelero", []string{"excelero"})
			Expect(k8sClient.Create(ctx, storagePolicy)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storagePolicy)

			objFetched := &snapstoragev1.StoragePolicy{}

			By("expecting the Status (Invaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateInvalid))

			By("creating storage class")
			storageClass := createStorageClassObj("low-latency")
			Expect(k8sClient.Create(ctx, storageClass)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageClass)

			By("expecting the Status (Vaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateValid))
		})

		It("check status change from invaild to vaild(create storage vendor)", func() {
			By("creating the obj")
			storageClass := createStorageClassObj("low-latency")
			Expect(k8sClient.Create(ctx, storageClass)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageClass)

			storagePolicy := createStoragePolicyObj("excelero", []string{"excelero"})
			Expect(k8sClient.Create(ctx, storagePolicy)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storagePolicy)

			objFetched := &snapstoragev1.StoragePolicy{}

			By("expecting the Status (Invaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateInvalid))

			By("creating storage vendor")
			storageVendor := createStorageVendorObj("excelero", "low-latency", "excelero-plugin")
			Expect(k8sClient.Create(ctx, storageVendor)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, storageVendor)

			By("expecting the Status (Vaild)")
			Eventually(func(g Gomega) snapstoragev1.StorageVendorState {
				g.Expect(k8sClient.Get(ctx, getObjKey(storagePolicy), objFetched)).To(Succeed())
				return objFetched.Status.State
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(snapstoragev1.StorageVendorStateValid))
		})
	})
})
