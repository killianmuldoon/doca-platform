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

package collector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd/api/application/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

type Collector struct {
	clusters []*Cluster
}

func New(clusters []*Cluster) *Collector {
	return &Collector{
		clusters: clusters,
	}
}

type Cluster struct {
	clusterName           string
	client                client.Client
	artifactsDir          string
	inventoryManifestsDir string
	clientset             *kubernetes.Clientset
}

func GetClusterCollectors(ctx context.Context, client client.Client, artifactsDirectory string, inventoryManifestsDirectory string, config *rest.Config) ([]*Cluster, error) {
	log := ctrllog.FromContext(ctx)
	directory := filepath.Join(artifactsDirectory, "main")
	mainCluster, err := NewCluster(client, directory, inventoryManifestsDirectory, config, "main")
	if err != nil {
		// If the main cluster client isn't created return early.
		return nil, err
	}
	collectors := make([]*Cluster, 0)
	collectors = append(collectors, mainCluster)

	errs := make([]error, 0)
	// Get collectors for DPFClusters.
	dpfCluster, err := controlplane.GetDPFClusters(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, cluster := range dpfCluster {
		dpuClusterClient, err := cluster.NewClient(ctx, client)
		if err != nil {
			errs = append(errs, err)

		}
		dpuClusterRestConfig, err := cluster.GetRestConfig(ctx, client)
		if err != nil {
			errs = append(errs, err)
		}
		directory = filepath.Join(artifactsDirectory, cluster.Name)
		c, err := NewCluster(dpuClusterClient, directory, inventoryManifestsDirectory, dpuClusterRestConfig, cluster.Name)
		if err != nil {
			errs = append(errs, err)
		}
		collectors = append(collectors, c)
	}
	if len(errs) > 0 {
		log.Error(kerrors.NewAggregate(errs), "failed creating collectors for hosted control planes")
	}
	return collectors, nil
}

func NewCluster(client client.Client, artifactsDirectory string, inventoryManifestsDirectory string, config *rest.Config, name string) (*Cluster, error) {
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		clusterName:           name,
		client:                client,
		artifactsDir:          artifactsDirectory,
		inventoryManifestsDir: inventoryManifestsDirectory,
		clientset:             clientset,
	}, nil
}

func (c *Cluster) Name() string {
	return c.clusterName
}

func (c *Collector) Run(ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	errs := make([]error, 0)
	for _, cluster := range c.clusters {
		log.Info(fmt.Sprintf("Running collector for %s", cluster.Name()))
		if err := cluster.run(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}

func (c *Cluster) run(ctx context.Context) error {
	// You can add entries here for resources that are not part of the inventory. Inventory resources are collected
	// automatically.
	resourcesToCollect := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},    // Pods
		{Group: "", Version: "v1", Kind: "Node"},   // Nodes
		{Group: "", Version: "v1", Kind: "Secret"}, // Secrets
		argov1.ApplicationSchemaGroupVersionKind,   // ArgoCD Applications
	}
	namespacesToCollectEvents := []string{
		"dpf-operator-system",
	}
	errs := make([]error, 0)

	gvks, err := c.getDPFOperatorInventoryGVKs()
	if err != nil {
		errs = append(errs, fmt.Errorf("error collecting GVKs that the DPF Operator inventory contains: %w", err))
	}
	// best effort to include as many GVKs as possible
	resourcesToCollect = append(resourcesToCollect, gvks...)
	resourcesToCollect = slices.Compact(resourcesToCollect)

	for _, resource := range resourcesToCollect {
		err := c.dumpResource(ctx, resource)
		if err != nil {
			errs = append(errs, fmt.Errorf("error dumping %vs %w", resource.Kind, err))
		}
	}

	// Dump the logs from all the pods on the cluster.+
	err = c.dumpPodLogsAndEvents(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("error dumping pod logs %w", err))
	}

	for _, ns := range namespacesToCollectEvents {
		if err := c.dumpEventsForNamespace(ctx, ns); err != nil {
			errs = append(errs, fmt.Errorf("error dumping events for namespace %s: %w", ns, err))
		}
	}
	return kerrors.NewAggregate(errs)
}

// getDPFOperatorInventoryGVKs returns the GVKs that are part of the inventory which DPF Operator is using to deploy
// resources.
func (c *Cluster) getDPFOperatorInventoryGVKs() ([]schema.GroupVersionKind, error) {
	output := []schema.GroupVersionKind{}
	err := filepath.WalkDir(c.inventoryManifestsDir, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		objs, err := utils.BytesToUnstructured(content)
		if err != nil {
			return fmt.Errorf("error while converting bytes to unstructured for path %s: %w", path, err)
		}
		for _, obj := range objs {
			output = append(output, obj.GetObjectKind().GroupVersionKind())
		}
		return nil
	})
	return output, err
}

func (c *Cluster) dumpPodLogsAndEvents(ctx context.Context) error {
	podList := &corev1.PodList{}
	err := c.client.List(ctx, podList)
	if err != nil {
		return err
	}
	errs := []error{}
	for _, pod := range podList.Items {
		if err = c.dumpLogsForPod(ctx, &pod); err != nil {
			errs = append(errs, err)
		}
		if err = c.dumpEventsForNamespacedResource(ctx, "Pod", types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}); err != nil {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}

func (c *Cluster) dumpLogsForPod(ctx context.Context, pod *corev1.Pod) (reterr error) {
	errs := []error{}
	for _, container := range pod.Spec.Containers {
		podLogOpts := corev1.PodLogOptions{Container: container.Name}
		if err := c.dumpLogsForContainer(ctx, pod.Namespace, pod.Name, "", podLogOpts); err != nil {
			errs = append(errs, err)
		}

		// Also collect the logs from a previous container if one existed.
		previousContainerOpts := corev1.PodLogOptions{Container: container.Name, Previous: true}
		if err := c.dumpLogsForContainer(ctx, pod.Namespace, pod.Name, ".previous", previousContainerOpts); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				errs = append(errs, err)
			}
		}

	}
	return kerrors.NewAggregate(errs)
}

func (c *Cluster) dumpLogsForContainer(ctx context.Context, podNamespace, podName, fileSuffix string, options corev1.PodLogOptions) (reterr error) {
	req := c.clientset.CoreV1().Pods(podNamespace).GetLogs(podName, &options)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := podLogs.Close()
		if err != nil {
			reterr = err
		}
	}()

	logs := new(bytes.Buffer)
	_, err = io.Copy(logs, podLogs)
	if err != nil {
		return err
	}
	filePath := filepath.Join(c.artifactsDir, "Logs", podNamespace, podName, fmt.Sprintf("%v.log%s", options.Container, fileSuffix))
	if err := c.writeToFile(logs.Bytes(), filePath); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) writeToFile(data []byte, filePath string) error {
	err := os.MkdirAll(filepath.Dir(filePath), 0750)
	if err != nil {
		return err
	}
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	err = os.WriteFile(f.Name(), data, 0600)
	if err != nil {
		return err
	}
	return nil
}

func (c *Cluster) dumpEventsForNamespacedResource(ctx context.Context, kind string, ref types.NamespacedName) error {
	fieldSelector := fmt.Sprintf("regarding.name=%s", ref.Name)
	events, _ := c.clientset.EventsV1().Events(ref.Namespace).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector, TypeMeta: metav1.TypeMeta{Kind: kind}})
	filePath := filepath.Join(c.artifactsDir, "Events", kind, ref.Namespace, fmt.Sprintf("%v.events", ref.Name))
	if err := c.writeResourceToFile(events, filePath); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) dumpEventsForNamespace(ctx context.Context, namespace string) error {
	events, err := c.clientset.EventsV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error while listing events: %w", err)
	}
	filePath := filepath.Join(c.artifactsDir, "Events", "Namespace", fmt.Sprintf("%v.events", namespace))
	if err := c.writeResourceToFile(events, filePath); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) dumpResource(ctx context.Context, kind schema.GroupVersionKind) error {
	resourceList := unstructured.UnstructuredList{}
	resourceList.SetKind(kind.Kind)
	resourceList.SetAPIVersion(kind.GroupVersion().String())
	if err := c.client.List(ctx, &resourceList); err != nil {
		return err
	}
	for _, resource := range resourceList.Items {
		filePath := filepath.Join(c.artifactsDir, "Resources", resource.GetObjectKind().GroupVersionKind().Kind, resource.GetNamespace(), fmt.Sprintf("%v.yaml", resource.GetName()))
		err := c.writeResourceToFile(&resource, filePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) writeResourceToFile(resource runtime.Object, filePath string) error {
	yaml, err := yaml.Marshal(resource)
	if err != nil {
		return err
	}
	if err := c.writeToFile(yaml, filePath); err != nil {
		return err
	}
	return nil
}
