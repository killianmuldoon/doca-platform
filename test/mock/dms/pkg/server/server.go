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
	"math/rand"
	"net"
	"sync"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/test/mock/dms/pkg/certs"
	"github.com/nvidia/doca-platform/test/mock/dms/pkg/config"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnoi/os"
	"github.com/openconfig/gnoi/system"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"k8s.io/klog/v2"
)

const (
	setCallName      = "set"
	activateCallName = "activate"
	installCallName  = "install"
	rebootCallName   = "reboot"
)

func NewDMSServerMux(minPort, maxPort int, ip string, cert *x509.Certificate, key *rsa.PrivateKey, config config.Config) *DMSServerMux {
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
		configForPort:           map[string]dpuResponseConfig{},
		RWMutex:                 sync.RWMutex{},
		config:                  config,
	}
	gnmi.RegisterGNMIServer(d.Server, d)
	os.RegisterOSServer(d.Server, d)
	system.RegisterSystemServer(d.Server, d)
	return d
}

type dpuResponseConfig struct {
	responseConfigs  map[string]responseConfig
	dpuNamespaceName string
}
type responseConfig struct {
	delay     time.Duration
	errorRate float64
}

type DMSServerMux struct {
	os.UnimplementedOSServer
	system.UnimplementedSystemServer
	gnmi.UnimplementedGNMIServer
	*grpc.Server
	config config.Config
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
	// configForPort maps port numbers to the DPUs they act as DMS servers for.
	configForPort map[string]dpuResponseConfig

	sync.RWMutex
}

func (d *DMSServerMux) EnsureListenerForDPU(dpu *provisioningv1.DPU) error {
	d.Lock()
	defer d.Unlock()

	// If we already have a lister for this DPU return early.
	// TODO: Should we check if this listener is working for re-entrancy?
	if _, ok := d.listenerForDPU[fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)]; ok {
		return nil
	}
	port, err := d.allocatePort(dpu)
	if err != nil {
		return err
	}
	listener, err := d.newListenerForDPU(dpu, d.ipAddress, port)
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
		if _, ok := d.configForPort[fmt.Sprintf("%d", port)]; ok {
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
func (d *DMSServerMux) newListenerForDPU(dpu *provisioningv1.DPU, address, port string) (net.Listener, error) {
	// If there is already a healthy listener return it.
	if l, ok := d.listenerForDPU[fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)]; ok {
		return l, nil
	}
	conf := dpuResponseConfig{
		responseConfigs: map[string]responseConfig{
			setCallName: {
				delay:     delayWithJitter(d.config.Set.MeanDelaySeconds, d.config.Set.DelayJitter),
				errorRate: d.config.Set.ErrorRate,
			},
			activateCallName: {
				delay:     delayWithJitter(d.config.Activate.MeanDelaySeconds, d.config.Activate.DelayJitter),
				errorRate: d.config.Activate.ErrorRate,
			},
			installCallName: {
				delay:     delayWithJitter(d.config.Install.MeanDelaySeconds, d.config.Install.DelayJitter),
				errorRate: d.config.Install.ErrorRate,
			},
			rebootCallName: {
				delay:     delayWithJitter(d.config.Reboot.MeanDelaySeconds, d.config.Reboot.DelayJitter),
				errorRate: d.config.Reboot.ErrorRate,
			},
		},
		dpuNamespaceName: fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name),
	}
	d.configForPort[port] = conf
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", address, port))
	if err != nil {
		return nil, err
	}
	return listener, nil
}

func (d *DMSServerMux) RebootStatus(ctx context.Context, req *system.RebootStatusRequest) (*system.RebootStatusResponse, error) {
	dpu, conf, err := d.configForRequest(ctx, rebootCallName)
	if err != nil {
		return nil, err
	}
	logger := klog.LoggerWithValues(klog.Background(), "api", rebootCallName, "dpu", dpu)
	logger.Info("Calling")
	if rand.Float64() < conf.errorRate {
		logger.Info("returning error")
		return &system.RebootStatusResponse{
			Status: &system.RebootStatus{
				Status: system.RebootStatus_STATUS_FAILURE,
			},
		}, fmt.Errorf("error during activation")
	}
	logger.Info(fmt.Sprintf("waiting %f seconds ", conf.delay.Seconds()))
	time.Sleep(conf.delay)

	return &system.RebootStatusResponse{Active: false, Status: &system.RebootStatus{Status: system.RebootStatus_STATUS_SUCCESS}}, nil
}
func (d *DMSServerMux) Install(req os.OS_InstallServer) error {
	dpu, conf, err := d.configForRequest(req.Context(), installCallName)
	if err != nil {
		return err
	}
	logger := klog.LoggerWithValues(klog.Background(), "api", installCallName, "dpu", dpu)
	logger.Info("Calling")
	if rand.Float64() < conf.errorRate {
		logger.Info("returning error")
		return req.Send(&os.InstallResponse{Response: &os.InstallResponse_InstallError{
			InstallError: &os.InstallError{
				Type:   os.InstallError_INCOMPATIBLE,
				Detail: "install failed due to failure rate",
			},
		}})
	}
	logger.Info(fmt.Sprintf("waiting %f seconds ", conf.delay.Seconds()))
	time.Sleep(conf.delay)

	return req.Send(&os.InstallResponse{Response: &os.InstallResponse_Validated{
		Validated: &os.Validated{
			Version: "one",
		}}})
}

func (d *DMSServerMux) Activate(ctx context.Context, req *os.ActivateRequest) (*os.ActivateResponse, error) {
	dpu, conf, err := d.configForRequest(ctx, activateCallName)
	if err != nil {
		return nil, err
	}
	logger := klog.LoggerWithValues(klog.Background(), "api", activateCallName, "dpu", dpu)
	logger.Info("Calling")
	if rand.Float64() < conf.errorRate {
		logger.Info("returning error")
		return &os.ActivateResponse{
			Response: &os.ActivateResponse_ActivateError{
				ActivateError: &os.ActivateError{
					Type:   os.ActivateError_UNSPECIFIED,
					Detail: "it's an error",
				},
			},
		}, fmt.Errorf("error during activation")
	}
	logger.Info(fmt.Sprintf("waiting %f seconds ", conf.delay.Seconds()))
	time.Sleep(conf.delay)
	return nil, nil
}

func (d *DMSServerMux) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	dpu, conf, err := d.configForRequest(ctx, setCallName)
	if err != nil {
		return nil, err
	}
	logger := klog.LoggerWithValues(klog.Background(), "api", setCallName, "dpu", dpu)
	logger.Info("Calling")
	if rand.Float64() < conf.errorRate {
		return &gnmi.SetResponse{
			Response: []*gnmi.UpdateResult{
				{
					Op: gnmi.UpdateResult_INVALID,
				},
			},
		}, fmt.Errorf("error during activation")
	}
	logger.Info(fmt.Sprintf("waiting %f seconds ", conf.delay.Seconds()))
	time.Sleep(conf.delay)

	return &gnmi.SetResponse{}, nil
}

func delayWithJitter(mean int64, maxJitter float64) time.Duration {
	meanDuration := time.Duration(mean * time.Second.Nanoseconds())
	jitterDuration := float64(meanDuration.Nanoseconds()) * maxJitter
	if jitterDuration <= 0 {
		return time.Duration(0)
	}
	jitter := time.Duration(rand.Int63n(int64(jitterDuration)*2)) - time.Duration(jitterDuration)
	return meanDuration + jitter
}

func (d *DMSServerMux) configForRequest(ctx context.Context, requestType string) (string, *responseConfig, error) {
	d.Lock()
	defer d.Unlock()
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", nil, fmt.Errorf("could not get peer from context")
	}
	a, ok := p.LocalAddr.(*net.TCPAddr)
	if !ok {
		return "", nil, fmt.Errorf("could not get port from local address")
	}
	dpuConf, ok := d.configForPort[fmt.Sprintf("%d", a.Port)]
	if !ok {
		return "", nil, fmt.Errorf("could not get config for port %d", a.Port)
	}
	conf, ok := dpuConf.responseConfigs[requestType]
	if !ok {
		return "", nil, fmt.Errorf("could not get config for requestType %s", requestType)
	}
	return dpuConf.dpuNamespaceName, &conf, nil
}
