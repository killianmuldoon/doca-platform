#
#Copyright 2024 NVIDIA
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

TOOLSDIR ?= $(CURDIR)/hack/tools/bin
$(TOOLSDIR):
	@mkdir -p $@

# Detect architecture and platform
TOOL_ARCH ?= $(shell uname -m)
TOOL_OS ?= $(shell uname -s | tr A-Z a-z)

# PROTOC uses values for Mac OS and arch which are distinct from the uname values.
PROTOC_OS = $(TOOL_OS)
ifeq ($(TOOL_OS),darwin)
  PROTOC_OS = osx
endif

PROTOC_ARCH = $(TOOL_ARCH)
ifeq ($(TOOL_ARCH),arm64)
  PROTOC_ARCH = aarch_64
endif

LYCHEE_ARCH_LINUX = $(TOOL_ARCH)
ifeq ($(TOOL_ARCH),arm64)
  LYCHEE_ARCH_LINUX = aarch64
endif

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0
CONTROLLER_TOOLS_VERSION ?= v0.16.5
ENVTEST_VERSION ?= v0.0.0-20240110160329-8f8247fdc1c3
GOLANGCI_LINT_VERSION ?= v1.63.4
MOCKGEN_VERSION ?= v0.5.0
GOTESTSUM_VERSION ?= v1.12.0
ENVSUBST_VERSION ?= v1.4.2
HELM_VER ?= v3.16.3
MINIKUBE_VER ?= v1.34.0
GEN_API_REF_DOCS_VERSION ?= 0ad85c56e5a611240525e8b4a641b9cee33acd9a
MDTOC_VER ?= v1.4.0
STERN_VER ?= v1.30.0
HELM_DOCS_VER := v1.14.2
EMBEDMD_VER ?= v1.0.0
PROTOC_GEN_GO_VER ?= 1.35.2
PROTOC_GEN_GO_GRPC_VER ?= 1.5.1
BUF_VERSION ?= 1.47.2
PROTOC_VER ?= 28.3
CONFORM_VERSION ?= v0.1.0-alpha.30
YQ_VERSION ?= v4.45.1
LYCHEE_VER ?= 0.18.0


## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(TOOLSDIR)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(TOOLSDIR)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(TOOLSDIR)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT ?= $(TOOLSDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)
MOCKGEN ?= $(TOOLSDIR)/mockgen-$(MOCKGEN_VERSION)
GOTESTSUM ?= $(TOOLSDIR)/gotestsum-$(GOTESTSUM_VERSION)
ENVSUBST ?= $(TOOLSDIR)/envsubst-$(ENVSUBST_VERSION)
HELM ?= $(TOOLSDIR)/helm-$(HELM_VER)
MINIKUBE ?= $(TOOLSDIR)/minikube-$(MINIKUBE_VER)
GEN_CRD_API_REFERENCE_DOCS ?= $(TOOLSDIR)/crd-ref-docs-$(GEN_API_REF_DOCS_VERSION)
MDTOC ?= $(TOOLSDIR)/mdtoc-$(MDTOC_VER)
STERN ?= $(TOOLSDIR)/stern-$(STERN_VER)
HELM_DOCS ?= $(TOOLSDIR)/helm-docs-$(HELM_DOCS_VER)
EMBEDMD ?= $(TOOLSDIR)/embedmd-$(EMBEDMD_VER)
PROTOC ?= $(TOOLSDIR)/protoc/bin/protoc
PROTOC_GEN_GO ?= $(TOOLSDIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC ?= $(TOOLSDIR)/protoc-gen-go-grpc
BUF ?= $(TOOLSDIR)/buf
CONFORM ?= $(TOOLSDIR)/conform-$(CONFORM_VERSION)
YQ ?= $(TOOLSDIR)/yq-$(YQ_VERSION)
LYCHEE ?= $(TOOLSDIR)/lychee-$(LYCHEE_VER)

##@ Tools

.PHONY: yq
yq: $(YQ) ## Download conform locally if necessary.
$(YQ): | $(TOOLSDIR)
	$(call go-install-tool,$(YQ),github.com/mikefarah/yq/v4,$(YQ_VERSION))

.PHONY: conform
conform: $(CONFORM) ## Download conform locally if necessary.
$(CONFORM): | $(TOOLSDIR)
	$(call go-install-tool,$(CONFORM),github.com/siderolabs/conform/cmd/conform,$(CONFORM_VERSION))

.PHONY: protoc
PROTOC_REL ?= https://github.com/protocolbuffers/protobuf/releases
protoc: $(PROTOC) ## Download protoc locally if necessary.
$(PROTOC): | $(TOOLSDIR)
	cd $(TOOLSDIR) && \
	curl -L --output tmp.zip $(PROTOC_REL)/download/v$(PROTOC_VER)/protoc-$(PROTOC_VER)-$(PROTOC_OS)-$(PROTOC_ARCH).zip && \
	unzip tmp.zip -d protoc && rm tmp.zip

.PHONY: protoc-gen-go
protoc-gen-go: $(PROTOC_GEN_GO) ## Download protoc-gen-go locally if necessary.
$(PROTOC_GEN_GO): | $(TOOLSDIR)
	GOBIN=$(TOOLSDIR) go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VER)

.PHONY: protoc-gen-go-grpc
protoc-gen-go-grpc: $(PROTOC_GEN_GO_GRPC) ## Download protoc-gen-go locally if necessary.
$(PROTOC_GEN_GO_GRPC): | $(TOOLSDIR)
	GOBIN=$(TOOLSDIR) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$(PROTOC_GEN_GO_GRPC_VER)

.PHONY: buf
BUF_REL ?= https://github.com/bufbuild/buf/releases/download
buf: $(BUF) ## Download buf locally if necessary
$(BUF): | $(TOOLSDIR)
	cd $(TOOLSDIR) && \
	curl -sSL "$(BUF_REL)/v$(BUF_VERSION)/buf-$(TOOL_OS)-$(TOOL_ARCH)" -o "$(TOOLSDIR)/buf" && \
	chmod +x "$(TOOLSDIR)/buf"

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): | $(TOOLSDIR)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): | $(TOOLSDIR)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): | $(TOOLSDIR)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): | $(TOOLSDIR)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

.PHONY: mockgen
mockgen: $(MOCKGEN) ## Download mockgen locally if necessary.
$(MOCKGEN): | $(TOOLSDIR)
	$(call go-install-tool,$(MOCKGEN),go.uber.org/mock/mockgen,${MOCKGEN_VERSION})
	ln -f $(MOCKGEN) $(abspath $(TOOLSDIR)/mockgen)

# gotestsum is used to generate junit style test reports
.PHONY: gotestsum
gotestsum: $(GOTESTSUM) # download gotestsum locally if necessary
$(GOTESTSUM): | $(TOOLSDIR)
	$(call go-install-tool,$(GOTESTSUM),gotest.tools/gotestsum,${GOTESTSUM_VERSION})

# envsubst is used to template files with environment variables
.PHONY: envsubst
envsubst: $(ENVSUBST) # download envsubst locally if necessary
$(ENVSUBST): | $(TOOLSDIR)
	$(call go-install-tool,$(ENVSUBST),github.com/a8m/envsubst/cmd/envsubst,${ENVSUBST_VERSION})

# helm is used to manage helm deployments and artifacts.
.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
GET_HELM = $(TOOLSDIR)/get_helm.sh
$(HELM): | $(TOOLSDIR)
	$Q echo "Installing helm-$(HELM_VER) to $(TOOLSDIR)"
	$Q curl -fsSL -o $(GET_HELM) https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
	$Q chmod +x $(GET_HELM)
	$Q env HELM_INSTALL_DIR=$(TOOLSDIR) PATH="$(PATH):$(TOOLSDIR)" $(GET_HELM) --no-sudo -v $(HELM_VER)
	$Q mv $(TOOLSDIR)/helm $(TOOLSDIR)/helm-$(HELM_VER)
	$Q rm -f $(GET_HELM)

# gen-crd-api-reference-docs is used for CRD API doc generation
.PHONY: gen-crd-api-reference-docs
gen-crd-api-reference-docs: $(GEN_CRD_API_REFERENCE_DOCS) ## Download gen-crd-api-reference-docs locally if necessary.
$(GEN_CRD_API_REFERENCE_DOCS): | $(TOOLSDIR)
	$(call go-install-tool,$(GEN_CRD_API_REFERENCE_DOCS),github.com/elastic/crd-ref-docs,$(GEN_API_REF_DOCS_VERSION))

# mdtoc is used to generate a table of contents for our documentation
.PHONY: mdtoc
mdtoc: $(MDTOC) ## Download mdtoc locally if necessary.
$(MDTOC): | $(TOOLSDIR)
	$(call go-install-tool,$(MDTOC),sigs.k8s.io/mdtoc,$(MDTOC_VER))

# stern is used to collect logs for our e2e tests
.PHONY: stern
stern: $(STERN) ## Download stern locally if necessary.
$(STERN): | $(TOOLSDIR)
	$(call go-install-tool,$(STERN),github.com/stern/stern,$(STERN_VER))

# stern is used to collect logs for our e2e tests
.PHONY: embedmd
embedmd: $(EMBEDMD) ## Download stern locally if necessary.
$(EMBEDMD): | $(TOOLSDIR)
	$(call go-install-tool,$(EMBEDMD),github.com/campoy/embedmd,$(EMBEDMD_VER))

# minikube is used to set-up a local kubernetes cluster for dev work.
.PHONY: minikube
minikube: $(MINIKUBE) ## Download minikube locally if necessary.
$(MINIKUBE): | $(TOOLSDIR)
	$Q echo "Installing minikube-$(MINIKUBE_VER) to $(TOOLSDIR)"
	$Q curl -fsSL https://storage.googleapis.com/minikube/releases/$(MINIKUBE_VER)/minikube-$(OS)-$(ARCH) -o $(MINIKUBE)
	$Q chmod +x $(MINIKUBE)

.PHONY: lychee
lychee: $(LYCHEE) ## Download lychee locally if necessary.
$(LYCHEE): | $(TOOLSDIR)
	$Q echo "Installing lychee-$(LYCHEE_VER) to $(TOOLSDIR)"
ifeq ($(TOOL_OS),linux)
	$Q curl -fsSL https://github.com/lycheeverse/lychee/releases/download/lychee-v$(LYCHEE_VER)/lychee-$(LYCHEE_ARCH_LINUX)-unknown-linux-gnu.tar.gz | tar  xvzf  - -C $(TOOLSDIR)
	$Q mv $(TOOLSDIR)/lychee $(LYCHEE)
	$Q chmod +x $(LYCHEE)
else ifeq ($(TOOL_OS),darwin) ## Lychee is only published as a .dmg for MacOS.
	$Q hdiutil detach $(TOOLSDIR)/lychee || true ## Always attempt to unmount in case the volume was previously mounted
	$Q curl -fsSL https://github.com/lycheeverse/lychee/releases/download/lychee-v$(LYCHEE_VER)/lychee-arm64-macos.dmg -o $(TOOLSDIR)/lychee.dmg
	$Q hdiutil mount -mountroot $(TOOLSDIR) $(TOOLSDIR)/lychee.dmg
	$Q cp $(TOOLSDIR)/lychee/lychee $(LYCHEE)
	$Q hdiutil detach $(TOOLSDIR)/lychee
	$Q rm -rf $(TOOLSDIR)/lychee.dmg
	$Q chmod +x $(LYCHEE)
else
	$Q echo "lychee is only available for linux and arm64 MacOS"
	$Q exit 1
endif

# helm-docs is used to generate helm chart documentation
helm-docs: $(HELM_DOCS)
$(HELM_DOCS): | $(TOOLSDIR)
	$(call go-install-tool,$(HELM_DOCS),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELM_DOCS_VER))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(TOOLSDIR) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
