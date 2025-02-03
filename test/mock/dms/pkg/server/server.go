/*
Copyright 2025 NVIDIA

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

package server

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"sync"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/test/mock/dms/pkg/certs"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnoi/os"
	"github.com/openconfig/gnoi/system"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewDMSServerMux(minPort, maxPort int, ip string, cert *x509.Certificate, key *rsa.PrivateKey) *DMSServerMux {
	tlsCert, err := tls.X509KeyPair(certs.EncodeCertPEM(cert), certs.EncodePrivateKeyPEM(key))
	if err != nil {
		panic("Failed to create key pair")
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(cert)

	tlsConfig := &tls.Config{
		ServerName:   ip,
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	d := &DMSServerMux{
		Server: grpc.NewServer(
			grpc.Creds(credentials.NewTLS(tlsConfig))),
		minPort:                 minPort,
		maxPort:                 maxPort,
		mostRecentAllocatedPort: minPort,
		ipAddress:               ip,
		listenerForDPU:          map[string]net.Listener{},
		dpuForPort:              map[string]string{},
		lock:                    sync.RWMutex{},
	}
	gnmi.RegisterGNMIServer(d.Server, &GNMIServer{})
	os.RegisterOSServer(d.Server, &GNOIServer{})
	system.RegisterSystemServer(d.Server, &GNOIServer{})
	return d
}

type DMSServerMux struct {
	*grpc.Server
	// minPort is the highest port number the DMSServerMux can use when creating listeners.
	minPort int // Minimum port number for use when creating listener instances.
	// maxPort is the highest port number the DMSServerMux can use when creating listeners.
	maxPort int
	// mostRecentAllocatedPort allocated by the DMSServerMux.
	mostRecentAllocatedPort int
	// ipAddress to bind listener instances to.
	ipAddress string // IP address to bind listener instances too.
	// listenerForDPU maps the DPU namespace name to its Listener.
	listenerForDPU map[string]net.Listener
	// dpuForPort maps port numbers to the DPUs they act as DMS servers for.
	dpuForPort map[string]string

	lock sync.RWMutex
}

func (d *DMSServerMux) IPAddress() string {
	return d.ipAddress
}
func (d *DMSServerMux) EnsureListenerForDPU(dpu *provisioningv1.DPU) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	// If we already have a lister for this DPU return early.
	// TODO: Should we check if this listener is working?
	if _, ok := d.listenerForDPU[fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)]; ok {
		return nil
	}
	port, err := d.allocatePort(dpu)
	if err != nil {
		return err
	}
	address := fmt.Sprintf("%s:%s", d.ipAddress, port)
	listener, err := d.newListenerForDPU(dpu, address)
	if err != nil {
		return err
	}

	go func() {
		if err := d.Serve(listener); err != nil {
			// TODO: Not sure this should be log.Fatal in the long term - but useful for now to uplevel error in serving.
			log.Fatal("failed to serve")
		}
	}()
	d.listenerForDPU[fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)] = listener

	return nil
}

func (d *DMSServerMux) allocatePort(dpu *provisioningv1.DPU) (string, error) {
	// If the DPU has already been annotated return the port set in its
	if port, ok := dpu.Annotations[util.OverrideDMSPortAnnotationKey]; ok {
		return port, nil
	}

	allocatedPort := 0
	for port := d.mostRecentAllocatedPort + 1; port <= d.maxPort; port++ {
		// If the port is already allocated continue.
		if _, ok := d.dpuForPort[fmt.Sprintf("%d", port)]; ok {
			continue
		}
		allocatedPort = port
		break
	}
	if allocatedPort == 0 {
		return "", fmt.Errorf("no port allocated. most recent allocated port %d. max port %d", d.mostRecentAllocatedPort, d.maxPort)
	}

	d.mostRecentAllocatedPort = allocatedPort
	// Set the annotation on the DPU. This will be patched to the object at the end of the reconcile.
	dpu.Annotations[util.OverrideDMSPortAnnotationKey] = fmt.Sprintf("%d", allocatedPort)
	return fmt.Sprintf("%d", allocatedPort), nil
}

// newListenerForDPU adds a new fake DMS listener to the DMSServerMux.
func (d *DMSServerMux) newListenerForDPU(dpu *provisioningv1.DPU, address string) (net.Listener, error) {
	// If there is already a healthy listener return it.
	if l, ok := d.listenerForDPU[fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)]; ok {
		return l, nil
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	return listener, nil
}

type GNOIServer struct {
	os.UnimplementedOSServer
	system.UnimplementedSystemServer
}

func (s *GNOIServer) RebootStatus(context.Context, *system.RebootStatusRequest) (*system.RebootStatusResponse, error) {
	log.Printf("Reboot status called")
	return &system.RebootStatusResponse{Active: false, Status: &system.RebootStatus{Status: system.RebootStatus_STATUS_SUCCESS}}, nil
}
func (s *GNOIServer) Install(req os.OS_InstallServer) error {
	log.Printf("Install called")
	return req.Send(&os.InstallResponse{Response: &os.InstallResponse_Validated{
		Validated: &os.Validated{
			Version: "one",
		},
	}})
}

func (s *GNOIServer) Activate(context.Context, *os.ActivateRequest) (*os.ActivateResponse, error) {
	log.Printf("Activate called")
	return nil, nil
}

type GNMIServer struct {
	gnmi.UnimplementedGNMIServer
}

func (s *GNMIServer) Set(context.Context, *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	log.Printf("Set called")
	return &gnmi.SetResponse{}, nil
}
