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

package provisioning_controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"time"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These tests are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.
var _ = Describe("Bfb", func() {
	const (
		DefaultNS            = "dpf-provisioning-test"
		BFBPathFileSize512KB = "/BlueField/BFBs/bf-bundle-dummy-512KB.bfb"
		BFBPathFileSize132MB = "/BlueField/BFBs/bf-bundle-dummy-132MB.bfb"
		BFBPathFileNotFound  = "/BlueField/BFBs/bf-bundle-dummy-notfound.bfb"
		DefaultBFBFileName   = "dummy.bfb"
	)

	var (
		testNS        *corev1.Namespace
		server        *httptest.Server
		symlink       string
		symlinkTarget string
	)

	var getObjKey = func(obj *provisioningv1.Bfb) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.Bfb {
		return &provisioningv1.Bfb{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec:   provisioningv1.BfbSpec{},
			Status: provisioningv1.BfbStatus{},
		}
	}

	BeforeEach(func() {
		By("creating the namespaces")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: DefaultNS}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())

		By("creating location for bfb files")
		symlink = string(os.PathSeparator) + cutil.BFBBaseDir
		_, err := os.Stat(symlink)
		if err == nil {
			Skip("Setup is not suitable. Remove " + symlink + " that is used by test and rerun.")
		}

		symlinkTarget, err = os.MkdirTemp(os.TempDir(), "dpf-bfb-*")
		Expect(err).ToNot(HaveOccurred())

		err = os.Symlink(symlinkTarget, symlink)
		if err != nil {
			if os.IsPermission(err) {
				err := exec.Command("sh", "-c", "sudo ln -s "+symlinkTarget+" "+symlink).Run()
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		}
		symlink = string(os.PathSeparator) + cutil.BFBBaseDir
		_, err = os.Stat(symlink)
		Expect(err).NotTo(HaveOccurred())

		By("creating server for bfb download")
		data512KB := make([]byte, 512*1024)
		data132MB := make([]byte, 132*1024*1024)

		mux := http.NewServeMux()
		handler512KB := func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal("GET"))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data512KB)
		}
		mux.HandleFunc(BFBPathFileSize512KB, handler512KB)
		handler132MB := func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal("GET"))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data132MB)
		}
		mux.HandleFunc(BFBPathFileSize132MB, handler132MB)
		server = httptest.NewUnstartedServer(mux)
		server.Start()
		Expect(server).ToNot(BeNil())
		By("server is listening:" + server.URL)
	})

	AfterEach(func() {
		By("deleting the namespace")
		Expect(k8sClient.Delete(ctx, testNS)).To(Succeed())

		By("cleanup location for bfb files")
		Expect(os.RemoveAll(symlinkTarget)).To(Succeed())
		err := os.Remove(symlink)
		if err != nil {
			Expect(exec.Command("sh", "-c", "sudo rm "+symlink).Run()).To(Succeed())
		}

		By("closing server")
		// Sleep is needed to overcome potential race with http handler initialization
		t := time.AfterFunc(2000*time.Millisecond, server.Close)
		defer t.Stop()
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("check status (Initializing) and destroy", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileSize512KB
			obj.Spec.FileName = DefaultBFBFileName
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BfbInitializing))

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.BFBFinalizer}))
			Expect(obj_fetched.Spec.FileName).To(BeEquivalentTo(DefaultBFBFileName))
		})

		It("check status (Downloading) and destroy", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileSize512KB
			obj.Spec.FileName = DefaultBFBFileName
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BfbInitializing))

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.BFBFinalizer}))
			Expect(obj_fetched.Spec.FileName).To(BeEquivalentTo(DefaultBFBFileName))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))
		})

		It("check (Downloading)->(Error) when URL is not valid (status 404)", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileNotFound
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BfbInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))

			By("expecting the Status (Error)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbError))
		})

		It("check status (Ready)", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileSize512KB
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BfbInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BfbReady))
			file, err := os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 512*1024))
		})

		It("check status (Ready) in case bfb file is cached manually", func() {
			BfbFileName := cutil.GenerateBFBFilePath(DefaultBFBFileName)

			By("caching bfb file before start")
			f, err := os.Create(BfbFileName)
			Expect(err).NotTo(HaveOccurred())
			file_size, err := f.Write(make([]byte, 512*1024))
			Expect(err).NotTo(HaveOccurred())
			Expect(file_size).Should(BeNumerically("==", 512*1024))
			Expect(f.Close()).To(Succeed())
			file, err := os.Stat(BfbFileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 512*1024))

			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileSize512KB
			obj.Spec.FileName = DefaultBFBFileName
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BfbReady))
			file, err = os.Stat(BfbFileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 512*1024))
		})

		It("cleanup cached file on obj deletion", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileSize132MB
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BfbInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BfbReady))
			file, err := os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 132*1024*1024))

			By("removing obj")
			Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
			Eventually(func() (done bool, err error) {
				if err := k8sClient.Get(ctx, getObjKey(obj), obj_fetched); err != nil {
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				}
				return false, nil
			}).WithTimeout(10 * time.Second).Should(BeTrue())
			_, err = os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
			Expect(err).To(HaveOccurred())
		})

		It("remove cached bfb file from Status (Ready)", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileSize512KB
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BfbInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BfbReady))
			file, err := os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 512*1024))

			By("removing cached bfb file")
			Expect(os.Remove(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))).NotTo(HaveOccurred())

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BfbReady))
			file, err = os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 512*1024))
		})

		It("remove cached bfb file from Status (Ready) when server is down", func() {
			By("creating the obj")
			obj := createObj("obj-bfb")
			obj.Spec.URL = server.URL + BFBPathFileSize512KB
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Bfb{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.BfbInitializing))

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))

			By("expecting the Status (Ready)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).Should(Equal(provisioningv1.BfbReady))
			file, err := os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Size()).Should(BeNumerically("==", 512*1024))

			By("stopping server")
			server.Close()

			By("removing cached bfb file")
			Expect(os.Remove(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))).NotTo(HaveOccurred())

			By("expecting the Status (Downloading)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbDownloading))

			By("expecting the Status (Error)")
			Eventually(func(g Gomega) provisioningv1.BfbPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BfbError))
		})

		It("creating number of objs", func() {
			const num_objs = 128
			var objs []*provisioningv1.Bfb

			By("creating the objs")
			for i := 1; i < num_objs; i++ {
				index := fmt.Sprintf("%d", i)
				obj := createObj("obj-bfb" + index)
				obj.Spec.URL = server.URL + BFBPathFileSize512KB
				Expect(k8sClient.Create(ctx, obj)).To(Succeed())
				objs = append(objs, obj)
			}

			By("checking the objs have Status (Ready)")
			obj_fetched := &provisioningv1.Bfb{}
			for _, o := range objs {
				Eventually(func(g Gomega) provisioningv1.BfbPhase {
					g.Expect(k8sClient.Get(ctx, getObjKey(o), obj_fetched)).To(Succeed())
					return obj_fetched.Status.Phase
				}).WithTimeout(10 * time.Second).Should(Equal(provisioningv1.BfbReady))
			}

			By("removing all objs")
			for _, o := range objs {
				Expect(k8sClient.Delete(ctx, o)).To(Succeed())
				Eventually(func() (done bool, err error) {
					if err := k8sClient.Get(ctx, getObjKey(o), obj_fetched); err != nil {
						if apierrors.IsNotFound(err) {
							return true, nil
						}
						return false, err
					}
					return false, nil
				}).WithTimeout(10 * time.Second).Should(BeTrue())
				_, err := os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
				Expect(err).To(HaveOccurred())
			}
		})
	})
})
