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

package gnoi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/dms"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createGRPCConnection(ctx context.Context, client client.Client, dpu *provisioningv1.DPU, ctrlContext *dutil.ControllerContext) (*grpc.ClientConn, error) {
	nn := types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      cutil.GenerateDMSPodName(dpu),
	}
	pod := &corev1.Pod{}
	if err := client.Get(ctx, nn, pod); err != nil {
		return nil, fmt.Errorf("failed to get pod: %v", err)
	}

	dmsClientSecretName := dms.DMSClientSecret
	if ctrlContext.Options.CustomCASecretName != "" {
		dmsClientSecretName = ctrlContext.Options.CustomCASecretName
	}

	nn = types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      dmsClientSecretName,
	}
	dmsClientSecret := &corev1.Secret{}
	if err := client.Get(ctx, nn, dmsClientSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("client secret not found: %v", err)
		} else {
			return nil, fmt.Errorf("failed to get client secret: %v", err)
		}
	}

	// Extract the certificate and key from the secret
	dmsClientCert, certOk := dmsClientSecret.Data["tls.crt"]
	if !certOk {
		return nil, fmt.Errorf("tls.crt not found in client secret")
	}
	dmsClientKey, keyOk := dmsClientSecret.Data["tls.key"]
	if !keyOk {
		return nil, fmt.Errorf("tls.key not found in client secret")
	}

	// Load the DMS client's certificate and private key
	clientCert, err := tls.X509KeyPair(dmsClientCert, dmsClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert and key: %v", err)
	}

	// Extract the CA certificate
	caCert, caCertOk := dmsClientSecret.Data["ca.crt"]
	if !caCertOk {
		return nil, fmt.Errorf("ca.crt not found in Server secret")
	}

	// Create a certificate pool and add the CA certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append Server certificate")
	}

	// Create a mTLS config with the client certificate and CA certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	}

	serverAddress := dms.Address(pod.Status.PodIP, dpu)

	// Create a gRPC connection using grpc.NewClient
	conn, err := grpc.NewClient(serverAddress, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %v", err)
	}

	return conn, nil
}

func generateDMSTaskName(dpu *provisioningv1.DPU) string {
	return fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)
}

func handleDMSPodFailure(state *provisioningv1.DPUStatus, reason string, message string) (provisioningv1.DPUStatus, error) {
	cond := cutil.DPUCondition(provisioningv1.DPUCondDMSRunning, reason, message)
	cond.Status = metav1.ConditionFalse
	cutil.SetDPUCondition(state, cond)
	state.Phase = provisioningv1.DPUError
	return *state, fmt.Errorf("%s", message)
}
