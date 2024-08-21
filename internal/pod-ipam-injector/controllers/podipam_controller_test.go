/*
COPYRIGHT 2024 NVIDIA

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

package controller //nolint:dupl

import (
	"context"
	"fmt"
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	nvipamv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/nvipam/api/v1alpha1"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	svcName1     = "chain1"
	svcName2     = "chain2"
	defaultNS    = "default"
	ifcName      = "sfceth1"
	ifcName2     = "sfceth2"
	ipamRefName  = "pool-1"
	ipamRefName2 = "pool-2"
	podName      = "test-pod"
	multusKey    = "k8s.v1.cni.cncf.io/networks"
	nodeName     = "worker-1"
)

//nolint:dupl
var _ = Describe("PodIpam Controller", func() {
	Context("When reconciling a resource", func() {
		var cleanupObjects []client.Object
		BeforeEach(func() {
			cleanupObjects = []client.Object{}
		})
		AfterEach(func() {
			By("Cleaning up the objects")
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjects...)).To(Succeed())
		})
		It("should successfully update Network annotation on Pod - IPAM ref IpPool", func() {
			By("Create ServiceChain with IPAM Reference")
			defaultGateway := false
			ipam := &sfcv1.IPAM{
				DefaultGateway: ptr.To(defaultGateway),
				Reference: &sfcv1.ObjectRef{
					Namespace: ptr.To(defaultNS),
					Name:      ipamRefName,
				},
			}
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName1, ipam, ifcName))
			By("Create IpPool")
			cleanupObjects = append(cleanupObjects, createIpPool(ctx, ipamRefName))
			By("Create Pod with Network Annotation")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, singleNetAnnotation()))
			By("Check that Pod annotation has been updated")
			Eventually(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedSingleNetAnnotation("ippool", defaultGateway)))
			}).WithTimeout(30 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should successfully update Network annotation on Pod - IPAM ref CIDRPool", func() {
			By("Create ServiceChain with IPAM Reference")
			defaultGateway := true
			ipam := &sfcv1.IPAM{
				DefaultGateway: ptr.To(defaultGateway),
				Reference: &sfcv1.ObjectRef{
					Namespace: ptr.To(defaultNS),
					Name:      ipamRefName,
				},
			}
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName1, ipam, ifcName))
			By("Create CIDRPool")
			cleanupObjects = append(cleanupObjects, createCidrPool(ctx, ipamRefName))
			By("Create Pod with Network Annotation")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, singleNetAnnotation()))
			By("Check that Pod annotation has been updated")
			Eventually(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedSingleNetAnnotation("cidrpool", defaultGateway)))
			}).WithTimeout(30 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should successfully update Network annotation on Pod - IPAM match labels IpPool", func() {
			By("Create ServiceChain with IPAM MatchLabels")
			defaultGateway := false
			ipam := &sfcv1.IPAM{
				DefaultGateway: ptr.To(defaultGateway),
				MatchLabels:    testutils.GetTestLabels(),
			}
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName1, ipam, ifcName))
			By("Create IpPool")
			cleanupObjects = append(cleanupObjects, createIpPool(ctx, ipamRefName))
			By("Create Pod with Network Annotation")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, singleNetAnnotation()))
			By("Check that Pod annotation has been updated")
			Eventually(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedSingleNetAnnotation("ippool", defaultGateway)))
			}).WithTimeout(30 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should successfully update Network annotation on Pod - IPAM match labels CIDRPool", func() {
			By("Create ServiceChain with IPAM MatchLabels")
			defaultGateway := true
			ipam := &sfcv1.IPAM{
				DefaultGateway: ptr.To(defaultGateway),
				MatchLabels:    testutils.GetTestLabels(),
			}
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName1, ipam, ifcName))
			By("Create CIDRPool")
			cleanupObjects = append(cleanupObjects, createCidrPool(ctx, ipamRefName))
			By("Create Pod with Network Annotation")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, singleNetAnnotation()))
			By("Check that Pod annotation has been updated")
			Eventually(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedSingleNetAnnotation("cidrpool", defaultGateway)))
			}).WithTimeout(30 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should successfully update Network annotation on Pod - Multiple networks, Multiple IPAM", func() {
			By("Create ServiceChain2 with IPAM MatchLabels")
			defaultGateway := false
			ipam := &sfcv1.IPAM{
				DefaultGateway: ptr.To(defaultGateway),
				MatchLabels:    testutils.GetTestLabels(),
			}
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName1, ipam, ifcName))
			By("Create ServiceChain2 with IPAM ref")
			ipam2 := &sfcv1.IPAM{
				DefaultGateway: ptr.To(defaultGateway),
				Reference: &sfcv1.ObjectRef{
					Namespace: ptr.To(defaultNS),
					Name:      ipamRefName2,
				},
			}
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName2, ipam2, ifcName2))
			By("Create IpPool1")
			cleanupObjects = append(cleanupObjects, createIpPool(ctx, ipamRefName))
			By("Create IpPool2")
			cleanupObjects = append(cleanupObjects, createIpPool(ctx, ipamRefName2))
			By("Create Pod with Network Annotation - multiple networks")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, multipleNetAnnotation()))
			By("Check that Pod annotation has been updated")
			Eventually(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedMultipleNetAnnotation("ippool", defaultGateway)))
			}).WithTimeout(60 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should successfully update Network annotation on Pod - Pod Created before Chain", func() {
			defaultGateway := false
			By("Create Pod with Network Annotation")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, singleNetAnnotation()))
			By("Verify Pod annotations is not updated")
			Consistently(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedSingleNetAnnotation("ippool", defaultGateway)))
			}).WithTimeout(20 * time.Second).Should(Not(BeNil()))
			By("Create ServiceChain with IPAM MatchLabels")
			ipam := &sfcv1.IPAM{
				DefaultGateway: ptr.To(defaultGateway),
				MatchLabels:    testutils.GetTestLabels(),
			}
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName1, ipam, ifcName))
			By("Verify Pod annotations is not updated")
			Consistently(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedSingleNetAnnotation("ippool", defaultGateway)))
			}).WithTimeout(20 * time.Second).Should(Not(BeNil()))
			By("Create IpPool")
			cleanupObjects = append(cleanupObjects, createIpPool(ctx, ipamRefName))
			By("Check that Pod annotation has been updated")
			Eventually(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(expectedSingleNetAnnotation("ippool", defaultGateway)))
			}).WithTimeout(30 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should not update Network annotation on Pod - no br-sfc network", func() {
			By("Create Pod without Network Annotation")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, ""))
			By("Check that Pod annotation has not been updated")
			Consistently(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(""))
			}).WithTimeout(10 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should not update Network annotation on Pod - no interface requested", func() {
			By("Create Pod with Network Annotation, no interface requested")
			annotation := `[{"name":"mybrsfc"}]`
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, annotation))
			By("Check that Pod annotation has not been updated")
			Consistently(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(annotation))
			}).WithTimeout(10 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
		It("should not update Network annotation on Pod - no IPAM requested", func() {
			By("Create ServiceChain without IPAM")
			cleanupObjects = append(cleanupObjects, createServiceChain(ctx, svcName1, nil, ifcName))
			By("Create Pod with Network Annotation")
			cleanupObjects = append(cleanupObjects, createPodWithAnnotation(ctx, singleNetAnnotation()))
			By("Check that Pod annotation has been updated")
			Consistently(func(g Gomega) {
				pod := &corev1.Pod{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
				g.Expect(pod.Annotations[multusKey]).To(Equal(singleNetAnnotation()))
			}).WithTimeout(10 * time.Second).Should(BeNil())
			By("Turning the Pod State to Succeed")
			changePodState(ctx, corev1.PodSucceeded)
		})
	})
})

func expectedSingleNetAnnotation(pooltype string, assignGW bool) string {
	s := fmt.Sprintf("[{\"name\":\"mybrsfc\",\"namespace\":\"default\",\"interface\":\"sfceth1\",\"cni-args\":{\"allocateDefaultGateway\":%v,\"poolNames\":[\"pool-1\"],\"poolType\":\"%s\"}}]", assignGW, pooltype)
	return s
}

func singleNetAnnotation() string {
	return fmt.Sprintf(`[{"name":"mybrsfc","interface": "%s"}]`, ifcName)
}

func multipleNetAnnotation() string {
	return fmt.Sprintf(`[{"name":"mybrsfc","interface": "%s"}, {"name":"othernet"},
	{"name":"mybrsfc","interface": "%s"}]`, ifcName, ifcName2)
}

func expectedMultipleNetAnnotation(pooltype string, assignGW bool) string {
	s := fmt.Sprintf(
		"[{\"name\":\"mybrsfc\",\"namespace\":\"default\",\"interface\":\"sfceth1\",\"cni-args\":{\"allocateDefaultGateway\":%v,\"poolNames\":[\"pool-1\"],\"poolType\":\"%s\"}},"+
			"{\"name\":\"othernet\",\"namespace\":\"default\",\"cni-args\":null},"+
			"{\"name\":\"mybrsfc\",\"namespace\":\"default\",\"interface\":\"sfceth2\",\"cni-args\":{\"allocateDefaultGateway\":%v,\"poolNames\":[\"pool-2\"],\"poolType\":\"%s\"}}]",
		assignGW, pooltype, assignGW, pooltype)
	return s
}

func changePodState(ctx context.Context, phase corev1.PodPhase) {
	Eventually(func(g Gomega) {
		pod := &corev1.Pod{}
		g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: defaultNS, Name: podName}, pod)).To(Succeed())
		pod.Status.Phase = phase
		pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
		pod.ManagedFields = nil
		g.Expect(testClient.Status().Patch(ctx, pod, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
	}).WithTimeout(10 * time.Second).Should(Succeed())
}

func createPodWithAnnotation(ctx context.Context, nets string) *corev1.Pod {
	grace := int64(0)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       podName,
			Namespace:                  defaultNS,
			Annotations:                map[string]string{},
			DeletionGracePeriodSeconds: &grace,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{Name: "ctr1", Image: "image"},
			},
		},
	}
	if nets != "" {
		pod.Annotations[multusKey] = nets
	}
	Expect(testClient.Create(ctx, pod)).NotTo(HaveOccurred())
	By("Turning the Pod to Pending")
	changePodState(ctx, corev1.PodPending)
	return pod
}

func createIpPool(ctx context.Context, name string) *nvipamv1.IPPool {
	pool := &nvipamv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNS,
			Labels:    testutils.GetTestLabels(),
		},
		Spec: nvipamv1.IPPoolSpec{
			Subnet:           "192.168.0.0/16",
			Gateway:          "192.168.0.1",
			PerNodeBlockSize: 10,
		},
	}
	Expect(testClient.Create(ctx, pool)).NotTo(HaveOccurred())
	return pool
}

func createCidrPool(ctx context.Context, name string) *nvipamv1.CIDRPool {
	pool := &nvipamv1.CIDRPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNS,
			Labels:    testutils.GetTestLabels(),
		},
		Spec: nvipamv1.CIDRPoolSpec{
			CIDR:                 "192.168.100.0/24",
			PerNodeNetworkPrefix: 31,
		},
	}
	Expect(testClient.Create(ctx, pool)).NotTo(HaveOccurred())
	return pool
}

func createServiceChain(ctx context.Context, name string, ipam *sfcv1.IPAM, ifcName string) *sfcv1.ServiceChain {
	scs := &sfcv1.ServiceChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNS,
		},
		Spec: sfcv1.ServiceChainSpec{
			Node: ptr.To(nodeName),
			Switches: []sfcv1.Switch{
				{
					Ports: []sfcv1.Port{
						{
							Service: &sfcv1.Service{
								InterfaceName: ifcName,
								Reference: &sfcv1.ObjectRef{
									Name: "p0",
								},
								IPAM: ipam,
							},
						},
					},
				},
			},
		},
	}
	Expect(testClient.Create(ctx, scs)).NotTo(HaveOccurred())
	return scs
}
