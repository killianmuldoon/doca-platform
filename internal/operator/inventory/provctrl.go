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

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ProvCtrlName          = "dpf-provisioning-controller-manager"
	ProvCtrlContainerName = "manager"
	BFBVolumeName         = "bfb-volume"
)

//go:embed manifests/provisioningctrl.yaml
var provisioningCtrlData []byte

// ProvCtrlObjects contains objects that are used to generate Provisioning Controller manifests.
// ProvCtrlObjects objects should be immutable after Parse()
type ProvCtrlObjects struct {
	data       []byte
	objs       []unstructured.Unstructured
	deployIdx  int
	deployment *appsv1.Deployment
}

func NewProvisionCtrlObjects() ProvCtrlObjects {
	return ProvCtrlObjects{
		data:      provisioningCtrlData,
		deployIdx: -1,
	}
}

// Parse returns typed objects for the Provisioning controller deployment.
func (p *ProvCtrlObjects) Parse() (err error) {
	if p.data == nil {
		return fmt.Errorf("ProvisioiningCtrlObjects.data can not be empty")
	}
	objs, err := utils.BytesToUnstructured(p.data)
	if err != nil {
		return fmt.Errorf("error while converting Provisioning Controller manifests to objects: %w", err)
	} else if len(objs) == 0 {
		return fmt.Errorf("no objects found in Provisioning Controller manifests")
	}
	for i, obj := range objs {
		p.objs = append(p.objs, *obj)
		if obj.GetKind() == utils.Deployment && obj.GetName() == ProvCtrlName {
			deploy := &appsv1.Deployment{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), deploy); err != nil {
				return fmt.Errorf("error while parsing Deployment for Provisioning Controller: %w", err)
			}
			p.deployIdx = i
			p.deployment = deploy
		}
	}
	if p.deployIdx == -1 {
		return fmt.Errorf("error while converting Provisioning Controller manifests to objects: Deployment not found")
	}
	return p.validateDeployment()
}

func (p ProvCtrlObjects) GenerateManifests(config *operatorv1.DPFOperatorConfig) ([]*unstructured.Unstructured, error) {
	if p.deployment == nil {
		return nil, fmt.Errorf("no Deployment in manifest")
	}

	ret := []*unstructured.Unstructured{}
	for _, obj := range p.objs {
		ret = append(ret, obj.DeepCopy())
	}
	deploy := p.deployment.DeepCopy()
	mods := []func(*appsv1.Deployment, *operatorv1.DPFOperatorConfig) error{
		p.setBFBPVC,
		p.setImagePullSecret,
	}
	for _, mod := range mods {
		if err := mod(deploy, config); err != nil {
			return nil, fmt.Errorf("error while generating Deployment for Provisioning Controller: %w", err)
		}
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return nil, fmt.Errorf("error while convert to unstructed, err: %v", err)
	}
	ret[p.deployIdx] = &unstructured.Unstructured{Object: obj}
	return ret, nil
}

func (p ProvCtrlObjects) validateDeployment() error {
	if p.deployIdx < 0 {
		return fmt.Errorf("no deployment found")
	}
	vol := p.getVolume(p.deployment, "bfb-volume")
	if vol == nil {
		return fmt.Errorf("invalid Provisioning Controller deployment, no bfb volume found")
	}
	c := p.getContainer(p.deployment, ProvCtrlContainerName)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", ProvCtrlContainerName)
	}
	return nil
}

func (p ProvCtrlObjects) getContainer(deploy *appsv1.Deployment, name string) *corev1.Container {
	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == name {
			return &deploy.Spec.Template.Spec.Containers[i]
		}
	}
	return nil
}

func (p ProvCtrlObjects) getVolume(deploy *appsv1.Deployment, volName string) *corev1.Volume {
	for i, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == volName && vol.PersistentVolumeClaim != nil {
			return &deploy.Spec.Template.Spec.Volumes[i]
		}
	}
	return nil
}

func (p ProvCtrlObjects) setBFBPVC(deploy *appsv1.Deployment, config *operatorv1.DPFOperatorConfig) error {
	if strings.TrimSpace(config.Spec.ProvisioningConfiguration.BFBPVCName) == "" {
		return fmt.Errorf("empty bfbPVCName")
	}
	vol := p.getVolume(deploy, BFBVolumeName)
	if vol == nil {
		return fmt.Errorf("error while generating Deployment for Provisioning Controller: no bfb volume found")
	}
	vol.PersistentVolumeClaim.ClaimName = config.Spec.ProvisioningConfiguration.BFBPVCName
	c := p.getContainer(deploy, ProvCtrlContainerName)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", ProvCtrlContainerName)
	}
	return p.setFlags(c, fmt.Sprintf("--bfb-pvc=%s", vol.PersistentVolumeClaim.ClaimName))
}

func (p ProvCtrlObjects) setImagePullSecret(deploy *appsv1.Deployment, config *operatorv1.DPFOperatorConfig) error {
	if strings.TrimSpace(config.Spec.ProvisioningConfiguration.ImagePullSecret) == "" {
		return fmt.Errorf("empty imagePullSecret")
	}
	deploy.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: config.Spec.ProvisioningConfiguration.ImagePullSecret}}
	c := p.getContainer(deploy, ProvCtrlContainerName)
	if c == nil {
		return fmt.Errorf("container %q not found in Provisioning Controller deployment", ProvCtrlContainerName)
	}
	return p.setFlags(c, fmt.Sprintf("--dms-image-pull-secret=%s", config.Spec.ProvisioningConfiguration.ImagePullSecret))
}

func (p ProvCtrlObjects) setFlags(c *corev1.Container, newFlags ...string) error {
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

func (p ProvCtrlObjects) parseFlagName(arg string) string {
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
