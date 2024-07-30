/*
Copyright 2024 NVIDIA.

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

package ipallocator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types"
	types040 "github.com/containernetworking/cni/pkg/types/040"
)

const sharedResultFile = "/tmp/ips/addresses"
const CNIBinDir = "/opt/cni/bin"

// TODO: Use 1.0.0 after a release is cut that will include https://github.com/containernetworking/cni/pull/1052 so that
// we can parse 1.0.0 cni netConf from bytes.
var netConf = `
{
	"cniVersion": "0.4.0",
	"name": "sidecar",
	"type": "nv-ipam",
	"ipam": {
		"type": "nv-ipam",
		"poolName": "%s",
		"poolType": "%s"
	}
}
`

// IPAllocator uses the CNI spec to allocate an IP from NVIPAM
type NVIPAMIPAllocator struct {
	cninet       *libcni.CNIConfig
	poolName     string
	poolType     string
	podName      string
	podNamespace string
	podUID       string
	// FileSystemRoot controls the file system root. It's used for enabling easier testing of the package. Defaults to
	// empty.
	FileSystemRoot string
}

// New creates a new IPAllocator that can handle allocation of IPs using the NVIPAM.
func New(cninet *libcni.CNIConfig, poolName string, poolType string, podName string, podNamespace string, podUID string) *NVIPAMIPAllocator {
	return &NVIPAMIPAllocator{
		cninet:       cninet,
		poolName:     poolName,
		poolType:     poolType,
		podName:      podName,
		podNamespace: podNamespace,
		podUID:       podUID,
	}
}

// allocate allocates an IP from the NVIPAM
func (a *NVIPAMIPAllocator) Allocate(ctx context.Context) error {
	rt := a.constructRuntimeConf()

	netconf, err := libcni.ConfFromBytes([]byte(fmt.Sprintf(netConf, a.poolName, a.poolType)))
	if err != nil {
		return fmt.Errorf("error while unmarshaling netconf: %w", err)
	}

	res, err := a.cninet.GetNetworkCachedResult(netconf, rt)
	if err != nil {
		return fmt.Errorf("error while getting result from cache: %w", err)
	}

	// CNI already called for this pod
	if res != nil {
		if err := a.populateSharedResultFile(res); err != nil {
			return fmt.Errorf("error while populating file with cache result: %w", err)
		}
		return nil
	}

	res, err = a.cninet.AddNetwork(ctx, netconf, rt)
	if err != nil {
		return fmt.Errorf("error while calling CNI ADD: %w", err)
	}

	if res == nil {
		return errors.New("no result, something went wrong")
	}

	if err := a.populateSharedResultFile(res); err != nil {
		return fmt.Errorf("error while populating file with CNI ADD result: %w", err)
	}
	return nil
}

// Deallocate deallocates the allocated IP from the NVIPAM
func (a *NVIPAMIPAllocator) Deallocate(ctx context.Context) error {
	rt := a.constructRuntimeConf()

	netconf, err := libcni.ConfFromBytes([]byte(fmt.Sprintf(netConf, a.poolName, a.poolType)))
	if err != nil {
		return fmt.Errorf("error while unmarshaling netconf: %w", err)
	}

	res, err := a.cninet.GetNetworkCachedResult(netconf, rt)
	if err != nil {
		return fmt.Errorf("error while getting result from cache: %w", err)
	}

	if res == nil {
		return errors.New("allocation can't be found in cache, we may leak some IP")
	}

	err = a.cninet.DelNetwork(ctx, netconf, rt)
	if err != nil {
		return fmt.Errorf("error while calling CNI DEL: %w", err)
	}

	return nil
}

func (a *NVIPAMIPAllocator) constructRuntimeConf() *libcni.RuntimeConf {
	return &libcni.RuntimeConf{
		// ContainerID is used by NVIPAM to construct the key for the allocation entry in the local store. We don't need
		// a real container id.
		ContainerID: a.podUID,
		// NetNS is used by NVIPAM to construct the key for the allocation entry in the local store. We don't need a
		// a real network namespace.
		NetNS:  a.podUID,
		IfName: "allocator",
		Args: [][2]string{
			{"K8S_POD_NAME", a.podName},
			{"K8S_POD_NAMESPACE", a.podNamespace},
			{"K8S_POD_UID", a.podUID},
		},
	}
}

// populateSharedResultFile writes the IPs to the path which the consumer will read
func (a *NVIPAMIPAllocator) populateSharedResultFile(res types.Result) error {
	res040, ok := res.(*types040.Result)
	if !ok {
		return errors.New("error converting result to 0.4.0 result")
	}

	ips := make([]string, 0, len(res040.IPs))
	for _, ip := range res040.IPs {
		ips = append(ips, ip.Address.String())
	}
	bytes, err := json.Marshal(ips)
	if err != nil {
		return err
	}

	resultFilePath := filepath.Join(a.FileSystemRoot, sharedResultFile)
	err = os.MkdirAll(filepath.Dir(resultFilePath), 0755)
	if err != nil {
		return fmt.Errorf("error while creating dir %s: %w", resultFilePath, err)
	}

	err = os.WriteFile(resultFilePath, bytes, 0644)
	if err != nil {
		return fmt.Errorf("error while writing file %s: %w", resultFilePath, err)
	}

	return nil
}
