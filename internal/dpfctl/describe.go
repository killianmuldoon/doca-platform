/*
Copyright 2025 NVIDIA

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

package dpfctl

import (
	"context"
	"fmt"

	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TreeDiscovery returns a tree of objects representing the DPF status.
func TreeDiscovery(ctx context.Context, c client.Client, opts ObjectTreeOptions) (*ObjectTree, error) {
	dpfOperatorConfig, err := getDPFOperatorConfig(ctx, c)
	if err != nil {
		return nil, err
	}

	t := NewObjectTree(dpfOperatorConfig, opts)

	return t, nil
}

func getDPFOperatorConfig(ctx context.Context, c client.Client) (*operatorv1.DPFOperatorConfig, error) {
	dpfOperatorConfigList := &operatorv1.DPFOperatorConfigList{}
	if err := c.List(ctx, dpfOperatorConfigList); err != nil {
		return nil, err
	}
	if len(dpfOperatorConfigList.Items) == 0 {
		return nil, fmt.Errorf("DPFOperatorConfig not found")
	}
	if len(dpfOperatorConfigList.Items) > 1 {
		return nil, fmt.Errorf("more than one DPFOperatorConfigs found")
	}
	dpfOperatorConfig := dpfOperatorConfigList.Items[0].DeepCopy()
	dpfOperatorConfig.TypeMeta = metav1.TypeMeta{
		Kind:       operatorv1.DPFOperatorConfigKind,
		APIVersion: operatorv1.GroupVersion.String(),
	}
	return dpfOperatorConfig, nil
}
