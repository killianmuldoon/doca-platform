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

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceChainReconciler reconciles a ServiceChain object
type ServiceChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	ServiceChainNameLabel      = "sfc.dpf.nvidia.com/ServiceChain-name"
	ServiceChainNamespaceLabel = "sfc.dpf.nvidia.com/ServiceChain-namespace"
	ServiceChainControllerName = "service-chain-controller"
)

// Hashing function, will be used when adding and removing OpenFlow flows
// This hash will take in the service chain name and return the coresponding hash
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

// Retrieve pod list base on matching labels
func getPort(ctx context.Context, k8sClient client.Client, matchingLabels map[string]string, containerIntfName string) (string, error) {
	podList := &corev1.PodList{}
	listOptions := client.MatchingLabels(matchingLabels)
	var condition string
	var err error
	if err = k8sClient.List(ctx, podList, &listOptions); err != nil {
		return "", err
	}
	var port string
	for _, pod := range podList.Items {
		if pod.Spec.NodeName != os.Getenv("SFC_NODE_NAME") {
			err = errors.New("Skipped different name. Pod not found")
			continue
		}
		condition = pod.Namespace + "/" + pod.GetName() + "/" + containerIntfName
		port, err = findInterface(condition)
		if err != nil {
			return "", err
		}
		// Consider only first element
		// TODO add checks for multiple elements and disallow such a configuration
		if port != "" {
			return port, nil
		} else {
			err = fmt.Errorf("Port with condition %s not found", condition)
		}
	}

	return "", err
}

// Retrieve port list base on matching labels
func getInterface(ctx context.Context, k8sClient client.Client, matchingLabels map[string]string) (string, error) {
	interfaceList := &sfcv1.ServiceInterfaceList{}
	listOptions := client.MatchingLabels(matchingLabels)
	var condition string
	var err error
	if err = k8sClient.List(ctx, interfaceList, &listOptions); err != nil {
		return "", err
	}
	var port string
	for _, intf := range interfaceList.Items {
		if intf.Spec.Node == nil {
			continue
		}
		if *intf.Spec.Node != os.Getenv("SFC_NODE_NAME") {
			err = errors.New("Skipped different name. Interface not found")
			continue
		}
		condition = intf.Namespace + "/" + intf.GetName()
		port, err = findInterface(condition)
		if err != nil {
			return "", err
		}
		// Consider only first element
		// TODO add checks for multiple elements and disallow such a configuration
		if port != "" {
			return port, nil
		} else {
			err = fmt.Errorf("Interface with condition %s not found", condition)
		}
	}

	return "", err
}

//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceChains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceChains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceChains/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *ServiceChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling")
	sc := &sfcv1.ServiceChain{}
	if err := r.Client.Get(ctx, req.NamespacedName, sc); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			// Always ensure delete operation in case of errors
			flowErrors := delFlows(fmt.Sprintf("cookie=%d/-1", hash(req.NamespacedName.String())))
			if flowErrors != nil {
				log.Error(flowErrors, "failed to delete flows")
				return ctrl.Result{}, flowErrors
			}

			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	node := os.Getenv("SFC_NODE_NAME")
	if sc.Spec.Node == nil {
		log.Info("sc.Spec.Node: not set, skip the object")
		return ctrl.Result{}, nil
	}
	if *sc.Spec.Node != node {
		// this object was not intended for this nodes
		// skip
		log.Info(fmt.Sprintf("sc.Spec.Node: %s != node: %s", *sc.Spec.Node, node))
		return ctrl.Result{}, nil
	}

	// Construct an array of arrays of ports in order to traverse it
	// and construct the flows
	var err error
	var ports [][]string
	for sw_pos, sw := range sc.Spec.Switches {
		ports = append(ports, [][]string{{}}...)
		for _, port := range sw.Ports {
			if port.Service != nil {
				servicePort, err := getPort(ctx, r.Client, port.Service.MatchLabels, port.Service.InterfaceName)
				if servicePort != "" {
					ports[sw_pos] = append(ports[sw_pos], servicePort)
				}
				if err != nil {
					log.Error(err, "failed to get port")
					return ctrl.Result{}, err
				}
			}
			if port.ServiceInterface != nil {
				intfName, err := getInterface(ctx, r.Client, port.ServiceInterface.MatchLabels)
				if intfName != "" {
					ports[sw_pos] = append(ports[sw_pos], intfName)
				}
				if err != nil {
					log.Error(err, "failed to get interface")
					return ctrl.Result{}, err
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
			return ctrl.Result{}, err
		}
	}

	// Always requeue after 5 seconds in order to do another sweep over the rules
	// This ensures the flow addition / removal in case ovs-vswitchd stops / crashes
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// TODO Match current node and go only if we have a match
		For(&sfcv1.ServiceChain{}).
		Complete(r)
}
