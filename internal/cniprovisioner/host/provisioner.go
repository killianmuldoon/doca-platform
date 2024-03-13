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

package hostcniprovisioner

import "k8s.io/klog/v2"

type HostCNIProvisioner struct {
	// FileSystemRoot controls the file system root. It's used for enabling easier testing of the package. Defaults to
	// empty.
	FileSystemRoot string
}

// New creates a HostCNIProvisioner that can configure the system
func New() *HostCNIProvisioner {
	return &HostCNIProvisioner{
		FileSystemRoot: "",
	}
}

// RunOnce runs the provisioning flow once and exits
func (p *HostCNIProvisioner) RunOnce() error {
	isConfigured, err := p.isSystemAlreadyConfigured()
	if err != nil {
		return err
	}
	if isConfigured {
		return nil
	}
	return p.configure()
}

// EnsureConfiguration ensures that particular configuration is in place. This is a blocking function.
func (p *HostCNIProvisioner) EnsureConfiguration() {
	// TODO: Use ticker + context
	for {
		// The VF used for OVN management is getting renamed when OVN Kubernetes starts. On restart, the device is no
		// longer there and OVN Kubernetes can't start, therefore we need to ensure the device always exists.
		err := p.ensureDummyNetDevice()
		if err != nil {
			klog.Error("failed to ensure dummy net device: %w", err)
		}
	}
}

// isSystemAlreadyConfigured checks if the system is already configured. No thorough checks are done, just a high level
// check to avoid re-running the configuration.
// TODO: Make rest of the calls idempotent and skip such check.
func (p *HostCNIProvisioner) isSystemAlreadyConfigured() (bool, error) { return false, nil }

// configure runs the provisioning flow without checking existing configuration
func (p *HostCNIProvisioner) configure() error {
	err := p.configurePF()
	if err != nil {
		return err
	}

	err = p.configureFakeEnvironment()
	if err != nil {
		return err
	}

	err = p.removeOVNKubernetesLeftovers()
	if err != nil {
		return err
	}

	return nil
}

// configurePF configures an IP on the PF that is in the same subnet as the VTEP. This is essential for enabling Pod
// to External connectivity.
func (p *HostCNIProvisioner) configurePF() error { return nil }

// configureFakeEnvironment configures a fake environment that tricks OVN Kubernetes to think that VF representors are on the host
func (p *HostCNIProvisioner) configureFakeEnvironment() error {
	err := p.createFakeFS()
	if err != nil {
		return err
	}
	return p.createDummyNetDevices()
}

// createFakeFS creates a fake filesystem with VF representors to trick OVN Kubernetes functions.
func (p *HostCNIProvisioner) createFakeFS() error { return nil }

// createFakeFS creates dummy devices that represent the VF representors and are used to trick OVN Kubernetes functions.
func (p *HostCNIProvisioner) createDummyNetDevices() error { return nil }

// ensureDummyNetDevice ensures that a dummy net device is in place
func (p *HostCNIProvisioner) ensureDummyNetDevice() error { return nil }

// removeOVNKubernetesLeftovers removes OVN Kubernetes leftovers that interfere with the custom OVN we deploy. This is
// needed so that the Host to Service connectivity works but also to avoid creating loops in OVS from patch ports created
// using the original OVS bridge names (br-int, br-ex etc).
func (p *HostCNIProvisioner) removeOVNKubernetesLeftovers() error { return nil }
