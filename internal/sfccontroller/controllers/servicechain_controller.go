/*
COPYRIGHT 2024 NVIDIA

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

package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"strings"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceChainReconciler reconciles a ServiceChain object
type ServiceChainReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
}

const (
	ServiceChainNameLabel      = "sfc.dpf.nvidia.com/ServiceChain-name"
	ServiceChainNamespaceLabel = "sfc.dpf.nvidia.com/ServiceChain-namespace"
	ServiceChainControllerName = "service-chain-controller"
	RequeueIntervalFlows       = 5 * time.Second

	podNodeNameKey = "spec.nodeName"
)

func requeueFlows() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: RequeueIntervalFlows}, nil
}

// Hashing function, will be used when adding and removing OpenFlow flows
// This hash will take in the service chain name and return the corresponding hash
func hash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// Utility function to find an OVS interface based on the condition that the
// external_id:dpf-id=condition which is sent as an input
func checkPortInBrSfc(condition string) (bool, error) {
	ovsVsctlPath, err := exec.LookPath("ovs-vsctl")
	if err != nil {
		return false, err
	}

	// Figure out if the port is on the expected sfc-chain bridge (br-sfc)
	args := []string{"-t", "5", "iface-to-br", condition}
	cmd := exec.Command(ovsVsctlPath, args...)
	var stderr bytes.Buffer
	var output bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &output
	err = cmd.Run()
	if err != nil {
		return false, fmt.Errorf("error running ovs-vsctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
	}

	if strings.TrimSuffix(output.String(), "\n") == "br-sfc" {
		return true, nil
	}

	return false, nil
}

// Utility function to find an OVS interface based on the condition that the
// external_id:dpf-id=condition which is sent as an input
func findInterface(condition string) (string, error) {
	ovsVsctlPath, err := exec.LookPath("ovs-vsctl")
	if err != nil {
		return "", err
	}
	args := []string{"-t", "5", "--oneline", "--no-heading", "--format=csv", "--data=bare",
		"--columns=name", "find", "interface", "external_ids:dpf-id=" + condition}
	cmd := exec.Command(ovsVsctlPath, args...)
	var stderr bytes.Buffer
	var output bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &output
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error running ovs-vsctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
	}

	portName := strings.TrimSuffix(output.String(), "\n")

	if portName == "" {
		return "", fmt.Errorf("could not find ovs interface with external_ids:dpf-id=%s", condition)
	}

	found, err := checkPortInBrSfc(portName)
	if err != nil {
		return "", err
	}

	if found {
		args := []string{"-t", "5", "--oneline", "--no-heading", "--format=csv", "--data=bare",
			"--columns=ofport", "find", "interface", "name=" + portName}
		cmd = exec.Command(ovsVsctlPath, args...)
		stderr.Reset()
		output.Reset()
		cmd.Stderr = &stderr
		cmd.Stdout = &output
		err = cmd.Run()
		if err != nil {
			return "", fmt.Errorf("error running ovs-vsctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
		}
		return strings.TrimSuffix(output.String(), "\n"), nil
	}

	// Build br-hbn patch p + intfname + brsfc
	portNameOld := portName
	portName = "p" + portNameOld + "brsfc"
	found, err = checkPortInBrSfc(portName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("port %s or %s not found in br-sfc", portNameOld, portName)
	}

	args = []string{"-t", "5", "--oneline", "--no-heading", "--format=csv", "--data=bare",
		"--columns=ofport", "find", "interface", "name=" + portName}
	cmd = exec.Command(ovsVsctlPath, args...)
	stderr.Reset()
	output.Reset()
	cmd.Stderr = &stderr
	cmd.Stdout = &output
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error running ovs-vsctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
	}
	return strings.TrimSuffix(output.String(), "\n"), nil
}

// Utility function to delete all flows which have a corresponding cookie
func delFlows(flows string) error {
	ovsOfctlPath, err := exec.LookPath("ovs-ofctl")
	if err != nil {
		return err
	}
	args := []string{"-t", "5", "--bundle", "del-flows", "br-sfc", flows}
	cmd := exec.Command(ovsOfctlPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error running ovs-ofctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
	}
	return nil
}

// Utility function which takes in a multiline string called flows
// Creates a temporary file on the system and writes the aforementioned
// string. This will file will be consumed by ovs-ofctl command with the
// bundle argument to ensure the fact that all flows are added in a atomic operation
func addFlows(ctx context.Context, flows string) (err error) {
	log := log.FromContext(ctx)
	var fileP *os.File
	fileP, err = os.Create("/tmp/of-output.txt")
	if err != nil {
		return err
	}
	defer func() {
		e := fileP.Close()
		if e != nil {
			err = errors.Join(e, err)
		}
	}()
	// do the actual work
	_, err = fileP.WriteString(flows)
	if err != nil {
		return err
	}
	ovsOfctlPath, err := exec.LookPath("ovs-ofctl")
	if err != nil {
		return err
	}
	args := []string{"-t", "5", "--bundle", "add-flows", "br-sfc", "/tmp/of-output.txt"}
	cmd := exec.Command(ovsOfctlPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error running ovs-ofctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
	}
	log.Info("Added flows:")
	log.Info(flows)
	return err
}

// getPodWithLabels returns pod in namespace that is scheduled on current node with given labels. if more than one or none matches, error out.
func (r *ServiceChainReconciler) getPodWithLabels(ctx context.Context, namespace string, lbls map[string]string) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{}
	listOpts = append(listOpts, client.MatchingLabels(lbls))
	listOpts = append(listOpts, client.MatchingFieldsSelector{Selector: fields.OneTermEqualSelector(podNodeNameKey, r.NodeName)})
	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if err := r.List(ctx, podList, listOpts...); err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pod in namespace(%s) matching labels(%v) on node(%s) found", namespace, lbls, r.NodeName)
	}

	if len(podList.Items) > 1 {
		return nil, fmt.Errorf("expected only one pod in namespace(%s) to match labels(%v) on node(%s). found %d",
			namespace, lbls, r.NodeName, len(podList.Items))
	}

	return &podList.Items[0], nil
}

// getServiceInterfaceWithLabels returns ServiceInterface in namespace that belongs to current node with given labels. if more than one or none matches, error out.
func (r *ServiceChainReconciler) getServiceInterfaceWithLabels(ctx context.Context, namespace string, lbls map[string]string) (*sfcv1.ServiceInterface, error) {
	sil := &sfcv1.ServiceInterfaceList{}
	listOpts := []client.ListOption{}

	listOpts = append(listOpts, client.MatchingLabels(lbls))
	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if err := r.List(ctx, sil, listOpts...); err != nil {
		return nil, err
	}

	// filter out serviceInterfaces not on this node
	matching := make([]*sfcv1.ServiceInterface, 0, len(sil.Items))
	for i := range sil.Items {
		if sil.Items[i].Spec.Node == nil ||
			*sil.Items[i].Spec.Node != r.NodeName {
			continue
		}
		matching = append(matching, &sil.Items[i])
	}

	if len(matching) == 0 {
		return nil, fmt.Errorf("no serviceInterface in namespace(%s) matching labels(%v) on node(%s) found", namespace, lbls, r.NodeName)
	}

	if len(matching) > 1 {
		return nil, fmt.Errorf("expected only one serviceInterface in namespace(%s) to match labels(%v) on node(%s). found %d",
			namespace, lbls, r.NodeName, len(sil.Items))
	}

	return matching[0], nil
}

// getPortNameForService returns the ovs port name matching the given service
func (r *ServiceChainReconciler) getPortNameForService(ctx context.Context, namespace string, svc *sfcv1.Service) (string, error) {
	pod, err := r.getPodWithLabels(ctx, namespace, svc.MatchLabels)
	if err != nil {
		return "", err
	}

	condition := pod.Namespace + "/" + pod.Name + "/" + svc.InterfaceName
	port, err := findInterface(condition)
	if err != nil {
		return "", err
	}

	if port == "" {
		return "", fmt.Errorf("port with condition %s not found", condition)
	}

	return port, nil
}

// getPortNameForServiceInterface returns the ovs port name matching the given service interface
func (r *ServiceChainReconciler) getPortNameForServiceInterface(ctx context.Context, namespace string, svcIfc *sfcv1.ServiceIfc) (string, error) {
	si, err := r.getServiceInterfaceWithLabels(ctx, namespace, svcIfc.MatchLabels)
	if err != nil {
		return "", err
	}

	condition := si.Namespace + "/" + si.Name
	if si.Spec.InterfaceType == sfcv1.InterfaceTypeService {
		// get pod matching serviceID
		pod, err := r.getPodWithLabels(ctx, namespace, map[string]string{dpuservicev1.DPFServiceIDLabelKey: si.Spec.Service.ServiceID})
		if err != nil {
			return "", err
		}
		if si.Spec.InterfaceName == nil || *si.Spec.InterfaceName == "" {
			return "", errors.New("nil or empty interface name for serviceInterface of type service")
		}
		// construct condition which identifies ovs port for the interface that is associated
		// with service and serviceInterface.
		condition = pod.Namespace + "/" + pod.Name + "/" + *si.Spec.InterfaceName
	}

	port, err := findInterface(condition)
	if err != nil {
		return "", err
	}

	if port == "" {
		return "", fmt.Errorf("port with condition %s not found", condition)
	}

	return port, nil
}

//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes;s,verbs=get;list;watch

func (r *ServiceChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling")
	sc := &sfcv1.ServiceChain{}
	if err := r.Client.Get(ctx, req.NamespacedName, sc); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			// Always ensure delete operation in case of errors
			flowErrors := delFlows(fmt.Sprintf("cookie=%d/-1", hash(req.NamespacedName.String())))
			if flowErrors != nil {
				log.Error(flowErrors, "failed to delete flows")
				return requeueFlows()
			}
			return requeueFlows()
		}
		log.Error(err, "failed to get ServiceChain")
		return requeueFlows()
	}

	if sc.Spec.Node == nil {
		log.Info("sc.Spec.Node: not set, skip the object")
		return requeueFlows()
	}
	if *sc.Spec.Node != r.NodeName {
		// this object was not intended for this nodes
		// skip
		log.Info(fmt.Sprintf("sc.Spec.Node: %s != node: %s", *sc.Spec.Node, r.NodeName))
		return requeueFlows()
	}

	// Construct an array of arrays of ports in order to traverse it
	// and construct the flows
	var err error
	var ports [][]string
	for swPos, sw := range sc.Spec.Switches {
		ports = append(ports, [][]string{{}}...)
		for _, port := range sw.Ports {
			if port.Service != nil {
				servicePort, err := r.getPortNameForService(ctx, sc.Namespace, port.Service)
				if err != nil {
					log.Error(err, "failed to get port")
					return requeueFlows()
				}
				if servicePort != "" {
					ports[swPos] = append(ports[swPos], servicePort)
				}
			}
			if port.ServiceInterface != nil {
				intfName, err := r.getPortNameForServiceInterface(ctx, sc.Namespace, port.ServiceInterface)
				if err != nil {
					log.Error(err, "failed to get interface")
					return requeueFlows()
				}
				if intfName != "" {
					ports[swPos] = append(ports[swPos], intfName)
				}
			}
		}
	}

	// Go through the array of arrays `ports`
	// Sample of generated flows inside an array
	// ovs-ofctl add-flow br-sfc "in_port=$a,actions=learn(idle_timeout=10,priority=1,in_port=$b,dl_dst=dl_src,output:NXM_OF_IN_PORT[]),learn(idle_timeout=10,priority=1,
	//													   in_port=$c,dl_dst=dl_src,output:NXM_OF_IN_PORT[]),output:$b,output:$c"
	// ovs-ofctl add-flow br-sfc "in_port=$b,actions=learn(idle_timeout=10,priority=1,in_port=$a,dl_dst=dl_src,output:NXM_OF_IN_PORT[]),learn(idle_timeout=10,priority=1,
	//													   in_port=$c,dl_dst=dl_src,output:NXM_OF_IN_PORT[]),output:$a,output:$c"
	// ovs-ofctl add-flow br-sfc "in_port=$c,actions=learn(idle_timeout=10,priority=1,in_port=$a,dl_dst=dl_src,output:NXM_OF_IN_PORT[]),learn(idle_timeout=10,priority=1,
	//													   in_port=$b,dl_dst=dl_src,output:NXM_OF_IN_PORT[]),output:$a,output:$b"
	for arrayPos := range ports {
		// Reset flows string
		flowsPerArray := ""
		for i, arrayPort := range ports[arrayPos] {
			if len(ports[arrayPos]) < 2 {
				// We need at least two elements to construct flows
				continue
			}

			if flowsPerArray != "" {
				// Add new line for each position
				flowsPerArray += "\n"
			}

			// Add unique cookie based on hashing the namespace name together with the table, priority constants and input port
			// this will result in the following string:
			//  cookie=0x24592fc503504d3, table=0, priority=20, in_port=97 actions=
			flowsPerArray += fmt.Sprintf("cookie=%d, table=0, priority=20, in_port=%s actions=", hash(req.NamespacedName.String()), arrayPort)

			// Reset output string
			outputFlowPart := ""
			// Reset learn string
			learnAction := ""

			for j, iter := range ports[arrayPos] {
				if i == j {
					// Skip self
					continue
				}

				if learnAction != "" {
					// If it's not the first learn action add comma
					learnAction += ","
				}

				// Add learn action
				learnAction += fmt.Sprintf("learn(idle_timeout=10,table=0,priority=30,in_port=%s,dl_dst=dl_src,output:NXM_OF_IN_PORT[])", iter)

				if outputFlowPart != "" {
					// If it's not the first output action add comma
					outputFlowPart += ","
				}
				// Add output action
				outputFlowPart += fmt.Sprintf("output:%s", iter)
			}
			if learnAction != "" && outputFlowPart != "" {
				flowsPerArray += learnAction + "," + outputFlowPart
			}
		}

		// Try adding flows to vswitchd
		err = addFlows(ctx, flowsPerArray)
		if err != nil {
			log.Error(err, "failed to add flows")
			return requeueFlows()
		}
	}
	return requeueFlows()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceChainReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	err := mgr.GetCache().IndexField(ctx, &corev1.Pod{}, podNodeNameKey, func(o client.Object) []string {
		return []string{o.(*corev1.Pod).Spec.NodeName}
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		// TODO Match current node and go only if we have a match
		For(&sfcv1.ServiceChain{}).
		Complete(r)
}
