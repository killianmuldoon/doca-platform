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

package util

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/provisioning/v1alpha1"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RequeueInterval            = 5 * time.Second
	BFBBaseDir                 = "bfb"
	CFGExtension               = ".cfg"
	DpuSetNameLabel            = "provisioning.dpf.nvidia.com/dpuset-name"
	DpuSetNamespaceLabel       = "provisioning.dpf.nvidia.com/dpuset-namespace"
	DpuPCIAddress              = "dpu-pciAddress"
	DpuPFName                  = "dpu-pf-name"
	DpuPCIAddressLabel         = "provisioning.dpf.nvidia.com/dpu-pciAddress"
	DpuPFNameLabel             = "provisioning.dpf.nvidia.com/dpu-pf-name"
	DpuHostIPLabel             = "provisioning.dpf.nvidia.com/dpu-host-ip"
	TolerationNotReadyKey      = "node.kubernetes.io/not-ready"
	TolerationUnreachableyKey  = "node.kubernetes.io/unreachable"
	TolerationNodeDrainKey     = "medik8s.io/drain"
	TolerationUnschedulableKey = "node.kubernetes.io/unschedulable"
	// clusterConfigConfigMapName is the name of the ConfigMap that contains the cluster configuration in
	// OpenShift.
	ClusterConfigConfigMapName = "cluster-config-v1"
	// clusterConfigNamespace is the Namespace where the OpenShift cluster configuration ConfigMap exists.
	ClusterConfigNamespace = "kube-system"
)

func GenerateBFBTaskName(bfb provisioningdpfv1alpha1.Bfb) string {
	return fmt.Sprintf("%s-%s", bfb.Namespace, bfb.Name)
}

func GenerateBFBFilePath(filename string) string {
	return string(os.PathSeparator) + BFBBaseDir + string(os.PathSeparator) + filename
}

func GenerateBFBVersionFromURL(bfbUrl string) string {
	base := path.Base(bfbUrl)
	version := strings.TrimSuffix(base, path.Ext(base))
	return version
}

func GenerateBFConfigPath(dpu string) string {
	return string(os.PathSeparator) + BFBBaseDir + string(os.PathSeparator) + dpu + CFGExtension
}

func GenerateKubeConfigFileName(dpusetName, dpusetNamespace string) string {
	return string(os.PathSeparator) + "kubeconfig" + string(os.PathSeparator) +
		fmt.Sprintf("%s-%s%s", dpusetNamespace, dpusetName, ".kubeconfig")
}

func GenerateDMSPodName(dpuName string) string {
	return fmt.Sprintf("%s-%s", dpuName, "dms")
}

func GenerateCACertName(dpuNamespace string) string {
	return fmt.Sprintf("%s-%s", dpuNamespace, "ca-cert")
}

func GenerateDMSServerCertName(dpuName string) string {
	return fmt.Sprintf("%s-%s", dpuName, "dms-server-cert")
}

func GenerateDMSClientCertName(dpuNamespace string) string {
	return fmt.Sprintf("%s-%s", dpuNamespace, "dms-client-cert")
}

func GenerateHostnetworkPodName(dpuName string) string {
	return fmt.Sprintf("%s-%s", dpuName, "hostnetwork")
}

func GenerateCASecretName(dpuNamespace string) string {
	return fmt.Sprintf("%s-%s", dpuNamespace, "ca-secret")
}

func GenerateDMSServerSecretName(dpuName string) string {
	return fmt.Sprintf("%s-%s", dpuName, "server-secret")
}

func GenerateDMSClientSecretName(dpuNamespace string) string {
	return fmt.Sprintf("%s-%s", dpuNamespace, "client-secret")
}

func GenerateDMSServerIssuerName(dpuName string) string {
	return fmt.Sprintf("%s-%s", dpuName, "dms-server-issuer")
}

func GenerateDMSClientIssuerName(dpuNamespace string) string {
	return fmt.Sprintf("%s-%s", dpuNamespace, "dms-client-issuer")
}

func RemoteExec(ns, name, container, cmd string) (string, error) {
	config, err := restclient.InClusterConfig()
	if err != nil {
		return "", nil
	}

	// Create a Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(name).
		Namespace(ns).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   strings.Fields(cmd),
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	out_buf := new(bytes.Buffer)
	err_buf := new(bytes.Buffer)
	if err := executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: bufio.NewWriter(out_buf),
		Stderr: bufio.NewWriter(err_buf),
		Tty:    false,
	}); err != nil {
		return err_buf.String(), err
	}

	return out_buf.String(), nil
}

func RetrieveK8sClientUsingKubeConfig(ctx context.Context, client crclient.Client, namespace string, name string) (crclient.Client, error) {
	secretName := fmt.Sprintf("%s-admin-kubeconfig", name)

	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, secret); err != nil {
		return nil, err
	}

	kubeConfig, ok := secret.Data["admin.conf"]
	if !ok {
		return nil, fmt.Errorf("kubeconfig not found in secret")
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	newClient, err := crclient.New(config, crclient.Options{})
	if err != nil {
		return nil, err
	}
	return newClient, nil
}

func IsNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func AddLabelsToNode(ctx context.Context, client crclient.Client, node *corev1.Node, labels map[string]string) error {

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	for key, value := range labels {
		node.Labels[key] = value
	}
	if err := client.Update(ctx, node); err != nil {
		return err
	}
	return nil
}

func DeleteObject(client crclient.Client, obj crclient.Object) error {
	err := client.Delete(context.TODO(), obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func GetObjects(client crclient.Client, objects []crclient.Object) (existObjects []crclient.Object, err error) {
	for _, obj := range objects {
		nn := types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}
		if err := client.Get(context.TODO(), nn, obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return existObjects, err
		}
		existObjects = append(existObjects, obj)
	}

	return existObjects, nil
}

func GetDPUCondition(status *provisioningdpfv1alpha1.DpuStatus, conditionType string) (int, *metav1.Condition) {
	if status == nil {
		return -1, nil
	}
	conditions := status.Conditions
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

func DPUCondition(condType provisioningdpfv1alpha1.DPUConditionType, reason, message string) *metav1.Condition {
	cond := &metav1.Condition{
		Type:    condType.String(),
		Status:  metav1.ConditionTrue,
		Message: message,
	}
	if reason != "" {
		cond.Reason = reason
	} else {
		cond.Reason = condType.String()
	}
	return cond
}

func SetDPUCondition(status *provisioningdpfv1alpha1.DpuStatus, condition *metav1.Condition) bool {
	condition.LastTransitionTime = metav1.Now()
	// Try to find this condition.
	conditionIndex, oldCondition := GetDPUCondition(status, condition.Type)

	if oldCondition == nil {
		// We are adding new condition.
		status.Conditions = append(status.Conditions, *condition)
		return true
	}
	// We are updating an existing condition, so we need to check if it has changed.
	if condition.Status == oldCondition.Status {
		condition.LastTransitionTime = oldCondition.LastTransitionTime
	}

	isEqual := condition.Status == oldCondition.Status &&
		condition.Reason == oldCondition.Reason &&
		condition.Message == oldCondition.Message &&
		condition.LastTransitionTime.Equal(&oldCondition.LastTransitionTime)

	status.Conditions[conditionIndex] = *condition
	// Return true if one of the fields have changed.
	return !isEqual
}

func ComputeMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	defer file.Close() //nolint: errcheck

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		fmt.Println(err)
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getHostCIDRFromOpenShiftClusterConfig extracts the Host CIDR from the given OpenShift Cluster Configuration.
func GetHostCIDRFromOpenShiftClusterConfig(ctx context.Context, client crclient.Client) (net.IPNet, error) {
	openshiftClusterConfig := &corev1.ConfigMap{}

	nn := types.NamespacedName{
		Namespace: ClusterConfigNamespace,
		Name:      ClusterConfigConfigMapName,
	}

	if err := client.Get(ctx, nn, openshiftClusterConfig); err != nil {
		return net.IPNet{}, fmt.Errorf("error while getting %s %s: %w", openshiftClusterConfig.GetObjectKind().GroupVersionKind().String(), nn.String(), err)
	}

	// Unfortunately I couldn't find good documentation for what fields are available to add a link here. The best I could
	// find is this IBM specific (?) documentation:
	// https://docs.openshift.com/container-platform/4.14/installing/installing_ibm_cloud/install-ibm-cloud-installation-workflow.html#additional-install-config-parameters_install-ibm-cloud-installation-workflow
	type machineNetworkEntry struct {
		CIDR string `yaml:"cidr"`
	}
	type installConfig struct {
		Networking struct {
			MachineNetwork []machineNetworkEntry `yaml:"machineNetwork"`
		} `yaml:"networking"`
	}

	var config installConfig
	data, ok := openshiftClusterConfig.Data["install-config"]
	if !ok {
		return net.IPNet{}, errors.New("install-config key is not found in ConfigMap data")
	}

	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return net.IPNet{}, fmt.Errorf("error while unmarshalling data into struct: %w", err)
	}

	if len(config.Networking.MachineNetwork) == 0 {
		return net.IPNet{}, errors.New("host CIDR not found in cluster config")
	}

	// We use the first CIDR that we find. If there are clusters with multiple CIDRs defined here, we need to adjust the
	// logic and see how each CIDR is correlated with the primary IP of the node. Ultimately, we want to CIDR that contains
	// the primary IP of the node or else, the encap IP that is set by the OVN Kubernetes for the node.
	cidrRaw := config.Networking.MachineNetwork[0].CIDR
	_, cidr, err := net.ParseCIDR(cidrRaw)
	if err != nil {
		return net.IPNet{}, fmt.Errorf("error while parsing CIDR from %s: %w", cidrRaw, err)
	}

	return *cidr, nil
}

func GeneratePodToleration(nodeEffect provisioningdpfv1alpha1.NodeEffect) []corev1.Toleration {
	tolerations := []corev1.Toleration{
		{
			Key:      TolerationNotReadyKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		},
		{
			Key:      TolerationUnreachableyKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		},
		{
			Key:      TolerationNodeDrainKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      TolerationNodeDrainKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		},
		{
			Key:      TolerationUnschedulableKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	if nodeEffect.Taint != nil {
		t := corev1.Toleration{
			Key:      nodeEffect.Taint.Key,
			Operator: corev1.TolerationOpEqual,
			Value:    nodeEffect.Taint.Value,
			Effect:   nodeEffect.Taint.Effect,
		}
		tolerations = append(tolerations, t)
	}

	return tolerations
}
