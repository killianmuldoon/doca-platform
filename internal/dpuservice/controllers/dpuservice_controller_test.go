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
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/dpuservice/kubeconfig"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("DPUService Controller", func() {
	Context("When reconciling a resource", func() {
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
		dpuServiceName := "dpu-one"

		dpuServiceKey := types.NamespacedName{}
		dpuService := &dpuservicev1.DPUService{}

		BeforeEach(func() {
			By("creating the namespace")
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			dpuServiceKey = types.NamespacedName{Name: dpuServiceName, Namespace: testNS.Name}
			dpuService = &dpuservicev1.DPUService{
				ObjectMeta: metav1.ObjectMeta{Name: dpuServiceName, Namespace: testNS.Name},
			}
			By("creating the DPUService")
			Expect(testClient.Create(ctx, dpuService)).To(Succeed())
		})
		AfterEach(func() {
			By("Cleanup the DPUService and Namespace")
			Expect(testClient.Delete(ctx, dpuService)).To(Succeed())
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
		})
		It("should successfully add the DPUService finalizer", func() {
			By("Reconciling the created resource for the first time")
			r := &DPUServiceReconciler{
				Client: testClient,
				Scheme: testClient.Scheme(),
			}
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: dpuServiceKey,
			})
			Expect(err).NotTo(HaveOccurred())
			gotDPUService := &dpuservicev1.DPUService{}
			Expect(testClient.Get(ctx, dpuServiceKey, gotDPUService)).To(Succeed())
			Expect(gotDPUService.GetFinalizers()).To(ConsistOf(dpuservicev1.DPUServiceFinalizer))
		})
	})
})

var _ = Describe("test DPUService reconciler step-by-step", func() {
	Context("When reconciling", func() {
		var testNS *corev1.Namespace
		dpuService := &dpuservicev1.DPUService{ObjectMeta: metav1.ObjectMeta{Name: "service-01", Namespace: "", UID: "one"}}
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
			Expect(cleanupAndWait(ctx, testClient, objs)).To(Succeed())
		})
		// Get Clusters.
		It("should list the clusters referenced by admin-kubeconfig secrets", func() {
			clusters := []types.NamespacedName{
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
			clusters := []types.NamespacedName{
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
			clusters := []types.NamespacedName{
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
			err := r.reconcileSecrets(ctx, dpuService, clusters)
			Expect(err).NotTo(HaveOccurred())
			secretList := &corev1.SecretList{}
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, dpfClusterNameLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(3))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
		It("should create secrets for existing clusters when one cluster does not exist", func() {
			clusters := []types.NamespacedName{
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

			err := r.reconcileSecrets(ctx, dpuService, clusters)
			// Expect an error to be reported.
			Expect(err).To(HaveOccurred())

			// Expect reconciliation to have continued and created the other secrets.
			secretList := &corev1.SecretList{}
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, dpfClusterNameLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(2))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
		It("should create secrets for existing clusters when one cluster secret is malformed", func() {
			clusters := []types.NamespacedName{
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

			err := r.reconcileSecrets(ctx, dpuService, clusters)
			// Expect an error to be reported.
			Expect(err).To(HaveOccurred())

			// Expect reconciliation to have continued and created the other secrets.
			secretList := &corev1.SecretList{}
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, dpfClusterNameLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(2))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
	})
})

func testKamajiClusterSecret(cluster types.NamespacedName) *corev1.Secret {
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
}

func cleanupAndWait(ctx context.Context, c client.Client, objs []client.Object) error {
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
				Steps:    8,
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
