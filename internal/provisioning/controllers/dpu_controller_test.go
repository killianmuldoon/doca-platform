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
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"time"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/bfcfg"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/dms"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils/informer"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These tests are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.
var _ = Describe("Dpu", func() {
	const (
		DefaultNS  = "dpf-provisioning-test"
		DefaultBFB = "dpf-provisioning-bfb-test"
	)

	var (
		testNS         *corev1.Namespace
		testDPUCluster *provisioningv1.DPUCluster
		testNode       *corev1.Node
		i              *informer.TestInformer
	)

	var getObjKey = func(obj *provisioningv1.Dpu) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.Dpu {
		return &provisioningv1.Dpu{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec:   provisioningv1.DpuSpec{},
			Status: provisioningv1.DpuStatus{},
		}
	}

	var createBFB = func(ctx context.Context, name string) (*provisioningv1.BFB, string) {
		const BFBPathFileSize = "/bf-bundle-dummy.bfb"
		var (
			server        *httptest.Server
			symlink       string
			symlinkTarget string
		)

		By("creating location for bfb files")
		symlink = string(os.PathSeparator) + cutil.BFBBaseDir
		_, err := os.Stat(symlink)
		if err == nil {
			AbortSuite("Setup is not suitable. Remove " + symlink + " that is used by test and rerun.")
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

		mux := http.NewServeMux()
		handler := func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal("GET"))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(make([]byte, 1024))
		}
		mux.HandleFunc(BFBPathFileSize, handler)
		server = httptest.NewUnstartedServer(mux)
		server.Start()
		Expect(server).ToNot(BeNil())
		By("server is listening:" + server.URL)

		By("creating the obj")
		obj := &provisioningv1.BFB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
		}
		obj.Spec.URL = server.URL + BFBPathFileSize
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())

		obj_fetched := &provisioningv1.BFB{}

		By("expecting the Status (Ready)")
		Eventually(func(g Gomega) provisioningv1.BFBPhase {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj_fetched)).To(Succeed())
			return obj_fetched.Status.Phase
		}).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(provisioningv1.BFBReady))
		_, err = os.Stat(cutil.GenerateBFBFilePath(obj_fetched.Spec.FileName))
		Expect(err).NotTo(HaveOccurred())

		By("closing server")
		// Sleep is needed to overcome potential race with http handler initialization
		t := time.AfterFunc(2000*time.Millisecond, server.Close)
		defer t.Stop()

		return obj, symlinkTarget
	}

	var destroyBFB = func(ctx context.Context, obj *provisioningv1.BFB, symlinkTarget string) {

		By("Cleaning the bfb")
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())

		By("cleanup location for bfb files")
		symlink := string(os.PathSeparator) + cutil.BFBBaseDir
		Expect(os.RemoveAll(symlinkTarget)).To(Succeed())
		err := os.Remove(symlink)
		if err != nil {
			Expect(exec.Command("sh", "-c", "sudo rm "+symlink).Run()).To(Succeed())
		}
	}

	var createDPUCluster = func(ctx context.Context, name string) *provisioningv1.DPUCluster {
		cluster := &provisioningv1.DPUCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec: provisioningv1.DPUClusterSpec{
				Type: string(provisioningv1.StaticCluster),
			},
			Status: provisioningv1.DPUClusterStatus{},
		}
		Expect(k8sClient.Create(ctx, cluster)).NotTo(HaveOccurred())
		return cluster
	}

	var createNode = func(ctx context.Context, name string, labels map[string]string) *corev1.Node {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
		Expect(k8sClient.Create(ctx, node)).NotTo(HaveOccurred())
		return node
	}

	BeforeEach(func() {
		By("creating the namespace")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: DefaultNS}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())

		By("creating the dpucluster")
		testDPUCluster = createDPUCluster(ctx, "dpucluster-default")

		By("creating the node")
		testNode = createNode(ctx, "node-default", make(map[string]string))

		By("Creating the informer infrastructure for Dpu")
		i = informer.NewInformer(cfg, provisioningv1.DpuGroupVersionKind, testNS.Name, "dpus")
		DeferCleanup(i.Cleanup)
		go i.Run()
	})

	AfterEach(func() {
		By("deleting the namespace")
		Expect(k8sClient.Delete(ctx, testNS)).To(Succeed())

		By("deleting the dpucluster")
		Expect(k8sClient.Delete(ctx, testDPUCluster)).To(Succeed())

		By("Cleaning the node")
		Expect(k8sClient.Delete(ctx, testNode)).To(Succeed())
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("check status (Initializing) and destroy", func() {
			By("creating the obj")
			obj := createObj("obj-dpu")
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Consistently(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))
		})

		It("check status (Initializing) and cluster NotReady", func() {
			By("creating the obj")
			obj := createObj("obj-dpu")
			obj.Spec.NodeName = testNode.Name
			obj.Spec.NodeEffect = &provisioningv1.NodeEffect{
				NoEffect: true,
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(30 * time.Second).WithPolling(10 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))

			By("expecting the Condition (DPUClusterNotReady)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Phase).Should(Equal(provisioningv1.DPUInitializing))
				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", "DPUClusterNotReady"),
				),
			))
			Expect(obj_fetched.Finalizers).Should(ConsistOf([]string{provisioningv1.DPUFinalizer}))
		})

		It("attaching to node w/o bfb object (Node Effect) is NoEffect", func() {
			By("creating the obj")
			obj := createObj("obj-dpu")
			obj.Spec.NodeName = testNode.Name
			obj.Spec.NodeEffect = &provisioningv1.NodeEffect{
				NoEffect: true,
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))

			Skip("TODO: DPUCluter should be in Ready state to proceed")

			By("expecting the Status (Node Effect)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Phase).Should(Equal(provisioningv1.DPUInitializing))
				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
			))
			Expect(obj_fetched.Finalizers).Should(ConsistOf([]string{provisioningv1.DPUFinalizer}))

			By("expecting the Status (Pending)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUPending))
		})

		It("attaching to node (Node Effect) is Drain", func() {
			By("creating the obj")
			obj := createObj("obj-dpu")
			obj.Spec.NodeName = testNode.Name
			obj.Spec.BFB = DefaultBFB
			obj.Spec.NodeEffect = &provisioningv1.NodeEffect{
				Drain: &provisioningv1.Drain{
					AutomaticNodeReboot: true,
				},
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))

			Skip("TODO: DPUCluter should be in Ready state to proceed")

			By("expecting the Status (Node Effect)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Phase).Should(Equal(provisioningv1.DPUInitializing))
				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
			))
			Expect(obj_fetched.Finalizers).Should(ConsistOf([]string{provisioningv1.DPUFinalizer}))

			By("expecting the Status (Error)")
			Eventually(func(g Gomega) []metav1.Condition {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				g.Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUError))
				return obj_fetched.Status.Conditions
			}).WithTimeout(30 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))

			By("deleting objs")
			cleanupObjs := []client.Object{}
			cleanupObjs = append(cleanupObjs, obj)
			Expect(testutils.CleanupAndWait(ctx, k8sClient, cleanupObjs...)).To(Succeed())
		})

		It("attaching to node (Node Effect) is NoEffect", func() {
			By("creating the obj")
			obj := createObj("obj-dpu")
			obj.Spec.BFB = DefaultBFB
			obj.Spec.NodeName = testNode.Name
			obj.Spec.NodeEffect = &provisioningv1.NodeEffect{
				NoEffect: true,
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))

			Skip("TODO: DPUCluter should be in Ready state to proceed")

			By("expecting the Status (Node Effect)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Phase).Should(Equal(provisioningv1.DPUInitializing))
				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
			))
			Expect(obj_fetched.Finalizers).Should(ConsistOf([]string{provisioningv1.DPUFinalizer}))

			By("expecting the Status (Pending)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUPending))

			By("creating the bfb")
			testBFB, symlinkTarget := createBFB(ctx, DefaultBFB)

			By("expecting the Status (DMSRunning)")
			Eventually(func(g Gomega) []metav1.Condition {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Conditions
			}).WithTimeout(30 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondBFBReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondDMSRunning.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUDMSDeployment))

			By("deleting objs")
			cleanupObjs := []client.Object{}
			cleanupObjs = append(cleanupObjs, obj)
			Expect(testutils.CleanupAndWait(ctx, k8sClient, cleanupObjs...)).To(Succeed())

			By("Cleaning the bfb")
			destroyBFB(ctx, testBFB, symlinkTarget)
		})

		It("attaching to node w/o Labels (Node Effect) is CustomLabel", func() {
			By("creating the obj")
			obj := createObj("obj-dpu")
			obj.Spec.BFB = DefaultBFB
			obj.Spec.NodeName = testNode.Name
			obj.Spec.NodeEffect = &provisioningv1.NodeEffect{
				CustomLabel: map[string]string{
					"provisioning.dpf.nvidia.com/bfb": "dummy.bfb",
					"version":                         "1.2.3",
				},
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))

			Skip("TODO: DPUCluter should be in Ready state to proceed")

			By("expecting the Status (Node Effect)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Phase).Should(Equal(provisioningv1.DPUInitializing))
				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
			))
			Expect(obj_fetched.Finalizers).Should(ConsistOf([]string{provisioningv1.DPUFinalizer}))

			By("expecting the Status (Pending)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUPending))

			By("checking the node`s Labels")
			node_fetched := &corev1.Node{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testNode), node_fetched)).To(Succeed())
			Expect(node_fetched.Labels).To(HaveLen(2))
			Expect(node_fetched.Labels).To(HaveKeyWithValue("provisioning.dpf.nvidia.com/bfb", "dummy.bfb"))
			Expect(node_fetched.Labels).To(HaveKeyWithValue("version", "1.2.3"))

			By("creating the bfb")
			testBFB, symlinkTarget := createBFB(ctx, DefaultBFB)

			By("expecting the Status (DMSRunning)")
			Eventually(func(g Gomega) []metav1.Condition {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Conditions
			}).WithTimeout(30 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondBFBReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondDMSRunning.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUDMSDeployment))

			By("deleting objs")
			cleanupObjs := []client.Object{}
			cleanupObjs = append(cleanupObjs, obj)
			Expect(testutils.CleanupAndWait(ctx, k8sClient, cleanupObjs...)).To(Succeed())

			By("Cleaning the bfb")
			destroyBFB(ctx, testBFB, symlinkTarget)
		})

		It("attaching to node with Labels (Node Effect) is CustomLabel", func() {
			By("creating the node obj with Labels")
			node_obj := createNode(ctx, "node-with-labels", map[string]string{
				"label1": "value1",
			})
			DeferCleanup(k8sClient.Delete, ctx, node_obj)

			By("creating the obj")
			obj := createObj("obj-dpu")
			obj.Spec.BFB = DefaultBFB
			obj.Spec.NodeName = node_obj.Name
			obj.Spec.NodeEffect = &provisioningv1.NodeEffect{
				CustomLabel: map[string]string{
					"provisioning.dpf.nvidia.com/bfb": "dummy.bfb",
					"version":                         "1.2.3",
				},
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))

			Skip("TODO: DPUCluter should be in Ready state to proceed")

			By("expecting the Status (Node Effect)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Phase).Should(Equal(provisioningv1.DPUInitializing))
				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
			))
			Expect(obj_fetched.Finalizers).Should(ConsistOf([]string{provisioningv1.DPUFinalizer}))

			By("expecting the Status (Pending)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUPending))

			By("checking the node`s Labels")
			node_fetched := &corev1.Node{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(node_obj), node_fetched)).To(Succeed())
			Expect(node_fetched.Labels).To(HaveLen(3))
			Expect(node_fetched.Labels).To(HaveKeyWithValue("label1", "value1"))
			Expect(node_fetched.Labels).To(HaveKeyWithValue("provisioning.dpf.nvidia.com/bfb", "dummy.bfb"))
			Expect(node_fetched.Labels).To(HaveKeyWithValue("version", "1.2.3"))

			By("creating the bfb")
			testBFB, symlinkTarget := createBFB(ctx, DefaultBFB)

			By("expecting the Status (DMSRunning)")
			Eventually(func(g Gomega) []metav1.Condition {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Conditions
			}).WithTimeout(30 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondBFBReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondDMSRunning.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUDMSDeployment))

			By("deleting objs")
			cleanupObjs := []client.Object{}
			cleanupObjs = append(cleanupObjs, obj)
			Expect(testutils.CleanupAndWait(ctx, k8sClient, cleanupObjs...)).To(Succeed())

			By("Cleaning the bfb")
			destroyBFB(ctx, testBFB, symlinkTarget)
		})

		It("attaching to node w/o Taints (Node Effect) is Taint", func() {
			By("creating the obj")
			obj := createObj("obj-dpu")
			obj.Spec.BFB = DefaultBFB
			obj.Spec.NodeName = testNode.Name
			taint_obj := &corev1.Taint{
				Key:       "testTaint1",
				Value:     "value1",
				Effect:    corev1.TaintEffectNoSchedule,
				TimeAdded: nil,
			}
			obj.Spec.NodeEffect = &provisioningv1.NodeEffect{
				Taint: taint_obj,
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			obj_fetched := &provisioningv1.Dpu{}

			By("expecting the Status (Initializing)")
			Eventually(func(g Gomega) provisioningv1.DpuPhase {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Phase
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Millisecond).Should(Equal(provisioningv1.DPUInitializing))

			Skip("TODO: DPUCluter should be in Ready state to proceed")

			By("expecting the Status (Node Effect)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Phase).Should(Equal(provisioningv1.DPUInitializing))
				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
			))
			Expect(obj_fetched.Finalizers).Should(ConsistOf([]string{provisioningv1.DPUFinalizer}))

			By("expecting the Status (Pending)")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &provisioningv1.Dpu{}
				newObj := &provisioningv1.Dpu{}
				g.Expect(k8sClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(k8sClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				obj_fetched = newObj
				return obj_fetched.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUPending))

			By("checking the node`s Taints")
			// The error typically comes if there is a taint on nodes for which you don't have corresponding toleration in pod spec.
			// node.kubernetes.io/not-ready: Node is not ready. This corresponds to the NodeCondition Ready being "False".
			taint_error_obj := corev1.Taint{
				Key:       "node.kubernetes.io/not-ready",
				Value:     "",
				Effect:    corev1.TaintEffectNoSchedule,
				TimeAdded: nil,
			}
			node_fetched := &corev1.Node{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testNode), node_fetched)).To(Succeed())
			Expect(testNode.Spec.Taints).To(HaveLen(1))
			Expect(testNode.Spec.Taints[0]).Should(Equal(taint_error_obj))

			By("creating the bfb")
			testBFB, symlinkTarget := createBFB(ctx, DefaultBFB)

			By("expecting the Status (DMSRunning)")
			Eventually(func(g Gomega) []metav1.Condition {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Status.Conditions
			}).WithTimeout(30 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", provisioningv1.DPUCondInitialized.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondInitialized.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondNodeEffectReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondNodeEffectReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondBFBReady.String()),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", provisioningv1.DPUCondBFBReady.String()),
				),
				And(
					HaveField("Type", provisioningv1.DPUCondDMSRunning.String()),
					HaveField("Status", metav1.ConditionFalse),
				),
			))
			Expect(obj_fetched.Status.Phase).Should(Equal(provisioningv1.DPUDMSDeployment))

			By("deleting objs")
			cleanupObjs := []client.Object{}
			cleanupObjs = append(cleanupObjs, obj)
			Expect(testutils.CleanupAndWait(ctx, k8sClient, cleanupObjs...)).To(Succeed())

			By("Cleaning the bfb")
			destroyBFB(ctx, testBFB, symlinkTarget)
		})
	})
})

var _ = Describe("DPUFlavor", func() {

	const (
		DefaultNS      = "dpf-provisioning-test"
		DefaultDpuName = "dpf-dpu"
	)

	var (
		testNS *corev1.Namespace
	)

	var getObjKey = func(obj *provisioningv1.DPUFlavor) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.DPUFlavor {
		return &provisioningv1.DPUFlavor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec: provisioningv1.DPUFlavorSpec{},
		}
	}

	BeforeEach(func() {
		By("creating the namespace")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: DefaultNS}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())
	})

	AfterEach(func() {
		By("deleting the namespace")
		Expect(k8sClient.Delete(ctx, testNS)).To(Succeed())
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("create and get object minimal", func() {
			By("creating the obj-1")
			obj1 := createObj("obj-dpuflavor-1")
			err := k8sClient.Create(ctx, obj1)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &provisioningv1.DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj1), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj1))

			data1, err := bfcfg.Generate(obj1, DefaultDpuName, "", "")
			Expect(err).To(Succeed())
			Expect(data1).ShouldNot(BeNil())

			By("creating the obj-2")
			yml := []byte(`
apiVersion: provisioning.dpf.nvidia.com/v1alpha1
kind: DPUFlavor
metadata:
  name: obj-dpuflavor-2
  namespace: default
`)
			obj2 := &provisioningv1.DPUFlavor{}
			err = yaml.UnmarshalStrict(yml, obj2)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj2)
			Expect(err).NotTo(HaveOccurred())

			data2, err := bfcfg.Generate(obj2, DefaultDpuName, "", "")
			Expect(err).To(Succeed())
			Expect(data2).ShouldNot(BeNil())

			By("compare the obj-1 and obj-2")
			Expect(data1).Should(Equal(data2))
		})

		It("create obj", func() {
			yml := []byte(`
apiVersion: provisioning.dpf.nvidia.com/v1alpha1
kind: DPUFlavor
metadata:
  name: obj
  namespace: default
spec:
  grub:
    kernelParameters:
      - console=hvc0
      - console=ttyAMA0
      - earlycon=pl011,0x13010000
      - fixrttc
      - net.ifnames=0
      - biosdevname=0
      - iommu.passthrough=1
      - cgroup_no_v1=net_prio,net_cls
      - hugepagesz=2048kB
      - hugepages=3072
  sysctl:
    parameters:
    - net.ipv4.ip_forward=1
    - net.ipv4.ip_forward_update_priority=0
  nvconfig:
    - device: "*"
      parameters:
        - PF_BAR2_ENABLE=0
        - PER_PF_NUM_SF=1
        - PF_TOTAL_SF=40
        - PF_SF_BAR_SIZE=10
        - NUM_PF_MSIX_VALID=0
        - PF_NUM_PF_MSIX_VALID=1
        - PF_NUM_PF_MSIX=228
        - INTERNAL_CPU_MODEL=1
        - SRIOV_EN=1
        - NUM_OF_VFS=30
        - LAG_RESOURCE_ALLOCATION=1
  ovs:
    rawConfigScript: |
      ovs-vsctl set Open_vSwitch . other_config:doca-init=true
      ovs-vsctl set Open_vSwitch . other_config:dpdk-extra="-a 0000:00:00.0"
      ovs-vsctl set Open_vSwitch . other_config:hw-offload-ct-size=64000
      ovs-vsctl set Open_vSwitch . other_config:dpdk-max-memzones="50000"
      ovs-vsctl set Open_vSwitch . other_config:hw-offload="true"
      ovs-vsctl set Open_vSwitch . other_config:pmd-quiet-idle=true
      ovs-vsctl set Open_vSwitch . other_config:max-idle=20000
      ovs-vsctl set Open_vSwitch . other_config:max-revalidator=5000
  bfcfgParameters:
    - ubuntu_PASSWORD=$1$rvRv4qpw$mS6kYODr8oMxORt.TkiTB0
    - WITH_NIC_FW_UPDATE=yes
    - ENABLE_SFC_HBN=no
  configFiles:
  - path: /etc/bla/blabla.cfg
    operation: append
    raw: |
        CREATE_OVS_BRIDGES="no"
        CREATE_OVS_BRIDGES="no"
    permissions: "0755"
`)
			obj := &provisioningv1.DPUFlavor{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			data, err := bfcfg.Generate(obj, DefaultDpuName, "", "")
			Expect(err).To(Succeed())
			Expect(data).ShouldNot(BeNil())
		})
	})
})

var _ = Describe("DMS Pod", func() {

	const (
		DefaultNS       = "dpf-provisioning-test"
		DefaultNodeName = "dpf-node"
		DefaultDpuName  = "dpf-dpu"
	)

	var (
		testNS   *corev1.Namespace
		testNode *corev1.Node
	)

	var createNode = func(ctx context.Context, name string) *corev1.Node {
		obj := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
				Labels: map[string]string{
					"feature.node.kubernetes.io/dpu.features-dpu-pciAddress": "0000-90-00",
					"feature.node.kubernetes.io/dpu.features-dpu-pf-name":    "ens1f0np0",
				}},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "127.0.0.1",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, obj)).NotTo(HaveOccurred())
		return obj
	}

	BeforeEach(func() {
		By("creating the namespace")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: DefaultNS}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())

		By("creating the node")
		testNode = createNode(ctx, DefaultNodeName)
	})

	AfterEach(func() {
		By("deleting the namespace")
		Expect(k8sClient.Delete(ctx, testNS)).To(Succeed())

		By("Cleaning the node")
		Expect(k8sClient.Delete(ctx, testNode)).To(Succeed())
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("create DMS Pod w/o Issuer", func() {
			By("creating the dpu")
			obj_dpu := &provisioningv1.Dpu{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultDpuName,
					Namespace: testNS.Name,
				},
				Spec:   provisioningv1.DpuSpec{},
				Status: provisioningv1.DpuStatus{},
			}
			Expect(k8sClient.Create(ctx, obj_dpu)).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj_dpu)

			By("creating DMS Pod")
			option := util.DPUOptions{}
			err := dms.CreateDMSPod(ctx, k8sClient, obj_dpu, option)
			Expect(err).To(MatchError(ContainSubstring("dpf-provisioning-selfsigned-issuer")))
		})

		It("create DMS Pod w/o Node", func() {
			By("creating Issuer")
			obj := &certmanagerv1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dpf-provisioning-selfsigned-issuer",
					Namespace: testNS.Name,
				},
			}
			Expect(k8sClient.Create(ctx, obj)).NotTo(HaveOccurred())

			By("creating the dpu")
			obj_dpu := &provisioningv1.Dpu{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultDpuName,
					Namespace: testNS.Name,
				},
				Spec:   provisioningv1.DpuSpec{},
				Status: provisioningv1.DpuStatus{},
			}
			Expect(k8sClient.Create(ctx, obj_dpu)).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj_dpu)

			By("creating DMS Pod")
			option := util.DPUOptions{}
			err := dms.CreateDMSPod(ctx, k8sClient, obj_dpu, option)
			Expect(err).To(HaveOccurred())
		})

		It("create DMS Pod w/o options", func() {
			By("creating Issuer")
			obj := &certmanagerv1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dpf-provisioning-selfsigned-issuer",
					Namespace: testNS.Name,
				},
			}
			Expect(k8sClient.Create(ctx, obj)).NotTo(HaveOccurred())

			By("creating the dpu")
			obj_dpu := &provisioningv1.Dpu{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultDpuName,
					Namespace: testNS.Name,
					Labels: map[string]string{
						"provisioning.dpf.nvidia.com/dpu-pciAddress": "0000-90-00",
						"provisioning.dpf.nvidia.com/dpu-pf-name":    "ens1f0np0",
					},
				},
				Spec: provisioningv1.DpuSpec{
					NodeName: testNode.Name,
				},
				Status: provisioningv1.DpuStatus{},
			}
			Expect(k8sClient.Create(ctx, obj_dpu)).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj_dpu)

			By("creating DMS Pod w/o options")
			option := util.DPUOptions{}
			err := dms.CreateDMSPod(ctx, k8sClient, obj_dpu, option)
			Expect(err).To(MatchError(ContainSubstring("persistentVolumeClaim.claimName")))
		})

		It("creating DMS Pod wit minimul options", func() {
			By("creating Issuer")
			obj := &certmanagerv1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dpf-provisioning-selfsigned-issuer",
					Namespace: testNS.Name,
				},
			}
			Expect(k8sClient.Create(ctx, obj)).NotTo(HaveOccurred())

			By("creating the dpu")
			obj_dpu := &provisioningv1.Dpu{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultDpuName,
					Namespace: testNS.Name,
					Labels: map[string]string{
						"provisioning.dpf.nvidia.com/dpu-pciAddress": "0000-90-00",
						"provisioning.dpf.nvidia.com/dpu-pf-name":    "ens1f0np0",
					},
				},
				Spec: provisioningv1.DpuSpec{
					NodeName: testNode.Name,
				},
				Status: provisioningv1.DpuStatus{},
			}
			Expect(k8sClient.Create(ctx, obj_dpu)).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj_dpu)

			By("creating DMS Pod")
			option := util.DPUOptions{
				DMSImageWithTag: "gitlab-master.nvidia.com:5005/doca-platform-foundation/dpf-provisioning-controller/dms-server:latest",
				BFBPVC:          "bfb-pvc",
			}
			err := dms.CreateDMSPod(ctx, k8sClient, obj_dpu, option)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &corev1.Pod{}

			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: obj_dpu.Namespace,
				Name:      cutil.GenerateDMSPodName(obj_dpu.Name)},
				obj_fetched)).To(Succeed())
			Expect(obj_fetched.OwnerReferences[0].Kind).Should(Equal("Dpu"))
			Expect(obj_fetched.OwnerReferences[0].Name).Should(Equal(obj_dpu.Name))
		})
	})
})
