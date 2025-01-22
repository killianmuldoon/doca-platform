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

	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"
	"github.com/nvidia/doca-platform/internal/conditions"

	"github.com/olekukonko/tablewriter"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_dpfctlTreeDiscovery(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	tests := []struct {
		name           string
		objectsTree    []client.Object
		expectedPrefix []string
	}{
		{
			name: "Add DPFOperatorConfig",
			objectsTree: []client.Object{
				defaultDPFOperatorConfig(),
			},
			expectedPrefix: []string{
				"DPFOperatorConfig/test  True  Success",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, obj := range tt.objectsTree {
				g.Expect(testClient.Create(ctx, obj)).To(Succeed())
				updateStatus(ctx, g, obj)
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
		})
	}
}

func updateStatus(ctx context.Context, g *WithT, obj client.Object) {
	switch v := obj.(type) {
	case *operatorv1.DPFOperatorConfig:
		conditions.AddTrue(v, conditions.TypeReady)
	}
	g.Expect(testClient.Status().Update(ctx, obj)).To(Succeed())
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
