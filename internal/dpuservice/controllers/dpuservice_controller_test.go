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
	"fmt"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd/api/application/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/metadata"
	operatorcontroller "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/controllers"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DPUService Controller", func() {
	Context("When reconciling a resource", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			dpfOperatorConfig := getMinimalDPFOperatorConfig()
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpu-one"}})).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpu-two"}})).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpu-three"}})).To(Succeed())
			Expect(
				// this namespace can be created multiple times.
				client.IgnoreAlreadyExists(
					testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpfOperatorConfig.GetNamespace()}})),
			).To(Succeed())
			Expect(client.IgnoreAlreadyExists(testClient.Create(ctx, dpfOperatorConfig))).To(Succeed())
		})
		AfterEach(func() {
			By("Cleanup the Namespace and Secrets")
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
		})
		It("should successfully reconcile the DPUService", func() {
			cleanupObjs := []client.Object{}
			defer func() {
				Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjs...)).To(Succeed())
			}()

			clusters := []controlplane.DPFCluster{
				{Namespace: "dpu-one", Name: "cluster-one"},
				{Namespace: "dpu-two", Name: "cluster-two"},
				{Namespace: "dpu-three", Name: "cluster-three"},
			}
			for i := range clusters {
				kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(clusters[i], cfg)
				Expect(err).NotTo(HaveOccurred())
				Expect(testClient.Create(ctx, kamajiSecret)).To(Succeed())
				cleanupObjs = append(cleanupObjs, kamajiSecret)
			}

			dpuServices := getMinimalDPUServices(testNS.Name)
			// A DPUService that should be deployed to the same cluster the DPF system is deployed in.
			var deployInCluster = true
			hostDPUService := &dpuservicev1.DPUService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host-dpu-service",
					Namespace: testNS.Name},
				Spec: dpuservicev1.DPUServiceSpec{
					DeployInCluster: &deployInCluster,
				},
			}

			By("create dpuservices and check the correct secrets, appproject and applications are created")
			// Create DPUServices which are reconciled to the DPU clusters.
			for i := range dpuServices {
				Expect(testClient.Create(ctx, dpuServices[i])).To(Succeed())
			}

			// Expect a hostDPUService to be reconciled to the host cluster.
			Expect(testClient.Create(ctx, hostDPUService)).To(Succeed())

			Eventually(func(g Gomega) {
				assertDPUService(g, testClient, dpuServices)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Check that argo secrets have been created correctly.
			Eventually(func(g Gomega) {
				assertArgoCDSecrets(g, testClient, clusters, &cleanupObjs)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Check that the argo AppProject has been created correctly
			Eventually(func(g Gomega) {
				assertAppProject(g, testClient, operatorcontroller.DefaultDPFOperatorConfigSingletonNamespace, clusters)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Check that the argo Application has been created correctly
			Eventually(func(g Gomega) {
				assertApplication(g, testClient, dpuServices, clusters)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			Eventually(func(g Gomega) {
				assertDPUServiceCondition(g, testClient, dpuServices)
			}).WithTimeout(30 * time.Second).Should(BeNil())

			By("delete the DPUService and ensure the application associated with it are deleted")
			for i := range dpuServices {
				Expect(testClient.Delete(ctx, dpuServices[i])).To(Succeed())
			}
			Expect(testClient.Delete(ctx, hostDPUService)).To(Succeed())
			// Ensure the applications are deleted.
			Eventually(func(g Gomega) {
				applications := &argov1.ApplicationList{}
				g.Expect(testClient.List(ctx, applications)).To(Succeed())
				// We're not running the ArgoCD controllers in this test so the finalizers must be removed here.
				// Do this in each loop as there's a race condition where the Application is patched again
				// by the DPUService controller.
				for i := range applications.Items {
					err := testClient.Patch(ctx, &applications.Items[i], client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`)))
					if err != nil && !apierrors.IsNotFound(err) {
						g.Expect(err).ToNot(HaveOccurred())
					}
				}
				g.Expect(applications.Items).To(BeEmpty())

			}).WithTimeout(30 * time.Second).Should(BeNil())

			// Ensure the DPUService finalizer is removed and they are deleted.
			Eventually(func(g Gomega) {
				gotDpuServices := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDpuServices)).To(Succeed())
				g.Expect(gotDpuServices.Items).To(BeEmpty())
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

func assertDPUServiceCondition(g Gomega, testClient client.Client, dpuServices []*dpuservicev1.DPUService) {
	for i := range dpuServices {
		gotDPUService := &dpuservicev1.DPUService{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServices[i]), gotDPUService)).To(Succeed())
		g.Expect(gotDPUService.Status.Conditions).NotTo(BeNil())
		g.Expect(gotDPUService.Status.Conditions).To(ConsistOf(
			And(
				HaveField("Type", string(conditions.TypeReady)),
				HaveField("Status", metav1.ConditionFalse),
				HaveField("Reason", string(conditions.ReasonPending)),
				HaveField("Message", fmt.Sprintf(conditions.MessageNotReadyTemplate, "ApplicationsReady")),
			),
			And(
				HaveField("Type", string(dpuservicev1.ConditionApplicationPrereqsReconciled)),
				HaveField("Status", metav1.ConditionTrue),
				HaveField("Reason", string(conditions.ReasonSuccess)),
			),
			And(
				HaveField("Type", string(dpuservicev1.ConditionApplicationsReconciled)),
				HaveField("Status", metav1.ConditionTrue),
				HaveField("Reason", string(conditions.ReasonSuccess)),
			),
			// Argo can not deploy anything on the DPUs during unit tests.
			And(
				HaveField("Type", string(dpuservicev1.ConditionApplicationsReady)),
				HaveField("Status", metav1.ConditionFalse),
				HaveField("Reason", string(conditions.ReasonPending)),
			),
		))
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

func assertAppProject(g Gomega, testClient client.Client, argoCDNamespace string, clusters []controlplane.DPFCluster) {
	// Check that the DPU cluster argo project has been created.
	appProject := &argov1.AppProject{}
	g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: argoCDNamespace, Name: dpuAppProjectName}, appProject)).To(Succeed())
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
	g.Expect(appProject.GetOwnerReferences()[0].Name).To(Equal(operatorcontroller.DefaultDPFOperatorConfigSingletonName))

	// Check that the host argo project has been created.
	g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: argoCDNamespace, Name: hostAppProjectName}, appProject)).To(Succeed())
	gotDestinations = appProject.Spec.Destinations
	expectedDestinations = []argov1.ApplicationDestination{
		{
			Name:      "in-cluster",
			Namespace: "*",
		}}
	g.Expect(gotDestinations).To(ConsistOf(expectedDestinations))
}

func assertApplication(g Gomega, testClient client.Client, dpuServices []*dpuservicev1.DPUService, clusters []controlplane.DPFCluster) {
	// Check that argoApplications are created for each of the clusters.
	applications := &argov1.ApplicationList{}
	g.Expect(testClient.List(ctx, applications)).To(Succeed())

	// Check that we have one application for each cluster and dpuService and an additional application for the hostDPUService.
	g.Expect(applications.Items).To(HaveLen(len(clusters)*len(dpuServices) + 1))
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
			g.Expect(app.Spec.Source.Chart).To(Equal(service.Spec.HelmChart.Source.Chart))
			g.Expect(app.Spec.Source.Path).To(Equal(service.Spec.HelmChart.Source.Path))
			g.Expect(app.Spec.Source.RepoURL).To(Equal(service.Spec.HelmChart.Source.RepoURL))
			g.Expect(app.Spec.Source.TargetRevision).To(Equal(service.Spec.HelmChart.Source.Version))
			g.Expect(app.Spec.Source.Helm.ReleaseName).To(Equal(service.Spec.HelmChart.Source.ReleaseName))

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
			if service.Spec.HelmChart.Values == nil {
				continue
			}

			// Expect every value passed in the service spec `.values` to be set in the application helm valuesObject.
			g.Expect(json.Unmarshal(service.Spec.HelmChart.Values.Raw, &serviceValuesMap)).To(Succeed())
			for k, v := range serviceValuesMap {
				g.Expect(appValuesMap).To(HaveKeyWithValue(k, v))
			}
		}
	}
}

func getMinimalDPUServices(testNamespace string) []*dpuservicev1.DPUService {
	return []*dpuservicev1.DPUService{
		{ObjectMeta: metav1.ObjectMeta{Name: "dpu-one", Namespace: testNamespace},
			Spec: dpuservicev1.DPUServiceSpec{
				HelmChart: dpuservicev1.HelmChart{
					Source: dpuservicev1.ApplicationSource{
						RepoURL:     "repository.com",
						Version:     "v1.1",
						Chart:       "first-chart",
						ReleaseName: "release-one",
					},
					Values: &runtime.RawExtension{
						Object: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"value": "one",
								"other": "two",
							},
						},
					},
				},
				ServiceID: ptr.To("service-one"),
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
		{ObjectMeta: metav1.ObjectMeta{Name: "dpu-two", Namespace: testNamespace}},
	}
}

var _ = Describe("test DPUService reconciler step-by-step", func() {
	Context("When reconciling", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			// Create the DPF System Namespace
			err := testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorcontroller.DefaultDPFOperatorConfigSingletonNamespace}})
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(client.IgnoreAlreadyExists(testClient.Create(ctx, getMinimalDPFOperatorConfig()))).To(Succeed())
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
			Expect(testutils.CleanupAndWait(ctx, testClient, objs...)).To(Succeed())
		})

		// reconcileArgoSecrets
		It("should create an Argo secret based on the admin-kubeconfig for each cluster", func() {
			clusters := []controlplane.DPFCluster{
				{Namespace: testNS.Name, Name: "cluster-one"},
				{Namespace: testNS.Name, Name: "cluster-two"},
				{Namespace: testNS.Name, Name: "cluster-three"},
			}

			secrets := []*corev1.Secret{}
			for _, cluster := range clusters {
				kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(cluster, cfg)
				Expect(err).ToNot(HaveOccurred())
				secrets = append(secrets, kamajiSecret)
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}

			r := &DPUServiceReconciler{Client: testClient, Scheme: testClient.Scheme()}
			err := r.reconcileArgoSecrets(ctx, clusters, getMinimalDPFOperatorConfig().GetNamespace())
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
			secrets := []*corev1.Secret{}
			for _, cluster := range clusters {
				// Not creating a kamaji secret for this cluster.
				if cluster.Name == "cluster-six" {
					continue
				}
				kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(cluster, cfg)
				Expect(err).ToNot(HaveOccurred())
				secrets = append(secrets, kamajiSecret)
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}

			r := &DPUServiceReconciler{Client: testClient, Scheme: testClient.Scheme()}
			err := r.reconcileArgoSecrets(ctx, clusters, getMinimalDPFOperatorConfig().GetNamespace())
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
			secrets := []*corev1.Secret{}
			for _, cluster := range clusters {
				kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(cluster, cfg)
				Expect(err).ToNot(HaveOccurred())
				// the third secret is malformed.
				if cluster.Name == "cluster-nine" {
					kamajiSecret.Data["admin.conf"] = []byte("just-a-field")
				}
				secrets = append(secrets, kamajiSecret)
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}

			r := &DPUServiceReconciler{Client: testClient, Scheme: testClient.Scheme()}
			err := r.reconcileArgoSecrets(ctx, clusters, getMinimalDPFOperatorConfig().GetNamespace())
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
		It("should reconcile image pull secrets created with the correct labels", func() {
			// Create a fake Kamaji cluster using the envtest cluster
			dpuCluster := controlplane.DPFCluster{Namespace: testNS.Name, Name: "cluster-seven"}
			secret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(dpuCluster, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			// Create some secrets that should be mirrored to the DPUCluster.
			labels := map[string]string{dpuservicev1.DPFImagePullSecretLabelKey: ""}
			Expect(testClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dpf-secret-one", Namespace: testNS.Name, Labels: labels}})).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dpf-secret-two", Namespace: testNS.Name, Labels: labels}})).To(Succeed())

			// Create a secret that should not be mirrored as it does not have the correct labels.
			Expect(testClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "not-a-dpf-secret", Namespace: testNS.Name}})).To(Succeed())

			// Create a secret that should not be mirrored as it does not have the correct namespace.
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "anothernamespace"}})).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "another-not-a-dpf-secret", Namespace: "anothernamespace"}})).To(Succeed())

			// Reconcile a DPUService in a different namespace and check to see that it has been cloned.
			cloningNamespace := "namespace-to-clone-to"
			Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cloningNamespace}})).To(Succeed())
			dpuService := &dpuservicev1.DPUService{ObjectMeta: metav1.ObjectMeta{Name: "name", Namespace: cloningNamespace}}
			r := &DPUServiceReconciler{Client: testClient, Scheme: testClient.Scheme()}
			Expect(r.reconcileImagePullSecrets(ctx, []controlplane.DPFCluster{dpuCluster}, dpuService)).To(Succeed())

			// Check we have the correct secrets cloned to the intended namespace
			gotSecrets := &corev1.SecretList{}
			Expect(testClient.List(ctx, gotSecrets, client.InNamespace(cloningNamespace))).To(Succeed())
			for _, gotSecret := range gotSecrets.Items {
				Expect(gotSecret.Name).To(BeElementOf([]string{"dpf-secret-one", "dpf-secret-two"}))
			}
		})
	})
})

func getMinimalDPFOperatorConfig() *operatorv1.DPFOperatorConfig {
	return &operatorv1.DPFOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorcontroller.DefaultDPFOperatorConfigSingletonName,
			Namespace: operatorcontroller.DefaultDPFOperatorConfigSingletonNamespace,
		},
		Spec: operatorv1.DPFOperatorConfigSpec{},
	}
}
