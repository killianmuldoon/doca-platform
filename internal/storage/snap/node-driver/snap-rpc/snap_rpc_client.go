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

func ExposeDevice(snapProvider, deviceName string) (int, string, error) {
	unixSocketPath := filepath.Join(basePath, snapProvider, "snap.sock")
	log.Printf("Constructed Socket Path: %s", unixSocketPath)

	client, err := NewJSONRPCSnapClient(unixSocketPath, timeout)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create JSON-RPC client: %v", err)
	}

	emulationFunctions, err := EmulationFunctionList(client)
	if err != nil {
		fmt.Println(err)
		return 0, "", fmt.Errorf("failed to retrieve emulation functions: %v", err)
	}

	subsystems, err := NvmeSubsystemList(client)
	if err != nil {
		return 0, "", fmt.Errorf("failed to retrieve NVMe subsystems: %v", err)
	}

	nsid := getNamespaceByDeviceName(deviceName, subsystems)

	if nsid == -1 {
		nsid, err = NvmeNamespaceCreate(client, deviceName, subsystems)
		if err != nil {
			return 0, "", fmt.Errorf("failed to create namespace: %v", err)
		}
		log.Printf("Created new namespace: NSID=%d", nsid)
	} else {
		log.Printf("Namespace already exists: NSID=%d", nsid)
	}

	ctrlID := getCtrlByDeviceName(deviceName, subsystems)
	if err != nil {
		log.Printf("Error retrieving NVMe controllers: %v", err)
		return 0, "", err
	}

	var pciBDF string
	if ctrlID != "" {
		log.Printf("Controller already exists: ID=%s", ctrlID)
		pciBDF, err = getPciAddrByCtrlID(ctrlID, emulationFunctions)
		if err != nil {
			return 0, "", fmt.Errorf("failed to get PCI BDF for controller: %v", err)
		}
	} else {
		ctrlID, pciBDF, err = NvmeControllerCreate(client, subsystems, emulationFunctions)
		if err != nil {
			return 0, "", fmt.Errorf("failed to create controller: %v", err)
		}

		err = NvmeControllerAttachNs(client, ctrlID, nsid)
		if err != nil {
			return 0, "", fmt.Errorf("failed to attach namespace to controller: %v", err)
		}

		err = NvmeControllerResume(client, ctrlID)
		if err != nil {
			return 0, "", fmt.Errorf("failed to resume controller: %v", err)
		}

		log.Printf("Created new controller: ID=%s, PCI BDF=%s", ctrlID, pciBDF)
	}

	log.Printf("Final Device State -> NSID=%d, PCI BDF=%s", nsid, pciBDF)
	return nsid, pciBDF, nil
}

func DestroyDevice(snapProvider string, nsid int, pciAddr string) error {
	unixSocketPath := filepath.Join(basePath, snapProvider, "snap.sock")
	log.Printf("Constructed Socket Path: %s", unixSocketPath)

	client, err := NewJSONRPCSnapClient(unixSocketPath, timeout)
	if err != nil {
		return fmt.Errorf("failed to create JSON-RPC client: %v", err)
	}

	emulationFunctions, err := EmulationFunctionList(client)
	if err != nil {
		log.Printf("Failed to get emulation functions list: %v", err)
		return fmt.Errorf("failed to retrieve emulation functions: %v", err)
	}

	subsystems, err := NvmeSubsystemList(client)
	if err != nil {
		return fmt.Errorf("failed to retrieve NVMe subsystems: %v", err)
	}

	ctrlID, err := getNvmeControllerByPciAddr(pciAddr, emulationFunctions)
	if err != nil {
		log.Printf("Error finding NVMe controller for PCI Address %s: %v", pciAddr, err)
		return fmt.Errorf("error finding NVMe controller: %v", err)
	}

	namespaceExists := checkNamespaceExist(nsid, subsystems)
	if err != nil {
		log.Printf("Failed to check namespace existence: %v", err)
		return err
	}

	if ctrlID == "" && !namespaceExists {
		log.Printf("NVMe Controller and namespace not found for PCI Address %s. Skipping detach and destroy.", pciAddr)
		return nil
	}

	// Detach the namespace only if both exist
	if ctrlID == "" || !namespaceExists {
		log.Printf("Namespace/Controller does not exist. Skipping detach.")
	} else {
		err = NvmeControllerDetachNs(client, ctrlID, nsid)
		if err != nil {
			log.Printf("Failed to detach namespace: %v", err)
			return fmt.Errorf("failed to detach namespace: %v", err)
		}
		log.Printf("Successfully detached namespace ID %d", nsid)
	}

	if ctrlID == "" {
		log.Printf("Controller ID does not exist. Skipping destroy.")
	} else {
		err = NvmeControllerDestroy(client, ctrlID)
		if err != nil {
			log.Printf("Failed to destroy controller: %v", err)
			return fmt.Errorf("failed to destroy controller: %v", err)
		}
		log.Printf("Successfully destroyed controller ID %s", ctrlID)
	}

	if !namespaceExists {
		log.Printf("Namespace ID %d does not exist. Skipping detach.", nsid)
	} else {
		err = NvmeNamespaceDestroy(client, nsid, subsystems)
		if err != nil {
			log.Printf("Failed to destroy namespace ID %d: %v", nsid, err)
			return fmt.Errorf("failed to destroy namespace: %v", err)
		}
		log.Printf("Successfully destroyed namespace ID %d", nsid)
	}

	return nil
}
