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

//go:generate mockgen -copyright_file ../../../../../hack/boilerplate.go.txt -destination mock/Utils.go -source nvme.go

package nvme

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nvidia/doca-platform/internal/storage/csi-plugin/utils/common"
	osWrapperPkg "github.com/nvidia/doca-platform/internal/storage/csi-plugin/wrappers/os"

	"k8s.io/klog/v2"
)

var ErrBlockDeviceNotFound = fmt.Errorf("block device not found")

// Utils is the interface provided by nvme utils package
type Utils interface {
	// GetBlockDeviceNameForNS return block device name for NVME namespace
	// address - PCI device address,
	// namespace - NVME namespace id
	// return ErrBlockDeviceNotFound error if device not found
	GetBlockDeviceNameForNS(address string, namespace int32) (string, error)
}

// New initialize and return a new instance of nvme utils
func New(osWrapper osWrapperPkg.PkgWrapper) Utils {
	return &nvmeUtils{
		os: osWrapper,
	}
}

type nvmeUtils struct {
	os osWrapperPkg.PkgWrapper
}

// GetBlockDeviceNameForNS is an Utils interface implementation for nvmeUtils
func (n *nvmeUtils) GetBlockDeviceNameForNS(deviceID string, namespace int32) (string, error) {
	devName, err := n.getBlockDeviceName(deviceID, namespace)
	if err != nil {
		return "", fmt.Errorf("failed to get block device name for dev %s ns %d: %v", deviceID, namespace, err)
	}
	if devName == "" {
		klog.V(3).InfoS("block device not found", "deviceID", deviceID, "namespace", namespace)
		return "", ErrBlockDeviceNotFound
	}
	klog.V(2).InfoS("found block device name for NVME namespace", "deviceID", deviceID, "namespace", namespace, "block_device", devName)
	return devName, nil
}

func (n *nvmeUtils) getBlockDeviceName(deviceID string, namespace int32) (string, error) {
	// e.g. /sys/bus/pci/drivers/nvme/0000:3b:00.6/nvme
	devDriverPath := filepath.Join(common.SysfsPCIDriverPath, common.NVMEDriver, deviceID, common.NVMEDriver)
	ctrlDirs, err := n.os.ReadDir(devDriverPath)
	if err != nil {
		return "", fmt.Errorf("failed to read device info: %w", err)
	}
	if len(ctrlDirs) == 0 {
		return "", fmt.Errorf("can't find NVME controller dir")
	}
	// e.g. /sys/bus/pci/drivers/nvme/0000:3b:00.6/nvme/nvme3
	// assumption is that each VF will have only one NVME controller
	ctrlDirPath := filepath.Join(devDriverPath, ctrlDirs[0].Name())
	dirs, err := n.os.ReadDir(ctrlDirPath)
	if err != nil {
		return "", err
	}
	for _, d := range dirs {
		// e.g. /sys/bus/pci/drivers/nvme/0000:3b:00.6/nvme/nvme3/nvme2n1/nsid
		nsIDFilePath := filepath.Join(ctrlDirPath, d.Name(), "nsid")
		data, err := n.os.ReadFile(nsIDFilePath)
		if err != nil {
			continue
		}
		nsid, err := strconv.ParseInt(strings.TrimSuffix(string(data), "\n"), 10, 32)
		if err != nil {
			continue
		}
		if int32(nsid) == namespace {
			return d.Name(), nil
		}
	}
	return "", nil
}
