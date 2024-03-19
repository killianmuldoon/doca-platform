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
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper"

	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
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
	// fakeFSPath represents the directory where the fake filesystem is created.
	fakeFSPath = "/var/dpf"

	// vfRepresentorPattern is the naming pattern of the VF representors.
	vfRepresentorPattern = "pf%dvf%d"
	//vfRepresentorPortPattern is the naming pattern of the VF representor part. This can be found in
	// /sys/class/net/<vf_rep>/phys_port_name
	vfRepresentorPortPattern = "c1pf%dvf%d"
	// physSwitchID is the content of /sys/class/net/<vf_rep>/phys_switch_id. In our case, we don't care about the value
	// as long as the value is the same for the PF and VF representors.
	physSwitchIDValue = "custom_value"

	// ovnManagementVFRep is the VF *representor* used by OVN Kubernetes. This assumes that we select that particular VF
	// (name would be something like enp23s0f0v0) for the OVN management in the OVN Kubernetes configuration
	// (i.e. OVNKUBE_NODE_MGMT_PORT_NETDEV env variable).
	ovnManagementVFRep = "pf0vf0"
)

type HostCNIProvisioner struct {
	ctx                       context.Context
	ensureConfigurationTicker clock.Ticker
	networkHelper             networkhelper.NetworkHelper

	// FileSystemRoot controls the file system root. It's used for enabling easier testing of the package. Defaults to
	// empty.
	FileSystemRoot string
}

// New creates a HostCNIProvisioner that can configure the system
func New(ctx context.Context, clock clock.WithTicker, networkHelper networkhelper.NetworkHelper) *HostCNIProvisioner {
	return &HostCNIProvisioner{
		ctx:                       ctx,
		ensureConfigurationTicker: clock.NewTicker(2 * time.Second),
		networkHelper:             networkHelper,
		FileSystemRoot:            "",
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
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.ensureConfigurationTicker.C():
			// The VF used for OVN management is getting renamed when OVN Kubernetes starts. On restart, the link is no
			// longer there and OVN Kubernetes can't start, therefore we need to ensure the link always exists.
			err := p.ensureDummyLink(ovnManagementVFRep)
			if err != nil {
				klog.Error("failed to ensure dummy link: ", err)
			}
		}
	}
}

// isSystemAlreadyConfigured checks if the system is already configured. No thorough checks are done, just a high level
// check to avoid re-running the configuration.
// TODO: Make rest of the calls idempotent and skip such check.
func (p *HostCNIProvisioner) isSystemAlreadyConfigured() (bool, error) { return false, nil }

// configure runs the provisioning flow without checking existing configuration
func (p *HostCNIProvisioner) configure() error {
	klog.Info("Configuring PF")
	err := p.configurePF()
	if err != nil {
		return err
	}

	klog.Info("Configuring Fake Environment")
	err = p.configureFakeEnvironment()
	if err != nil {
		return err
	}

	klog.Info("Removing OVN Kubernetes leftovers")
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
		return fmt.Errorf("error while parsing PF IP: %w", err)
	}

	err = p.networkHelper.SetLinkIPAddress(pf, ipNet)
	if err != nil {
		return fmt.Errorf("error while setting PF IP: %w", err)
	}
	return nil
}

// configureFakeEnvironment configures a fake environment that tricks OVN Kubernetes to think that VF representors are
// on the host.
func (p *HostCNIProvisioner) configureFakeEnvironment() error {
	pfToNumVFs, err := p.discoverVFs()
	if err != nil {
		return err
	}
	err = p.createFakeFS(pfToNumVFs)
	if err != nil {
		return err
	}
	return p.createDummyLinks(pfToNumVFs)
}

// discoverVFs discovers the available VFs on the system and returns a map of PF to number of VFs
// TODO: Discover VFs from multiple PFs
func (p *HostCNIProvisioner) discoverVFs() (map[string]int, error) {
	m := make(map[string]int)
	sriovNumVfsPath := filepath.Join(p.FileSystemRoot, fmt.Sprintf("/sys/class/net/%s/device/sriov_numvfs", pf))
	content, err := os.ReadFile(sriovNumVfsPath)
	if err != nil {
		return nil, fmt.Errorf("error while reading number of VFs: %w", err)
	}

	s := string(content)
	s = strings.TrimSpace(s)
	numVfs, err := strconv.Atoi(s)
	if err != nil {
		return nil, fmt.Errorf("error while converting string to int: %w", err)
	}

	m[pf] = numVfs
	return m, nil
}

// createFakeFS creates a fake filesystem with VF representors to trick OVN Kubernetes functions.
func (p *HostCNIProvisioner) createFakeFS(pfToNumVFs map[string]int) error {
	fsPath := filepath.Join(p.FileSystemRoot, fakeFSPath, "/sys/class/net")
	err := os.MkdirAll(fsPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating fake dir %s: %w", fsPath, err)
	}

	for pf, numVfs := range pfToNumVFs {
		// /var/dpf/sys/class/net/ens2f0np0
		pfPath := filepath.Join(fsPath, pf)
		err = os.Mkdir(pfPath, 0755)
		if err != nil {
			return fmt.Errorf("error creating fake dir %s: %w", pfPath, err)
		}

		// /var/dpf/sys/class/net/ens2f0np0/phys_port_name
		path := filepath.Join(pfPath, "phys_port_name")
		// TODO: Find port number and parameterize static p0 when introducing support for VFs on both PFs
		err = os.WriteFile(path, []byte("p0"), 0444)
		if err != nil {
			return fmt.Errorf("error creating fake phys_port_name file %s: %w", path, err)
		}

		// /var/dpf/sys/class/net/ens2f0np0/phys_switch_id
		path = filepath.Join(pfPath, "phys_switch_id")
		err = os.WriteFile(path, []byte(physSwitchIDValue), 0444)
		if err != nil {
			return fmt.Errorf("error creating fake phys_switch_id file %s: %w", path, err)
		}

		// /var/dpf/sys/class/net/ens2f0np0/subsystem -> /var/dpf/sys/class/net/
		path = filepath.Join(pfPath, "subsystem")
		err = os.Symlink(fsPath, path)
		if err != nil {
			return fmt.Errorf("error creating subsystem symlink %s->%s: %w", path, fsPath, err)
		}

		for i := 0; i < numVfs; i++ {
			// TODO: Find port number and parameterize static 0 when introducing support for VFs on both PFs
			// /var/dpf/sys/class/net/pf0vf0
			vfRepPath := filepath.Join(fsPath, fmt.Sprintf(vfRepresentorPattern, 0, i))
			err := os.Mkdir(vfRepPath, 0755)
			if err != nil {
				return fmt.Errorf("error creating fake dir %s: %w", vfRepPath, err)
			}

			// /var/dpf/sys/class/net/pf0vf0/phys_port_name
			path := filepath.Join(vfRepPath, "phys_port_name")
			err = os.WriteFile(path, []byte(fmt.Sprintf(vfRepresentorPortPattern, 0, i)), 0444)
			if err != nil {
				return fmt.Errorf("error creating fake phys_port_name file %s: %w", path, err)
			}

			// /var/dpf/sys/class/net/pf0vf0/phys_switch_id
			path = filepath.Join(vfRepPath, "phys_switch_id")
			err = os.WriteFile(path, []byte(physSwitchIDValue), 0444)
			if err != nil {
				return fmt.Errorf("error creating fake phys_switch_id file %s: %w", path, err)
			}
		}
	}

	return nil
}

// createDummyLinks creates dummy links for the VF representors (that exist in the DPU but not on the host) which are
// used to trick OVN Kubernetes functions.
func (p *HostCNIProvisioner) createDummyLinks(pfToNumVFs map[string]int) error {
	for _, numVfs := range pfToNumVFs {
		for i := 0; i < numVfs; i++ {
			// TODO: Find port number and parameterize static 0 when introducing support for VFs on both PFs
			vfRep := fmt.Sprintf(vfRepresentorPattern, 0, i)
			err := p.networkHelper.AddDummyLink(vfRep)
			if err != nil {
				return fmt.Errorf("error adding dummy link %s: %w", vfRep, err)
			}
		}
	}
	return nil
}

// ensureDummyLink ensures that a dummy link is in place
func (p *HostCNIProvisioner) ensureDummyLink(link string) error {
	exists, err := p.networkHelper.DummyLinkExists(link)
	if err != nil {
		return fmt.Errorf("error checking for dummy link existence with name %s: %w", link, err)
	}

	if exists {
		return nil
	}

	err = p.networkHelper.AddDummyLink(link)
	if err != nil {
		return fmt.Errorf("error adding dummy link %s: %w", link, err)
	}

	return nil
}

// removeOVNKubernetesLeftovers removes OVN Kubernetes leftovers that interfere with the custom OVN we deploy. This is
// needed so that the Host to Service connectivity works but also to avoid creating loops in OVS from patch ports created
// using the original OVS bridge names (br-int, br-ex etc).
func (p *HostCNIProvisioner) removeOVNKubernetesLeftovers() error {
	ovnMasqueradeIP, err := netlink.ParseIPNet(hostToServiceOVNMasqueradeIP)
	if err != nil {
		return fmt.Errorf("error parsing IPNet %s: %w", hostToServiceHostMasqueradeIP, err)
	}

	err = p.networkHelper.DeleteNeighbour(ovnMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting neighbour %s %s: %w", ovnMasqueradeIP.IP.String(), originalBrEx, err)
	}

	dummyNextHopMasqueradeIP, err := netlink.ParseIPNet(hostToServiceDummyNextHopMasqueradeIP)
	if err != nil {
		return fmt.Errorf("error parsing IPNet %s: %w", hostToServiceDummyNextHopMasqueradeIP, err)
	}

	err = p.networkHelper.DeleteNeighbour(dummyNextHopMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting neighbour %s %s: %w", dummyNextHopMasqueradeIP.IP.String(), originalBrEx, err)
	}

	hostMasqueradeIP, err := netlink.ParseIPNet(hostToServiceHostMasqueradeIP)
	if err != nil {
		return fmt.Errorf("error parsing IPNet %s: %w", hostToServiceHostMasqueradeIP, err)
	}
	err = p.networkHelper.DeleteLinkIPAddress(originalBrEx, hostMasqueradeIP)
	if err != nil {
		return fmt.Errorf("error deleting IP address %s from link IPNet %s: %w", hostMasqueradeIP, originalBrEx, err)
	}

	svcNet, err := netlink.ParseIPNet(kubernetesServiceCIDR)
	if err != nil {
		return fmt.Errorf("error parsing IPNet %s: %w", kubernetesServiceCIDR, err)
	}

	err = p.networkHelper.DeleteRoute(svcNet, dummyNextHopMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting route %s %s %s: %w", svcNet, dummyNextHopMasqueradeIP.IP.String(), originalBrEx, err)
	}

	ovnMasqueradeIP.Mask = net.CIDRMask(32, 32)
	err = p.networkHelper.DeleteRoute(ovnMasqueradeIP, nil, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting route %s %s: %w", svcNet, originalBrEx, err)
	}

	// We need to drop the OVN databases so that the custom OVN Kubernetes which will be deployed doesn't recreate
	// OVS flows and patch ports using the naming scheme of the previous run.
	path := filepath.Join(p.FileSystemRoot, ovnDBsPath)
	err = os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("error removing OVN DB path %s: %w", path, err)
	}

	err = os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("error creating new OVN DB path %s: %w", path, err)
	}

	return nil
}
