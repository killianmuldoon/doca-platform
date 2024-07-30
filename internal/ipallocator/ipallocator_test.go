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

package ipallocator_test

import (
	"context"
	"os"
	"path/filepath"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/ipallocator"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/invoke/fakes"
	"github.com/containernetworking/cni/pkg/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IPAllocator", func() {
	var tmpDir string
	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "ipallocator")
		defer func() {
			err := os.RemoveAll(tmpDir)
			Expect(err).ToNot(HaveOccurred())
		}()
		Expect(err).NotTo(HaveOccurred())
	})
	It("should call CNI ADD with correct parameters", func() {
		rawExec := &fakes.RawExec{}
		rawExec.ExecPluginCall.Returns.ResultBytes = []byte(`{ "cniVersion": "0.4.0", "ips": [ { "version": "4", "address": "192.168.0.5/16", "gateway": "192.168.0.1" } ], "dns": {} }`)
		versionDecoder := &fakes.VersionDecoder{}
		versionDecoder.DecodeCall.Returns.PluginInfo = version.PluginSupports("0.4.0")
		pluginExec := &struct {
			*fakes.RawExec
			*fakes.VersionDecoder
		}{
			RawExec:        rawExec,
			VersionDecoder: versionDecoder,
		}

		allocator := ipallocator.New(
			libcni.NewCNIConfigWithCacheDir([]string{filepath.Join(tmpDir, "/opt/cni/bin")}, filepath.Join(tmpDir, "/cache"), pluginExec),
			"some-pool",
			"ippool",
			"some-pod",
			"some-namespace",
			"6e29ade3-5ec4-436d-a5bd-c53ec3ca816e",
		)
		allocator.FileSystemRoot = tmpDir
		err := allocator.Allocate(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_COMMAND=ADD"))
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_ARGS=K8S_POD_NAME=some-pod;K8S_POD_NAMESPACE=some-namespace;K8S_POD_UID=6e29ade3-5ec4-436d-a5bd-c53ec3ca816e"))
		Expect(rawExec.ExecPluginCall.Received.StdinData).To(Equal([]byte(`{"cniVersion":"0.4.0","ipam":{"poolName":"some-pool","poolType":"ippool","type":"nv-ipam"},"name":"sidecar","type":"nv-ipam"}`)))
		Expect(rawExec.FindInPathCall.Received.Paths).To(ContainElement(filepath.Join(tmpDir, ipallocator.CNIBinDir)))
		Expect(rawExec.FindInPathCall.Received.Plugin).To(Equal("nv-ipam"))
	})
	It("should not run CNI ADD if entry already exists in the cache", func() {
		rawExec := &fakes.RawExec{}
		rawExec.ExecPluginCall.Returns.ResultBytes = []byte(`{ "cniVersion": "0.4.0", "ips": [ { "version": "4", "address": "192.168.0.5/16", "gateway": "192.168.0.1" } ], "dns": {} }`)
		versionDecoder := &fakes.VersionDecoder{}
		versionDecoder.DecodeCall.Returns.PluginInfo = version.PluginSupports("0.4.0")
		pluginExec := &struct {
			*fakes.RawExec
			*fakes.VersionDecoder
		}{
			RawExec:        rawExec,
			VersionDecoder: versionDecoder,
		}

		allocator := ipallocator.New(
			libcni.NewCNIConfigWithCacheDir([]string{filepath.Join(tmpDir, "/opt/cni/bin")}, filepath.Join(tmpDir, "/cache"), pluginExec),
			"some-pool",
			"ippool",
			"some-pod",
			"some-namespace",
			"6e29ade3-5ec4-436d-a5bd-c53ec3ca816e",
		)
		allocator.FileSystemRoot = tmpDir

		By("Calling the allocator for the first time, we expect the CNI ADD to happen")
		err := allocator.Allocate(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_COMMAND=ADD"))
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_ARGS=K8S_POD_NAME=some-pod;K8S_POD_NAMESPACE=some-namespace;K8S_POD_UID=6e29ade3-5ec4-436d-a5bd-c53ec3ca816e"))
		Expect(rawExec.ExecPluginCall.Received.StdinData).To(Equal([]byte(`{"cniVersion":"0.4.0","ipam":{"poolName":"some-pool","poolType":"ippool","type":"nv-ipam"},"name":"sidecar","type":"nv-ipam"}`)))
		Expect(rawExec.FindInPathCall.Received.Paths).To(ContainElement(filepath.Join(tmpDir, ipallocator.CNIBinDir)))
		Expect(rawExec.FindInPathCall.Received.Plugin).To(Equal("nv-ipam"))

		By("Calling the allocator for the second time, we don't expect any CNI call")
		rawExec.ExecPluginCall.Received.Environ = []string{}
		rawExec.ExecPluginCall.Received.StdinData = []byte{}
		rawExec.ExecPluginCall.Received.PluginPath = ""

		err = allocator.Allocate(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(BeEmpty())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(BeEmpty())
		Expect(rawExec.ExecPluginCall.Received.StdinData).To(BeEmpty())
	})
	It("should populate a result file with IP", func() {
		rawExec := &fakes.RawExec{}
		rawExec.ExecPluginCall.Returns.ResultBytes = []byte(`{ "cniVersion": "0.4.0", "ips": [ { "version": "4", "address": "192.168.0.5/16", "gateway": "192.168.0.1" } ], "dns": {} }`)
		versionDecoder := &fakes.VersionDecoder{}
		versionDecoder.DecodeCall.Returns.PluginInfo = version.PluginSupports("0.4.0")
		pluginExec := &struct {
			*fakes.RawExec
			*fakes.VersionDecoder
		}{
			RawExec:        rawExec,
			VersionDecoder: versionDecoder,
		}

		allocator := ipallocator.New(
			libcni.NewCNIConfigWithCacheDir([]string{filepath.Join(tmpDir, "/opt/cni/bin")}, filepath.Join(tmpDir, "/cache"), pluginExec),
			"some-pool",
			"ippool",
			"some-pod",
			"some-namespace",
			"6e29ade3-5ec4-436d-a5bd-c53ec3ca816e",
		)
		allocator.FileSystemRoot = tmpDir
		err := allocator.Allocate(context.Background())
		Expect(err).ToNot(HaveOccurred())

		content, err := os.ReadFile(filepath.Join(tmpDir, "/tmp/ips/addresses"))
		Expect(err).ToNot(HaveOccurred())
		Expect(content).To(Equal([]byte(`["192.168.0.5/16"]`)))
	})
	It("should overwrite an existing result file with IP", func() {
		rawExec := &fakes.RawExec{}
		rawExec.ExecPluginCall.Returns.ResultBytes = []byte(`{ "cniVersion": "0.4.0", "ips": [ { "version": "4", "address": "192.168.0.5/16", "gateway": "192.168.0.1" } ], "dns": {} }`)
		versionDecoder := &fakes.VersionDecoder{}
		versionDecoder.DecodeCall.Returns.PluginInfo = version.PluginSupports("0.4.0")
		pluginExec := &struct {
			*fakes.RawExec
			*fakes.VersionDecoder
		}{
			RawExec:        rawExec,
			VersionDecoder: versionDecoder,
		}

		allocator := ipallocator.New(
			libcni.NewCNIConfigWithCacheDir([]string{filepath.Join(tmpDir, "/opt/cni/bin")}, filepath.Join(tmpDir, "/cache"), pluginExec),
			"some-pool",
			"ippool",
			"some-pod",
			"some-namespace",
			"6e29ade3-5ec4-436d-a5bd-c53ec3ca816e",
		)
		allocator.FileSystemRoot = tmpDir

		resultFile := filepath.Join(tmpDir, "/tmp/ips/addresses")
		Expect(os.MkdirAll(filepath.Dir(resultFile), 0755)).ToNot(HaveOccurred())
		Expect(os.WriteFile(resultFile, []byte("some-content"), 0644)).ToNot(HaveOccurred())

		err := allocator.Allocate(context.Background())
		Expect(err).ToNot(HaveOccurred())

		content, err := os.ReadFile(resultFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(content).To(Equal([]byte(`["192.168.0.5/16"]`)))
	})
	It("should call CNI DEL with correct parameters", func() {
		rawExec := &fakes.RawExec{}
		rawExec.ExecPluginCall.Returns.ResultBytes = []byte(`{ "cniVersion": "0.4.0", "ips": [ { "version": "4", "address": "192.168.0.5/16", "gateway": "192.168.0.1" } ], "dns": {} }`)
		versionDecoder := &fakes.VersionDecoder{}
		versionDecoder.DecodeCall.Returns.PluginInfo = version.PluginSupports("0.4.0")
		pluginExec := &struct {
			*fakes.RawExec
			*fakes.VersionDecoder
		}{
			RawExec:        rawExec,
			VersionDecoder: versionDecoder,
		}

		allocator := ipallocator.New(
			libcni.NewCNIConfigWithCacheDir([]string{filepath.Join(tmpDir, "/opt/cni/bin")}, filepath.Join(tmpDir, "/cache"), pluginExec),
			"some-pool",
			"ippool",
			"some-pod",
			"some-namespace",
			"6e29ade3-5ec4-436d-a5bd-c53ec3ca816e",
		)
		allocator.FileSystemRoot = tmpDir

		By("Doing a CNI ADD to populate the cache")
		err := allocator.Allocate(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_COMMAND=ADD"))
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_ARGS=K8S_POD_NAME=some-pod;K8S_POD_NAMESPACE=some-namespace;K8S_POD_UID=6e29ade3-5ec4-436d-a5bd-c53ec3ca816e"))
		Expect(rawExec.ExecPluginCall.Received.StdinData).To(Equal([]byte(`{"cniVersion":"0.4.0","ipam":{"poolName":"some-pool","poolType":"ippool","type":"nv-ipam"},"name":"sidecar","type":"nv-ipam"}`)))
		Expect(rawExec.FindInPathCall.Received.Paths).To(ContainElement(filepath.Join(tmpDir, ipallocator.CNIBinDir)))
		Expect(rawExec.FindInPathCall.Received.Plugin).To(Equal("nv-ipam"))

		By("Doing a CNI DEL")
		rawExec.ExecPluginCall.Received.Environ = []string{}
		rawExec.ExecPluginCall.Received.StdinData = []byte{}
		rawExec.ExecPluginCall.Received.PluginPath = ""

		err = allocator.Deallocate(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_COMMAND=DEL"))
		Expect(rawExec.ExecPluginCall.Received.Environ).To(ContainElement("CNI_ARGS=K8S_POD_NAME=some-pod;K8S_POD_NAMESPACE=some-namespace;K8S_POD_UID=6e29ade3-5ec4-436d-a5bd-c53ec3ca816e"))
		Expect(rawExec.ExecPluginCall.Received.StdinData).To(Equal([]byte(`{"cniVersion":"0.4.0","ipam":{"poolName":"some-pool","poolType":"ippool","type":"nv-ipam"},"name":"sidecar","prevResult":{"cniVersion":"0.4.0","ips":[{"version":"4","address":"192.168.0.5/16","gateway":"192.168.0.1"}],"dns":{}},"type":"nv-ipam"}`)))
		Expect(rawExec.FindInPathCall.Received.Paths).To(ContainElement(filepath.Join(tmpDir, ipallocator.CNIBinDir)))
		Expect(rawExec.FindInPathCall.Received.Plugin).To(Equal("nv-ipam"))
	})
	It("should not run CNI DEL if cache doesn't have entry", func() {
		rawExec := &fakes.RawExec{}
		rawExec.ExecPluginCall.Returns.ResultBytes = []byte(`{ "cniVersion": "0.4.0", "ips": [ { "version": "4", "address": "192.168.0.5/16", "gateway": "192.168.0.1" } ], "dns": {} }`)
		versionDecoder := &fakes.VersionDecoder{}
		versionDecoder.DecodeCall.Returns.PluginInfo = version.PluginSupports("0.4.0")
		pluginExec := &struct {
			*fakes.RawExec
			*fakes.VersionDecoder
		}{
			RawExec:        rawExec,
			VersionDecoder: versionDecoder,
		}

		allocator := ipallocator.New(
			libcni.NewCNIConfigWithCacheDir([]string{filepath.Join(tmpDir, "/opt/cni/bin")}, filepath.Join(tmpDir, "/cache"), pluginExec),
			"some-pool",
			"ippool",
			"some-pod",
			"some-namespace",
			"6e29ade3-5ec4-436d-a5bd-c53ec3ca816e",
		)
		allocator.FileSystemRoot = tmpDir

		err := allocator.Deallocate(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(BeEmpty())
		Expect(rawExec.ExecPluginCall.Received.Environ).To(BeEmpty())
		Expect(rawExec.ExecPluginCall.Received.StdinData).To(BeEmpty())
	})
})
