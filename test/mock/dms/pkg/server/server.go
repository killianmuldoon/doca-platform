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

package server

import (
	"crypto/rsa"
	"crypto/x509"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
)

type DMSServerMux struct{}

type DMSListener struct {
}

// AddDMSListener adds a new fake DMS listener to the DMSServerMux.
func (d *DMSServerMux) AddDMSListener(dpu *provisioningv1.DPU, address string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	return nil
}
