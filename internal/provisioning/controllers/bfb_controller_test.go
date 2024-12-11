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

package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("BFB", func() {
	const (
		DefaultNS           = "dpf-provisioning-test"
		BFBURLFileSize512KB = "http://BlueField/BFBs/bf-bundle-dummy-512KB.bfb"
		BFBURLFileSize8KB   = "http://BlueField/BFBs/bf-bundle-dummy-8KB.bfb"
		DefaultBFBFileName  = "dummy.bfb"
	)

	var (
		testNS *corev1.Namespace
	)

	var getObjKey = func(obj *provisioningv1.BFB) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.BFB {
		return &provisioningv1.BFB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec:   provisioningv1.BFBSpec{},
			Status: provisioningv1.BFBStatus{},
		}
	}

	BeforeEach(func() {
		By("creating the namespaces")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: DefaultNS}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())

		By("creating location for bfb files")
		_, err := os.Stat(cutil.BFBBaseDir)
		if err == nil {
			AbortSuite("Setup is not suitable. Remove " + cutil.BFBBaseDir + " that is used by test and rerun.")
		}

		err = os.Mkdir(cutil.BFBBaseDir, 0755)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("deleting the namespace")
		Expect(k8sClient.Delete(ctx, testNS)).To(Succeed())

		By("cleanup location for bfb files")
		Expect(os.RemoveAll(cutil.BFBBaseDir)).To(Succeed())
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("check status (Initializing) and destroy", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = BFBURLFileSize512KB
			obj.Spec.FileName = DefaultBFBFileName
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &provisioningv1.BFB{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BFBInitializing))

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.BFBFinalizer}))
			Expect(objFetched.Spec.FileName).To(BeEquivalentTo(DefaultBFBFileName))
		})

		It("check status (Downloading) and destroy", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = BFBURLFileSize512KB
			obj.Spec.FileName = DefaultBFBFileName
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &provisioningv1.BFB{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BFBInitializing))

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.BFBFinalizer}))
			Expect(objFetched.Spec.FileName).To(BeEquivalentTo(DefaultBFBFileName))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BFBDownloading))
		})

		It("check status (Ready)", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = BFBURLFileSize512KB
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &provisioningv1.BFB{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BFBInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BFBDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BFBReady))
			_, err := os.Stat(cutil.GenerateBFBFilePath(objFetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())
		})

		It("check status (Ready) in case bfb file is cached manually", func() {
			BFBFileName := cutil.GenerateBFBFilePath(DefaultBFBFileName)

			By("caching bfb file before start")
			f, err := os.Create(BFBFileName)
			Expect(err).NotTo(HaveOccurred())
			fileSize, err := f.Write(make([]byte, 512*1024))
			Expect(err).NotTo(HaveOccurred())
			Expect(fileSize).Should(BeNumerically("==", 512*1024))
			Expect(f.Close()).To(Succeed())
			file, err := os.Stat(BFBFileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 512*1024))

			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = BFBURLFileSize512KB
			obj.Spec.FileName = DefaultBFBFileName
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &provisioningv1.BFB{}

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BFBReady))
			_, err = os.Stat(BFBFileName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("cleanup cached file on obj deletion", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = BFBURLFileSize8KB
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			objFetched := &provisioningv1.BFB{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BFBInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BFBDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BFBReady))
			_, err := os.Stat(cutil.GenerateBFBFilePath(objFetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())

			By("removing obj")
			Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
			Eventually(func() (done bool, err error) {
				if err := k8sClient.Get(ctx, getObjKey(obj), objFetched); err != nil {
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				}
				return false, nil
			}).WithTimeout(30 * time.Second).Should(BeTrue())
			_, err = os.Stat(cutil.GenerateBFBFilePath(objFetched.Spec.FileName))
			Expect(err).To(HaveOccurred())
		})

		It("remove cached bfb file from Status (Ready)", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = BFBURLFileSize512KB
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &provisioningv1.BFB{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BFBInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BFBDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BFBReady))
			_, err := os.Stat(cutil.GenerateBFBFilePath(objFetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())

			By("removing cached bfb file")
			Expect(os.Remove(cutil.GenerateBFBFilePath(objFetched.Spec.FileName))).NotTo(HaveOccurred())

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BFBDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BFBPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BFBReady))
		})

		It("creating number of objs", func() {
			const numObjs = 64
			var objs []*provisioningv1.BFB

			By("creating the objs")
			for i := 1; i < numObjs; i++ {
				index := fmt.Sprintf("%d", i)
				obj := createObj("obj-bfb" + index)
				obj.Spec.URL = BFBURLFileSize512KB
				Expect(k8sClient.Create(ctx, obj)).To(Succeed())
				objs = append(objs, obj)
			}

			By("checking the objs have Status (Ready)")
			objFetched := &provisioningv1.BFB{}
			for _, o := range objs {
				Eventually(func(g Gomega) provisioningv1.BFBPhase {
					g.Expect(k8sClient.Get(ctx, getObjKey(o), objFetched)).To(Succeed())
					return objFetched.Status.Phase
				}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BFBReady))
			}

			By("removing all objs")
			for _, o := range objs {
				Expect(k8sClient.Delete(ctx, o)).To(Succeed())
				Eventually(func() (done bool, err error) {
					if err := k8sClient.Get(ctx, getObjKey(o), objFetched); err != nil {
						if apierrors.IsNotFound(err) {
							return true, nil
						}
						return false, err
					}
					return false, nil
				}).WithTimeout(60 * time.Second).Should(BeTrue())
				_, err := os.Stat(cutil.GenerateBFBFilePath(objFetched.Spec.FileName))
				Expect(err).To(HaveOccurred())
			}
		})
	})
})
