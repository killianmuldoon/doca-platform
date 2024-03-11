/*
Copyright 2024.

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
	"fmt"
	"time"

	controlplanev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/controlplane/v1alpha1"
	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/dpuservice/kubeconfig"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DPUService Controller", func() {
	Context("When reconciling a resource", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpu-one"}})).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpu-two"}})).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpu-three"}})).To(Succeed())
		})
		AfterEach(func() {
			By("Cleanup the Namespace and Secrets")
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
		})
		It("should successfully reconcile the DPUService", func() {
			cleanupObjs := []client.Object{}
			defer func() {
				Expect(cleanupAndWait(ctx, testClient, cleanupObjs...)).To(Succeed())
			}()

			clusters := []dpfCluster{
				{Namespace: "dpu-one", Name: "cluster-one"},
				{Namespace: "dpu-two", Name: "cluster-two"},
				{Namespace: "dpu-three", Name: "cluster-three"},
			}
			for i := range clusters {
				cleanupObjs = append(cleanupObjs, testKamajiClusterSecret(clusters[i]))
				Expect(testClient.Create(ctx, testKamajiClusterSecret(clusters[i]))).To(Succeed())
			}

			dpuServices := []*dpuservicev1.DPUService{
				{ObjectMeta: metav1.ObjectMeta{Name: "dpu-one", Namespace: testNS.Name}},
				{ObjectMeta: metav1.ObjectMeta{Name: "dpu-two", Namespace: testNS.Name}},
			}
			for i := range dpuServices {
				cleanupObjs = append(cleanupObjs, dpuServices[i])
				Expect(testClient.Create(ctx, dpuServices[i])).To(Succeed())
			}

			Eventually(func(g Gomega) {
				assertDPUService(g, testClient, dpuServices)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Check that argo secrets have been created correctly.
			Eventually(func(g Gomega) {
				assertArgoCDSecrets(g, testClient, clusters)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Check that the argo AppProject has been created correctly
			Eventually(func(g Gomega) {
				assertAppProject(g, testClient, clusters)
			}).WithTimeout(30 * time.Second).Should(BeNil())
		})
	})
})

func assertDPUService(g Gomega, testClient client.Client, dpuServices []*dpuservicev1.DPUService) {
	for i := range dpuServices {
		gotDPUService := &dpuservicev1.DPUService{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServices[i]), gotDPUService)).To(Succeed())
		g.Expect(gotDPUService.Finalizers).To(ConsistOf([]string{dpuservicev1.DPUServiceFinalizer}))
	}
}

func assertArgoCDSecrets(g Gomega, testClient client.Client, clusters []dpfCluster) {
	gotArgoSecrets := &corev1.SecretList{}
	g.Expect(testClient.List(ctx, gotArgoSecrets, client.HasLabels{argoCDSecretLabelKey, dpfClusterLabelKey})).To(Succeed())
	// Assert the correct number of secrets was found.
	g.Expect(gotArgoSecrets.Items).To(HaveLen(len(clusters)))
	for _, s := range gotArgoSecrets.Items {
		// Assert each secret contains the required keys in Data.
		for _, key := range []string{"config", "name", "server"} {
			if _, ok := s.Data[key]; !ok {
				g.Expect(s.Data).To(HaveKey(key))
			}
		}
	}
}

func assertAppProject(g Gomega, testClient client.Client, clusters []dpfCluster) {
	// Check that an argo project has been created.
	appProject := &argov1.AppProject{}
	g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: argoCDNamespace, Name: appProjectName}, appProject)).To(Succeed())
	gotDestinations := appProject.Spec.Destinations
	g.Expect(gotDestinations).To(HaveLen(len(clusters)))
	expectedDestinations := []argov1.ApplicationDestination{}
	for _, c := range clusters {
		expectedDestinations = append(expectedDestinations, argov1.ApplicationDestination{
			Server:    fmt.Sprintf("%s-%s", c.Namespace, c.Name),
			Namespace: "*",
		})
	}
	g.Expect(gotDestinations).To(ConsistOf(expectedDestinations))
}

var _ = Describe("test DPUService reconciler step-by-step", func() {
	Context("When reconciling", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
		})
		AfterEach(func() {
			By("Cleanup the test Namespace")
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
			By("Cleanup the control plane and argoCD secrets")
			secretList := &corev1.SecretList{}
			objs := []client.Object{}

			// Delete all the secrets.
			Expect(testClient.List(ctx, secretList)).To(Succeed())
			for _, s := range secretList.Items {
				objs = append(objs, s.DeepCopy())
			}
			Expect(cleanupAndWait(ctx, testClient, objs...)).To(Succeed())
		})
		// Get Clusters.
		It("should list the clusters referenced by admin-kubeconfig secrets", func() {
			clusters := []dpfCluster{
				{Namespace: testNS.Name, Name: "cluster-one"},
				{Namespace: testNS.Name, Name: "cluster-two"},
				{Namespace: testNS.Name, Name: "cluster-three"},
			}
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				testKamajiClusterSecret(clusters[2]),
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}
			gotClusters, err := getClusters(ctx, testClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotClusters).To(ConsistOf(clusters))
		})
		It("should aggregate errors and return valid clusters when one secret is malformed", func() {
			clusters := []dpfCluster{
				{Namespace: testNS.Name, Name: "cluster-one"},
				{Namespace: testNS.Name, Name: "cluster-two"},
				{Namespace: testNS.Name, Name: "cluster-three"},
			}
			brokenSecret := testKamajiClusterSecret(clusters[2])
			delete(brokenSecret.Labels, controlplanev1.DPFClusterSecretClusterNameLabelKey)
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				// This secret doesn't have one of the expected labels.
				brokenSecret,
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}
			gotClusters, err := getClusters(ctx, testClient)

			// Expect an error to be reported.
			Expect(err).To(HaveOccurred())
			// Expect just the first two clusters to be returned.
			Expect(gotClusters).To(ConsistOf(clusters[:2]))
		})

		// reconcileSecrets
		It("should create an Argo secret based on the admin-kubeconfig for each cluster", func() {
			clusters := []dpfCluster{
				{Namespace: testNS.Name, Name: "cluster-one"},
				{Namespace: testNS.Name, Name: "cluster-two"},
				{Namespace: testNS.Name, Name: "cluster-three"},
			}
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				testKamajiClusterSecret(clusters[2]),
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}

			r := &DPUServiceReconciler{Client: testClient, Scheme: testClient.Scheme()}
			err := r.reconcileSecrets(ctx, clusters)
			Expect(err).NotTo(HaveOccurred())
			secretList := &corev1.SecretList{}
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, dpfClusterLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(3))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
		It("should create secrets for existing clusters when one cluster does not exist", func() {
			clusters := []dpfCluster{
				{Namespace: testNS.Name, Name: "cluster-four"},
				{Namespace: testNS.Name, Name: "cluster-five"},
				{Namespace: testNS.Name, Name: "cluster-six"},
			}
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				// Not creating a kamaji secret for this cluster.
				//testKamajiClusterSecret(clusters[2]),
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}

			r := &DPUServiceReconciler{Client: testClient, Scheme: testClient.Scheme()}

			err := r.reconcileSecrets(ctx, clusters)
			// Expect an error to be reported.
			Expect(err).To(HaveOccurred())

			// Expect reconciliation to have continued and created the other secrets.
			secretList := &corev1.SecretList{}
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, dpfClusterLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(2))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
		It("should create secrets for existing clusters when one cluster secret is malformed", func() {
			clusters := []dpfCluster{
				{Namespace: testNS.Name, Name: "cluster-seven"},
				{Namespace: testNS.Name, Name: "cluster-eight"},
				{Namespace: testNS.Name, Name: "cluster-nine"},
			}
			brokenSecret := testKamajiClusterSecret(clusters[2])
			brokenSecret.Data["admin.conf"] = []byte("just-a-field")
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				// the third secret is malformed.
				brokenSecret,
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}

			r := &DPUServiceReconciler{Client: testClient, Scheme: testClient.Scheme()}

			err := r.reconcileSecrets(ctx, clusters)
			// Expect an error to be reported.
			Expect(err).To(HaveOccurred())

			// Expect reconciliation to have continued and created the other secrets.
			secretList := &corev1.SecretList{}
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, dpfClusterLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(2))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
	})
})

func testKamajiClusterSecret(cluster dpfCluster) *corev1.Secret {
	adminConfig := &kubeconfig.Type{
		Clusters: []*kubeconfig.KubectlClusterWithName{
			{
				Name: cluster.Name,
				Cluster: kubeconfig.KubectlCluster{
					Server:                   "https://localhost.com:6443",
					CertificateAuthorityData: []byte("lotsofdifferentletterstobesecure"),
				},
			},
		},
		Users: []*kubeconfig.KubectlUserWithName{
			{
				Name: "not-used",
				User: kubeconfig.KubectlUser{
					ClientKeyData:         []byte("lotsofdifferentletterstobesecure"),
					ClientCertificateData: []byte("lotsofdifferentletterstobesecure"),
				},
			},
		},
	}
	confData, err := json.Marshal(adminConfig)
	Expect(err).To(Not(HaveOccurred()))
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-admin-kubeconfig", cluster.Name),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				controlplanev1.DPFClusterSecretClusterNameLabelKey: cluster.Name,
				"kamaji.clastix.io/component":                      "admin-kubeconfig",
				"kamaji.clastix.io/project":                        "kamaji",
			},
		},
		Data: map[string][]byte{
			"admin.conf": confData,
		},
	}
	// TODO: Test for ownerReferences.
}

func cleanupAndWait(ctx context.Context, c client.Client, objs ...client.Object) error {
	for _, o := range objs {
		if err := c.Delete(ctx, o); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	// Ensure each object is deleted by checking that each object returns an IsNotFound error in the api server.
	errs := []error{}
	for _, o := range objs {
		key := client.ObjectKeyFromObject(o)
		err := wait.ExponentialBackoff(
			wait.Backoff{
				Duration: 100 * time.Millisecond,
				Factor:   1.5,
				Steps:    10,
				Jitter:   0.4,
			},
			func() (done bool, err error) {
				if err := c.Get(ctx, key, o); err != nil {
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				}
				return false, nil
			})
		if err != nil {
			errs = append(errs, fmt.Errorf("key %s, %s is not being deleted: %s", o.GetObjectKind().GroupVersionKind().String(), key, err))
		}
	}
	return kerrors.NewAggregate(errs)
}
