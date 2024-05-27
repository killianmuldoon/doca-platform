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
	"net"
	"strings"

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/release"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dpfProvisioningControllerName          = "dpf-provisioning-controller-manager"
	dpfProvisioningControllerContainerName = "manager"
	bfbVolumeName                          = "bfb-volume"
	webhookServiceName                     = "dpf-provisioning-webhook-service"
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
		if obj.GetKind() == namespaceKind || obj.GetKind() == customResourceDefinitionKind {
			continue
		}
		// If the object is the dpf-provisioning-controller-manager Deployment store it as it's concrete type.
		if obj.GetKind() == deploymentKind && obj.GetName() == dpfProvisioningControllerName {
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
	if _, ok := vars.DisableSystemComponents[p.Name()]; ok {
		return []client.Object{}, nil
	}
	if p.deployment == nil {
		return nil, fmt.Errorf("no Deployment in manifest")
	}

	objects, err := setNamespace(vars.Namespace, p.otherObjects)
	if err != nil {
		return nil, fmt.Errorf("error while setting Namespace for Provisioning Controller: %w", err)
	}
	p.deployment.SetNamespace(vars.Namespace)

	// Fixup webhook service selector.

	ret := []client.Object{}
	for _, obj := range objects {
		if obj.GetKind() == "Service" && obj.GetName() == webhookServiceName {
			fixedService, err := fixupWebhookService(&obj)
			if err != nil {
				return nil, fmt.Errorf("error patching webhook service for Provisioning Controller: %w", err)
			}
			ret = append(ret, fixedService.DeepCopy())
			continue
		}
		ret = append(ret, obj.DeepCopy())
	}
	deploy := p.deployment.DeepCopy()
	mods := []func(*appsv1.Deployment, Variables) error{
		p.setBFBPersistentVolumeClaim,
		p.setImagePullSecret,
		p.setComponentLabel,
		p.setDefaultImageNames,
		p.setDHCP,
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

// Set the component label for the deployment.
func (p *dpfProvisioningControllerObjects) setComponentLabel(deployment *appsv1.Deployment, _ Variables) error {
	labels := deployment.Spec.Template.ObjectMeta.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[operatorv1.DPFComponentLabelKey] = dpfProvisioningControllerName
	deployment.Spec.Template.ObjectMeta.Labels = labels
	return nil
}

// Add a component label selector to the webhook service.
func fixupWebhookService(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// do the conversion to ensure we're dealing with the correct type, but deal with unstructured for the patch.
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &corev1.Service{})
	if err != nil {
		return nil, fmt.Errorf("error while converting DPU Provisioning Controller Service to objects: %w", err)
	}
	selector, found, err := unstructured.NestedMap(obj.UnstructuredContent(), "spec", "selector")
	if err != nil {
		return nil, fmt.Errorf("error while converting DPU Provisioning Controller Service to objects: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("DPU Provisioning Controller webhook secret does not have a selector")
	}
	selector[operatorv1.DPFComponentLabelKey] = dpfProvisioningControllerName
	err = unstructured.SetNestedMap(obj.UnstructuredContent(), selector, "spec", "selector")
	if err != nil {
		return nil, fmt.Errorf("error while converting DPU Provisioning Controller Service to objects: %w", err)
	}
	return obj, nil
}

func (p *dpfProvisioningControllerObjects) validateDeployment() error {
	if p.deployment == nil {
		return fmt.Errorf("error while converting Provisioning Controller manifests to objects: Deployment not found")
	}
	vol := p.getVolume(p.deployment, "bfb-volume")
	if vol == nil {
		return fmt.Errorf("invalid Provisioning Controller deployment, no bfb volume found")
	}
	c := p.getContainer(p.deployment)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", dpfProvisioningControllerContainerName)
	}
	return nil
}

func (p *dpfProvisioningControllerObjects) getContainer(deploy *appsv1.Deployment) *corev1.Container {
	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == dpfProvisioningControllerContainerName {
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
	c := p.getContainer(deploy)
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
	if vars.ImagePullSecrets != nil {
		var localObjectRefs []corev1.LocalObjectReference
		for _, secret := range vars.ImagePullSecrets {
			localObjectRefs = append(localObjectRefs, corev1.LocalObjectReference{Name: secret})
		}
		deploy.Spec.Template.Spec.ImagePullSecrets = append(deploy.Spec.Template.Spec.ImagePullSecrets, localObjectRefs...)
	}
	c := p.getContainer(deploy)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", dpfProvisioningControllerContainerName)
	}
	return p.setFlags(c, fmt.Sprintf("--image-pull-secret=%s", vars.DPFProvisioningController.ImagePullSecret))
}

func (p *dpfProvisioningControllerObjects) setDHCP(deploy *appsv1.Deployment, vars Variables) error {
	if ip := net.ParseIP(vars.DPFProvisioningController.DHCP); ip == nil {
		return fmt.Errorf("invalid dhcp")
	}
	c := p.getContainer(deploy)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", dpfProvisioningControllerContainerName)
	}
	return p.setFlags(c, fmt.Sprintf("--dhcp=%s", vars.DPFProvisioningController.DHCP))
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

func (p *dpfProvisioningControllerObjects) setDefaultImageNames(deployment *appsv1.Deployment, _ Variables) error {
	c := p.getContainer(deployment)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", dpfProvisioningControllerContainerName)
	}
	release := release.NewDefaults()
	err := release.Parse()
	if err != nil {
		return err
	}
	err = p.setFlags(c, fmt.Sprintf("--dms-image=%s", release.DMSImage))
	if err != nil {
		return err
	}
	err = p.setFlags(c, fmt.Sprintf("--hostnetwork-image=%s", release.HostNetworkSetupImage))
	if err != nil {
		return err
	}
	err = p.setFlags(c, fmt.Sprintf("--dhcrelay-image=%s", release.DHCRelayImage))
	if err != nil {
		return err
	}
	err = p.setFlags(c, fmt.Sprintf("--parprouterd-image=%s", release.ParprouterdImage))
	if err != nil {
		return err
	}
	return nil
}
