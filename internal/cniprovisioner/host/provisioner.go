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

package hostcniprovisioner

import (
	"context"
	"errors"
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
	kexec "k8s.io/utils/exec"
)

const (
	// networkManagerUnmanagedPFConfigurationPath is the path to the file that contains the NetworkManager
	// configuration related to marking the PF as unmanaged.
	networkManagerUnmanagedPFConfigurationPath = "/etc/NetworkManager/conf.d/dpf.conf"
	// ovnDBsPath is the path where the OVN Databases are stored
	ovnDBsPath = "/var/lib/ovn-ic/etc/"
	// ovnCleanUpDoneFile is the name of the file that indicates that OVN Kubernetes cleanup has happened
	ovnCleanUpDoneFile = "dpf-cleanup-done"
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
	exec                      kexec.Interface

	// FileSystemRoot controls the file system root. It's used for enabling easier testing of the package. Defaults to
	// empty.
	FileSystemRoot string

	// pf0 is the name of the pf0. It could be discovered in that component, but since we use this very same value for
	// the DPU CNI Provisioner where the name of that PF can't be discovered, it makes sense to make use of the same source of truth
	// for both components.
	// Note: In case you want to switch that to pf1 in the future, you need to ensure that the ovnManagementVFRep is
	// related to a VF part of pf1 and this VF is configured correctly on the OVN Kubernetes DaemonSet.
	pf0 string
	// pfIPNet is the IP that should be added to the PF interface.
	pfIPNet *net.IPNet
}

// New creates a HostCNIProvisioner that can configure the system
func New(ctx context.Context, clock clock.WithTicker, networkHelper networkhelper.NetworkHelper, exec kexec.Interface, pf0 string, pfIPNet *net.IPNet) *HostCNIProvisioner {
	return &HostCNIProvisioner{
		ctx:                       ctx,
		ensureConfigurationTicker: clock.NewTicker(2 * time.Second),
		networkHelper:             networkHelper,
		exec:                      exec,
		FileSystemRoot:            "",
		pf0:                       pf0,
		pfIPNet:                   pfIPNet,
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
	err = p.configure()
	if err != nil {
		return err
	}
	klog.Info("Configuration complete.")
	return nil
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
			err := p.addDummyLinkIfNotExists(ovnManagementVFRep)
			if err != nil {
				klog.Errorf("failed to ensure dummy link %s: %s", ovnManagementVFRep, err.Error())
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

// configurePF does two things:
//   - forces the NetworkManager to treat the PF as unmanaged (the default is to take it under its management and treat
//     it as externally managed when we add manually an IP)
//   - configures an IP on the PF that is in the same subnet as the VTEP.
//
// This configuration is essential for enabling Pod to External connectivity.
func (p *HostCNIProvisioner) configurePF() error {
	if err := p.configureUnmanagedPF(); err != nil {
		return fmt.Errorf("error while marking the PF as unmanaged for NetworkManager: %w", err)
	}

	if err := p.configurePFIP(); err != nil {
		return err
	}

	return nil
}

// configureUnmanagedPF configures the NetworkManager to treat the PF as unmanaged.
func (p *HostCNIProvisioner) configureUnmanagedPF() error {
	configPath := filepath.Join(p.FileSystemRoot, networkManagerUnmanagedPFConfigurationPath)
	content, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error while reading file %s: %w", configPath, err)
	}

	expectedConfigContent := fmt.Sprintf("[keyfile]\nunmanaged-devices=interface-name:%s", p.pf0)

	// Note that we don't check if the NetworkManager is reloaded if the file already exists and has the content we
	// expect it to have.
	if content != nil && expectedConfigContent == string(content) {
		klog.Info("PF is already configured as unmanaged in NetworkManager, skipping")
		return nil
	}

	if err := os.WriteFile(configPath, []byte(expectedConfigContent), 0644); err != nil {
		return fmt.Errorf("error while writing file %s: %w", configPath, err)
	}

	cmd := p.exec.Command("systemctl", "reload", "NetworkManager")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error while reloading the NetworkManager systemd service: %w", err)
	}

	return nil
}

// configurePFIP configures an IP for the PF.
func (p *HostCNIProvisioner) configurePFIP() error {
	hasAddress, err := p.networkHelper.LinkIPAddressExists(p.pf0, p.pfIPNet)
	if err != nil {
		return fmt.Errorf("error while listing PF IPs: %w", err)
	}
	if hasAddress {
		klog.Info("PF IP already configured, skipping")
		return nil
	}
	err = p.networkHelper.SetLinkIPAddress(p.pf0, p.pfIPNet)
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
	sriovNumVfsPath := filepath.Join(p.FileSystemRoot, fmt.Sprintf("/sys/class/net/%s/device/sriov_numvfs", p.pf0))
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

	m[p.pf0] = numVfs
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
		err = os.MkdirAll(pfPath, 0755)
		if err != nil {
			return fmt.Errorf("error creating fake dir %s: %w", pfPath, err)
		}

		// /var/dpf/sys/class/net/ens2f0np0/phys_port_name
		path := filepath.Join(pfPath, "phys_port_name")
		// TODO: Find port number and parameterize static p0 when introducing support for VFs on both PFs
		err = os.WriteFile(path, []byte("p0"), 0644)
		if err != nil {
			return fmt.Errorf("error creating fake phys_port_name file %s: %w", path, err)
		}

		// /var/dpf/sys/class/net/ens2f0np0/phys_switch_id
		path = filepath.Join(pfPath, "phys_switch_id")
		err = os.WriteFile(path, []byte(physSwitchIDValue), 0644)
		if err != nil {
			return fmt.Errorf("error creating fake phys_switch_id file %s: %w", path, err)
		}

		// /var/dpf/sys/class/net/ens2f0np0/subsystem -> /var/dpf/sys/class/net/
		path = filepath.Join(pfPath, "subsystem")
		err := createSymlinkIfNotExists(fsPath, path)
		if err != nil {
			return fmt.Errorf("error creating subsystem symlink %s->%s: %w", path, fsPath, err)
		}

		for i := 0; i < numVfs; i++ {
			// TODO: Find port number and parameterize static 0 when introducing support for VFs on both PFs
			// /var/dpf/sys/class/net/pf0vf0
			vfRepPath := filepath.Join(fsPath, fmt.Sprintf(vfRepresentorPattern, 0, i))
			err := os.MkdirAll(vfRepPath, 0755)
			if err != nil {
				return fmt.Errorf("error creating fake dir %s: %w", vfRepPath, err)
			}

			// /var/dpf/sys/class/net/pf0vf0/phys_port_name
			path := filepath.Join(vfRepPath, "phys_port_name")
			err = os.WriteFile(path, []byte(fmt.Sprintf(vfRepresentorPortPattern, 0, i)), 0644)
			if err != nil {
				return fmt.Errorf("error creating fake phys_port_name file %s: %w", path, err)
			}

			// /var/dpf/sys/class/net/pf0vf0/phys_switch_id
			path = filepath.Join(vfRepPath, "phys_switch_id")
			err = os.WriteFile(path, []byte(physSwitchIDValue), 0644)
			if err != nil {
				return fmt.Errorf("error creating fake phys_switch_id file %s: %w", path, err)
			}
		}
	}

	return nil
}

// createSymlinkIfNotExists creates a symlink if it doesn't exist
func createSymlinkIfNotExists(path string, symlink string) error {
	_, err := os.Lstat(symlink)
	if err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error when calling lstat on %s: %w", symlink, err)
	}

	err = os.Symlink(path, symlink)
	if err != nil {
		return fmt.Errorf("error creating symlink %s->%s: %w", symlink, path, err)
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
			err := p.addDummyLinkIfNotExists(vfRep)
			if err != nil {
				return fmt.Errorf("error adding dummy link %s: %w", vfRep, err)
			}
		}
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

	err = p.deleteNeighbourIfExists(ovnMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting neighbour %s %s: %w", ovnMasqueradeIP.IP.String(), originalBrEx, err)
	}

	dummyNextHopMasqueradeIP, err := netlink.ParseIPNet(hostToServiceDummyNextHopMasqueradeIP)
	if err != nil {
		return fmt.Errorf("error parsing IPNet %s: %w", hostToServiceDummyNextHopMasqueradeIP, err)
	}

	err = p.deleteNeighbourIfExists(dummyNextHopMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting neighbour %s %s: %w", dummyNextHopMasqueradeIP.IP.String(), originalBrEx, err)
	}

	hostMasqueradeIP, err := netlink.ParseIPNet(hostToServiceHostMasqueradeIP)
	if err != nil {
		return fmt.Errorf("error parsing IPNet %s: %w", hostToServiceHostMasqueradeIP, err)
	}
	err = p.deleteLinkIPAddressIfExists(originalBrEx, hostMasqueradeIP)
	if err != nil {
		return fmt.Errorf("error deleting IP address %s from link IPNet %s: %w", hostMasqueradeIP, originalBrEx, err)
	}

	svcNet, err := netlink.ParseIPNet(kubernetesServiceCIDR)
	if err != nil {
		return fmt.Errorf("error parsing IPNet %s: %w", kubernetesServiceCIDR, err)
	}

	err = p.deleteRouteIfExists(svcNet, dummyNextHopMasqueradeIP.IP, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting route %s %s %s: %w", svcNet, dummyNextHopMasqueradeIP.IP.String(), originalBrEx, err)
	}

	ovnMasqueradeIP.Mask = net.CIDRMask(32, 32)
	err = p.deleteRouteIfExists(ovnMasqueradeIP, nil, originalBrEx)
	if err != nil {
		return fmt.Errorf("error deleting route %s %s: %w", ovnMasqueradeIP, originalBrEx, err)
	}

	// We need to drop the OVN databases so that the custom OVN Kubernetes which will be deployed doesn't recreate
	// OVS flows and patch ports using the naming scheme of the previous run.
	err = p.cleanupOVNDatabases()
	if err != nil {
		return fmt.Errorf("error cleaning up OVN databases: %w", err)
	}

	return nil
}

// cleanupOVNDatabases cleans up the OVN Databases if they are not cleaned up before
func (p *HostCNIProvisioner) cleanupOVNDatabases() error {
	path := filepath.Join(p.FileSystemRoot, ovnDBsPath)

	cleanUpDonePath := filepath.Join(path, ovnCleanUpDoneFile)
	_, err := os.Stat(cleanUpDonePath)
	if err == nil {
		klog.Info("OVN databases already cleaned up, skipping cleanup")
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error when calling stat on %s: %w", cleanUpDonePath, err)
	}

	err = os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("error removing OVN DB path %s: %w", path, err)
	}

	err = os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("error creating new OVN DB path %s: %w", path, err)
	}

	_, err = os.Create(cleanUpDonePath)
	if err != nil {
		return fmt.Errorf("error creating %s file: %w", cleanUpDonePath, err)
	}

	return nil
}

// addDummyLinkIfNotExists ensures that a dummy link is in place
func (p *HostCNIProvisioner) addDummyLinkIfNotExists(link string) error {
	exists, err := p.networkHelper.DummyLinkExists(link)
	if err != nil {
		return fmt.Errorf("error checking whether dummy link exists: %w", err)
	}
	if exists {
		return nil
	}
	err = p.networkHelper.AddDummyLink(link)
	if err != nil {
		return fmt.Errorf("error adding dummy link: %w", err)
	}

	return nil
}

// deleteNeighbourIfExists deletes a neighbour if it exists
func (p *HostCNIProvisioner) deleteNeighbourIfExists(ip net.IP, device string) error {
	hasNeigh, err := p.networkHelper.NeighbourExists(ip, device)
	if err != nil {
		return fmt.Errorf("error checking whether neighbour exists: %w", err)
	}
	if !hasNeigh {
		klog.Infof("Neighbour %s %s doesn't exist, skipping deletion", ip.String(), device)
		return nil
	}
	err = p.networkHelper.DeleteNeighbour(ip, device)
	if err != nil {
		return fmt.Errorf("error deleting neighbour: %w", err)
	}
	return nil
}

// deleteRouteIfExists deletes a route if it exists
func (p *HostCNIProvisioner) deleteRouteIfExists(network *net.IPNet, gateway net.IP, device string) error {
	hasRoute, err := p.networkHelper.RouteExists(network, gateway, device)
	if err != nil {
		return fmt.Errorf("error checking whether route exists: %w", err)
	}
	if !hasRoute {
		klog.Infof("Route %s %s %s doesn't exist, skipping deletion", network, gateway, device)
		return nil
	}
	err = p.networkHelper.DeleteRoute(network, gateway, device)
	if err != nil {
		return fmt.Errorf("error deleting route: %w", err)
	}
	return nil
}

// deleteLinkIPAddresIfExists deletes an IP address from a link if it exists
func (p *HostCNIProvisioner) deleteLinkIPAddressIfExists(link string, ipNet *net.IPNet) error {
	hasRoute, err := p.networkHelper.LinkIPAddressExists(link, ipNet)
	if err != nil {
		return fmt.Errorf("error checking whether IP exists: %w", err)
	}
	if !hasRoute {
		klog.Infof("Link %s doesn't have IP %s, skipping deletion", link, ipNet)
		return nil
	}
	err = p.networkHelper.DeleteLinkIPAddress(link, ipNet)
	if err != nil {
		return fmt.Errorf("error deleting IP address: %w", err)
	}
	return nil
}
