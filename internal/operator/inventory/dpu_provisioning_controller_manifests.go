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

package inventory

import (
	_ "embed"
	"fmt"
	"strings"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dpfProvisioningControllerName          = "dpf-provisioning-controller-manager"
	dpfProvisioningControllerContainerName = "manager"
	bfbVolumeName                          = "bfb-volume"
)

var _ Component = &dpfProvisioningControllerObjects{}

// dpfProvisioningControllerObjects contains objects that are used to generate Provisioning Controller manifests.
// dpfProvisioningControllerObjects objects should be immutable after Parse()
type dpfProvisioningControllerObjects struct {
	data         []byte
	otherObjects []unstructured.Unstructured
	deployment   *appsv1.Deployment
}

func (p *dpfProvisioningControllerObjects) Name() string {
	return "DPFProvisioningController"
}

// Parse returns typed objects for the Provisioning controller deployment.
func (p *dpfProvisioningControllerObjects) Parse() (err error) {
	if p.data == nil {
		return fmt.Errorf("dpfProvisioningControllerObjects.data can not be empty")
	}
	objs, err := utils.BytesToUnstructured(p.data)
	if err != nil {
		return fmt.Errorf("error while converting DPU Provisioning Controller manifests to objects: %w", err)
	} else if len(objs) == 0 {
		return fmt.Errorf("no objects found in DPU Provisioning Controller manifests")
	}
	for _, obj := range objs {
		// Exclude Namespace and CustomResourceDefinition as the operator should not deploy these resources.
		if obj.GetKind() == "Namespace" || obj.GetKind() == "CustomResourceDefinition" {
			continue
		}
		// If the object is the dpf-provisioning-controller-manager Deployment store it as it's concrete type.
		if obj.GetKind() == utils.Deployment && obj.GetName() == dpfProvisioningControllerName {
			deployment := &appsv1.Deployment{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), deployment); err != nil {
				return fmt.Errorf("error while parsing Deployment for Provisioning Controller: %w", err)
			}
			p.deployment = deployment
			continue
		}
		p.otherObjects = append(p.otherObjects, *obj)
	}
	return p.validateDeployment()
}

func (p *dpfProvisioningControllerObjects) GenerateManifests(vars Variables) ([]client.Object, error) {
	if p.deployment == nil {
		return nil, fmt.Errorf("no Deployment in manifest")
	}

	err := p.setNamespace(vars.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error while setting Namespace for Provisioning Controller: %w", err)
	}

	ret := []client.Object{}
	for _, obj := range p.otherObjects {
		ret = append(ret, obj.DeepCopy())
	}
	deploy := p.deployment.DeepCopy()
	mods := []func(*appsv1.Deployment, Variables) error{
		p.setBFBPersistentVolumeClaim,
		p.setImagePullSecret,
	}
	for _, mod := range mods {
		if err := mod(deploy, vars); err != nil {
			return nil, fmt.Errorf("error while generating Deployment for Provisioning Controller: %w", err)
		}
	}
	// Add the deployment to the set of objects.
	ret = append(ret, deploy)
	return ret, nil
}

func (p *dpfProvisioningControllerObjects) validateDeployment() error {
	if p.deployment == nil {
		return fmt.Errorf("error while converting Provisioning Controller manifests to objects: Deployment not found")
	}
	vol := p.getVolume(p.deployment, "bfb-volume")
	if vol == nil {
		return fmt.Errorf("invalid Provisioning Controller deployment, no bfb volume found")
	}
	c := p.getContainer(p.deployment, dpfProvisioningControllerContainerName)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", dpfProvisioningControllerContainerName)
	}
	return nil
}

func (p *dpfProvisioningControllerObjects) getContainer(deploy *appsv1.Deployment, name string) *corev1.Container {
	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == name {
			return &deploy.Spec.Template.Spec.Containers[i]
		}
	}
	return nil
}

func (p *dpfProvisioningControllerObjects) getVolume(deploy *appsv1.Deployment, volName string) *corev1.Volume {
	for i, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == volName && vol.PersistentVolumeClaim != nil {
			return &deploy.Spec.Template.Spec.Volumes[i]
		}
	}
	return nil
}

func (p *dpfProvisioningControllerObjects) setBFBPersistentVolumeClaim(deploy *appsv1.Deployment, vars Variables) error {
	if strings.TrimSpace(vars.DPFProvisioningController.BFBPersistentVolumeClaimName) == "" {
		return fmt.Errorf("empty bfbPVCName")
	}
	vol := p.getVolume(deploy, bfbVolumeName)
	if vol == nil {
		return fmt.Errorf("error while generating Deployment for Provisioning Controller: no bfb volume found")
	}
	vol.PersistentVolumeClaim.ClaimName = vars.DPFProvisioningController.BFBPersistentVolumeClaimName
	c := p.getContainer(deploy, dpfProvisioningControllerContainerName)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", dpfProvisioningControllerContainerName)
	}
	return p.setFlags(c, fmt.Sprintf("--bfb-pvc=%s", vol.PersistentVolumeClaim.ClaimName))
}

func (p *dpfProvisioningControllerObjects) setImagePullSecret(deploy *appsv1.Deployment, vars Variables) error {
	if strings.TrimSpace(vars.DPFProvisioningController.ImagePullSecret) == "" {
		return fmt.Errorf("empty imagePullSecret")
	}
	deploy.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: vars.DPFProvisioningController.ImagePullSecret}}
	c := p.getContainer(deploy, dpfProvisioningControllerContainerName)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", dpfProvisioningControllerContainerName)
	}
	return p.setFlags(c, fmt.Sprintf("--dms-image-pull-secret=%s", vars.DPFProvisioningController.ImagePullSecret))
}

func (p *dpfProvisioningControllerObjects) setFlags(c *corev1.Container, newFlags ...string) error {
	pending := make(map[string]string)
	for _, f := range newFlags {
		name := p.parseFlagName(f)
		if name == "" {
			return fmt.Errorf("invalid flag %s", f)
		}
		pending[name] = f
	}
	for i, f := range c.Args {
		name := p.parseFlagName(f)
		if name == "" {
			continue
		}
		newFlag, ok := pending[name]
		if !ok {
			continue
		}
		delete(pending, name)
		c.Args[i] = newFlag
	}
	for _, flag := range pending {
		c.Args = append(c.Args, flag)
	}
	return nil
}

func (p *dpfProvisioningControllerObjects) parseFlagName(arg string) string {
	if len(arg) < 2 || arg[0] != '-' {
		return ""
	}
	numMinuses := 1
	if arg[1] == '-' {
		numMinuses++
	}
	name := arg[numMinuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return ""
	}
	for i := 1; i < len(name); i++ {
		if name[i] == '=' {
			return name[:i]
		}
	}
	return name
}

func (p *dpfProvisioningControllerObjects) setNamespace(namespace string) error {
	// The deployment is stored in a concrete type.
	p.deployment.SetNamespace(namespace)

	for i, obj := range p.otherObjects {
		obj.SetNamespace(namespace)
		switch obj.GetKind() {
		case "RoleBinding":
			roleBinding := &rbacv1.RoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), roleBinding); err != nil {
				return fmt.Errorf("error while converting object to RoleBinding: %w", err)
			}
			for _, subject := range roleBinding.Subjects {
				subject.Namespace = namespace
			}
		case "ClusterRoleBinding":
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), clusterRoleBinding); err != nil {
				return fmt.Errorf("error while converting object to ClusterRoleBinding: %w", err)
			}
			for _, subject := range clusterRoleBinding.Subjects {
				subject.Namespace = namespace
			}
		case "MutatingWebhookConfiguration":
			mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), mutatingWebhookConfiguration); err != nil {
				return fmt.Errorf("error while converting object to MutatingWebhookConfiguration: %w", err)
			}
			for _, webhook := range mutatingWebhookConfiguration.Webhooks {
				webhook.ClientConfig.Service.Namespace = namespace
			}
		case "ValidatingWebhookConfiguration":
			validatingWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), validatingWebhookConfiguration); err != nil {
				return fmt.Errorf("error while converting object to ValidatingWebhookConfiguration: %w", err)
			}
			for _, webhook := range validatingWebhookConfiguration.Webhooks {
				webhook.ClientConfig.Service.Namespace = namespace
			}
		}
		p.otherObjects[i] = obj
	}
	return nil
}
