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

package rpcclient

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

/* TODOs:
 * 1. use klog
 * 2. Ensure that the RPC client is thread-safe and maintains connections.
 */

const NVMeProtocol = "NVME"

// JSONRPCSnapClient represents the client for JSON-RPC communication
type JSONRPCSnapClient struct {
	Sock      net.Conn
	requestID int
	timeout   time.Duration
}

// Namespace represents a single namespace entry
type Namespace struct {
	Bdev                  string        `json:"bdev"`
	Controllers           []interface{} `json:"controllers"`
	MaxInflightsPerWeight int           `json:"max inflights per weight"`
	NQN                   string        `json:"nqn"`
	NSID                  int           `json:"nsid"`
	Ready                 string        `json:"ready"`
	UUID                  string        `json:"uuid"`
}

// Subsystem represents a single subsystem entry
type Subsystem struct {
	Controllers []interface{} `json:"controllers"`
	MN          string        `json:"mn"`
	MNAN        int           `json:"mnan"`
	Namespaces  []Namespace   `json:"namespaces"`
	NN          int           `json:"nn"`
	NQN         string        `json:"nqn"`
	SN          string        `json:"sn"`
}

// NvmeSubsystemListResponse represents the entire response which is a list of subsystems
type NvmeSubsystemListResponse []Subsystem

// VF represents a single virtual-function entry
type VF struct {
	Hotplugged    bool   `json:"hotplugged"`
	EmulationType string `json:"emulation_type"`
	PFIndex       int    `json:"pf_index"`
	VFIndex       int    `json:"vf_index"`
	PCIBDF        string `json:"pci_bdf"`
	VHCAID        int    `json:"vhca_id"`
	VUID          string `json:"vuid"`
	CtrlID        string `json:"ctrl_id,omitempty"`
}

// EmulationFunction represents the structure of each emulation function in the response
type EmulationFunction struct {
	Hotplugged    bool   `json:"hotplugged"`
	EmulationType string `json:"emulation_type"`
	PFIndex       int    `json:"pf_index"`
	PCIBDF        string `json:"pci_bdf"`
	VHCAID        int    `json:"vhca_id"`
	VUID          string `json:"vuid"`
	VFs           []VF   `json:"vfs"`
}

// EmulationFunctionListResponse represents the response, which is a list of emulation functions
type EmulationFunctionListResponse []EmulationFunction

// NewJSONRPCSnapClient initializes a new JSON-RPC Snap client
func NewJSONRPCSnapClient(sockPath string, timeout time.Duration) (*JSONRPCSnapClient, error) {
	client := &JSONRPCSnapClient{
		timeout: timeout,
	}

	if err := client.connect(sockPath); err != nil {
		return nil, err
	}

	return client, nil
}

// Close closes the underlying connection of the JSONRPCSnapClient.
func (client *JSONRPCSnapClient) Close() error {
	if client.Sock != nil {
		err := client.Sock.Close()
		client.Sock = nil
		return err
	}
	return nil
}

// connect establishes a connection to the server (TCP or Unix socket)
func (client *JSONRPCSnapClient) connect(uri string) error {
	var err error

	if uri[0] == '/' || uri[:5] == "unix:" {
		path := uri
		if uri[:5] == "unix:" {
			path = uri[5:]
		}
		client.Sock, err = net.Dial("unix", path)
	} else if uri[:6] == "tcp://" {
		client.Sock, err = net.DialTimeout("tcp", uri[6:], client.timeout)
	} else {
		return errors.New("unsupported socket address")
	}

	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("error while connecting to %s: %v", uri, err)
	}

	return nil
}

// Send sends a JSON-RPC request with the given method and parameters
func (client *JSONRPCSnapClient) Send(method string, params map[string]interface{}) (int, error) {
	client.requestID++
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      client.requestID,
	}
	if params != nil {
		req["params"] = params
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		fmt.Println(err)
		return 0, fmt.Errorf("failed to marshal request: %v", err)
	}

	_, err = client.Sock.Write(reqBytes)
	if err != nil {
		fmt.Println(err)
		return 0, fmt.Errorf("failed to send request: %v", err)
	}

	return client.requestID, nil
}

// Recv reads a JSON-RPC response from the socket, decodes it, and returns the result
func (client *JSONRPCSnapClient) Recv() (map[string]interface{}, error) {
	var response map[string]interface{}

	reader := bufio.NewReader(client.Sock)
	decoder := json.NewDecoder(reader)

	if err := decoder.Decode(&response); err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if errField, ok := response["error"]; ok {
		fmt.Println("RPC error:", errField)
		return nil, fmt.Errorf("RPC error: %v", errField)
	}

	return response, nil
}

// Call combines Send and Recv for a single RPC call
func (client *JSONRPCSnapClient) Call(method string, params map[string]interface{}) (interface{}, error) {
	_, err := client.Send(method, params)

	if err != nil {
		return nil, err
	}

	response, err := client.Recv()

	if err != nil {
		return nil, err
	}

	return response["result"], nil
}

// NvmeSubsystemList retrieves the list of NVMe subsystems
func NvmeSubsystemList(client *JSONRPCSnapClient) (NvmeSubsystemListResponse, error) {
	result, err := client.Call("nvme_subsystem_list", nil)

	if err != nil {
		return nil, err
	}

	if result == nil {
		fmt.Println("received empty response from nvme_subsystem_list RPC call")
		return nil, fmt.Errorf("received empty response from RPC call")
	}

	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("failed to marshal response for debugging: %v", err)
	}

	var subsystems NvmeSubsystemListResponse
	if err := json.Unmarshal(resultBytes, &subsystems); err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return subsystems, nil
}

// EmulationFunctionList retrieves and prints the list of emulation functions
func EmulationFunctionList(client *JSONRPCSnapClient) (EmulationFunctionListResponse, error) {
	params := map[string]interface{}{
		"all": true,
	}

	result, err := client.Call("emulation_function_list", params)
	if err != nil {
		return nil, err
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")

	var emulationFunctions EmulationFunctionListResponse
	if err := json.Unmarshal(resultBytes, &emulationFunctions); err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return emulationFunctions, nil
}

// NvmeNamespaceCreate creates a new NVMe namespace for a given device name
// TODO: consider using uuid in NvmeNamespaceCreate
func NvmeNamespaceCreate(client *JSONRPCSnapClient, crdDeviceName string, subsystems NvmeSubsystemListResponse) (int, error) {
	if len(subsystems) == 0 {
		fmt.Println("no subsystems found")
		return 0, fmt.Errorf("no subsystems found")
	}

	var nsid int
	targetSubsystem := &subsystems[0]
	if len(targetSubsystem.Namespaces) == 0 {
		nsid = 1
	} else {
		nsid = targetSubsystem.Namespaces[0].NSID + 1
	}

	params := map[string]interface{}{
		"bdev_type": "spdk",
		"nqn":       targetSubsystem.NQN,
		"nsid":      nsid,
		"bdev_name": crdDeviceName,
	}

	result, err := client.Call("nvme_namespace_create", params)

	if err != nil {
		return 0, fmt.Errorf("RPC call failed: %v", err)
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("Namespace created successfully:", string(resultBytes))

	return nsid, nil
}

// NvmeControllerCreate creates a new NVMe controller with only nqn, pf_id, and vf_id
func NvmeControllerCreate(client *JSONRPCSnapClient, subsystems NvmeSubsystemListResponse, emulationFunctions EmulationFunctionListResponse) (string, string, error) {
	if len(subsystems) == 0 {
		fmt.Println("no subsystems found")
		return "", "", fmt.Errorf("no subsystems found")
	}
	targetSubsystem := &subsystems[0]

	var currEmFunc EmulationFunction
	for _, emFunc := range emulationFunctions {
		if emFunc.EmulationType == NVMeProtocol {
			currEmFunc = emFunc
			break
		}
	}

	pciBDF := ""
	for _, vf := range currEmFunc.VFs {
		if vf.CtrlID == "" {
			pciBDF = vf.PCIBDF
			break
		}
	}

	if pciBDF == "" {
		fmt.Println("no pci bdf found")
		return "", "", fmt.Errorf("no pci bdf found")
	}

	params := map[string]interface{}{
		"nqn":     targetSubsystem.NQN,
		"pci_bdf": pciBDF,
	}

	result, err := client.Call("nvme_controller_create", params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create controller: %v", err)
	}

	ctrlID, ok := result.(map[string]interface{})["ctrl_id"].(string)
	if !ok {
		fmt.Println(err)
		return "", "", fmt.Errorf("ctrl_id not found or not a string in response")
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("Controller created successfully:", string(resultBytes))

	return ctrlID, pciBDF, nil
}

// NvmeControllerAttachNs attaches a namespace to a controller
func NvmeControllerAttachNs(client *JSONRPCSnapClient, ctrlID string, nsid int) error {
	params := map[string]interface{}{
		"nsid":    nsid,
		"ctrl_id": ctrlID,
	}

	result, err := client.Call("nvme_controller_attach_ns", params)
	if err != nil {
		return fmt.Errorf("failed to attach namespace: %v", err)
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("Namespace attached successfully:", string(resultBytes))

	return nil
}

// NvmeControllerResume resumes a controller
func NvmeControllerResume(client *JSONRPCSnapClient, ctrlID string) error {
	// Prepare parameters
	params := map[string]interface{}{
		"ctrl_id": ctrlID,
	}

	result, err := client.Call("nvme_controller_resume", params)
	if err != nil {
		return fmt.Errorf("failed to resume controller: %v", err)
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("Controller resumed successfully:", string(resultBytes))

	return nil
}

// NvmeControllerDestroy destroys an NVMe controller
func NvmeControllerDestroy(client *JSONRPCSnapClient, ctrlID string) error {
	params := map[string]interface{}{
		"ctrl_id": ctrlID,
	}

	result, err := client.Call("nvme_controller_destroy", params)
	if err != nil {
		return fmt.Errorf("failed to destroy controller: %v", err)
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("Controller destroyed successfully:", string(resultBytes))

	return nil
}

// NvmeNamespaceDestroy destroys an NVMe namespace
func NvmeNamespaceDestroy(client *JSONRPCSnapClient, nsid int, subsystems NvmeSubsystemListResponse) error {
	if len(subsystems) == 0 {
		fmt.Println("no subsystems found")
		return fmt.Errorf("no subsystems found")
	}
	targetSubsystem := &subsystems[0]

	params := map[string]interface{}{
		"nqn":  targetSubsystem.NQN,
		"nsid": nsid,
	}

	result, err := client.Call("nvme_namespace_destroy", params)
	if err != nil {
		return fmt.Errorf("failed to destroy namespace: %v", err)
	}

	resultBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("Namespace destroyed successfully:", string(resultBytes))

	return nil
}

// NvmeControllerDetachNs detaches a namespace to a controller
func NvmeControllerDetachNs(client *JSONRPCSnapClient, ctrlID string, nsid int) error {
	params := map[string]interface{}{
		"nsid":    nsid,
		"ctrl_id": ctrlID,
	}

	result, err := client.Call("nvme_controller_detach_ns", params)
	if err != nil {
		return fmt.Errorf("failed to detach namespace: %v", err)
	}

	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("failed to marshal response: %v", err)
	}

	fmt.Println("Namespace detached successfully:", string(resultBytes))

	return nil
}

// getNvmeControllerByPciAddr retrieves the NVMe controller ID associated with a given PCI address
func getNvmeControllerByPciAddr(pciAddr string, emulationFunctions EmulationFunctionListResponse) (string, error) {
	var currEmFunc EmulationFunction
	for _, emFunc := range emulationFunctions {
		if emFunc.EmulationType == NVMeProtocol {
			currEmFunc = emFunc
			break
		}
	}

	for _, vf := range currEmFunc.VFs {
		if vf.EmulationType == NVMeProtocol && vf.PCIBDF == pciAddr {
			return vf.CtrlID, nil
		}
	}

	return "", fmt.Errorf("no NVMe controller found for PCI BDF %s", pciAddr)
}

// getPciAddrByCtrlID retrieves the PCI address associated with a given NVMe controller ID
func getPciAddrByCtrlID(ctrlID string, emulationFunctions EmulationFunctionListResponse) (string, error) {
	for _, emFunc := range emulationFunctions {
		if emFunc.EmulationType != NVMeProtocol {
			continue
		}

		for _, vf := range emFunc.VFs {
			if vf.EmulationType != NVMeProtocol || vf.CtrlID != ctrlID {
				continue
			}
			return vf.PCIBDF, nil
		}
	}

	return "", fmt.Errorf("no PCI address found for NVMe controller ID %s", ctrlID)
}

// getNamespaceByDeviceName retrieves the namespace ID (NSID) associated with a given block device name
func getNamespaceByDeviceName(deviceName string, subsystems NvmeSubsystemListResponse) int {
	for _, subsystem := range subsystems {
		for _, ns := range subsystem.Namespaces {
			if ns.Bdev == deviceName {
				fmt.Printf("Namespace found for device %s: NSID=%d", deviceName, ns.NSID)
				return ns.NSID
			}
		}
	}

	fmt.Printf("No namespace found for device %s", deviceName)
	return -1
}

// checkNamespaceExist checks if the namespace exists and returns its ID if found; otherwise, it returns -1
func checkNamespaceExist(nsid int, subsystems NvmeSubsystemListResponse) bool {
	for _, subsystem := range subsystems {
		for _, ns := range subsystem.Namespaces {
			if ns.NSID == nsid {
				return true
			}
		}
	}

	return false
}

// getCtrlByDeviceName retrieves the controller ID associated with a given block device name
func getCtrlByDeviceName(deviceName string, subsystems NvmeSubsystemListResponse) string {
	for _, subsystem := range subsystems {
		for _, ns := range subsystem.Namespaces {
			if ns.Bdev != deviceName {
				continue
			}
			for _, ctrl := range ns.Controllers {
				ctrlMap, ok := ctrl.(map[string]interface{})
				if !ok {
					continue
				}
				ctrlID, exists := ctrlMap["ctrl_id"].(string)
				if !exists {
					continue
				}
				fmt.Printf("Controller found for device %s: CtrlID=%s\n", deviceName, ctrlID)
				return ctrlID
			}
		}
	}

	fmt.Printf("No controller found for device %s\n", deviceName)
	return ""
}
