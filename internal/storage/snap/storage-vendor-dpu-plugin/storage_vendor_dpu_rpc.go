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

package storagevendordpuplugin

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

/* TODOs:
 * 1. request and response rpc shouldn't be anonymous structs
 * 2. check errors.As to replace current errors
 * 3. Ensure that the RPC client is thread-safe and maintains connections.
 */

// Common JSON-RPC errors
var (
	ErrJSONNoSpaceLeft       = errors.New("json: No space left")
	ErrJSONNoSuchDevice      = errors.New("json: No such device")
	ErrJSONInvalidParameters = errors.New("json: Invalid parameters")
)

// rpcClient handles JSON-RPC over a Unix domain socket.
type rpcClient struct {
	conn  net.Conn
	rpcID int32
}

// NewRPCClient creates an rpcClient that communicates over a Unix domain socket directly.
func NewRPCClient(socketPath string) (*rpcClient, error) {
	c, err := net.DialTimeout("unix", socketPath, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to unix socket %s: %w", socketPath, err)
	}
	return &rpcClient{conn: c}, nil
}

// Call executes a JSON-RPC call with given method and params
func (c *rpcClient) Call(method string, params interface{}) (interface{}, error) {
	type rpcRequest struct {
		Ver    string      `json:"jsonrpc"`
		ID     int32       `json:"id"`
		Method string      `json:"method"`
		Params interface{} `json:"params,omitempty"`
	}

	id := atomic.AddInt32(&c.rpcID, 1)
	req := rpcRequest{
		Ver:    "2.0",
		ID:     id,
		Method: method,
		Params: params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}

	log.Printf("Sending RPC (method=%s, id=%d): %s", method, id, string(data))

	// Write request line
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("%s: write request failed: %w", method, err)
	}

	// Read response line
	reader := bufio.NewReader(c.conn)
	respLine, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("%s: read response failed: %w", method, err)
	}

	log.Printf("Received response (method=%s, id=%d): %s", method, id, string(respLine))

	// Decode response
	response := struct {
		ID    int32 `json:"id"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}{}

	if err := json.Unmarshal(respLine, &response); err != nil {
		return nil, fmt.Errorf("%s: decode response failed: %w", method, err)
	}

	if response.ID != id {
		return nil, fmt.Errorf("%s: response ID mismatch (got %d, expected %d)", method, response.ID, id)
	}
	if response.Error.Code != 0 {
		return nil, fmt.Errorf("%s: json response error: %s", method, response.Error.Message)
	}

	return &response.Result, nil
}

// errorMatches checks if errFull message contains the substring of errJSON
func errorMatches(errFull, errJSON error) bool {
	if errFull == nil {
		return false
	}
	strFull := strings.ToLower(errFull.Error())
	strJSON := strings.ToLower(errJSON.Error())
	strJSON = strings.TrimPrefix(strJSON, "json:")
	strJSON = strings.TrimSpace(strJSON)
	return strings.Contains(strFull, strJSON)
}

// Bdev represents a block device.
type Bdev struct {
	Name           string                 `json:"name"`
	DriverSpecific map[string]interface{} `json:"driver_specific"`
}

// BdevGetBdevsResponse contains a list of block devices.
type BdevGetBdevsResponse struct {
	Bdevs []Bdev `json:"bdevs"`
}

// BdevGetBdevs retrieves a list of all block devices (bdevs)
func (c *rpcClient) BdevGetBdevs() (BdevGetBdevsResponse, error) {
	log.Println("Calling BdevGetBdevs RPC")
	result, err := c.Call("bdev_get_bdevs", nil)
	if err != nil {
		return BdevGetBdevsResponse{}, err
	}

	raw, ok := result.(*json.RawMessage)
	if !ok {
		return BdevGetBdevsResponse{}, fmt.Errorf("unexpected result type")
	}

	// Unmarshal directly into a slice of Bdev
	var bdevs []Bdev
	if err := json.Unmarshal(*raw, &bdevs); err != nil {
		return BdevGetBdevsResponse{}, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return BdevGetBdevsResponse{Bdevs: bdevs}, nil
}

type NVMeTrid struct {
	TrType  string `json:"trtype"`
	AdrFam  string `json:"adrfam"`
	TrAddr  string `json:"traddr"`
	TrSvcID string `json:"trsvcid"`
	SubNQN  string `json:"subnqn"`
}

type NvmeController struct {
	Name   string `json:"name"`
	Ctrlrs []struct {
		Trid NVMeTrid `json:"trid"`
	} `json:"ctrlrs"`
}

type BdevNvmeGetControllersResponse struct {
	Controllers []NvmeController `json:"controllers"`
}

// BdevNvmeGetControllers retrieves the list of NVMe controllers
func (c *rpcClient) BdevNvmeGetControllers() (BdevNvmeGetControllersResponse, error) {
	log.Println("Calling BdevNvmeGetControllers RPC")

	result, err := c.Call("bdev_nvme_get_controllers", nil)
	if err != nil {
		return BdevNvmeGetControllersResponse{}, err
	}

	raw, ok := result.(*json.RawMessage)
	if !ok {
		return BdevNvmeGetControllersResponse{}, fmt.Errorf("unexpected result type")
	}

	var controllers []NvmeController
	if err := json.Unmarshal(*raw, &controllers); err != nil {
		return BdevNvmeGetControllersResponse{}, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return BdevNvmeGetControllersResponse{Controllers: controllers}, nil
}

type BdevNvmeAttachControllerRequest struct {
	Name    string `json:"name"`
	Trtype  string `json:"trtype"`
	Traddr  string `json:"traddr"`
	Adrfam  string `json:"adrfam,omitempty"`
	Trsvcid string `json:"trsvcid,omitempty"`
	Subnqn  string `json:"subnqn,omitempty"`
}

type BdevNvmeAttachControllerResponse struct {
	BdevName string `json:"bdev_name"`
}

// CheckBdevExistsByTrid checks if an NVMe controller with the given parameters already exists.
func CheckBdevExistsByTrid(req BdevNvmeAttachControllerRequest, bdevResponse BdevGetBdevsResponse) (string, error) {
	log.Printf("Checking if NVMe controller exists: %+v", req)

	for _, bdev := range bdevResponse.Bdevs {
		nvmeData, ok := bdev.DriverSpecific["nvme"]
		if !ok {
			continue
		}

		nvmeList, ok := nvmeData.([]interface{})
		if !ok {
			continue
		}

		for _, nvme := range nvmeList {
			nvmeMap, ok := nvme.(map[string]interface{})
			if !ok {
				continue
			}

			trid, ok := nvmeMap["trid"].(map[string]interface{})
			if !ok {
				continue
			}

			if strings.EqualFold(fmt.Sprint(trid["trtype"]), req.Trtype) &&
				strings.EqualFold(fmt.Sprint(trid["adrfam"]), req.Adrfam) &&
				strings.EqualFold(fmt.Sprint(trid["traddr"]), req.Traddr) &&
				strings.EqualFold(fmt.Sprint(trid["trsvcid"]), req.Trsvcid) &&
				strings.EqualFold(fmt.Sprint(trid["subnqn"]), req.Subnqn) {
				log.Printf("Found existing NVMe controller: %s", bdev.Name)
				return bdev.Name, nil
			}
		}
	}

	log.Println("No matching NVMe controller found.")
	return "", nil
}

// CheckBdevExistsByBdev verifies if a block device (bdev) with the given device name exists.
func CheckBdevExistsByBdev(deviceName string, bdevResponse BdevGetBdevsResponse) (bool, error) {
	for _, bdev := range bdevResponse.Bdevs {
		if bdev.Name == deviceName {
			log.Printf("Bdev found: %s", deviceName)
			return true, nil
		}
	}

	log.Printf("Bdev not found: %s", deviceName)
	return false, nil
}

func (c *rpcClient) BdevNvmeAttachController(req BdevNvmeAttachControllerRequest) (BdevNvmeAttachControllerResponse, error) {
	log.Printf("Attaching NVMe controller with request: %+v", req)
	result, err := c.Call("bdev_nvme_attach_controller", req)
	if err != nil {
		if errorMatches(err, ErrJSONNoSpaceLeft) {
			return BdevNvmeAttachControllerResponse{}, ErrJSONNoSpaceLeft
		}
		return BdevNvmeAttachControllerResponse{}, err
	}

	raw, ok := result.(*json.RawMessage)
	if !ok {
		return BdevNvmeAttachControllerResponse{}, fmt.Errorf("unexpected result type")
	}

	// Handle response type variants
	var v interface{}
	if err := json.Unmarshal(*raw, &v); err != nil {
		return BdevNvmeAttachControllerResponse{}, fmt.Errorf("unmarshal result: %v", err)
	}

	var response BdevNvmeAttachControllerResponse
	switch val := v.(type) {
	case string:
		response.BdevName = val
	case []interface{}:
		if len(val) > 0 {
			name, ok := val[0].(string)
			if !ok {
				return BdevNvmeAttachControllerResponse{}, fmt.Errorf("unexpected item type in array")
			}
			response.BdevName = name
		} else {
			return BdevNvmeAttachControllerResponse{}, fmt.Errorf("response array is empty")
		}
	default:
		return BdevNvmeAttachControllerResponse{}, fmt.Errorf("unexpected response type: %T", v)
	}

	log.Printf("NVMe controller attached, BdevName: %s", response.BdevName)
	return response, nil
}

type BdevNvmeDetachControllerRequest struct {
	Name string `json:"name"`
}

// getTridByBdev extracts the NVMeTrid from a given bdev name
func getTridByBdev(bdevName string, bdevResponse BdevGetBdevsResponse) (NVMeTrid, error) {
	for _, bdev := range bdevResponse.Bdevs {
		if bdev.Name != bdevName {
			continue
		}

		nvmeInterfaces, ok := bdev.DriverSpecific["nvme"].([]interface{})
		if !ok || len(nvmeInterfaces) == 0 {
			continue
		}

		nvmeMap, ok := nvmeInterfaces[0].(map[string]interface{})
		if !ok {
			continue
		}

		tridData, exists := nvmeMap["trid"]
		if !exists {
			continue
		}

		tridBytes, err := json.Marshal(tridData)
		if err != nil {
			return NVMeTrid{}, fmt.Errorf("failed to marshal trid: %v", err)
		}

		var targetTrid NVMeTrid
		err = json.Unmarshal(tridBytes, &targetTrid)
		if err != nil {
			return NVMeTrid{}, fmt.Errorf("failed to unmarshal trid into NVMeTrid: %v", err)
		}

		return targetTrid, nil
	}

	return NVMeTrid{}, fmt.Errorf("could not find trid for bdev name: %s", bdevName)
}

// getControllerByTrid extracts the NVMeTrid from a given bdev name
func getControllerByTrid(targetTrid NVMeTrid, controllersResponse BdevNvmeGetControllersResponse) (string, error) {
	for _, controller := range controllersResponse.Controllers {
		for _, ctrlr := range controller.Ctrlrs {
			if ctrlr.Trid == targetTrid {
				return controller.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no matching controller found for trid: %+v", targetTrid)
}

func (c *rpcClient) BdevNvmeDetachController(req BdevNvmeDetachControllerRequest) error {
	log.Printf("Detaching NVMe controller: %+v", req)

	_, err := c.Call("bdev_nvme_detach_controller", req)
	if err != nil {
		if errorMatches(err, ErrJSONNoSuchDevice) {
			log.Printf("NVMe controller %s not found: %v", req.Name, err)
			return ErrJSONNoSuchDevice
		}
		log.Printf("Error detaching NVMe controller %s: %v", req.Name, err)
		return fmt.Errorf("failed to detach NVMe controller %s: %v", req.Name, err)
	}

	log.Printf("Successfully detached NVMe controller: %s", req.Name)
	return nil
}
