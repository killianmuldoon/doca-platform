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
	"fmt"
	"log"
	"path/filepath"
)

const basePath = "/var/lib/nvidia/storage/snap/providers"
const timeout = 60

// ExposeDevice creates a new NVMe namespace, controller, and attaches them.
func ExposeDevice(snapProvider, deviceName string) (int, string, error) {
	unixSocketPath := filepath.Join(basePath, snapProvider, "snap.sock")
	log.Printf("Constructed Socket Path: %s", unixSocketPath)

	// Create the JSON-RPC client from the given Unix socket path
	client, err := NewJSONRPCSnapClient(unixSocketPath, timeout)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create JSON-RPC client: %v", err)
	}

	// Create the NVMe Namespace
	nsid, err := NvmeNamespaceCreate(client, deviceName)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create namespace: %v", err)
	}

	// Create the NVMe Controller
	ctrlID, pciBDF, err := NvmeControllerCreate(client)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create controller: %v", err)
	}

	// Attach the Namespace to the Controller
	err = NvmeControllerAttachNs(client, ctrlID, nsid)
	if err != nil {
		return 0, "", fmt.Errorf("failed to attach namespace to controller: %v", err)
	}

	// Resume the Controller
	err = NvmeControllerResume(client, ctrlID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to resume controller: %v", err)
	}

	log.Printf("nsID=%d, pciBDF=%s", nsid, pciBDF)

	if err := client.Close(); err != nil {
		log.Printf("Failed to close client: %v", err)
	}

	return nsid, pciBDF, nil
}

// DestroyDevice detaches and destroys the NVMe namespace and controller.
func DestroyDevice(snapProvider string, nsid int, pciAddr string) error {
	unixSocketPath := filepath.Join(basePath, snapProvider, "snap.sock")
	log.Printf("Constructed Socket Path: %s", unixSocketPath)

	// Create the JSON-RPC client from the given Unix socket path
	client, err := NewJSONRPCSnapClient(unixSocketPath, timeout)
	if err != nil {
		return fmt.Errorf("failed to create JSON-RPC client: %v", err)
	}

	ctrlID, err := findNvmeControllerByPciAddr(client, pciAddr)
	if err != nil {
		return fmt.Errorf("failed to detach namespace: %v", err)
	}

	// Detach the Namespace from the Controller
	err = NvmeControllerDetachNs(client, ctrlID, nsid)
	if err != nil {
		return fmt.Errorf("failed to detach namespace: %v", err)
	}

	// Destroy the Controller
	err = NvmeControllerDestroy(client, ctrlID)
	if err != nil {
		return fmt.Errorf("failed to destroy controller: %v", err)
	}

	// Destroy the Namespace
	err = NvmeNamespaceDestroy(client, nsid)
	if err != nil {
		return fmt.Errorf("failed to destroy namespace: %v", err)
	}

	if err := client.Close(); err != nil {
		log.Printf("Failed to close client: %v", err)
	}

	return nil
}
