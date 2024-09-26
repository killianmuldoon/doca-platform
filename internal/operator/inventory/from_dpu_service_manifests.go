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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Component = &fromDPUService{}

type fromDPUService struct {
	data       []byte
	name       string
	dpuService *unstructured.Unstructured
}

// dpuNetworkingSubCharts are the DPUServices that use the dpu-networking helm chart by default.
var dpuNetworkingSubCharts = map[string]bool{
	FlannelName:              true,
	ServiceSetControllerName: true,
	MultusName:               true,
	SRIOVDevicePluginName:    true,
	OVSCNIName:               true,
	NVIPAMName:               true,
	SFCControllerName:        true,
}

func (f *fromDPUService) Name() string {
	return f.name
}

func (f *fromDPUService) Parse() error {
	if f.data == nil {
		return fmt.Errorf("data for DPUService %s can not be empty", f.name)
	}

	objects, err := utils.BytesToUnstructured(f.data)
	if err != nil {
		return fmt.Errorf("error while converting DPUService %v manifest to object: %w", f.name, err)
	}

	for _, obj := range objects {
		if ObjectKind(obj.GetKind()) != DPUServiceKind {
			return fmt.Errorf("manifests for %s should only contain a DPUService object: found %v", f.name, obj.GetObjectKind().GroupVersionKind().Kind)
		}
	}

	if len(objects) != 1 {
		return fmt.Errorf("manifests for %s should contain exactly one DPUService. found %v", f.name, len(objects))
	}

	f.dpuService = objects[0]

	return nil
}

func (f *fromDPUService) GenerateManifests(vars Variables, options ...GenerateManifestOption) ([]client.Object, error) {
	ret := []client.Object{}
	opts := &GenerateManifestOptions{}
	for _, option := range options {
		option.Apply(opts)
	}
	if _, ok := vars.DisableSystemComponents[f.Name()]; ok {
		return nil, nil
	}

	// copy object
	dpuServiceCopy := f.dpuService.DeepCopy()

	dpuServiceCopy.SetName(f.Name())

	labelsToAdd := map[string]string{operatorv1.DPFComponentLabelKey: f.Name()}
	applySetID := ApplySetID(vars.Namespace, f)
	// Add the ApplySet labels to the manifests unless disabled.
	if !opts.skipApplySet {
		labelsToAdd[applysetPartOfLabel] = applySetID
	}

	// apply edits
	edits := NewEdits().AddForAll(
		NamespaceEdit(vars.Namespace),
		LabelsEdit(labelsToAdd))

	// The DPUNetworking helm chart has all components disabled by default. Enable this DPUService in the helm chart values.
	if _, ok := dpuNetworkingSubCharts[f.Name()]; ok {
		edits.AddForKindS(DPUServiceKind, dpuServiceAddValueEdit(true, f.Name(), "enabled"))
	}

	// Add the helm chart.
	helmChartString, ok := vars.HelmCharts[f.Name()]
	if !ok {
		return []client.Object{}, fmt.Errorf("could not find helm chart source for DPUService %s", f.Name())
	}
	edits.AddForKindS(DPUServiceKind, dpuServiceSetHelmChartEdit(helmChartString))

	if vars.ImagePullSecrets != nil {
		secrets := pullSecretValueFromStrings(vars.ImagePullSecrets...)
		edits.AddForKindS(DPUServiceKind, dpuServiceAddValueEdit(secrets, "imagePullSecrets"))
	}
	if err := edits.Apply([]*unstructured.Unstructured{dpuServiceCopy}); err != nil {
		return nil, err
	}

	// Add the ApplySet to the manifests if this hasn't been disabled.
	if !opts.skipApplySet {
		ret = append(ret, applySetParentForComponent(f, applySetID, vars, applySetInventoryString(dpuServiceCopy)))
	}
	return append(ret, dpuServiceCopy), nil
}

func pullSecretValueFromStrings(names ...string) []interface{} {
	pullSecrets := make([]interface{}, 0, len(names))
	for _, name := range names {
		pullSecrets = append(pullSecrets, map[string]interface{}{"name": name})
	}
	return pullSecrets
}

func dpuServiceSetHelmChartEdit(helmChart string) StructuredEdit {
	return func(obj client.Object) error {
		dpuService, ok := obj.(*dpuservicev1.DPUService)
		if !ok {
			return fmt.Errorf("unexpected object kind %s. expected DPUService", obj.GetObjectKind().GroupVersionKind())
		}

		chart, err := parseHelmChartString(helmChart)
		if err != nil {
			return fmt.Errorf("failed parsing %s: %w", dpuService.Name, err)
		}

		dpuService.Spec.HelmChart.Source.Chart = chart.chart
		dpuService.Spec.HelmChart.Source.RepoURL = chart.repo
		dpuService.Spec.HelmChart.Source.Version = chart.version
		return nil
	}
}

func dpuServiceAddValueEdit(value interface{}, key ...string) StructuredEdit {
	return func(obj client.Object) error {
		dpuService, ok := obj.(*dpuservicev1.DPUService)
		if !ok {
			return fmt.Errorf("unexpected object kind %s. expected DPUService", obj.GetObjectKind().GroupVersionKind())
		}

		if dpuService.Spec.HelmChart.Values == nil {
			dpuService.Spec.HelmChart.Values = &runtime.RawExtension{}
		}
		currentValues := map[string]interface{}{}
		if dpuService.Spec.HelmChart.Values.Raw != nil {
			err := json.Unmarshal(dpuService.Spec.HelmChart.Values.Raw, &currentValues)
			if err != nil {
				return fmt.Errorf("error merging values in DPUService manifests: %w", err)
			}
			// This function needs to handle the case where dpuService.Spec.HelmChart.Values.Object holds the values.
			// This is because the function can be called multiple times and stores updated values there.
		} else if dpuService.Spec.HelmChart.Values.Object != nil {
			uns, ok := dpuService.Spec.HelmChart.Values.Object.(*unstructured.Unstructured)
			if !ok {
				return fmt.Errorf("could not treat values object field as unstructured")
			}
			currentValues = uns.UnstructuredContent()
		}

		valuesObject := &unstructured.Unstructured{Object: currentValues}
		if err := unstructured.SetNestedField(valuesObject.UnstructuredContent(), value, key...); err != nil {
			return err
		}

		dpuService.Spec.HelmChart.Values.Object = valuesObject
		dpuService.Spec.HelmChart.Values.Raw = nil
		return nil
	}
}

// IsReady returns an error if the DPUService does not have a Ready status condition.
func (f *fromDPUService) IsReady(ctx context.Context, c client.Client, namespace string) error {
	obj := &dpuservicev1.DPUService{}
	err := c.Get(ctx, client.ObjectKey{Name: f.dpuService.GetName(), Namespace: namespace}, obj)
	if err != nil {
		return err
	}
	if !meta.IsStatusConditionTrue(obj.GetConditions(), string(conditions.TypeReady)) {
		return fmt.Errorf("DPUService %s/%s is not ready", obj.Namespace, obj.Name)
	}
	return nil
}

type helmChartSource struct {
	repo    string
	chart   string
	version string
}

func parseHelmChartString(repoChartVersion string) (*helmChartSource, error) {
	versionStart := strings.LastIndex(repoChartVersion, ":")

	if versionStart == -1 {
		return nil, fmt.Errorf("failed to parse helm chart source: invalid format %s", repoChartVersion)
	}
	version := repoChartVersion[versionStart+1:]

	repoChart := repoChartVersion[:versionStart]
	imageStart := strings.LastIndex(repoChart, "/")
	if imageStart == -1 {
		return nil, fmt.Errorf("failed to parse helm chart source: invalid format %s", repoChartVersion)
	}

	image := repoChart[imageStart+1:]
	repo := repoChart[:imageStart]

	return &helmChartSource{
		version: version,
		chart:   image,
		repo:    repo,
	}, nil
}
