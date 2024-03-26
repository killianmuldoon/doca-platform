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

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"
	controlplane "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/kubeconfig"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
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

			clusters := []controlplane.DPFCluster{
				{Namespace: "dpu-one", Name: "cluster-one"},
				{Namespace: "dpu-two", Name: "cluster-two"},
				{Namespace: "dpu-three", Name: "cluster-three"},
			}
			for i := range clusters {
				cleanupObjs = append(cleanupObjs, testKamajiClusterSecret(clusters[i]))
				Expect(testClient.Create(ctx, testKamajiClusterSecret(clusters[i]))).To(Succeed())
			}

			dpuServices := []*dpuservicev1.DPUService{
				{ObjectMeta: metav1.ObjectMeta{Name: "dpu-one", Namespace: testNS.Name},
					Spec: dpuservicev1.DPUServiceSpec{
						Source: dpuservicev1.ApplicationSource{
							RepoURL:     "repository.com",
							Version:     "v1.1",
							Chart:       "first-chart",
							ReleaseName: "release-one",
						},
						ServiceID: ptr.To("service-one"),
						Values: &runtime.RawExtension{
							Object: &unstructured.Unstructured{
								Object: map[string]interface{}{
									"value": "one",
									"other": "two",
								},
							},
						},
						ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
							NodeSelector: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "key",
												Operator: "Exists",
											},
										},
									},
								},
							},
							Resources: corev1.ResourceList{
								"cpu": *resource.NewQuantity(5, resource.DecimalSI),
							},
							UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{
								Type: appsv1.RollingUpdateDaemonSetStrategyType,
								RollingUpdate: &appsv1.RollingUpdateDaemonSet{
									MaxUnavailable: &intstr.IntOrString{
										Type:   0,
										IntVal: 0,
										StrVal: "",
									},
									MaxSurge: &intstr.IntOrString{
										Type:   0,
										IntVal: 0,
										StrVal: "",
									},
								},
							},
							Labels: map[string]string{
								"label-one": "label-value",
							},
							Annotations: map[string]string{
								"annotation-one": "annotation",
							},
						},
					},
				},
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
				assertArgoCDSecrets(g, testClient, clusters, &cleanupObjs)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Check that the argo AppProject has been created correctly
			Eventually(func(g Gomega) {
				assertAppProject(g, testClient, clusters)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Check that the argo Application has been created correctly
			Eventually(func(g Gomega) {
				assertApplication(g, testClient, dpuServices, clusters)
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

func assertArgoCDSecrets(g Gomega, testClient client.Client, clusters []controlplane.DPFCluster, cleanupObjs *[]client.Object) {
	gotArgoSecrets := &corev1.SecretList{}
	g.Expect(testClient.List(ctx, gotArgoSecrets, client.HasLabels{argoCDSecretLabelKey, controlplanemeta.DPFClusterLabelKey})).To(Succeed())
	// Assert the correct number of secrets was found.
	g.Expect(gotArgoSecrets.Items).To(HaveLen(len(clusters)))
	for _, s := range gotArgoSecrets.Items {
		// Assert each secret contains the required keys in Data.
		for _, key := range []string{"config", "name", "server"} {
			if _, ok := s.Data[key]; !ok {
				g.Expect(s.Data).To(HaveKey(key))
			}
		}
		*cleanupObjs = append(*cleanupObjs, s.DeepCopy())
	}
}

func assertAppProject(g Gomega, testClient client.Client, clusters []controlplane.DPFCluster) {
	// Check that an argo project has been created.
	appProject := &argov1.AppProject{}
	g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: argoCDNamespace, Name: appProjectName}, appProject)).To(Succeed())
	gotDestinations := appProject.Spec.Destinations
	g.Expect(gotDestinations).To(HaveLen(len(clusters)))
	expectedDestinations := []argov1.ApplicationDestination{}
	for _, c := range clusters {
		expectedDestinations = append(expectedDestinations, argov1.ApplicationDestination{
			Name:      c.Name,
			Namespace: "*",
		})
	}
	g.Expect(gotDestinations).To(ConsistOf(expectedDestinations))
}

func assertApplication(g Gomega, testClient client.Client, dpuServices []*dpuservicev1.DPUService, clusters []controlplane.DPFCluster) {
	// Check that argoApplications are created for each of the clusters.
	applications := &argov1.ApplicationList{}
	g.Expect(testClient.List(ctx, applications)).To(Succeed())

	// Check that we have one application for each cluster and dpuService.
	g.Expect(applications.Items).To(HaveLen(len(clusters) * len(dpuServices)))
	for _, app := range applications.Items {
		g.Expect(app.Labels).To(HaveKey(dpuservicev1.DPUServiceNameLabelKey))
		g.Expect(app.Labels).To(HaveKey(dpuservicev1.DPUServiceNamespaceLabelKey))
		dpuServiceName := app.Labels[dpuservicev1.DPUServiceNameLabelKey]
		dpuServiceNS := app.Labels[dpuservicev1.DPUServiceNamespaceLabelKey]
		for _, service := range dpuServices {
			if service.Name != dpuServiceName || service.Namespace != dpuServiceNS {
				continue
			}
			// Check the helm fields are set as expected.
			g.Expect(app.Spec.Source.Chart).To(Equal(service.Spec.Source.Chart))
			g.Expect(app.Spec.Source.Path).To(Equal(service.Spec.Source.Path))
			g.Expect(app.Spec.Source.RepoURL).To(Equal(service.Spec.Source.RepoURL))
			g.Expect(app.Spec.Source.TargetRevision).To(Equal(service.Spec.Source.Version))
			g.Expect(app.Spec.Source.Helm.ReleaseName).To(Equal(service.Spec.Source.ReleaseName))

			// If the DPUService doesn't define a ServiceDaemonSet the below assertions are not applicable.
			if service.Spec.ServiceDaemonSet == nil {
				return
			}

			// Check that the values fields are set as expected.
			var appValuesMap, serviceValuesMap map[string]interface{}
			g.Expect(json.Unmarshal(app.Spec.Source.Helm.ValuesObject.Raw, &appValuesMap)).To(Succeed())

			// Expect values to be correctly transposed from the DPUService serviceDaemonSet values.
			Expect(appValuesMap).To(HaveKey("serviceDaemonSet"))
			var appServiceDaemonSet = struct {
				ServiceDaemonSet dpuservicev1.ServiceDaemonSetValues `json:"serviceDaemonSet"`
			}{}
			Expect(json.Unmarshal(app.Spec.Source.Helm.ValuesObject.Raw, &appServiceDaemonSet)).To(Succeed())
			appService := appServiceDaemonSet.ServiceDaemonSet
			for k, v := range service.Spec.ServiceDaemonSet.Labels {
				Expect(appService.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(appService.Labels).To(HaveKeyWithValue(dpfServiceIDLabelKey, *service.Spec.ServiceID))
			Expect(appService.Resources).To(Equal(service.Spec.ServiceDaemonSet.Resources))
			Expect(appService.Annotations).To(Equal(service.Spec.ServiceDaemonSet.Annotations))
			Expect(appService.NodeSelector).To(Equal(service.Spec.ServiceDaemonSet.NodeSelector))
			Expect(appService.UpdateStrategy).To(Equal(service.Spec.ServiceDaemonSet.UpdateStrategy))

			// If this field is unset skip this assertion.
			if service.Spec.Values == nil {
				continue
			}

			// Expect every value passed in the service spec `.values` to be set in the application helm valuesObject.
			g.Expect(json.Unmarshal(service.Spec.Values.Raw, &serviceValuesMap)).To(Succeed())
			for k, v := range serviceValuesMap {
				g.Expect(appValuesMap).To(HaveKeyWithValue(k, v))
			}
		}
	}

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

		// reconcileSecrets
		It("should create an Argo secret based on the admin-kubeconfig for each cluster", func() {
			clusters := []controlplane.DPFCluster{
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
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, controlplanemeta.DPFClusterLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(3))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
		It("should create secrets for existing clusters when one cluster does not exist", func() {
			clusters := []controlplane.DPFCluster{
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
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, controlplanemeta.DPFClusterLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(2))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
		It("should create secrets for existing clusters when one cluster secret is malformed", func() {
			clusters := []controlplane.DPFCluster{
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
			Expect(testClient.List(ctx, secretList, client.HasLabels{argoCDSecretLabelKey, controlplanemeta.DPFClusterLabelKey})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(2))
			for _, s := range secretList.Items {
				Expect(s.Data).To(HaveKey("config"))
				Expect(s.Data).To(HaveKey("name"))
				Expect(s.Data).To(HaveKey("server"))
			}
		})
	})
})

func testKamajiClusterSecret(cluster controlplane.DPFCluster) *corev1.Secret {
	adminConfig := &kubeconfig.Type{
		Clusters: []*kubeconfig.ClusterWithName{
			{
				Name: cluster.Name,
				Cluster: kubeconfig.Cluster{
					Server:                   "https://localhost.com:6443",
					CertificateAuthorityData: []byte("lotsofdifferentletterstobesecure"),
				},
			},
		},
		Users: []*kubeconfig.UserWithName{
			{
				Name: "not-used",
				User: kubeconfig.User{
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
				controlplanemeta.DPFClusterSecretClusterNameLabelKey: cluster.Name,
				"kamaji.clastix.io/component":                        "admin-kubeconfig",
				"kamaji.clastix.io/project":                          "kamaji",
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
