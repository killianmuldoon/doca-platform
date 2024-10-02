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
	"os/exec"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ServiceInterfaceFinalizer = "svc.dpu.nvidia.com/ServiceInterface-finalizer"
	RequeueIntervalSuccess    = 20 * time.Second
	RequeueIntervalError      = 5 * time.Second
)

func requeueSuccess() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: RequeueIntervalSuccess}, nil
}

func requeueError() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
}

// ServiceInterfaceReconciler reconciles a ServiceInterface object
type ServiceInterfaceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
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

// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=ServiceInterfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=ServiceInterfaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=ServiceInterfaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=serviceinterfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=serviceinterfaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
func DelPort(portName string) error {
	return runOVSVsctl("-t", "5", "--if-exist", "del-port", portName)
}

func AddPort(portName string, metadata string) error {
	return runOVSVsctl("-t", "5", "--may-exist", "add-port", "br-sfc", portName, "--", "set", "int", portName, "type=dpdk",
		"--", "set", "int", portName, "external_ids:dpf-id="+metadata)
}

func AddPatchPort(brA, brB, metadata string) error {
	// match on ovs-cni naming
	portBrA := "puplinkbrovn"
	portBrB := "puplinkbrsfc"
	err := runOVSVsctl("-t", "5", "--may-exist", "add-port", brA, portBrA, "--", "set", "int", portBrA, "type=patch",
		"--", "set", "int", portBrA, fmt.Sprintf("options:peer=%s", portBrB))
	if err != nil {
		return err
	}
	err = runOVSVsctl("-t", "5", "--may-exist", "add-port", brB, portBrB, "--", "set", "int", portBrB, "type=patch",
		"--", "set", "int", portBrB, fmt.Sprintf("options:peer=%s", portBrA),
		"--", "set", "int", portBrB, "external_ids:dpf-id="+metadata)

	return err
}

func FigureOutName(ctx context.Context, serviceInterface *dpuservicev1.ServiceInterface) string {
	log := ctrllog.FromContext(ctx)
	portName := ""

	switch serviceInterface.Spec.InterfaceType {
	case dpuservicev1.InterfaceTypePhysical:
		log.Info("matched on physical")
		portName = *serviceInterface.Spec.InterfaceName
	case dpuservicev1.InterfaceTypePF:
		log.Info("matched on pf")
		portName = fmt.Sprintf("pf%dhpf", serviceInterface.Spec.PF.ID)
	case dpuservicev1.InterfaceTypeVF:
		log.Info("matched on vf")
		portName = fmt.Sprintf("pf%dvf%d", serviceInterface.Spec.VF.PFID, serviceInterface.Spec.VF.VFID)
	case dpuservicev1.InterfaceTypeVLAN:
		log.Info("matched on vlan skipping")
		// TODO for MVP it is out of scope
		// revisit this once we do not have collisions on patches.
	case dpuservicev1.InterfaceTypeService:
		// service interfaces add/remove from ovs is handled part of CNI ADD/DEL flow
		log.Info("matched on service skipping")
	}

	return portName
}

func AddInterfacesToOvs(ctx context.Context, serviceInterface *dpuservicev1.ServiceInterface, metadata string) error {
	log := ctrllog.FromContext(ctx)

	if serviceInterface.Spec.InterfaceType == dpuservicev1.InterfaceTypeOVN {
		log.Info("matched on ovn")
		err := AddPatchPort("br-ovn", "br-sfc", metadata)
		if err != nil {
			log.Info(fmt.Sprintf("failed to add port %s", err.Error()))
			return err
		}
		return nil
	}

	portName := FigureOutName(ctx, serviceInterface)

	if portName != "" {
		err := AddPort(portName, metadata)
		if err != nil {
			log.Info(fmt.Sprintf("failed to add port %s", err.Error()))
			return err
		}
	}

	return nil
}

func DeleteInterfacesFromOvs(ctx context.Context, serviceInterface *dpuservicev1.ServiceInterface) error {
	log := ctrllog.FromContext(ctx)
	log.Info("deleteInterfacesFromOvs")

	if serviceInterface.Spec.InterfaceType == dpuservicev1.InterfaceTypeOVN {
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

	if serviceInterface.Spec.InterfaceType == dpuservicev1.InterfaceTypePhysical {
		log.Info("ignoring delete on physical interfaces.")
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
	log := ctrllog.FromContext(ctx)
	log.Info("reconciling")
	serviceInterface := &dpuservicev1.ServiceInterface{}
	if err := r.Client.Get(ctx, req.NamespacedName, serviceInterface); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			log.Info("object not found")
			return requeueSuccess()
		}
		log.Error(err, "failed to get ServiceInterface")
		return requeueError()
	}

	if serviceInterface.Spec.Node == nil {
		log.Info("si.Spec.Node: not set, skip the object")
		return requeueSuccess()
	}

	if *serviceInterface.Spec.Node != r.NodeName {
		// this object was not intended for this nodes
		// skip
		log.Info("serviceInterface.Spec.Node: %s != node: %s", *serviceInterface.Spec.Node, r.NodeName)
		return ctrl.Result{}, nil
	}

	if !serviceInterface.ObjectMeta.DeletionTimestamp.IsZero() {

		err := DeleteInterfacesFromOvs(ctx, serviceInterface)
		if err != nil {
			log.Error(err, "failed to delete DeleteInterfacesFromOvs")
			return requeueError()
		}

		// If there are no associated applications remove the finalizer
		log.Info("removing finalizer")
		controllerutil.RemoveFinalizer(serviceInterface, ServiceInterfaceFinalizer)
		if err := r.Client.Update(ctx, serviceInterface); err != nil {
			return requeueError()
		}
		return requeueSuccess()
	}

	err := AddInterfacesToOvs(ctx, serviceInterface, req.NamespacedName.String())
	if err != nil {
		log.Info("failed to add AddInterfacesToOvs")
		return requeueError()
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(serviceInterface, ServiceInterfaceFinalizer) {
		log.Info("adding finalizer")
		controllerutil.AddFinalizer(serviceInterface, ServiceInterfaceFinalizer)
		err := r.Update(ctx, serviceInterface)
		if err != nil {
			log.Error(err, "failed to add finalizer")
			return requeueError()
		}
		log.Info("added finalizer")
		return requeueSuccess()
	}
	return requeueSuccess()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceInterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.ServiceInterface{}).
		Complete(r)
}
