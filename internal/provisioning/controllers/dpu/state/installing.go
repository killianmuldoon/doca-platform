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

package state

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/bfcfg"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/dms"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/future"

	ospb "github.com/openconfig/gnoi/os"
	"github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/gnoigo"
	gos "github.com/openconfig/gnoigo/os"
	gsystem "github.com/openconfig/gnoigo/system"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	TemplateFile            = "bf.cfg.template"
	CloudInitDefaultTimeout = 90
	MaxBFSize               = 1024 * 16
	MaxRetryCount           = 10
)

type BFConfig struct {
	KUBEADM_JOIN   string
	HOSTNAME       string
	UbuntuPassword string
	NVConfigParams map[string]string
	Sysctl         []string
	ConfigFiles    []BFCfgWriteFile
	OVSRawScript   string
}

type BFCfgWriteFile struct {
	Path        string
	IsAppend    bool
	Content     string
	Permissions string
}

type dpuOSInstallingState struct {
	dpu *provisioningdpfv1alpha1.Dpu
}

type ExecuteResult struct {
	Response *ospb.InstallResponse
	Err      error
}

func (st *dpuOSInstallingState) Handle(ctx context.Context, client client.Client, _ dutil.DPUOptions) (provisioningdpfv1alpha1.DpuStatus, error) {
	logger := log.FromContext(ctx)
	dmsTaskName := generateDMSTaskName(st.dpu)
	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		dutil.OsInstallTaskMap.Delete(dmsTaskName)
		state.Phase = provisioningdpfv1alpha1.DPUDeleting
		return *state, nil
	}

	// check whether bfb exist
	nn := types.NamespacedName{
		Namespace: st.dpu.Namespace,
		Name:      st.dpu.Spec.BFB,
	}
	bfb := &provisioningdpfv1alpha1.Bfb{}
	if err := client.Get(ctx, nn, bfb); err != nil {
		if apierrors.IsNotFound(err) {
			// BFB does not exist, dpu keeps in installing state.
			logger.V(3).Info(fmt.Sprintf("BFB %v is not ready for DMS %v", st.dpu.Spec.BFB, dmsTaskName))
			return *state, nil
		}
		return *state, err
	}

	if dmsTask, ok := dutil.OsInstallTaskMap.Load(dmsTaskName); ok {
		result := dmsTask.(*future.Future)
		if result.GetState() == future.Ready {
			dutil.OsInstallTaskMap.Delete(dmsTaskName)
			if _, err := result.GetResult(); err == nil {
				return updateState(state, provisioningdpfv1alpha1.DPURebooting, "DPU is rebooting"), nil
			} else {
				return updateState(state, provisioningdpfv1alpha1.DPUError, err.Error()), err
			}
		}
	} else {
		dmsHandler(ctx, client, st.dpu, bfb)
	}

	return *state, nil
}

func updateState(state *provisioningdpfv1alpha1.DpuStatus, phase provisioningdpfv1alpha1.DpuPhase, message string) provisioningdpfv1alpha1.DpuStatus {
	state.Phase = phase
	cond := cutil.DPUCondition(provisioningdpfv1alpha1.DPUCondOSInstalled, "", message)
	if phase == provisioningdpfv1alpha1.DPUError {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "InstallationFailed"
	}
	cutil.SetDPUCondition(state, cond)
	return *state
}

func createGRPCConnection(ctx context.Context, client client.Client, dpu *provisioningdpfv1alpha1.Dpu) (*grpc.ClientConn, error) {
	nn := types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      cutil.GenerateDMSPodName(dpu.Name),
	}
	pod := &corev1.Pod{}
	if err := client.Get(ctx, nn, pod); err != nil {
		return nil, fmt.Errorf("failed to get pod: %v", err)
	}

	nn = types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      cutil.GenerateDMSClientSecretName(dpu.Namespace),
	}
	dmsClientSecret := &corev1.Secret{}
	if err := client.Get(ctx, nn, dmsClientSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("client secret not found: %v", err)
		} else {
			return nil, fmt.Errorf("failed to get client secret: %v", err)
		}
	}

	// Extract the certificate and key from the secret
	dmsClientCert, certOk := dmsClientSecret.Data["tls.crt"]
	if !certOk {
		return nil, fmt.Errorf("tls.crt not found in client secret")
	}
	dmsClientKey, keyOk := dmsClientSecret.Data["tls.key"]
	if !keyOk {
		return nil, fmt.Errorf("tls.key not found in client secret")
	}

	// Load the DMS client's certificate and private key
	clientCert, err := tls.X509KeyPair(dmsClientCert, dmsClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert and key: %v", err)
	}

	// Retrieve the DMS Server secret
	dmsServerSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: dpu.Namespace, Name: cutil.GenerateDMSServerSecretName(dpu.Name)}, dmsServerSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("Server secret not found: %v", err)
		} else {
			return nil, fmt.Errorf("failed to get Server secret: %v", err)
		}
	}

	// Extract the CA certificate from the Server secret
	serverCACert, caCertOk := dmsServerSecret.Data["ca.crt"]
	if !caCertOk {
		return nil, fmt.Errorf("ca.crt not found in Server secret")
	}

	// Create a certificate pool and add the CA certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(serverCACert) {
		return nil, fmt.Errorf("failed to append Server certificate")
	}

	// Create a mTLS config with the client certificate and CA certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	}

	serverAddress := pod.Status.PodIP + ":" + dms.ContainerPortStr

	// Create a gRPC connection using grpc.NewClient
	conn, err := grpc.NewClient(serverAddress, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %v", err)
	}

	return conn, nil
}

func dmsHandler(ctx context.Context, client client.Client, dpu *provisioningdpfv1alpha1.Dpu, bfb *provisioningdpfv1alpha1.Bfb) {
	dmsTaskName := generateDMSTaskName(dpu)
	dmsTask := future.New(func() (any, error) {
		logger := log.FromContext(ctx)
		logger.V(3).Info(fmt.Sprintf("DMS %s start os installation", dmsTaskName))

		conn, err := createGRPCConnection(ctx, client, dpu)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Error creating gRPC connection: %v", err))
			return future.Ready, err
		}
		defer conn.Close() //nolint: errcheck
		clients := gnoigo.NewClients(conn)

		logger.V(3).Info(fmt.Sprintf("TLS Connection established between DPU controller to DMS %s", dmsTaskName))

		osInstall := gos.NewInstallOperation()

		osInstall.Version(bfb.Spec.FileName)
		// Open the file at the specified path
		fullFileName := cutil.GenerateBFBFilePath(bfb.Spec.FileName)
		bfbfile, err := os.Open(fullFileName)
		if err != nil {
			logger.Error(err, "failed to open file", "name", bfb.Spec.FileName)
			return nil, err
		}
		defer bfbfile.Close() //nolint: errcheck
		osInstall.Reader(bfbfile)
		if response, err := gnoigo.Execute(ctx, clients, osInstall); err != nil {
			return nil, err
		} else {
			logger.V(3).Info(fmt.Sprintf("DMS %s Performed InstallOperation %v", dmsTaskName, response))
		}

		flavor := &provisioningdpfv1alpha1.DPUFlavor{}
		if err := client.Get(ctx, types.NamespacedName{
			Namespace: dpu.Namespace,
			Name:      dpu.Spec.DPUFlavor,
		}, flavor); err != nil {
			return nil, err
		}
		cfgfile, err := generateBFConfig(ctx, dpu, flavor)
		if err != nil {
			logger.Error(err, fmt.Sprintf("%s/%s generate bf config failed", dpu.Namespace, dpu.Name))
			return nil, err
		}
		config, err := os.Open(cfgfile)
		if err != nil {
			logger.Error(err, "failed to open file", "name", cfgfile)
			return nil, err
		}
		defer config.Close() //nolint: errcheck
		cfg := gos.NewInstallOperation()
		cfgVersion := path.Base(cfgfile)
		cfg.Version(cfgVersion)
		cfg.Reader(config)
		if response, err := gnoigo.Execute(ctx, clients, cfg); err != nil {
			return nil, err
		} else {
			logger.V(3).Info(fmt.Sprintf("DMS %s Performed BF configuration %v", dmsTaskName, response))
		}

		bfbPathInDms := dms.DMSImageFolder + string(filepath.Separator) + bfb.Spec.FileName
		if md5, err := computeBFBMD5InDms(dpu.Namespace, cutil.GenerateDMSPodName(dpu.Name), "", bfbPathInDms); err == nil {
			logger.V(3).Info(fmt.Sprintf("md5sum of %s in DMS is %s", bfb.Spec.FileName, md5))
		} else {
			logger.Error(err, "Failed to get md5sum from dms", "md5", dpu)
		}

		activateOp := gos.NewActivateOperation()
		noReboot := false
		activateOp.NoReboot(noReboot)
		activateOp.Version(fmt.Sprintf("%s;%s", bfb.Spec.FileName, cfgVersion))

		logger.V(3).Info("starting execute activate operation")
		var response *ospb.ActivateResponse
		for retry := 1; retry <= MaxRetryCount; retry++ {
			response, err = gnoigo.Execute(ctx, clients, activateOp)
			if err == nil {
				break
			} else {
				logger.Error(err, fmt.Sprintf("DMS %s failed to Execute activateOp, retry %d", dmsTaskName, retry))
			}
		}
		if err != nil {
			return nil, err
		}

		logger.V(3).Info("Performed activateOp", "response", response)

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Error(ctx.Err(), fmt.Sprintf("DMS %s RebootStatusOperation timed out or was cancelled", dmsTaskName))
				return nil, ctx.Err()
			case <-ticker.C:
				rebootStatusOp := gsystem.NewRebootStatusOperation().Subcomponents([]*tpb.Path{{Elem: []*tpb.PathElem{{Name: "CPU"}}}})
				response, err := gnoigo.Execute(ctx, clients, rebootStatusOp)
				if err != nil {
					logger.Error(err, fmt.Sprintf("DMS %s failed to Execute rebootStatusOp", dmsTaskName))
					return nil, err
				}
				if !response.Active {
					// Once the reboot is no longer active, check the status
					switch response.Status.Status {
					case system.RebootStatus_STATUS_SUCCESS:

						// TODO: wait for 90 sec for cloud-init
						cloudInitTimeout := CloudInitDefaultTimeout
						if value, exists := os.LookupEnv("CLOUD_INIT_TIMEOUT"); exists {
							if cloudInitTimeout, err = strconv.Atoi(value); err != nil {
								cloudInitTimeout = CloudInitDefaultTimeout
							}
						}

						time.Sleep(time.Duration(cloudInitTimeout) * time.Second)
						logger.V(3).Info(fmt.Sprintf("DMS %s rebooted DPU successfully", dmsTaskName))

						/*verifyOp := gos.NewVerifyOperation()
						if response, err := gnoigo.Execute(ctx, clients, verifyOp); err != nil {
							logger.Error(err, fmt.Sprintf("DMS %s failed to Execute verifyOp", dmsTaskName))
							return future.Ready, err
						} else {
							logger.V(3).Info("Performed verifyOp", "response", response)
							if response.Version != fmt.Sprintf("%s;%s", bfb.Spec.FileName, cfgVersion) {
								return future.Ready, fmt.Errorf("verifyOp failure: expected: %s, received: %s", fmt.Sprintf("%s;%s", bfb.Spec.FileName, cfgVersion), response.Version)
							}
						}
						*/

						if dpu.Spec.Cluster.NodeLabels == nil {
							dpu.Spec.Cluster.NodeLabels = make(map[string]string)
						}

						// Add DOCA_BFB version to DPU NodeLabels as it is found in DPU ARM under /etc/mlnx-release. e.g.,
						// cat /etc/mlnx-release
						// bf-bundle-2.7.0-33_24.04_ubuntu-22.04_prod
						dpu.Spec.Cluster.NodeLabels["provisioning.dpf.nvidia.com/DOCA-BFB-version"] = cutil.GenerateBFBVersionFromURL(bfb.Spec.URL)
						if err := client.Update(ctx, dpu); err != nil {
							return future.Ready, err
						}
						return nil, nil
					case system.RebootStatus_STATUS_RETRIABLE_FAILURE:
						logger.Error(fmt.Errorf("DMS %s reboot encountered a retriable failure: %s", dmsTaskName, response.Reason), fmt.Sprintf("DMS %s reboot failed", dmsTaskName))
						return nil, fmt.Errorf("retriable failure: %s", response.Reason)
					case system.RebootStatus_STATUS_FAILURE:
						logger.Error(fmt.Errorf("reboot failure: %s", response.Reason), fmt.Sprintf("DMS %s reboot failed", dmsTaskName))
						return nil, fmt.Errorf("reboot failure: %s", response.Reason)
					default:
						logger.Error(fmt.Errorf("unknown reboot status: %s", response.Status), fmt.Sprintf("DMS %s encountered an unknown reboot status", dmsTaskName))
						return nil, fmt.Errorf("unknown reboot status: %s", response.Status)
					}
				}
			}
		}
	})
	dutil.OsInstallTaskMap.Store(dmsTaskName, dmsTask)
}

func generateDMSTaskName(dpu *provisioningdpfv1alpha1.Dpu) string {
	return fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)
}

func generateBFConfig(ctx context.Context, dpu *provisioningdpfv1alpha1.Dpu, flavor *provisioningdpfv1alpha1.DPUFlavor) (string, error) {
	logger := log.FromContext(ctx)
	var dpusetName, dpusetNamespace string
	var ok bool
	dpusetName, ok = dpu.Labels[cutil.DpuSetNameLabel]
	if !ok {
		return "", fmt.Errorf("label %s is empty", cutil.DpuSetNameLabel)
	}
	dpusetNamespace, ok = dpu.Labels[cutil.DpuSetNamespaceLabel]
	if !ok {
		return "", fmt.Errorf("label %s is empty", cutil.DpuSetNamespaceLabel)
	}

	joinCommand, err := generateJoinCommand(dpusetName, dpusetNamespace)
	if err != nil {
		return "", err
	}

	buf, err := bfcfg.Generate(flavor, dpu.Name, joinCommand)
	if err != nil {
		return "", err
	}
	logger.V(3).Info(fmt.Sprintf("bf.cfg for %s is: %s", dpu.Name, string(buf)))
	cfgPath := cutil.GenerateBFConfigPath(dpu.Name)
	if err := os.WriteFile(cfgPath, buf, 0644); err != nil {
		return "", err
	}
	if len(buf) <= MaxBFSize {
		logger.V(3).Info(fmt.Sprintf("the size of %s is %d", cfgPath, len(buf)))
	} else {
		return "", fmt.Errorf("the file size of %s is %d exceeds which the maximum limit(%d)", cfgPath, len(buf), MaxBFSize)
	}

	return cfgPath, nil
}

func generateJoinCommand(dpuSetName, dpusetNamespace string) (string, error) {
	kubeConfigFile := cutil.GenerateKubeConfigFileName(dpuSetName, dpusetNamespace)
	kubeconfig := "--kubeconfig=" + kubeConfigFile
	cmd := exec.Command("kubeadm", "token", "create", "--print-join-command", kubeconfig)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	joinCommand := strings.TrimRight(string(output), "\r\n") + " --v=5"
	return joinCommand, nil
}

func computeBFBMD5InDms(ns, name, container, filepath string) (string, error) {
	md5CMD := fmt.Sprintf("md5sum %s", filepath)
	return cutil.RemoteExec(ns, name, container, md5CMD)
}
