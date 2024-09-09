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
	"encoding/base64"
	"fmt"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var testKubeconfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: %s
contexts:
- context:
    cluster: %s
    namespace: default
    user: test-service-account
  name: test-service-account
current-context: test-service-account
kind: Config
preferences: {}
users:
- name: test-service-account
  user:
    token: %s
`

var _ = Describe("DPUServiceCredentialRequest Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			testNS      *corev1.Namespace
			testDPU1NS  *corev1.Namespace
			testDPU2NS  *corev1.Namespace
			testDPU3NS  *corev1.Namespace
			cleanupObjs []client.Object
		)
		BeforeEach(func() {
			By("creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			testDPU1NS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "dpu-dsr-"}}
			testDPU2NS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "dpu-dsr-"}}
			testDPU3NS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "dpu-dsr-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			Expect(testClient.Create(ctx, testDPU1NS)).To(Succeed())
			Expect(testClient.Create(ctx, testDPU2NS)).To(Succeed())
			Expect(testClient.Create(ctx, testDPU3NS)).To(Succeed())

			clusters := []controlplane.DPFCluster{
				{Namespace: testDPU1NS.Name, Name: testDPU1NS.Name},
				{Namespace: testDPU2NS.Name, Name: testDPU2NS.Name},
				{Namespace: testDPU3NS.Name, Name: testDPU3NS.Name},
			}
			for i := range clusters {
				kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(clusters[i], cfg)
				Expect(err).NotTo(HaveOccurred())
				Expect(testClient.Create(ctx, kamajiSecret)).To(Succeed())
				cleanupObjs = append(cleanupObjs, kamajiSecret)
			}
		})
		AfterEach(func() {
			By("Cleanup the Namespace and Secrets")
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjs...)).To(Succeed())
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
			Expect(testClient.Delete(ctx, testDPU1NS)).To(Succeed())
			Expect(testClient.Delete(ctx, testDPU2NS)).To(Succeed())
			Expect(testClient.Delete(ctx, testDPU3NS)).To(Succeed())
		})

		It("should successfully reconcile the DPUServiceCredentialRequest on a DPUCluster", func() {
			dsr := getMinimalDPUServiceCredentialRequest(testNS.Name, testDPU1NS.Name)

			By("Creating the DPUServiceCredentialRequest")
			Expect(testClient.Create(ctx, dsr)).To(Succeed())

			Eventually(func(g Gomega) {
				assertDPUServiceCredentialRequest(g, testClient, dsr)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			Eventually(func(g Gomega) {
				assertDPUServiceCredentialRequestCondition(g, testClient, dsr)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			By("Verifying the DPUServiceCredentialRequest has created the Secret")
			assertDPUServiceCredentialRequestSecret(testClient, dsr, testDPU1NS.Name)
		})

		It("should successfully reconcile the DPUServiceCredentialRequest on a Host", func() {
			dsr := getMinimalDPUServiceCredentialRequest(testNS.Name, "")

			By("Creating the DPUServiceCredentialRequest")
			Expect(testClient.Create(ctx, dsr)).To(Succeed())

			By("Reconciling the created resource")
			Eventually(func(g Gomega) {
				assertDPUServiceCredentialRequest(g, testClient, dsr)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			Eventually(func(g Gomega) {
				assertDPUServiceCredentialRequestCondition(g, testClient, dsr)
			}).WithTimeout(30 * time.Second).Should(BeNil())
		})

		It("should successfully delete the DPUServiceCredentialRequest", func() {
			dsr := getMinimalDPUServiceCredentialRequest(testNS.Name, testDPU2NS.Name)

			By("Creating DPUServiceCredentialRequest")
			Expect(testClient.Create(ctx, dsr)).To(Succeed())

			By("Reconciling the created resource")
			Eventually(func(g Gomega) {
				assertDPUServiceCredentialRequest(g, testClient, dsr)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			By("Deleting the DPUServiceCredentialRequest")
			Expect(testClient.Delete(ctx, dsr)).NotTo(HaveOccurred())

			By("Verifying the DPUServiceCredentialRequest is deleted")
			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(dsr), dsr)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})

		It("should successfully update expired or soon expiring token for the DPUServiceCredentialRequest", func() {
			dsr := getMinimalDPUServiceCredentialRequest(testNS.Name, testDPU1NS.Name)

			// Set status with expiry in 5 minutes
			dsr.Status = dpuservicev1.DPUServiceCredentialRequestStatus{
				ServiceAccount:      ptr.To("default/test-service-account"),
				ExpirationTimestamp: &metav1.Time{Time: time.Now().Add(5 * time.Minute)},
				TargetClusterName:   ptr.To(testDPU1NS.Name),
			}

			By("Creating the DPUServiceCredentialRequest")
			Expect(testClient.Create(ctx, dsr)).To(Succeed())

			Eventually(func(g Gomega) {
				assertDPUServiceCredentialRequest(g, testClient, dsr)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			Eventually(func(g Gomega) {
				assertDPUServiceCredentialRequestCondition(g, testClient, dsr)
			}).WithTimeout(30 * time.Second).Should(BeNil())
		})
	})
})

func assertDPUServiceCredentialRequest(g Gomega, testClient client.Client, dsr *dpuservicev1.DPUServiceCredentialRequest) {
	gotDsr := &dpuservicev1.DPUServiceCredentialRequest{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dsr), gotDsr)).To(Succeed())
	g.Expect(gotDsr.Finalizers).To(ConsistOf([]string{dpuservicev1.DPUServiceCredentialRequestFinalizer}))
	g.Expect(gotDsr.Status.ServiceAccount).NotTo(BeNil())
	g.Expect(*gotDsr.Status.ServiceAccount).To(Equal(dsr.Spec.ServiceAccount.String()))
	g.Expect(gotDsr.Status.ExpirationTimestamp.Time).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))
	g.Expect(gotDsr.Status.IssuedAt).NotTo(BeNil())
}

func assertDPUServiceCredentialRequestSecret(testClient client.Client, dsr *dpuservicev1.DPUServiceCredentialRequest, clusterName string) {
	secret := &corev1.Secret{}
	err := testClient.Get(ctx, types.NamespacedName{Name: dsr.Spec.Secret.Name, Namespace: *dsr.Spec.Secret.Namespace}, secret)
	Expect(err).NotTo(HaveOccurred())
	Expect(secret.Data).NotTo(BeEmpty())
	Expect(secret.Data).To(HaveKey("kubeconfig"))
	Expect(secret.Data["kubeconfig"]).NotTo(BeEmpty())
	config, err := clientcmd.Load(secret.Data["kubeconfig"])
	Expect(err).NotTo(HaveOccurred())
	token := config.AuthInfos["test-service-account"].Token
	base64EncodeCA := base64.StdEncoding.EncodeToString(cfg.CAData)
	Expect(string(secret.Data["kubeconfig"])).To(Equal(fmt.Sprintf(testKubeconfig, base64EncodeCA, cfg.Host, clusterName, clusterName, token)))
}

func assertDPUServiceCredentialRequestCondition(g Gomega, testClient client.Client, dsr *dpuservicev1.DPUServiceCredentialRequest) {
	gotDsr := &dpuservicev1.DPUServiceCredentialRequest{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dsr), gotDsr)).To(Succeed())
	g.Expect(gotDsr.Status.Conditions).NotTo(BeNil())
	g.Expect(gotDsr.Status.Conditions).To(ConsistOf(
		And(
			HaveField("Type", string(conditions.TypeReady)),
			HaveField("Status", metav1.ConditionTrue),
			HaveField("Reason", string(conditions.ReasonSuccess)),
		),
		And(
			HaveField("Type", string(dpuservicev1.ConditionServiceAccountReconciled)),
			HaveField("Status", metav1.ConditionTrue),
			HaveField("Reason", string(conditions.ReasonSuccess)),
		),
		And(
			HaveField("Type", string(dpuservicev1.ConditionSecretReconciled)),
			HaveField("Status", metav1.ConditionTrue),
			HaveField("Reason", string(conditions.ReasonSuccess)),
		),
	))
}

func getMinimalDPUServiceCredentialRequest(testNamespace, targetCluster string) *dpuservicev1.DPUServiceCredentialRequest {
	spec := dpuservicev1.DPUServiceCredentialRequestSpec{
		ServiceAccount: dpuservicev1.NamespacedName{
			Name:      "test-service-account",
			Namespace: ptr.To("default"),
		},
		Duration: &metav1.Duration{
			Duration: time.Hour,
		},
		Type: "kubeconfig",
		Secret: dpuservicev1.NamespacedName{
			Name:      "test-secret",
			Namespace: ptr.To("default"),
		},
	}

	if targetCluster != "" {
		spec.TargetClusterName = &targetCluster
	}

	return &dpuservicev1.DPUServiceCredentialRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dpuservice-credential-request",
			Namespace: testNamespace,
		},
		Spec: spec,
	}
}
