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

package controlplane

import (
	"encoding/json"
	"fmt"

	"github.com/nvidia/doca-platform/internal/controlplane/kubeconfig"
	controlplanemeta "github.com/nvidia/doca-platform/internal/controlplane/metadata"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Control Plane Helper Functions", func() {
	Context("GetDPFClusters() tests", func() {
		It("should list the clusters referenced by admin-kubeconfig secrets", func() {
			c := fakeClient.NewFakeClient()
			clusters := []DPFCluster{
				{Namespace: "one", Name: "cluster-one"},
				{Namespace: "two", Name: "cluster-two"},
				{Namespace: "three", Name: "cluster-three"},
			}
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				testKamajiClusterSecret(clusters[2]),
			}
			for _, s := range secrets {
				Expect(c.Create(ctx, s)).To(Succeed())
			}
			gotClusters, err := GetDPFClusters(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotClusters).To(ConsistOf(clusters))
		})
		It("should aggregate errors and return valid clusters when one secret is malformed", func() {
			c := fakeClient.NewFakeClient()
			clusters := []DPFCluster{
				{Namespace: "one", Name: "cluster-one"},
				{Namespace: "two", Name: "cluster-two"},
				{Namespace: "three", Name: "cluster-three"},
			}
			brokenSecret := testKamajiClusterSecret(clusters[2])
			delete(brokenSecret.Labels, controlplanemeta.DPFClusterSecretClusterNameLabelKey)
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				// This secret doesn't have one of the expected labels.
				brokenSecret,
			}
			for _, s := range secrets {
				Expect(c.Create(ctx, s)).To(Succeed())
			}
			gotClusters, err := GetDPFClusters(ctx, c)

			// Expect an error to be reported.
			Expect(err).To(HaveOccurred())
			// Expect just the first two clusters to be returned.
			Expect(gotClusters).To(ConsistOf(clusters[:2]))
		})
	})
})

func testKamajiClusterSecret(cluster DPFCluster) *corev1.Secret {
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
