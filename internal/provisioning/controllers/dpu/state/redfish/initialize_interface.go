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

package redfish

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	rfclient "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/state/redfish/client"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	BMCUser              = "root"
	BMCPasswordSecret    = "bmc-shared-password"
	BMCSharedPasswordKey = "password"
	BMCDefaultPassword   = "0penBmc"
)

func InitializeInterface(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	state := dpu.Status.DeepCopy()

	basicAuthClient, err := initPassword(ctx, dpu, ctrlCtx)
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondInterfaceInitialized), err, "FailedToInitializePassword", ""))
		return *state, err
	}

	if err = setUpMTLS(ctx, dpu, ctrlCtx, basicAuthClient); err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondInterfaceInitialized), err, "FailedToSetUpMTLS", ""))
		return *state, err
	}

	// verify mTLS by checking BMC FW
	tlsClient, err := rfclient.NewTLSClient(ctx, dpu, ctrlCtx.Client)
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondInterfaceInitialized), err, "FailedToCreateTLSClient", ""))
		return *state, err
	}
	resp, fwRsp, err := tlsClient.CheckBMCFirmware()
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondInterfaceInitialized), err, "FailedToCheckFW", ""))
		return *state, err
	} else if resp.StatusCode() != http.StatusOK {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondInterfaceInitialized), fmt.Errorf("status code: %d", resp.StatusCode()), "FailedToCheckFW", ""))
		return *state, err
	}
	log.FromContext(ctx).Info(fmt.Sprintf("BMC firmware version %q", fwRsp.Version))

	state.Phase = provisioningv1.DPUConfigFWParameters
	cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondInterfaceInitialized), nil, "", ""))
	return *state, nil
}

// initPassword reads a password from the user-created Secret and set it to all DPUs
func initPassword(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (*rfclient.Client, error) {
	if net.ParseIP(dpu.Spec.BMCIP) == nil {
		return nil, fmt.Errorf("invalid IP: %s", dpu.Spec.BMCIP)
	}

	// get BMC password from the secret created by user
	nn := types.NamespacedName{Name: BMCPasswordSecret, Namespace: dpu.Namespace}
	passwdSecret := &corev1.Secret{}
	if err := ctrlCtx.Get(ctx, nn, passwdSecret); err != nil {
		return nil, err
	}
	passwd := string(passwdSecret.Data[BMCSharedPasswordKey])
	if passwd == "" {
		return nil, fmt.Errorf("password not specified in secret %s", nn.String())
	}

	// check if the default password has been changed as requested by the DOCA BMC manual
	client, err := rfclient.NewBasicAuthClient(dpu.Spec.BMCIP, BMCUser, passwd)
	if err != nil {
		return nil, err
	}
	resp, _, err := client.CheckBMCFirmware()
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case http.StatusUnauthorized:
		log.FromContext(ctx).Info("try to change password")
		defaultClient, err := rfclient.NewBasicAuthClient(dpu.Spec.BMCIP, BMCUser, BMCDefaultPassword)
		if err != nil {
			return nil, err
		}
		resp, _, err = defaultClient.ChangeBMCPassword(passwd)
		if err != nil {
			return nil, err
		} else if resp.StatusCode() == http.StatusUnauthorized {
			return nil, fmt.Errorf("the default BMC password has been changed and the given password is wrong")
		} else if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("expected status %q, received status %q", http.StatusOK, resp.Status())
		}
		log.FromContext(ctx).Info("successfully changed password")
		return client, nil
	case http.StatusOK:
		return client, nil
	default:
		return nil, fmt.Errorf("unexpected status: %q", resp.Status())
	}
}

// setUpMTLS sets up BMC mTLS in the same way as https://gitlab-master.nvidia.com/-/snippets/8219
func setUpMTLS(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext, basicAuthClient *rfclient.Client) error {
	caSecret := &corev1.Secret{}
	if err := ctrlCtx.Client.Get(ctx, types.NamespacedName{Name: rfclient.CASecret, Namespace: dpu.Namespace}, caSecret); err != nil {
		return fmt.Errorf("failed to get CA cert, err: %v", err)
	}
	caCert, ok := caSecret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("no CA cert in secret %s", caSecret.Name)
	}

	// step 1: install or replace CA certificate
	resp, _, err := basicAuthClient.InstallCert(string(caCert))
	if err != nil {
		return fmt.Errorf("failed to install CA cert, err: %v", err)
	} else if resp.StatusCode() == http.StatusInternalServerError {
		log.FromContext(ctx).Info("An existing CA certificate is likely already installed. Replacing...")
		resp, _, err = basicAuthClient.ReplaceCACert(string(caCert))
		if err != nil {
			return fmt.Errorf("failed to replace CA cert, err: %v", err)
		} else if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("failed to replace CA cert, unexpected response status: %s", resp.Status())
		}
		log.FromContext(ctx).Info("Successfully replaced CA certificate")
	} else if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("failed to install CA cert, unexpected response status: %s", resp.Status())
	}

	// step 2: replace server certificate
	log.FromContext(ctx).Info("Replace server certificate...")
	cr := &certmanagerv1.CertificateRequest{}
	if err := ctrlCtx.Client.Get(ctx, types.NamespacedName{Name: dpu.Name, Namespace: dpu.Namespace}, cr); apierrors.IsNotFound(err) {
		log.FromContext(ctx).Info("cert-manager CertificateRequest does not exist, try create one...")
		resp, csrInfo, err := basicAuthClient.GenerateCSR(dpu.Spec.BMCIP)
		if err != nil {
			return fmt.Errorf("failed to generate CSR, err: %v", err)
		} else if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("failed to generate CSR, unexpected response status: %s", resp.Status())
		}
		if err := createCR(ctx, dpu, ctrlCtx, []byte(csrInfo.CSRString)); err != nil {
			return fmt.Errorf("failed to create cert-manager CertificateRequest, err: %v", err)
		}
		log.FromContext(ctx).Info("successfully created cert-manager CertificateRequest")
	} else if err != nil {
		return fmt.Errorf("failed to get existing cert-manager CertificateRequest, err: %v", err)
	} else if cr.Status.Certificate == nil {
		return fmt.Errorf("cert-manager CertificateRequest is not issued yet, retry later")
	}
	resp, _, err = basicAuthClient.ReplaceServerCert(string(cr.Status.Certificate))
	if err != nil {
		return fmt.Errorf("failed to replace server cert, err: %v", err)
	} else if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("failed to replace server cert, unexpected response status: %s", resp.Status())
	}
	log.FromContext(ctx).Info("Successfully replaced server certificate")

	// step 3: install client certificate
	log.FromContext(ctx).Info("Install client certificate...")
	clientSecret := &corev1.Secret{}
	if err := ctrlCtx.Client.Get(ctx, types.NamespacedName{Name: rfclient.ClientCertSecret, Namespace: dpu.Namespace}, clientSecret); err != nil {
		return fmt.Errorf("failed to get client cert, err: %v", err)
	}
	clientCert, ok := clientSecret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("no client cert in client secret %s", clientSecret.Name)
	}
	resp, _, err = basicAuthClient.InstallCert(string(clientCert))
	if err != nil {
		return fmt.Errorf("failed to install client cert, err: %v", err)
	} else if resp.StatusCode() == http.StatusInternalServerError {
		log.FromContext(ctx).Info("An existing client certificate is likely already installed. Skip installing client certificate")
	} else if resp.StatusCode() == http.StatusOK {
		log.FromContext(ctx).Info("Successfully installed client certificate")
	} else {
		return fmt.Errorf("failed to install client cert, unexpected response status: %s", resp.Status())
	}

	// step 4: enable mTLS
	log.FromContext(ctx).Info("enable mTLS...")
	resp, _, err = basicAuthClient.EnableMTLS()
	if err != nil {
		return fmt.Errorf("failed to enable mTLS, err: %v", err)
	} else if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("failed to enable mTLS, unexpected response status: %s", resp.Status())
	}
	log.FromContext(ctx).Info("Successfully enabled mTLS")
	return nil
}

// createCR creates a cert-manager CertificateRequest for the given CSR
func createCR(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext, csr []byte) error {
	cr := &certmanagerv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dpu.Name,
			Namespace:       dpu.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(dpu, provisioningv1.DPUGroupVersionKind)},
		},
		Spec: certmanagerv1.CertificateRequestSpec{
			Request: csr,
			IsCA:    false,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageServerAuth,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageDigitalSignature,
			},
			Duration: &metav1.Duration{
				// 365 days
				Duration: 8796 * time.Hour,
			},
			IssuerRef: cmmeta.ObjectReference{
				Name:  rfclient.Issuer,
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}
	return ctrlCtx.Client.Create(ctx, cr)
}
