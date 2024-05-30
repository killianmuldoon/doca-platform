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
	"fmt"
	"os"
	"os/exec"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ServiceInterfaceNameLabel      = "sfc.dpf.nvidia.com/ServiceInterface-name"
	ServiceInterfaceNamespaceLabel = "sfc.dpf.nvidia.com/ServiceInterface-namespace"
	ServiceInterfaceControllerName = "service-interface-controller"
	ServiceInterfaceFinalizer      = "sfc.dpf.nvidia.com/ServiceInterface-finalizer"
)

// ServiceInterfaceReconciler reconciles a ServiceInterface object
type ServiceInterfaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func runOVSVsctl(args ...string) error {
	ovsVsctlPath, err := exec.LookPath("ovs-vsctl")
	if err != nil {
		return err
	}
	cmd := exec.Command(ovsVsctlPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error running ovs-vsctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
	}
	return nil
}

// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceInterfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceInterfaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceInterfaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
func DelPort(portName string) error {
	return runOVSVsctl("--if-exist", "del-port", portName)
}

func AddPort(portName string) error {
	return runOVSVsctl("--may-exist", "add-port", "br-sfc", portName, "--", "set", "int", portName, "type=dpdk")
}

func AddPatchPort(portName string, brA string, brB string) error {
	// portBrA := fmt.Sprintf("%s-%s", portName, brA)
	// portBrB := fmt.Sprintf("%s-%s", portName, brB)
	// match on ovs-cni naming
	portBrA := "puplinkbrovn"
	portBrB := "puplinkbrsfc"
	err := runOVSVsctl("--may-exist", "add-port", brA, portBrA, "--", "set", "int", portBrA, "type=patch",
		"--", "set", "int", portBrA, fmt.Sprintf("options:peer=%s", portBrB))
	if err != nil {
		return err
	}
	err = runOVSVsctl("--may-exist", "add-port", brB, portBrB, "--", "set", "int", portBrB, "type=patch",
		"--", "set", "int", portBrB, fmt.Sprintf("options:peer=%s", portBrA))

	return err
}

func FigureOutName(ctx context.Context, serviceInterface *sfcv1.ServiceInterface) string {
	log := log.FromContext(ctx)
	portName := ""
	if serviceInterface.Spec.InterfaceType == "physical" {
		log.Info("matched on physical")
		portName = serviceInterface.Spec.InterfaceName
	}
	if serviceInterface.Spec.InterfaceType == "pf" {
		log.Info("matched on pf")
		portName = fmt.Sprintf("pf%dhpf", serviceInterface.Spec.PF.ID)
	}
	if serviceInterface.Spec.InterfaceType == "vf" {
		log.Info("matched on vf")
		portName = fmt.Sprintf("pf%dvf%d", serviceInterface.Spec.VF.PFID, serviceInterface.Spec.VF.VFID)
	}
	if serviceInterface.Spec.InterfaceType == "vlan" {
		log.Info("matched on vlan skipping")
		// TODO for MVP it is out of scope
		// revisit this once we do not have collisions on patches.
	}

	return portName
}

func AddInterfacesToOvs(ctx context.Context, serviceInterface *sfcv1.ServiceInterface) error {
	log := log.FromContext(ctx)

	if serviceInterface.Spec.InterfaceType == "ovn" {
		log.Info("matched on ovn")
		err := AddPatchPort("patch", "br-ovn", "br-sfc")
		if err != nil {
			log.Info(fmt.Sprintf("failed to add port %s", err.Error()))
			return err
		}
		return nil
	}

	portName := FigureOutName(ctx, serviceInterface)

	if portName != "" {
		err := AddPort(portName)
		if err != nil {
			log.Info(fmt.Sprintf("failed to add port %s", err.Error()))
			return err
		}
	}

	return nil
}

func DeleteInterfacesFromOvs(ctx context.Context, serviceInterface *sfcv1.ServiceInterface) error {
	log := log.FromContext(ctx)
	log.Info("DeleteInterfacesFromOvs")

	if serviceInterface.Spec.InterfaceType == "ovn" {
		log.Info("matched on ovn")
		// match on ovs-cni naming
		portBrA := "puplinkbrovn"
		portBrB := "puplinkbrsfc"
		err := DelPort(portBrA)
		if err != nil {
			log.Info(fmt.Sprintf("failed to delete port %s", err.Error()))
			return err
		}
		err = DelPort(portBrB)
		if err != nil {
			log.Info(fmt.Sprintf("failed to delete port %s", err.Error()))
			return err
		}
		return nil
	}

	portName := FigureOutName(ctx, serviceInterface)

	if portName != "" {
		err := DelPort(portName)
		if err != nil {
			log.Info(fmt.Sprintf("failed to add port %s", err.Error()))
			return err
		}
	}
	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceInterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling")
	serviceInterface := &sfcv1.ServiceInterface{}

	if err := r.Client.Get(ctx, req.NamespacedName, serviceInterface); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			log.Info("Object not found")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	node := os.Getenv("SFC_NODE_NAME")

	if serviceInterface.Spec.Node != node {
		// this object was not intended for this nodes
		// skip
		log.Info("serviceInterface.Spec.Node: %s != node: %s", serviceInterface.Spec.Node, node)
		return ctrl.Result{}, nil
	}

	if !serviceInterface.ObjectMeta.DeletionTimestamp.IsZero() {

		err := DeleteInterfacesFromOvs(ctx, serviceInterface)
		if err != nil {
			log.Info("Failed to delete DeleteInterfacesFromOvs")
			return ctrl.Result{}, err
		}

		// If there are no associated applications remove the finalizer
		log.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(serviceInterface, ServiceInterfaceFinalizer)
		if err := r.Client.Update(ctx, serviceInterface); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	err := AddInterfacesToOvs(ctx, serviceInterface)
	if err != nil {
		log.Info("Failed to delete AddInterfacesToOvs")
		return ctrl.Result{}, err
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(serviceInterface, ServiceInterfaceFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(serviceInterface, ServiceInterfaceFinalizer)
		err := r.Update(ctx, serviceInterface)
		if err != nil {
			log.Info("Failed to add finalizer")
			return ctrl.Result{}, err
		} else {
			log.Info("Added finalizer")
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceInterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfcv1.ServiceInterface{}).
		Complete(r)
}
