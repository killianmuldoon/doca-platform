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

package dms

import (
	"context"
	"fmt"
	"time"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	dmsPath              string = "/opt/mellanox/doca/services/dms/dmsd"
	username             string = "admin"
	password             string = "admin"
	issuerKind           string = "Issuer"
	selfSignedIssuerName string = "dpf-provisioning-selfsigned-issuer"
	NumofVFDefaultValue  string = "16"
)

const (
	ContainerPort    int32  = 9339
	ContainerPortStr string = "9339"
	DMSImageFolder   string = "/bfb-folder"
)

func createIssuer(ctx context.Context, client client.Client, name string, namespace string, secretName string, owner *metav1.OwnerReference) error {
	issuer := &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				CA: &certmanagerv1.CAIssuer{
					SecretName: secretName,
				},
			},
		},
	}

	nn := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existingIssuer := &certmanagerv1.Issuer{}
	if err := client.Get(ctx, nn, existingIssuer); err != nil {
		if apierrors.IsNotFound(err) {
			err = client.Create(ctx, issuer)
			if err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("failed to get issuer: %v", err)
	}
	issuer.ResourceVersion = existingIssuer.ResourceVersion
	err := client.Update(ctx, issuer)
	if err != nil {
		return err
	}
	return nil
}

func createServerCertificate(ctx context.Context, client client.Client, name string, namespace string, secretName string, commonName string, issuerRef cmmeta.ObjectReference, owner *metav1.OwnerReference, ipAddresses []string) error {
	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName:  secretName,
			Duration:    &metav1.Duration{Duration: 24 * time.Hour},
			RenewBefore: &metav1.Duration{Duration: 8 * time.Hour},
			IssuerRef:   issuerRef,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageServerAuth,
			},
			CommonName:  commonName,
			IPAddresses: ipAddresses,
		},
	}

	nn := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existingCert := &certmanagerv1.Certificate{}
	if err := client.Get(ctx, nn, existingCert); err != nil {
		if apierrors.IsNotFound(err) {
			err = client.Create(ctx, cert)
			if err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("failed to get Certificate: %v", err)
	}
	return nil
}

func createClientCertificate(ctx context.Context, client client.Client, name string, namespace string, secretName string, commonName string, issuerRef cmmeta.ObjectReference, owner *metav1.OwnerReference) error {
	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName:  secretName,
			Duration:    &metav1.Duration{Duration: 24 * time.Hour},
			RenewBefore: &metav1.Duration{Duration: 8 * time.Hour},
			IssuerRef:   issuerRef,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageClientAuth},
			CommonName: commonName,
		},
	}

	nn := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existingCert := &certmanagerv1.Certificate{}
	if err := client.Get(ctx, nn, existingCert); err != nil {
		if apierrors.IsNotFound(err) {
			err = client.Create(ctx, cert)
			if err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("failed to get Certificate: %v", err)
	}
	return nil
}

func createRootCaCert(ctx context.Context, client client.Client, name string, namespace string, secretName string, commonName string, issuerRef cmmeta.ObjectReference) error {
	rootCertificate := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			IsCA:       true,
			CommonName: commonName,
			SecretName: secretName,
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.ECDSAKeyAlgorithm,
				Size:      256,
			},
			IssuerRef: issuerRef,
		},
	}

	nn := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	rootCert := &certmanagerv1.Certificate{}
	if err := client.Get(ctx, nn, rootCert); err != nil {
		if apierrors.IsNotFound(err) {
			err = client.Create(ctx, rootCertificate)
			if err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("failed to get Certificate: %v", err)
	}
	return nil
}

func CreateDMSPod(ctx context.Context, client client.Client, dpu *provisioningv1.DPU, option dutil.DPUOptions) error {
	logger := log.FromContext(ctx)
	dmsPodName := cutil.GenerateDMSPodName(dpu.Name)

	selfSignedIssuerName := selfSignedIssuerName

	nn := types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      selfSignedIssuerName,
	}

	selfSignedIssuer := &certmanagerv1.Issuer{}
	if err := client.Get(ctx, nn, selfSignedIssuer); err != nil {
		return fmt.Errorf("failed to get issuer: %v", err)
	}

	caCertName := cutil.GenerateCACertName(dpu.Namespace)
	caSecretName := cutil.GenerateCASecretName(dpu.Namespace)

	issuerRef := cmmeta.ObjectReference{
		Name: selfSignedIssuerName,
		Kind: issuerKind,
	}

	// Create Root CA certificate
	if err := createRootCaCert(ctx, client, caCertName, dpu.Namespace, caSecretName, caCertName, issuerRef); err != nil {
		logger.Error(err, fmt.Sprintf("Error creating server certificate: %v", err))
		return err
	}

	dmsServerIssuerName := cutil.GenerateDMSServerIssuerName(dpu.Name)

	owner := metav1.NewControllerRef(dpu,
		provisioningv1.GroupVersion.WithKind("DPU"))

	// Define the Server Issuer using the Self-Signed Issuer
	if err := createIssuer(ctx, client, dmsServerIssuerName, dpu.Namespace, caSecretName, owner); err != nil {
		logger.Error(err, fmt.Sprintf("Error creating server Issuer: %v", err))
		return err
	}

	dmsServerSecretName := cutil.GenerateDMSServerSecretName(dpu.Name)
	dmsServerCertName := cutil.GenerateDMSServerCertName(dpu.Name)

	issuerRef = cmmeta.ObjectReference{
		Name: dmsServerIssuerName,
		Kind: issuerKind,
	}

	nn = types.NamespacedName{
		Namespace: "",
		Name:      dpu.Spec.NodeName,
	}
	node := &corev1.Node{}
	if err := client.Get(ctx, nn, node); err != nil {
		return err
	}
	if len(node.Status.Addresses) == 0 {
		return fmt.Errorf("no IP addresses found in node %v status", node)
	}
	nodeInternalIP := node.Status.Addresses[0].Address

	// Create server certificate with Server Issuer
	if err := createServerCertificate(ctx, client, dmsServerCertName, dpu.Namespace, dmsServerSecretName, dmsServerCertName, issuerRef, owner, []string{nodeInternalIP}); err != nil {
		logger.Error(err, "Failed to create Server certificate", "dms", err)
		return err
	}

	dmsClientIssuerName := cutil.GenerateDMSClientIssuerName(dpu.Namespace)

	// Define the Client Issuer using the Self-Signed Issuer
	if err := createIssuer(ctx, client, dmsClientIssuerName, dpu.Namespace, caSecretName, owner); err != nil {
		logger.Error(err, fmt.Sprintf("Error creating client Issuer: %v", err))
		return err
	}

	issuerRef = cmmeta.ObjectReference{
		Name: dmsClientIssuerName,
		Kind: issuerKind,
	}

	dmsClientCertName := cutil.GenerateDMSClientCertName(dpu.Namespace)
	clientSecretName := cutil.GenerateDMSClientSecretName(dpu.Namespace)

	// Create client certificate with the Client Issuer
	if err := createClientCertificate(ctx, client, dmsClientCertName, dpu.Namespace, clientSecretName, dmsClientCertName, issuerRef, owner); err != nil {
		logger.Error(err, fmt.Sprintf("Error creating client certificate: %v", err))
		return err
	}

	removePrefix := true
	pciAddress, err := cutil.GetPCIAddrFromLabel(dpu.Labels, removePrefix)
	if err != nil {
		logger.Error(err, "Failed to get pci address from node label", "dms", err)
		return err
	}

	logger.V(3).Info(fmt.Sprintf("create %s DMS pod", dmsPodName))
	dmsCommand := fmt.Sprintf("./rshim.sh && %s -bind_address %s:%s -v 99 -auth cert -ca /etc/ssl/certs/server/ca.crt -key /etc/ssl/certs/server/tls.key -cert /etc/ssl/certs/server/tls.crt -password %s -username %s -image_folder %s -target_pci %s -exec_timeout %d -disable_unbind_at_activate -reboot_status_check none",
		dmsPath, nodeInternalIP, ContainerPortStr, password, username, DMSImageFolder, pciAddress, option.DMSTimeout)

	hostPathType := corev1.HostPathDirectory
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dmsPodName,
			Namespace:       dpu.Namespace,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{
					Name:            "dms",
					Image:           option.DMSImageWithTag,
					ImagePullPolicy: corev1.PullAlways,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: ContainerPort,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "server-volume",
							MountPath: "/etc/ssl/certs/server",
							ReadOnly:  true,
						},
						{
							Name:      "bfb",
							MountPath: "/bfb",
						},
						{
							Name:      "dev",
							MountPath: "/dev",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &[]bool{true}[0],
					},
					Command: []string{"/bin/bash", "-c", "--"},
					Args: []string{
						dmsCommand,
					},
				},
			},
			ImagePullSecrets: option.ImagePullSecrets,
			Volumes: []corev1.Volume{
				{
					Name: "server-volume",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: dmsServerSecretName,
						},
					},
				},
				{
					Name: "bfb",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: option.BFBPVC,
						},
					},
				},
				{
					Name: "dev",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/dev",
							Type: &hostPathType,
						},
					},
				},
			},
		},
	}

	pod.Spec.Affinity = cutil.ReplaceDaemonSetPodNodeNameNodeAffinity(
		pod.Spec.Affinity, dpu.Spec.NodeName)
	pod.Spec.Tolerations = cutil.GeneratePodToleration(*dpu.Spec.NodeEffect)

	err = client.Create(ctx, pod)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create %s DMS pod", dmsPodName))
		return err
	}
	logger.V(3).Info(fmt.Sprintf("%s DMS pod created", dmsPodName))
	return nil
}
