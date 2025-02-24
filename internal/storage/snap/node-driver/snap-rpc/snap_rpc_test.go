/*
COPYRIGHT 2025 NVIDIA

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package rpcclient

import (
	"fmt"
	"testing"
)

// Global Mock Emulation Function List Response
var mockEmulationFunctionList = EmulationFunctionListResponse{
	{
		Hotplugged:    false,
		EmulationType: NVMeProtocol,
		PFIndex:       0,
		PCIBDF:        "26:00.2",
		VHCAID:        2,
		VUID:          "MT2328XZ17DFNVMES0D0F2",
		VFs: []VF{
			{
				Hotplugged:    false,
				EmulationType: NVMeProtocol,
				PFIndex:       0,
				VFIndex:       0,
				PCIBDF:        "26:0c.0",
				VHCAID:        98,
				VUID:          "MT2328XZ17DFNVMES0D0F2VF1",
				CtrlID:        "NVMeCtrl2",
			},
			{
				Hotplugged:    false,
				EmulationType: NVMeProtocol,
				PFIndex:       0,
				VFIndex:       1,
				PCIBDF:        "26:0c.1",
				VHCAID:        99,
				VUID:          "MT2328XZ17DFNVMES0D0F2VF2",
			},
			{
				Hotplugged:    false,
				EmulationType: NVMeProtocol,
				PFIndex:       0,
				VFIndex:       2,
				PCIBDF:        "26:0c.2",
				VHCAID:        100,
				VUID:          "MT2328XZ17DFNVMES0D0F2VF3",
			},
		},
	},
}

// Global Mock NVMe Subsystem List Response
var mockNvmeSubsystemList = NvmeSubsystemListResponse{
	{
		NQN:  "nqn.2022-10.io.nvda.nvme:01",
		MN:   "BlueField NVMe SNAP Controller",
		SN:   "MNC12",
		MNAN: 1024,
		NN:   1024,
		Controllers: []interface{}{
			map[string]interface{}{
				"ctrl_id":    "NVMeCtrl2",
				"mdts":       7,
				"vhca_id":    98,
				"nqn":        "nqn.2022-10.io.nvda.nvme:01",
				"plugged":    true,
				"state":      "STARTED",
				"num_queues": 16,
				"max_queues": 32,
				"namespaces": []interface{}{
					map[string]interface{}{
						"nsid": 1,
						"bdev": "null1",
						"uuid": "263826ad-19a3-4feb-bc25-4bc81ee7748e",
					},
				},
			},
		},
		Namespaces: []Namespace{
			{
				NSID:                  1,
				Bdev:                  "null1",
				Ready:                 "No",
				NQN:                   "nqn.2022-10.io.nvda.nvme:01",
				UUID:                  "263826ad-19a3-4feb-bc25-4bc81ee7748e",
				MaxInflightsPerWeight: 65535,
				Controllers: []interface{}{
					map[string]interface{}{
						"ctrl_id": "NVMeCtrl2",
					},
				},
			},
		},
	},
	{
		NQN:  "nqn.2022-10.io.nvda.nvme:0",
		MN:   "BlueField NVMe SNAP Controller",
		SN:   "MNC12",
		MNAN: 1024,
		NN:   1024,
		Controllers: []interface{}{
			map[string]interface{}{
				"ctrl_id":    "NVMeCtrl1",
				"mdts":       7,
				"vhca_id":    2,
				"nqn":        "nqn.2022-10.io.nvda.nvme:0",
				"plugged":    true,
				"state":      "STARTED",
				"num_queues": 16,
				"max_queues": 32,
				"namespaces": []interface{}{
					map[string]interface{}{
						"nsid": 1,
						"bdev": "null0",
						"uuid": "263826ad-19a3-4feb-bc25-4bc81ee7749e",
					},
				},
			},
		},
		Namespaces: []Namespace{
			{
				NSID:                  1,
				Bdev:                  "null0",
				Ready:                 "No",
				NQN:                   "nqn.2022-10.io.nvda.nvme:0",
				UUID:                  "263826ad-19a3-4feb-bc25-4bc81ee7749e",
				MaxInflightsPerWeight: 65535,
				Controllers: []interface{}{
					map[string]interface{}{
						"ctrl_id": "NVMeCtrl1",
					},
				},
			},
		},
	},
}

func TestGetNvmeControllerByPciAddr(t *testing.T) {
	tests := []struct {
		name               string
		pciAddr            string
		expectedController string
		expectError        bool
	}{
		{
			name:               "Valid PCI BDF - Should return NVMeCtrl2",
			pciAddr:            "26:0c.0",
			expectedController: "NVMeCtrl2",
			expectError:        false,
		},
		{
			name:               "Invalid PCI BDF - Should return error",
			pciAddr:            "26:0c.3",
			expectedController: "",
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrlID, err := getNvmeControllerByPciAddr(tt.pciAddr, mockEmulationFunctionList)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got nil for PCI BDF %s", tt.pciAddr)
				}
				expectedErrMsg := fmt.Sprintf("no NVMe controller found for PCI BDF %s", tt.pciAddr)
				if err.Error() != expectedErrMsg {
					t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if ctrlID != tt.expectedController {
					t.Errorf("Expected controller '%s', got '%s'", tt.expectedController, ctrlID)
				}
			}
		})
	}
}

func TestGetPciAddrByCtrlID(t *testing.T) {
	tests := []struct {
		name        string
		ctrlID      string
		expectedBDF string
		expectError bool
	}{
		{
			name:        "Valid Controller ID - Should return PCI BDF 26:0c.0",
			ctrlID:      "NVMeCtrl2",
			expectedBDF: "26:0c.0",
			expectError: false,
		},
		{
			name:        "Invalid Controller ID - Should return error",
			ctrlID:      "NVMeCtrlX",
			expectedBDF: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bdf, err := getPciAddrByCtrlID(tt.ctrlID, mockEmulationFunctionList)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got nil for controller ID %s", tt.ctrlID)
				}
				expectedErrMsg := fmt.Sprintf("no PCI address found for NVMe controller ID %s", tt.ctrlID)
				if err.Error() != expectedErrMsg {
					t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if bdf != tt.expectedBDF {
					t.Errorf("Expected PCI BDF '%s', got '%s'", tt.expectedBDF, bdf)
				}
			}
		})
	}
}

func TestGetNamespaceByDeviceName(t *testing.T) {
	tests := []struct {
		name         string
		deviceName   string
		expectedNSID int
	}{
		{
			name:         "Valid Device Name - Should return NSID 1",
			deviceName:   "null1",
			expectedNSID: 1,
		},
		{
			name:         "Invalid Device Name - Should return -1",
			deviceName:   "non-existent-device",
			expectedNSID: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsid := getNamespaceByDeviceName(tt.deviceName, mockNvmeSubsystemList)

			if nsid != tt.expectedNSID {
				t.Errorf("Test failed: Expected NSID %d, but got %d", tt.expectedNSID, nsid)
			}
		})
	}
}

func TestCheckNamespaceExist(t *testing.T) {
	tests := []struct {
		name     string
		nsid     int
		expected bool
	}{
		{
			name:     "Valid NSID - Should return NSID 1",
			nsid:     1,
			expected: true,
		},
		{
			name:     "Invalid NSID - Should return -1",
			nsid:     999,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsidExist := checkNamespaceExist(tt.nsid, mockNvmeSubsystemList)

			if nsidExist != tt.expected {
				t.Errorf("Test failed: Expected %v, but got %v", tt.expected, nsidExist)
			}
		})
	}
}

func TestGetCtrlByDeviceName(t *testing.T) {
	tests := []struct {
		name           string
		deviceName     string
		expectedCtrlID string
	}{
		{
			name:           "Valid Device Name - Should return NVMeCtrl2",
			deviceName:     "null1",
			expectedCtrlID: "NVMeCtrl2",
		},
		{
			name:           "Invalid Device Name - Should return empty string",
			deviceName:     "non-existent-device",
			expectedCtrlID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrlID := getCtrlByDeviceName(tt.deviceName, mockNvmeSubsystemList)

			if ctrlID != tt.expectedCtrlID {
				t.Errorf("Test failed: Expected Controller ID %s, but got %s", tt.expectedCtrlID, ctrlID)
			}
		})
	}
}
