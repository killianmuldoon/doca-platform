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

package controllers

import (
	"context"
	"fmt"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type dpuDuplicateReconciler interface {
	createOrUpdateChild(context.Context, client.Client, client.Object) error
}

func reconcileDelete(ctx context.Context, k8sClient client.Client,
	replicatedObject client.Object, dpuServiceObject client.Object, finalizer string) error {
	log := ctrllog.FromContext(ctx)
	//TODO implement list clusters with label selector
	clusters, err := controlplane.GetDPFClusters(ctx, k8sClient)
	if err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return err
	}
	replicatedObjects := 0
	for _, c := range clusters {
		cl, err := c.NewClient(ctx, k8sClient)
		if err != nil {
			return err
		}
		err = cl.Get(ctx, client.ObjectKeyFromObject(replicatedObject), replicatedObject)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		replicatedObjects++
		err = cl.Delete(ctx, replicatedObject)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	if replicatedObjects > 0 {
		return fmt.Errorf("Not all replicated objects are deleted yet, requeuing")
	}

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuServiceObject, finalizer)
	if err := k8sClient.Update(ctx, dpuServiceObject); err != nil {
		return err
	}
	return nil
}

func reconcile(ctx context.Context, k8sClient client.Client, dpuServiceObject client.Object, r dpuDuplicateReconciler) (ctrl.Result, error) {
	// Get the list of clusters this DPUServiceObject targets.
	//TODO implement list clusters with label selector
	clusters, err := controlplane.GetDPFClusters(ctx, k8sClient)
	if err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}
	for _, c := range clusters {
		cl, err := c.NewClient(ctx, k8sClient)
		if err != nil {
			return ctrl.Result{}, err
		}
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceObject.GetNamespace()}}
		if err := cl.Create(ctx, ns); err != nil {
			// Fail if this returns any error other than alreadyExists.
			if !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, err
			}
		}
		err = r.createOrUpdateChild(ctx, cl, dpuServiceObject)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}
