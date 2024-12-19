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

//go:generate mockgen -copyright_file ../../../../../hack/boilerplate.go.txt -destination mock/Utils.go -source pci.go

package pci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/nvidia/doca-platform/internal/storage/csi-plugin/utils/common"
	osWrapperPkg "github.com/nvidia/doca-platform/internal/storage/csi-plugin/wrappers/os"

	"k8s.io/klog/v2"
)

const (
	// this path is used to check driver for device
	driverPath = "driver"
	// file path to trigger bind to the driver
	driverBindPath = "bind"
)

// regexp to validate PCI address, expected form is 0000:3b:00.5
// see https://github.com/jaypipes/ghw/pull/373 for details
var pciAddressRegexp = regexp.MustCompile(
	`^((1?[0-9a-f]{0,4}):)?([0-9a-f]{2}):([0-9a-f]{2})\.([0-9a-f]{1})$`,
)

// New initialize and return a new instance of pci utils
func New(osWrapper osWrapperPkg.PkgWrapper) Utils {
	return &pciUtils{
		os: osWrapper,
	}
}

// ParsePCIAddress check that PCI address format is in correct format.
// add domain if the address doesn't include it
// address should be in [DOMAIN]BUS:DEVICE.FUNCTION format (0000:3b:00.2)
func ParsePCIAddress(address string) (string, error) {
	match := pciAddressRegexp.FindStringSubmatch(address)
	if len(match) == 0 {
		return "", fmt.Errorf("invaid PCI address format, expected format is BUS:DEVICE.FUNCTION or DOMAIN:BUS:DEVICE.FUNCTION ")
	}
	switch match[1] {
	case ":":
		return "0000" + address, nil
	case "":
		return "0000:" + address, nil
	}
	return address, nil
}

// DeviceInfo holds information about PCI device
type DeviceInfo struct {
	Address  string
	VendorID string
	DeviceID string
	Driver   string
}

// ErrNotFound error indicates that device with specified address not found
var ErrNotFound = errors.New("device not found")

// Utils is the interface provided by pci utils package
type Utils interface {
	// LoadDriver tries to load driver for provided device, do nothing if device is already has required driver
	LoadDriver(dev, driver string) error
}

type pciUtils struct {
	// wrappers
	os osWrapperPkg.PkgWrapper
}

// LoadDriver is an Utils interface implementation for pciUtils
func (u *pciUtils) LoadDriver(dev, driver string) error {
	curDriver, err := u.getDeviceDriver(dev)
	if err != nil {
		return fmt.Errorf("failed to read driver for device: %v", err)
	}
	if curDriver != "" {
		if curDriver != driver {
			return fmt.Errorf("device is already binded to a different driver: %s", curDriver)
		}
		klog.V(2).InfoS("device already bound to required driver", "device", dev, "driver", driver)
		return nil
	}
	// e.g. /sys/bus/pci/drivers/nvme/bind
	bindPath := filepath.Join(common.SysfsPCIDriverPath, driver, driverBindPath)
	if _, err := u.os.Stat(bindPath); err != nil {
		return fmt.Errorf("failed to check bind path for the driver: %v", err)
	}
	klog.V(2).InfoS("try to bind device to the driver", "device", dev, "driver", driver)
	if err := u.os.WriteFile(bindPath, []byte(dev), 0200); err != nil {
		return fmt.Errorf("failed to bind device to the driver: %v", err)
	}
	klog.V(2).InfoS("device bind succeed", "device", dev, "driver", driver)
	return nil
}

func (u *pciUtils) getDeviceDriver(address string) (string, error) {
	return u.readSysFSLink(address, driverPath)
}

func (u *pciUtils) readSysFSLink(address, path string) (string, error) {
	_, err := u.os.Stat(sysFSDevPath(address, path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	link, err := u.os.Readlink(sysFSDevPath(address, path))
	if err != nil {
		return "", err
	}
	return filepath.Base(link), nil
}

func sysFSDevPath(paths ...string) string {
	return filepath.Join(common.SysfsPCIDevsPath, filepath.Join(paths...))
}
