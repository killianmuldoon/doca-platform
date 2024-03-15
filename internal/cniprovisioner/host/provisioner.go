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

import (
	"net"
	"os"
	"path/filepath"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper"

	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
)

const (
	// pf is the name of the PF on the host.
	// TODO: Discover that instead of hardcoding it
	pf = "ens2f0np0"

	// ovnDBsPath is the path where the OVN Databases are stored
	ovnDBsPath = "/var/lib/ovn-ic/etc/"
	//originalBrEx is the name of the original br-ex created by the upstream OVN Kubernetes
	originalBrEx = "br-ex"

	// These IPs are the default IPs used by OVN Kubernetes to implement Host to Service connectivity. See:
	// * https://github.com/openshift/ovn-kubernetes/blob/release-4.14/docs/design/host_to_services_OpenFlow.md
	// * https://github.com/openshift/ovn-kubernetes/blob/release-4.14/go-controller/pkg/config/config.go#L142-L149
	hostToServiceOVNMasqueradeIP          = "169.254.169.1/29"
	hostToServiceHostMasqueradeIP         = "169.254.169.2/29"
	hostToServiceDummyNextHopMasqueradeIP = "169.254.169.4/29"

	// kubernetesServiceCIDR represents the Kubernetes Service CIDR.
	kubernetesServiceCIDR = "172.30.0.0/16"
)

type HostCNIProvisioner struct {
	networkHelper networkhelper.NetworkHelper

	// FileSystemRoot controls the file system root. It's used for enabling easier testing of the package. Defaults to
	// empty.
	FileSystemRoot string
}

// New creates a HostCNIProvisioner that can configure the system
func New(networkHelper networkhelper.NetworkHelper) *HostCNIProvisioner {
	return &HostCNIProvisioner{
		networkHelper:  networkHelper,
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
func (p *HostCNIProvisioner) configurePF() error {
	// TODO: Still undecided on how we get that IP. Adjust as needed after decision is made.
	ipNet, err := netlink.ParseIPNet("192.168.1.2/24")
	if err != nil {
		return err
	}

	return p.networkHelper.SetLinkIPAddress(pf, ipNet)
}

// configureFakeEnvironment configures a fake environment that tricks OVN Kubernetes to think that VF representors are
// on the host.
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
func (p *HostCNIProvisioner) removeOVNKubernetesLeftovers() error {
	ovnMasqueradeIP, err := netlink.ParseIPNet(hostToServiceOVNMasqueradeIP)
	if err != nil {
		return err
	}

	err = p.networkHelper.DeleteNeighbour(ovnMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return err
	}

	dummyNextHopMasqueradeIP, err := netlink.ParseIPNet(hostToServiceDummyNextHopMasqueradeIP)
	if err != nil {
		return err
	}

	err = p.networkHelper.DeleteNeighbour(dummyNextHopMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return err
	}

	hostMasqueradeIP, err := netlink.ParseIPNet(hostToServiceHostMasqueradeIP)
	if err != nil {
		return err
	}
	err = p.networkHelper.DeleteLinkIPAddress(originalBrEx, hostMasqueradeIP)
	if err != nil {
		return err
	}

	svcNet, err := netlink.ParseIPNet(kubernetesServiceCIDR)
	if err != nil {
		return err
	}

	err = p.networkHelper.DeleteRoute(svcNet, dummyNextHopMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return err
	}

	ovnMasqueradeIP.Mask = net.CIDRMask(32, 32)
	err = p.networkHelper.DeleteRoute(ovnMasqueradeIP, nil, originalBrEx)
	if err != nil {
		return err
	}

	// We need to drop the OVN databases so that the custom OVN Kubernetes which will be deployed doesn't recreate
	// OVS flows and patch ports using the naming scheme of the previous run.
	path := filepath.Join(p.FileSystemRoot, ovnDBsPath)
	err = os.RemoveAll(path)
	if err != nil {
		return err
	}

	err = os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	return nil
}
