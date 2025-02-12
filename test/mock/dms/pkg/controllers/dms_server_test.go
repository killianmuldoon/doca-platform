/*
Copyright 2025 NVIDIA

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/test/mock/dms/pkg/certs"
	"github.com/nvidia/doca-platform/test/utils"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	nodeName       string = "node-1"
	bfbName        string = "bfb-1"
	dpuClusterName string = "dpu-cluster-1"
	dpuFlavorName  string = "dpu-flavor-1"
)

func TestDMSServerReconciler(t *testing.T) {
	t.Skip("not running due to race condition in the DPU controller")
	g := NewWithT(t)

	// 1) Initializing phase requires a node with the DPUOOBBridgeConfiguredLabel exists.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				// DPUs require a node with this label to advance past the "Initializing" phase
				cutil.NodeFeatureDiscoveryLabelPrefix + cutil.DPUOOBBridgeConfiguredLabel: "true",
			},
		},
	}
	g.Expect(testClient.Create(ctx, node)).To(Succeed())

	// 2) NodeEffect phase is handled by setting `.spec.NodeEffect.NoEffect=true` in the DPU.

	// 3) Pending phase requires that the BFB exist and is in Phase "BFBReady".
	// The BFB file path must exist
	_, err := os.Create(filepath.Join(cutil.BFBBaseDir, bfbName))
	g.Expect(err).NotTo(HaveOccurred())
	bfb := &provisioningv1.BFB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bfbName,
			Namespace: "default",
		},
		Spec: provisioningv1.BFBSpec{
			URL: "http://BlueField/BFBs/bf-bundle-dummy-8KB.bfb",
		},
	}
	g.Expect(testClient.Create(ctx, bfb)).To(Succeed())
	bfb.Status.Phase = provisioningv1.BFBReady
	bfb.Status.FileName = bfbName
	g.Expect(testClient.Status().Update(ctx, bfb)).To(Succeed())

	// 4) DPUInitializeInterface phase DMS Deploy checks that the DMS pod is "Running" and has a PodIP.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dmsServerPodName,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "bfb",
					Image: "nvidia/cuda/bundle:1.0",
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, pod)).To(Succeed())
	pod.Status.Phase = corev1.PodRunning
	// This IP will is used by the DPU controller to talk to DMS.
	pod.Status.PodIP = "127.0.0.1"

	// 4) DPUInitializeInterface OS Installing Phase requires the provisioning client Secret.
	// This allows communication with the DPF server.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			// This name is hard-coded in the provisioning controller
			Name:      "dpf-provisioning-client-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"tls.crt": certs.EncodeCertPEM(cert),
			"tls.key": certs.EncodePrivateKeyPEM(key),
			"ca.crt":  certs.EncodeCertPEM(cert),
		},
	}
	g.Expect(testClient.Create(ctx, secret)).To(Succeed())

	// The OS Installing phase requires the dpuFlavor.
	dpuFlavor := &provisioningv1.DPUFlavor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuFlavorName,
			Namespace: "default",
		},
	}
	g.Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())

	// The OS Installing phase requires the dpuCluster.
	dpuCluster := &provisioningv1.DPUCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuClusterName,
			Namespace: "default",
		},
		Spec: provisioningv1.DPUClusterSpec{
			Type:       "static",
			Kubeconfig: fmt.Sprintf("%v-admin-kubeconfig", dpuClusterName),
		},
	}
	g.Expect(testClient.Create(ctx, dpuCluster)).To(Succeed())
	// For this test the DPUCluster points to the envtest kubeconfig.
	kubeconfigSecret, err := utils.GetFakeKamajiClusterSecretFromEnvtest(*dpuCluster, cfg)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(testClient.Create(ctx, kubeconfigSecret)).To(Succeed())
	// Also write the kubeconfig to a local file where it can be read by the DPU Controller.
	data, err := json.Marshal(kubeconfigSecret)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(os.MkdirAll(filepath.Dir(cutil.AdminKubeConfigPath(*dpuCluster)), 0755)).To(Succeed())
	g.Expect(os.WriteFile(cutil.AdminKubeConfigPath(*dpuCluster), data, os.ModePerm)).To(Succeed())

	// CLOUD_INIT_TIMEOUT is used in the dmsHandler in installing.go. It's set here to reduce waiting time during the
	// test.
	g.Expect(os.Setenv("CLOUD_INIT_TIMEOUT", "1")).To(Succeed())

	// 5) Rebooting phase is handled by using a stub HostUptimeChecker which simulates a reboot.

	// 6) The DPUInitializeInterface HostNetworkingConfig phase checks that the first container in the pod has a Ready ContainerStatus.
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "Ready", Ready: true}}
	g.Expect(testClient.Status().Update(ctx, pod)).To(Succeed())

	// 7) The DPUClusterConfig phase requires the DPU node object be created and ready.
	// This is handled in the controller code.
	tests := []struct {
		name  string
		input []*provisioningv1.DPU
	}{
		{
			name:  "Provision many DPUs until ready",
			input: createDPUs(30),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the DPU and check its provisioning process.
			for _, dpu := range tt.input {
				g.Expect(testClient.Create(ctx, dpu)).To(Succeed())
			}

			g.Eventually(func(g Gomega) {
				got := &provisioningv1.DPUList{}
				g.Expect(testClient.List(ctx, got)).To(Succeed())
				for _, dpu := range got.Items {
					g.Expect(dpu.Annotations).To(HaveKey(cutil.OverrideDMSPodNameAnnotationKey))
					g.Expect(dpu.Annotations).To(HaveKey(cutil.OverrideHostNetworkAnnotationKey))
					g.Expect(dpu.Annotations).To(HaveKey(cutil.OverrideDMSPortAnnotationKey))
					g.Expect(dpu.Status.Phase).To(Equal(provisioningv1.DPUReady))
				}
			}).WithTimeout(100 * time.Second).Should(Succeed())
		})
	}
}

func createDPUs(n int) []*provisioningv1.DPU {
	dpus := []*provisioningv1.DPU{}
	for i := range n {
		dpus = append(dpus, &provisioningv1.DPU{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("dpu-%d", i),
				Namespace: "default",
			},
			Spec: provisioningv1.DPUSpec{
				NodeName:   nodeName,
				BFB:        bfbName,
				PCIAddress: "08",
				NodeEffect: &provisioningv1.NodeEffect{
					// TODO: NoEffect should be changed here to test more of the code.
					NoEffect: ptr.To(true),
				},
				Cluster: provisioningv1.K8sCluster{
					Namespace: "default",
					Name:      dpuClusterName,
				},
				DPUFlavor:           dpuFlavorName,
				AutomaticNodeReboot: false,
			},
		})
	}
	return dpus
}

// newCert creates a CA certificate.
func newCert(key *rsa.PrivateKey) (*x509.Certificate, error) {
	now := time.Now().UTC()

	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   "dms-server",
			Organization: []string{"doca-platform"},
		},
		IPAddresses:           []net.IP{{127, 0, 0, 1}},
		NotBefore:             now.Add(time.Minute * -5),
		NotAfter:              now.Add(time.Hour * 24 * 365 * 10), // 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		MaxPathLenZero:        true,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		IsCA:                  true,
	}

	b, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("failed to create self signed CA certificate")
	}

	c, err := x509.ParseCertificate(b)
	return c, err
}

// mockKubeadmJoinCommandGenerator implements the interface for generating a node join command for the DPUNode.
// The implementation is purely a stub as this command it outputs is never run.
type mockKubeadmJoinCommandGenerator struct{}

func (m *mockKubeadmJoinCommandGenerator) GenerateJoinCommand(*provisioningv1.DPUCluster) (string, error) {
	return "soup", nil
}

// mockHostUptimeReporter implements the interface for checking if a node reboot has occurred..
// The implementation returns 0 to speed up testing.
type mockHostUptimeReporter struct{}

func (m mockHostUptimeReporter) HostUptime(ns, name, container string) (int, error) {
	return 0, nil
}
