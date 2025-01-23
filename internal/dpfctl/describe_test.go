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
	"bytes"
	"context"
	"testing"
	"time"

	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"
	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/conditions"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	"github.com/olekukonko/tablewriter"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_dpfctlTreeDiscovery(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	tests := []struct {
		name           string
		objectsTree    []client.Object
		condition      metav1.Condition
		expectedPrefix []string
	}{
		{
			name: "Add DPFOperatorConfig",
			objectsTree: []client.Object{
				defaultDPFOperatorConfig(),
			},
			condition: getTrueCondition(),
			expectedPrefix: []string{
				"DPFOperatorConfig/test  True  Success",
			},
		},
		{
			name: "Add DPFOperatorConfig with false condition",
			objectsTree: []client.Object{
				defaultDPFOperatorConfig(),
			},
			condition: getFalseCondition(),
			expectedPrefix: []string{
				"DPFOperatorConfig/test  False  SomethingWentWrong",
			},
		},
		{
			name: "Add DPUCluster",
			objectsTree: []client.Object{
				defaultDPFOperatorConfig(),
				defaultDPUCluster(),
			},
			condition: getTrueCondition(),
			expectedPrefix: []string{
				"DPFOperatorConfig/test  True  Success",
				"└─DPUClusters",
				"  └─DPUCluster/test     True  Success",
			},
		},
		{
			name: "Add DPUSet",
			objectsTree: []client.Object{
				defaultDPFOperatorConfig(),
				defaultDPUSet(),
			},
			condition: getTrueCondition(),
			expectedPrefix: []string{
				"DPFOperatorConfig/test  True  Success",
				"└─DPUSets",
				"  └─DPUSet/test",
			},
		},
		{
			name: "Add DPU",
			objectsTree: []client.Object{
				defaultDPFOperatorConfig(),
				defaultDPU(),
			},
			condition: getTrueCondition(),
			expectedPrefix: []string{
				"DPFOperatorConfig/test  True  Success",
				"└─DPUs",
				"  └─DPU/orphaned-dpu    True  Success",
			},
		},
		{
			name: "Add DPUSet with DPU and DPU w/o DPUSet",
			objectsTree: []client.Object{
				defaultDPFOperatorConfig(),
				defaultDPUSet(),
				defaultDPU(),
				defaultDPUFromDPUSet(),
			},
			condition: getTrueCondition(),
			expectedPrefix: []string{
				"DPFOperatorConfig/test  True  Success",
				"├─DPUSets",
				"│ └─DPUSet/test",
				"│   └─DPU/test          True  Success",
				"└─DPUs",
				"  └─DPU/orphaned-dpu    True  Success",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, obj := range tt.objectsTree {
				g.Expect(testClient.Create(ctx, obj)).To(Succeed())

				// We have to convert the object to unstructured to set the status conditions.
				// We don't have access to the status field of the client.Object directly.
				u := unstructured.Unstructured{}
				g.Expect(scheme.Scheme.Convert(obj, &u, nil)).To(Succeed())
				unstructuredGetSet(&u).SetConditions([]metav1.Condition{tt.condition})

				// Update the status.
				g.Expect(testClient.Status().Update(ctx, &u)).To(Succeed())
			}

			td, err := TreeDiscovery(context.Background(), testClient, ObjectTreeOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(td).ToNot(BeNil())

			// Creates the output table
			var output bytes.Buffer
			tbl := tablewriter.NewWriter(&output)

			formatTableTree(tbl)

			addObjectRow("", tbl, td, td.GetRoot())
			tbl.Render()
			g.Expect(output.String()).Should(MatchTable(tt.expectedPrefix))

			// Cleanup resources for next run
			for _, obj := range tt.objectsTree {
				g.Expect(testClient.Delete(ctx, obj)).To(Succeed())
			}
		})
	}
}

func getTrueCondition() metav1.Condition {
	return metav1.Condition{
		Type:               string(conditions.TypeReady),
		Status:             metav1.ConditionTrue,
		Reason:             "Success",
		LastTransitionTime: metav1.Time{Time: time.Now()},
	}
}

func getFalseCondition() metav1.Condition {
	return metav1.Condition{
		Type:               string(conditions.TypeReady),
		Status:             metav1.ConditionFalse,
		Reason:             "SomethingWentWrong",
		Message:            "Failed",
		LastTransitionTime: metav1.Time{Time: time.Now()},
	}
}

func defaultDPFOperatorConfig() *operatorv1.DPFOperatorConfig {
	return &operatorv1.DPFOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: operatorv1.DPFOperatorConfigSpec{
			ProvisioningController: operatorv1.ProvisioningControllerConfiguration{
				BFBPersistentVolumeClaimName: "oof",
			},
		},
	}
}

func defaultDPUCluster() *provisioningv1.DPUCluster {
	return &provisioningv1.DPUCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: provisioningv1.DPUClusterSpec{
			Type: "static",
		},
	}
}

func defaultDPUSet() *provisioningv1.DPUSet {
	return &provisioningv1.DPUSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
}

func defaultDPU() *provisioningv1.DPU {
	return &provisioningv1.DPU{
		ObjectMeta: metav1.ObjectMeta{Name: "orphaned-dpu", Namespace: "default"},
	}
}

func defaultDPUFromDPUSet() *provisioningv1.DPU {
	return &provisioningv1.DPU{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Labels: map[string]string{
				util.DPUSetNameLabel:      "test",
				util.DPUSetNamespaceLabel: "default",
			},
		},
	}
}
