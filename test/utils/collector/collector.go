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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Cluster struct {
	client       client.Client
	artifactsDir string
	restConfig   *rest.Config
}

func NewCluster(client client.Client, artifactsDirectory string, config *rest.Config) *Cluster {
	return &Cluster{
		client:       client,
		artifactsDir: artifactsDirectory,
		restConfig:   config,
	}
}
func (c *Cluster) Run(ctx context.Context) error {
	log := klog.FromContext(ctx)
	resourcesToColect := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},  // Pods
		{Group: "", Version: "v1", Kind: "Node"}, // Nodes
		dpuservicev1.DPUServiceGroupVersionKind,  // DPUServices
		argov1.ApplicationSchemaGroupVersionKind, // Argov1 Applications
	}

	for _, resource := range resourcesToColect {
		err := c.dumpResource(ctx, resource)
		if err != nil {
			log.Error(err, fmt.Sprintf("error dumping %vs", resource.Kind))
		}

	}

	// Dump the logs from all the pods on the cluster.
	err := c.dumpPodLogs(ctx, c.artifactsDir)
	if err != nil {
		log.Error(err, "error dumping pod logs")

	}
	return nil
}

func (c *Cluster) dumpPodLogs(ctx context.Context, folderPath string) (reterr error) {
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(c.restConfig)
	if err != nil {
		return err
	}
	podList := &corev1.PodList{}
	err = c.client.List(ctx, podList)
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		for _, container := range pod.Spec.Containers {
			podLogOpts := corev1.PodLogOptions{Container: container.Name}

			req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
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
			filePath := filepath.Join(folderPath, "logs", "pods", pod.GetNamespace(), pod.GetName(), fmt.Sprintf("%v.yaml", container.Name))
			err = os.MkdirAll(filepath.Dir(filePath), 0750)
			if err != nil {
				return err
			}
			f, err := os.Create(filePath)
			if err != nil {
				return err
			}
			err = os.WriteFile(f.Name(), logs.Bytes(), 0600)
			if err != nil {
				return err
			}
		}
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
		err := c.writeResourceToFile(&resource, c.artifactsDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) writeResourceToFile(resource client.Object, folderPath string) error {
	yaml, err := yaml.Marshal(resource)
	if err != nil {
		return err
	}
	filePath := filepath.Join(folderPath, resource.GetObjectKind().GroupVersionKind().Kind, resource.GetNamespace(), fmt.Sprintf("%v.yaml", resource.GetName()))

	err = os.MkdirAll(filepath.Dir(filePath), 0750)
	if err != nil {
		return err
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	err = os.WriteFile(f.Name(), yaml, 0600)
	if err != nil {
		return err
	}
	return nil
}
