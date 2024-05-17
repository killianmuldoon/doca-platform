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
	"os"
	"path/filepath"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"

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
	clusterName  string
	client       client.Client
	artifactsDir string
	clientset    *kubernetes.Clientset
}

func GetClusterCollectors(ctx context.Context, client client.Client, artifactsDirectory string, config *rest.Config) ([]*Cluster, error) {
	log := ctrllog.FromContext(ctx)
	directory := filepath.Join(artifactsDirectory, "main")
	mainCluster, err := NewCluster(client, directory, config, "main")
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
		c, err := NewCluster(dpuClusterClient, directory, dpuClusterRestConfig, cluster.Name)
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

func NewCluster(client client.Client, artifactsDirectory string, config *rest.Config, name string) (*Cluster, error) {
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		clusterName:  name,
		client:       client,
		artifactsDir: artifactsDirectory,
		clientset:    clientset,
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
	resourcesToCollect := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},                   // Pods
		{Group: "", Version: "v1", Kind: "Node"},                  // Nodes
		{Group: "", Version: "v1", Kind: "Secret"},                // Secrets
		{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}, // PersistentVolumeClaim
		dpuservicev1.DPUServiceGroupVersionKind,                   // DPUServices
		argov1.ApplicationSchemaGroupVersionKind,                  // ArgoCD Applications
	}
	namespacesToCollectEvents := []string{
		"dpf-operator-system",
	}
	errs := make([]error, 0)

	for _, resource := range resourcesToCollect {
		err := c.dumpResource(ctx, resource)
		if err != nil {
			errs = append(errs, fmt.Errorf("error dumping %vs %w", resource.Kind, err))
		}
	}

	// Dump the logs from all the pods on the cluster.+
	err := c.dumpPodLogsAndEvents(ctx)
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

func (c *Cluster) dumpPodLogsAndEvents(ctx context.Context) error {
	podList := &corev1.PodList{}
	err := c.client.List(ctx, podList)
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if err = c.dumpLogsForPod(ctx, &pod); err != nil {
			return err
		}
		if err = c.dumpEventsForNamespacedResource(ctx, "Pod", types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}); err != nil {
			return err
		}
	}
	return nil
}
func (c *Cluster) dumpLogsForPod(ctx context.Context, pod *corev1.Pod) (reterr error) {
	for _, container := range pod.Spec.Containers {
		podLogOpts := corev1.PodLogOptions{Container: container.Name}
		req := c.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
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
		filePath := filepath.Join(c.artifactsDir, "Logs", pod.GetNamespace(), pod.GetName(), fmt.Sprintf("%v.log", container.Name))
		if err := c.writeToFile(logs.Bytes(), filePath); err != nil {
			return err
		}
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
