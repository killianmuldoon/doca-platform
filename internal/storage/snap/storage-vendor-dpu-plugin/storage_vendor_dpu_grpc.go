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
	"context"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/nvidia/doca-platform/api/grpc/nvidia/storage/plugins/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// StoragePluginServer represents the gRPC server for storage operations
type StoragePluginServer struct {
	pb.UnimplementedStoragePluginServiceServer
	pb.UnimplementedIdentityServiceServer
}

// TODO: make these paths configurable
const (
	pluginName          = "nvidia"
	socketPathPluginRPC = "/var/lib/nvidia/storage/snap/plugins/nvidia/dpu.sock"
	socketPathSNAPRPC   = "/var/lib/nvidia/storage/snap/providers/nvidia/snap.sock"
)

func CreateGRPCServer() (*grpc.Server, net.Listener, error) {
	log.Printf("Starting gRPC server initialization at %s", socketPathPluginRPC)

	// If the socket file already exists, remove it
	if _, err := os.Stat(socketPathPluginRPC); err == nil {
		log.Printf("Socket file %s already exists. Removing it.", socketPathPluginRPC)
		if rmErr := os.Remove(socketPathPluginRPC); rmErr != nil {
			return nil, nil, fmt.Errorf("failed to remove existing socket file: %v", rmErr)
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("error checking socket file: %v", err)
	}

	// Create a Unix domain socket listener
	log.Printf("Creating GRPC socket listener at %s", socketPathPluginRPC)
	listener, err := net.Listen("unix", socketPathPluginRPC)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create listener: %v", err)
	}

	// Initialize the StoragePluginServer
	storageServer := &StoragePluginServer{}

	// Create a new gRPC server
	server := grpc.NewServer()
	pb.RegisterStoragePluginServiceServer(server, storageServer)
	pb.RegisterIdentityServiceServer(server, storageServer)

	log.Printf("gRPC server is ready at %s (awaiting Serve call)", socketPathPluginRPC)

	return server, listener, nil
}

// GetPluginInfo RPC
func (s *StoragePluginServer) GetPluginInfo(ctx context.Context, req *pb.GetPluginInfoRequest) (*pb.GetPluginInfoResponse, error) {
	log.Printf("Received GetPluginInfo request: %+v", req)
	resp := &pb.GetPluginInfoResponse{
		Name:          "storage.dpu.nvidia.com",
		VendorVersion: "1.0",
		Manifest: map[string]string{
			"description": "NVIDIA SNAP Storage Plugin",
			"maintainer":  "NVIDIA",
		},
	}
	log.Printf("Responding with GetPluginInfo: %+v", resp)
	return resp, nil
}

// Probe RPC
func (s *StoragePluginServer) Probe(ctx context.Context, req *pb.ProbeRequest) (*pb.ProbeResponse, error) {
	log.Printf("Received Probe request: %+v", req)
	resp := &pb.ProbeResponse{
		Ready: &wrapperspb.BoolValue{Value: true},
	}
	log.Printf("Responding with Probe: %+v", resp)
	return resp, nil
}

// StoragePluginGetCapabilities RPC
func (s *StoragePluginServer) StoragePluginGetCapabilities(ctx context.Context, req *pb.StoragePluginGetCapabilitiesRequest) (*pb.StoragePluginGetCapabilitiesResponse, error) {
	log.Printf("Received StoragePluginGetCapabilities request: %+v", req)
	capabilities := []*pb.StoragePluginServiceCapability{
		{
			Type: &pb.StoragePluginServiceCapability_Rpc{
				Rpc: &pb.StoragePluginServiceCapability_RPC{
					Type: pb.StoragePluginServiceCapability_RPC_TYPE_CREATE_DELETE_BLOCK_DEVICE,
				},
			},
		},
	}
	resp := &pb.StoragePluginGetCapabilitiesResponse{
		Capabilities: capabilities,
	}
	log.Printf("Responding with StoragePluginGetCapabilities: %+v", resp)
	return resp, nil
}

// GetSNAPProvider RPC
func (s *StoragePluginServer) GetSNAPProvider(ctx context.Context, req *pb.GetSNAPProviderRequest) (*pb.GetSNAPProviderResponse, error) {
	log.Printf("Received GetSNAPProvider request: %+v", req)
	resp := &pb.GetSNAPProviderResponse{ProviderName: pluginName}
	log.Printf("Responding with GetSNAPProvider: %+v", resp)
	return resp, nil
}

// CreateDevice RPC
func (s *StoragePluginServer) CreateDevice(ctx context.Context, req *pb.CreateDeviceRequest) (*pb.CreateDeviceResponse, error) {
	log.Printf("Received CreateDevice request: %+v", req)

	log.Printf("Creating rpcClient to communicate with SNAP RPC at %s", socketPathSNAPRPC)
	c, err := NewRPCClient(socketPathSNAPRPC)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create rpcClient: %v", err)
		log.Println(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	bdevs, err := c.BdevGetBdevs()
	if err != nil {
		return nil, fmt.Errorf("failed to get bdevs: %v", err)
	}

	attachRequest := BdevNvmeAttachControllerRequest{
		Trtype:  req.VolumeContext["targetType"],
		Traddr:  req.VolumeContext["targetAddr"],
		Adrfam:  "ipv4",
		Trsvcid: req.VolumeContext["targetPort"],
		Subnqn:  req.VolumeContext["nqn"],
	}

	log.Printf("Checking if NVMe device already exists: %+v", attachRequest)
	deviceName, err := CheckBdevExistsByTrid(attachRequest, bdevs)
	if err != nil {
		errMsg := fmt.Sprintf("failed to check NVMe existence: %v", err)
		log.Println(errMsg)
		return nil, status.Errorf(codes.FailedPrecondition, "%s", errMsg)
	}

	if deviceName != "" {
		log.Printf("Device already exists: %s", deviceName)
		resp := &pb.CreateDeviceResponse{DeviceName: deviceName}
		log.Printf("Responding with CreateDevice: %+v", resp)
		return resp, nil
	}

	attachRequest.Name = "nvme_" + req.GetVolumeId()
	log.Printf("Attaching NVMe controller: %+v", attachRequest)
	respAttach, err := c.BdevNvmeAttachController(attachRequest)
	if err != nil {
		errMsg := fmt.Sprintf("failed to attach NVMe controller: %v", err)
		log.Println(errMsg)
		return nil, status.Errorf(codes.FailedPrecondition, "%s", errMsg)
	}

	resp := &pb.CreateDeviceResponse{DeviceName: respAttach.BdevName}
	log.Printf("Responding with CreateDevice: %+v", resp)
	return resp, nil
}

func (s *StoragePluginServer) DeleteDevice(ctx context.Context, req *pb.DeleteDeviceRequest) (*pb.DeleteDeviceResponse, error) {
	log.Printf("Received DeleteDevice request: %+v", req)

	log.Printf("Creating rpcClient to communicate with SNAP RPC at %s", socketPathSNAPRPC)
	c, err := NewRPCClient(socketPathSNAPRPC)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create rpcClient: %v", err)
		log.Println(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	bdevs, err := c.BdevGetBdevs()
	if err != nil {
		return nil, fmt.Errorf("failed to get bdevs: %v", err)
	}

	controllers, err := c.BdevNvmeGetControllers()
	if err != nil {
		return nil, fmt.Errorf("failed to get NVMe controllers: %v", err)
	}

	// Check if the Bdev exists before proceeding
	bdevExists, err := CheckBdevExistsByBdev(req.DeviceName, bdevs)
	if err != nil {
		errMsg := fmt.Sprintf("Error checking bdev existence: %v", err)
		log.Println(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	if !bdevExists {
		log.Printf("Bdev %s does not exist. Skipping deletion.", req.DeviceName)
		return &pb.DeleteDeviceResponse{}, nil
	}

	// Extract the trid for the given Bdev name
	targetTrid, err := getTridByBdev(req.DeviceName, bdevs)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to extract trid for bdev %s: %v", req.DeviceName, err)
		log.Println(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}
	log.Printf("Found trid: %+v", targetTrid)

	// Find the controller name that matches the trid
	controllerName, err := getControllerByTrid(targetTrid, controllers)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find controller for trid %+v: %v", targetTrid, err)
		log.Println(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	log.Printf("Found controller name: %s", controllerName)

	// Detach NVMe controller using the correct controller name
	detachRequest := BdevNvmeDetachControllerRequest{
		Name: controllerName,
	}

	log.Printf("Detaching NVMe controller: %+v", detachRequest)
	err = c.BdevNvmeDetachController(detachRequest)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to detach NVMe controller %s: %v", controllerName, err)
		log.Println(errMsg)
		return nil, status.Errorf(codes.FailedPrecondition, "%s", errMsg)
	}

	resp := &pb.DeleteDeviceResponse{}
	log.Printf("Successfully deleted device: %+v", controllerName)
	return resp, nil
}
