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

package storagevendordpuplugin

import (
	"testing"
)

// Mock BdevGetBdevsResponse
var mockBdevResponse = BdevGetBdevsResponse{
	Bdevs: []Bdev{
		{
			Name: "nvme_pv-namen1",
			DriverSpecific: map[string]interface{}{
				"nvme": []interface{}{
					map[string]interface{}{
						"trid": map[string]interface{}{
							"trtype":  "RDMA",
							"adrfam":  "IPv4",
							"traddr":  "192.168.10.62",
							"trsvcid": "4420",
							"subnqn":  "nqn.2016-06.io.nvmet:swx-mtvr-stor02",
						},
					},
				},
			},
		},
	},
}

// Mock BdevNvmeGetControllersResponse
var mockControllersResponse = BdevNvmeGetControllersResponse{
	Controllers: []NvmeController{
		{
			Name: "nvme_pv-name",
			Ctrlrs: []struct {
				Trid NVMeTrid `json:"trid"`
			}{
				{
					Trid: NVMeTrid{
						TrType:  "RDMA",
						AdrFam:  "IPv4",
						TrAddr:  "192.168.10.62",
						TrSvcID: "4420",
						SubNQN:  "nqn.2016-06.io.nvmet:swx-mtvr-stor02",
					},
				},
			},
		},
	},
}

func TestCheckBdevExistsByTrid(t *testing.T) {
	tests := []struct {
		name         string
		req          BdevNvmeAttachControllerRequest
		expectedBdev string
		expectError  bool
	}{
		{
			name: "Valid Trid - Should return existing Bdev",
			req: BdevNvmeAttachControllerRequest{
				Trtype:  "RDMA",
				Adrfam:  "IPv4",
				Traddr:  "192.168.10.62",
				Trsvcid: "4420",
				Subnqn:  "nqn.2016-06.io.nvmet:swx-mtvr-stor02",
			},
			expectedBdev: "nvme_pv-namen1",
			expectError:  false,
		},
		{
			name: "Non-existent Trid - Should return empty string",
			req: BdevNvmeAttachControllerRequest{
				Trtype:  "RDMA",
				Adrfam:  "IPv4",
				Traddr:  "192.168.10.99",
				Trsvcid: "4420",
				Subnqn:  "nqn.2016-06.io.nvmet:invalid",
			},
			expectedBdev: "",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bdevName, _ := CheckBdevExistsByTrid(tt.req, mockBdevResponse)

			if bdevName != tt.expectedBdev {
				t.Errorf("Expected bdevName: %q, got: %q", tt.expectedBdev, bdevName)
			}
		})
	}
}

func TestCheckBdevExistsByBdev(t *testing.T) {
	tests := []struct {
		name        string
		deviceName  string
		expectExist bool
	}{
		{
			name:        "Bdev Exists",
			deviceName:  "nvme_pv-namen1",
			expectExist: true,
		},
		{
			name:        "Bdev Does Not Exist",
			deviceName:  "non_existent_bdev",
			expectExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := CheckBdevExistsByBdev(tt.deviceName, mockBdevResponse)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if exists != tt.expectExist {
				t.Errorf("Expected exists: %v, got: %v", tt.expectExist, exists)
			}
		})
	}
}

func TestGetTridByBdev(t *testing.T) {
	tests := []struct {
		name         string
		bdevName     string
		expectedTrid NVMeTrid
		expectError  bool
	}{
		{
			name:     "Valid Bdev",
			bdevName: "nvme_pv-namen1",
			expectedTrid: NVMeTrid{
				TrType:  "RDMA",
				AdrFam:  "IPv4",
				TrAddr:  "192.168.10.62",
				TrSvcID: "4420",
				SubNQN:  "nqn.2016-06.io.nvmet:swx-mtvr-stor02",
			},
			expectError: false,
		},
		{
			name:         "Bdev Not Found",
			bdevName:     "non_existent_bdev",
			expectedTrid: NVMeTrid{},
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trid, err := getTridByBdev(tt.bdevName, mockBdevResponse)

			if tt.expectError && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tt.expectError && trid != tt.expectedTrid {
				t.Errorf("Expected trid: %+v, got: %+v", tt.expectedTrid, trid)
			}
		})
	}
}

func TestGetControllerByTrid(t *testing.T) {
	tests := []struct {
		name         string
		targetTrid   NVMeTrid
		expectedName string
		expectError  bool
	}{
		{
			name: "Valid Trid - Should return controller name",
			targetTrid: NVMeTrid{
				TrType:  "RDMA",
				AdrFam:  "IPv4",
				TrAddr:  "192.168.10.62",
				TrSvcID: "4420",
				SubNQN:  "nqn.2016-06.io.nvmet:swx-mtvr-stor02",
			},
			expectedName: "nvme_pv-name",
			expectError:  false,
		},
		{
			name: "Invalid Trid - Should return error",
			targetTrid: NVMeTrid{
				TrType:  "RDMA",
				AdrFam:  "IPv4",
				TrAddr:  "192.168.10.99",
				TrSvcID: "4420",
				SubNQN:  "nqn.2016-06.io.nvmet:invalid",
			},
			expectedName: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerName, err := getControllerByTrid(tt.targetTrid, mockControllersResponse)

			if tt.expectError && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tt.expectError && controllerName != tt.expectedName {
				t.Errorf("Expected controllerName: %q, got: %q", tt.expectedName, controllerName)
			}
		})
	}
}
