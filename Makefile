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

# Export is needed here so that the envsubst used in make targets has access to those variables even when they are not
# explictily set when calling make.
# The tag must have three digits with a leading v - i.e. v9.9.1
export TAG ?= v0.0.0
# Note: Registry defaults to non-existing registry intentionally to avoid overriding useful images.
export REGISTRY ?= nvidia.com

# If V is set to 1 the output will be verbose.
Q = $(if $(filter 1,$V),,@)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0

# Get the current OS and Architecture
ARCH ?= $(shell go env GOARCH)
OS ?= $(shell go env GOOS)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GO_VERSION ?= 1.22

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $@

TOOLSDIR ?= $(shell pwd)/hack/tools/bin
$(TOOLSDIR):
	@mkdir -p $@

CHARTSDIR ?= $(shell pwd)/hack/charts
$(CHARTSDIR):
	@mkdir -p $@

DPUSERVICESDIR ?= $(shell pwd)/deploy/dpuservices
$(DPUSERVICESDIR):
	@mkdir -p $@

REPOSDIR ?= $(shell pwd)/hack/repos
$(REPOSDIR):
	@mkdir -p $@

HELMDIR ?= $(shell pwd)/deploy/helm

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(TOOLSDIR)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(TOOLSDIR)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(TOOLSDIR)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT ?= $(TOOLSDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)
MOCKGEN ?= $(TOOLSDIR)/mockgen-$(MOCKGEN_VERSION)
GOTESTSUM ?= $(TOOLSDIR)/gotestsum-$(GOTESTSUM_VERSION)
ENVSUBST ?= $(TOOLSDIR)/envsubst-$(ENVSUBST_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= v0.0.0-20240110160329-8f8247fdc1c3
GOLANGCI_LINT_VERSION ?= v1.58.1
MOCKGEN_VERSION ?= v0.4.0
GOTESTSUM_VERSION ?= v1.11.0
DPF_PROVISIONING_CONTROLLER_REV ?= 51de20a46a98d97f29b93233de699937fb924a6f
ENVSUBST_VERSION ?= v1.4.2

## OVN Kubernetes Images
# We build 2 images for OVN Kubernetes. One for the DPU enabled nodes and another for the non DPU enabled ones. The
# reason we have to build 2 images is because the original code modifications that were done to support the DPU workers
# were not implemented in a way that they are non disruptive for the default flow. We thought we would not need to change
# the image running on the non DPU nodes but in fact that wasn't the case and we understood that very down the line.
# Given that this solution is not supposed to go beyond MVP, the solution with the 2 images should be sufficient for now.
# In case this solution needs to last longer, we should work on refactoring OVN Kubernetes and consolidate everything
# in one image and improve the maintainability of our fork.

# TODO: Find a way to build the base image via https://github.com/openshift/ovn-kubernetes/blob/release-4.14/Dockerfile
# You need to follow the commands below to produce the base image:
# 1. Setup OpenShift cluster 4.14 (this is what the tests were done against)
# 2. Run these commands to get the relevant images:
#    * `kubectl get ds -n openshift-ovn-kubernetes -o jsonpath='{.spec.template.spec.containers[?(@.name=="ovnkube-controller")].image}' ovnkube-node`
#    * `kubectl get ds -n openshift-ovn-kubernetes -o jsonpath='{.spec.template.spec.containers[?(@.name=="ovn-controller")].image}' ovnkube-node`
#    * In case these two match, we can use a single image. Otherwise, we might need to split this Dockerfile into two so
#      that each container gets its own image.
# 3. Login into the OpenShift node and retag the image to `harbor.mellanox.com/cloud-orchestration-dev/dpf/ovn-kubernetes-base:<SHA256_OF_INPUT_IMAGE>`
#    e.g. podman tag quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:ca01b0a7e924b17765df8145d8669611d513e3edb2ac6f3cd518d04b6d01de6e harbor.mellanox.com/cloud-orchestration-dev/dpf/ovn-kubernetes-base:ca01b0a7e924b17765df8145d8669611d513e3edb2ac6f3cd518d04b6d01de6e
# 4. Push the image
OVNKUBERNETES_BASE_IMAGE=gitlab-master.nvidia.com:5005/doca-platform-foundation/dpf-operator/ovn-kubernetes-base:ca01b0a7e924b17765df8145d8669611d513e3edb2ac6f3cd518d04b6d01de6e
# Points to branch dpf-23.09.0
OVN_REVISION=89fb67b6222d1e6a48fed3ae6d6ac486326c6ab2
# Points to branch dpf-4.14
OVNKUBERNETES_DPU_REVISION=4050b08d14eac5ccdac2e398a25ee790347b8cfa
# Points to branch dpf-4.14-non-dpu
OVNKUBERNETES_NON_DPU_REVISION=f73cdc1f764b36a1f4df9849e15f32eb0e0082c1

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Cleanup everything
	@rm -rf $(TOOLSDIR)
	@rm -rf $(CHARTSDIR)
	@rm -rf $(REPOSDIR)

##@ Dependencies

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(TOOLSDIR)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(TOOLSDIR)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(TOOLSDIR)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(TOOLSDIR)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

.PHONY: mockgen
mockgen: $(MOCKGEN) ## Download mockgen locally if necessary.
$(MOCKGEN): $(TOOLSDIR)
	$(call go-install-tool,$(MOCKGEN),go.uber.org/mock/mockgen,${MOCKGEN_VERSION})
	ln -f $(MOCKGEN) $(abspath $(TOOLSDIR)/mockgen)

# gotestsum is used to generate junit style test reports
.PHONY: gotestsum
gotestsum: $(GOTESTSUM) # download gotestsum locally if necessary
$(GOTESTSUM): $(TOOLSDIR)
	$(call go-install-tool,$(GOTESTSUM),gotest.tools/gotestsum,${GOTESTSUM_VERSION})

# envsubst is used to template files with environment variables
.PHONY: envsubst
envsubst: $(ENVSUBST) # download gotestsum locally if necessary
$(ENVSUBST): $(TOOLSDIR)
	$(call go-install-tool,$(ENVSUBST),github.com/a8m/envsubst/cmd/envsubst,${ENVSUBST_VERSION})


# helm is used to manage helm deployments and artifacts.
GET_HELM = $(TOOLSDIR)/get_helm.sh
HELM_VER = v3.13.3
HELM_BIN = helm
HELM = $(abspath $(TOOLSDIR)/$(HELM_BIN)-$(HELM_VER))
$(HELM): | $(TOOLSDIR)
	$Q echo "Installing helm-$(HELM_VER) to $(TOOLSDIR)"
	$Q curl -fsSL -o $(GET_HELM) https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
	$Q chmod +x $(GET_HELM)
	$Q env HELM_INSTALL_DIR=$(TOOLSDIR) PATH=$(PATH):$(TOOLSDIR) $(GET_HELM) --no-sudo -v $(HELM_VER)
	$Q mv $(TOOLSDIR)/$(HELM_BIN) $(TOOLSDIR)/$(HELM_BIN)-$(HELM_VER)
	$Q rm -f $(GET_HELM)


# helmify is used to generate helm charts.
HELMIFY_VER = v0.4.13
HELMIFY_BIN = helmify
HELMIFY = $(abspath $(TOOLSDIR)/$(HELMIFY_BIN)-$(HELMIFY_VER))
HELMIFY_ARCH = $(shell echo $(ARCH) | sed s/amd64/x86_64/) # The helmify URL uses the format x86_64 for its architecture.
$(HELMIFY): | $(TOOLSDIR)
	$Q echo "Installing helmify-$(HELMIFY_VER) to $(TOOLSDIR)"
	echo $(strip https://github.com/arttor/helmify/releases/download/$(HELMIFY_VER)/helmify_$(OS)_$(HELMIFY_ARCH).tar.gz)
	curl -fL -o $(TOOLSDIR)/helmify.tar.gz https://github.com/arttor/helmify/releases/download/$(HELMIFY_VER)/helmify_$(OS)_$(strip $(HELMIFY_ARCH)).tar.gz
	tar -xvf $(TOOLSDIR)/helmify.tar.gz -C $(TOOLSDIR) && rm $(TOOLSDIR)/helmify.tar.gz
	mv $(TOOLSDIR)/$(HELMIFY_BIN) $(HELMIFY)
	chmod +x $(HELMIFY)

# kamaji is the underlying control plane provider
KAMAJI_REPO_URL=https://clastix.github.io/charts
KAMAJI_REPO_NAME=clastix
KAMAJI_CHART_VERSION=0.15.2
KAMAJI_CHART_NAME=kamaji
KAMAJI := $(abspath $(CHARTSDIR)/$(KAMAJI_CHART_NAME)-$(KAMAJI_CHART_VERSION).tgz)
$(KAMAJI): | $(CHARTSDIR) $(HELM)
	$Q $(HELM) repo add $(KAMAJI_REPO_NAME) $(KAMAJI_REPO_URL)
	$Q $(HELM) repo update
	$Q $(HELM) pull $(KAMAJI_REPO_NAME)/$(KAMAJI_CHART_NAME) --version $(KAMAJI_CHART_VERSION) -d $(CHARTSDIR)

# skaffold is used to run a debug build of the network operator for dev work.
SKAFFOLD_VER := v2.10.0
SKAFFOLD_BIN := skaffold
SKAFFOLD := $(abspath $(TOOLSDIR)/$(SKAFFOLD_BIN)-$(SKAFFOLD_VER))
$(SKAFFOLD): | $(TOOLSDIR)
	$Q echo "Installing skaffold-$(SKAFFOLD_VER) to $(TOOLSDIR)"
	$Q curl -fsSL https://storage.googleapis.com/skaffold/releases/$(SKAFFOLD_VER)/skaffold-$(OS)-$(ARCH) -o $(SKAFFOLD)
	$Q chmod +x $(SKAFFOLD)

# minikube is used to set-up a local kubernetes cluster for dev work.
MINIKUBE_VER := v1.33.1
MINIKUBE_BIN := minikube
MINIKUBE := $(abspath $(TOOLSDIR)/$(MINIKUBE_BIN)-$(MINIKUBE_VER))
$(MINIKUBE): | $(TOOLSDIR)
	$Q echo "Installing minikube-$(MINIKUBE_VER) to $(TOOLSDIR)"
	$Q curl -fsSL https://storage.googleapis.com/minikube/releases/$(MINIKUBE_VER)/minikube-$(OS)-$(ARCH) -o $(MINIKUBE)
	$Q chmod +x $(MINIKUBE)

# cert-manager is used for webhook certs in the dev setup.
CERT_MANAGER_YAML=$(CHARTSDIR)/cert-manager.yaml
CERT_MANAGER_VER=v1.13.3
$(CERT_MANAGER_YAML): | $(CHARTSDIR)
	curl -fSsL "https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VER)/cert-manager.yaml" -o $(CERT_MANAGER_YAML)

# argoCD is the underlying application service provider
ARGOCD_YAML=$(CHARTSDIR)/argocd.yaml
ARGOCD_VER=v2.10.1
$(ARGOCD_YAML): | $(CHARTSDIR)
	curl -fSsL "https://raw.githubusercontent.com/argoproj/argo-cd/$(ARGOCD_VER)/manifests/install.yaml" -o $(ARGOCD_YAML)

# Token used to pull from internal git registries. Used to enable https authentication in git clones from the internal NVIDIA gitlab.
GITLAB_TOKEN ?= ""

# OVS CNI
OVS_CNI_REVISION ?= a88c163a7539aea1bfe07e8e00f722ccbfe07fc2
OVS_CNI_DIR=$(REPOSDIR)/ovs-cni-$(OVS_CNI_REVISION)
$(OVS_CNI_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/dpf-sfc-cni.git $(OVS_CNI_DIR) $(OVS_CNI_REVISION)

# HBN side car
HBN_SIDECAR_DIR=$(REPOSDIR)/hbn-sidecar
$(HBN_SIDECAR_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/aserdean/hbn-sidecar.git $(HBN_SIDECAR_DIR)

## Parprouted image with DPF patches
PARPROUTED_REVISION ?= cda116e6312bd583b5ceed7c814d1bf539f33f6f
PARPROUTED_DIR=$(REPOSDIR)/parprouted-$(PARPROUTED_REVISION)
$(PARPROUTED_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/parprouted.git $(PARPROUTED_DIR) $(PARPROUTED_REVISION)

# OVN Kubernetes dependencies to be able to build its docker image
OVNKUBERNETES_DPU_DIR=$(REPOSDIR)/ovn-kubernetes-dpu-$(OVNKUBERNETES_DPU_REVISION)
$(OVNKUBERNETES_DPU_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/ovn-kubernetes.git $(OVNKUBERNETES_DPU_DIR) $(OVNKUBERNETES_DPU_REVISION)

OVNKUBERNETES_NON_DPU_DIR=$(REPOSDIR)/ovn-kubernetes-non-dpu-$(OVNKUBERNETES_NON_DPU_REVISION)
$(OVNKUBERNETES_NON_DPU_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/ovn-kubernetes.git $(OVNKUBERNETES_NON_DPU_DIR) $(OVNKUBERNETES_NON_DPU_REVISION)

OVN_DIR=$(REPOSDIR)/ovn-$(OVN_REVISION)
$(OVN_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/ovn.git $(OVN_DIR) $(OVN_REVISION)

DPF_PROVISIONING_DIR=$(REPOSDIR)/dpf-provisioning-controller-$(DPF_PROVISIONING_CONTROLLER_REV)
$(DPF_PROVISIONING_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/dpf-provisioning-controller.git $(DPF_PROVISIONING_DIR) $(DPF_PROVISIONING_CONTROLLER_REV)

# operator-sdk is used to generate operator-sdk bundles
OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download
OPERATOR_SDK_BIN = operator-sdk
OPERATOR_SDK_VER = v1.34.1
OPERATOR_SDK = $(abspath $(TOOLSDIR)/$(OPERATOR_SDK_BIN)-$(OPERATOR_SDK_VER))
$(OPERATOR_SDK): | $(TOOLSDIR)
	$Q echo "Installing $(OPERATOR_SDK_BIN)-$(OPERATOR_SDK_VER) to $(TOOLSDIR)"
	$Q curl -sSfL $(OPERATOR_SDK_DL_URL)/$(OPERATOR_SDK_VER)/operator-sdk_$(OS)_$(ARCH) -o $(OPERATOR_SDK)
	$Q chmod +x $(OPERATOR_SDK)

##@ Development
GENERATE_TARGETS ?= operator dpuservice dpf-provisioning hostcniprovisioner dpucniprovisioner sfcset operator-embedded ovnkubernetes-operator ovnkubernetes-operator-embedded sfc-controller release-defaults hbn-dpuservice ovs-cni

.PHONY: generate
generate: ## Run all generate-* targets: generate-modules generate-manifests-* and generate-go-deepcopy-*.
	$(MAKE) generate-mocks generate-modules generate-manifests generate-go-deepcopy generate-operator-bundle generate-helm-chart-operator

.PHONY: generate-mocks
generate-mocks: $(MOCKGEN) ## Generate mocks
	## Add the TOOLSDIR to the path for this command as `mockgen` is called from the $PATH inline in the code.
	## See go:generate comments for examples.
	export PATH=$(PATH):$(TOOLSDIR); go generate ./...

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to update go modules
	go mod tidy

.PHONY: generate-manifests
generate-manifests: controller-gen kustomize $(addprefix generate-manifests-,$(GENERATE_TARGETS)) ## Run all generate-manifests-* targets

.PHONY: generate-manifests-operator
generate-manifests-operator: $(KUSTOMIZE) $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC. for the operator controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/operator/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/operator/..." \
	paths="./internal/operator/..." \
	paths="./api/operator/..." \
	crd:crdVersions=v1 \
	rbac:roleName=manager-role \
	output:crd:dir=./config/operator/crd/bases \
	output:rbac:dir=./config/operator/rbac
	cd config/operator/manager && $(KUSTOMIZE) edit set image controller=$(DPFOPERATOR_IMAGE):$(TAG)

.PHONY: generate-manifests-ovnkubernetes-operator
generate-manifests-ovnkubernetes-operator: $(KUSTOMIZE) $(CONTROLLER_GEN) $(ENVSUBST) ## Generate manifests e.g. CRD, RBAC. for the OVN Kubernetes operator controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/ovnkubernetesoperator/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/ovnkubernetesoperator/..." \
	paths="./internal/ovnkubernetesoperator/..." \
	paths="./internal/ovnkubernetesoperator/webhooks" \
	paths="./api/ovnkubernetesoperator/..." \
	crd:crdVersions=v1 \
	rbac:roleName=manager-role \
	output:crd:dir=./config/ovnkubernetesoperator/crd/bases \
	output:rbac:dir=./config/ovnkubernetesoperator/rbac \
	output:webhook:dir=./config/ovnkubernetesoperator/webhook \
    webhook
	cd config/ovnkubernetesoperator/manager && $(KUSTOMIZE) edit set image controller=$(DPFOVNKUBERNETESOPERATOR_IMAGE):$(TAG)
	rm -rf deploy/helm/dpf-ovn-kubernetes-operator/crds/* && find config/ovnkubernetesoperator/crd/bases/ -type f -exec cp {} deploy/helm/dpf-ovn-kubernetes-operator/crds/ \;
	$(ENVSUBST) < deploy/helm/dpf-ovn-kubernetes-operator/values.yaml.tmpl > deploy/helm/dpf-ovn-kubernetes-operator/values.yaml

.PHONY: generate-manifests-dpuservice
generate-manifests-dpuservice: $(KUSTOMIZE) $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC. for the dpuservice controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/dpuservice/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/dpuservice/..." \
	paths="./internal/dpuservice/..." \
	paths="./internal/dpuservicechain/..." \
	paths="./api/dpuservice/..." \
	crd:crdVersions=v1 \
	rbac:roleName=manager-role \
	output:crd:dir=./config/dpuservice/crd/bases \
	output:rbac:dir=./config/dpuservice/rbac
	cd config/dpuservice/manager && $(KUSTOMIZE) edit set image controller=$(DPUSERVICE_IMAGE):$(TAG)

.PHONY: generate-manifests-dpucniprovisioner
generate-manifests-dpucniprovisioner: $(KUSTOMIZE) ## Generates DPU CNI provisioner manifests
	cd config/dpucniprovisioner/default && $(KUSTOMIZE) edit set image controller=$(DPUCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: generate-manifests-hostcniprovisioner
generate-manifests-hostcniprovisioner: $(KUSTOMIZE) ## Generates Host CNI provisioner manifests
	cd config/hostcniprovisioner/default &&	$(KUSTOMIZE) edit set image controller=$(HOSTCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: generate-manifests-release-defaults
generate-manifests-release-defaults: $(ENVSUBST) ## Generates manifests that contain the default values that should be used by the operators
	$(ENVSUBST) < ./internal/release/templates/defaults.yaml.tmpl > ./internal/release/manifests/defaults.yaml

TEMPLATES_DIR ?= $(CURDIR)/internal/operator/inventory/templates
EMBEDDED_MANIFESTS_DIR ?= $(CURDIR)/internal/operator/inventory/manifests
.PHONY: generate-manifests-operator-embedded
generate-manifests-operator-embedded: $(ENVSUBST) generate-manifests-dpuservice generate-manifests-dpf-provisioning generate-manifests-release-defaults ## Generates manifests that are embedded into the operator binary.
	cp $(DPF_PROVISIONING_DIR)/output/deploy.yaml ./internal/operator/inventory/manifests/dpf-provisioning-controller.yaml
	$(KUSTOMIZE) build config/dpuservice/default > $(EMBEDDED_MANIFESTS_DIR)/dpuservice-controller.yaml
	# Substitute environment variables and generate embedded manifests from templates.
	$(ENVSUBST) < $(TEMPLATES_DIR)/servicefunctionchainset-controller.yaml.tmpl > $(EMBEDDED_MANIFESTS_DIR)/servicefunctionchainset-controller.yaml
	$(ENVSUBST) < $(TEMPLATES_DIR)/multus.yaml.tmpl > $(EMBEDDED_MANIFESTS_DIR)/multus.yaml
	$(ENVSUBST) < $(TEMPLATES_DIR)/sriov-device-plugin.yaml.tmpl > $(EMBEDDED_MANIFESTS_DIR)/sriov-device-plugin.yaml
	$(ENVSUBST) < $(TEMPLATES_DIR)/flannel.yaml.tmpl > $(EMBEDDED_MANIFESTS_DIR)/flannel.yaml
	$(ENVSUBST) < $(TEMPLATES_DIR)/nv-k8s-ipam.yaml.tmpl > $(EMBEDDED_MANIFESTS_DIR)/nv-k8s-ipam.yaml
	$(ENVSUBST) < $(TEMPLATES_DIR)/ovs-cni.yaml.tmpl > $(EMBEDDED_MANIFESTS_DIR)/ovs-cni.yaml
	$(ENVSUBST) < $(TEMPLATES_DIR)/sfc-controller.yaml.tmpl > $(EMBEDDED_MANIFESTS_DIR)/sfc-controller.yaml

.PHONY: generate-manifests-ovnkubernetes-operator-embedded
generate-manifests-ovnkubernetes-operator-embedded: generate-manifests-dpucniprovisioner generate-manifests-hostcniprovisioner ## Generates manifests that are embedded into the OVN Kubernetes Operator binary.
	$(KUSTOMIZE) build config/hostcniprovisioner/default > ./internal/ovnkubernetesoperator/controllers/manifests/hostcniprovisioner.yaml
	$(KUSTOMIZE) build config/dpucniprovisioner/default > ./internal/ovnkubernetesoperator/controllers/manifests/dpucniprovisioner.yaml

.PHONY: generate-manifests-sfcset
generate-manifests-sfcset: $(KUSTOMIZE) $(ENVSUBST) ## Generate manifests e.g. CRD, RBAC. for the sfcset controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/servicechainset/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/servicechainset/..." \
	paths="./internal/servicechainset/..." \
	paths="./api/servicechain/..." \
	crd:crdVersions=v1,generateEmbeddedObjectMeta=true \
	rbac:roleName=manager-role \
	output:crd:dir=./config/servicechainset/crd/bases \
	output:rbac:dir=./config/servicechainset/rbac
	cd config/servicechainset/manager && $(KUSTOMIZE) edit set image controller=$(SFCSET_IMAGE):$(TAG)
	find config/servicechainset/crd/bases/ -type f -not -name '*dpu*' -exec cp {} deploy/helm/servicechain/crds/ \;
	# Template the image name and tag used in the helm templates.
	$(ENVSUBST) < deploy/helm/servicechain/values.yaml.tmpl > deploy/helm/servicechain/values.yaml

.PHONY: generate-manifests-sfc-controller
generate-manifests-sfc-controller: generate-manifests-sfcset $(ENVSUBST)
	cp deploy/helm/servicechain/crds/sfc.dpf.nvidia.com_servicechains.yaml deploy/helm/sfc-controller/crds/
	cp deploy/helm/servicechain/crds/sfc.dpf.nvidia.com_serviceinterfaces.yaml deploy/helm/sfc-controller/crds/
	# Template the image name and tag used in the helm templates.
	$(ENVSUBST) < deploy/helm/sfc-controller/values.yaml.tmpl > deploy/helm/sfc-controller/values.yaml

.PHONY: generate-manifests-dpf-provisioning
generate-manifests-dpf-provisioning: $(KUSTOMIZE) $(DPF_PROVISIONING_DIR) ## Generate manifests e.g. CRD, RBAC. for the DPF provisioning controller.
	$(MAKE) IMG=$(DPFPROVISIONING_IMAGE):$(TAG) -C $(DPF_PROVISIONING_DIR) kustomize-build
	$(KUSTOMIZE) build $(DPF_PROVISIONING_DIR)/config/crd > ./config/dpf-provisioning/crd/bases/crds.yaml

.PHONY: generate-manifests-hbn-dpuservice
generate-manifests-hbn-dpuservice: $(ENVSUBST)
	$(ENVSUBST) < deploy/dpuservices/hbn/chart/values.yaml.tmpl > deploy/dpuservices/hbn/chart/values.yaml

.PHONY: generate-manifests-ovs-cni
generate-manifests-ovs-cni: $(ENVSUBST) ## Generate values for OVS helm chart.
	$(ENVSUBST) < deploy/helm/ovs-cni/values.yaml.tmpl > deploy/helm/ovs-cni/values.yaml

OPERATOR_HELM_CHART_NAME ?= dpf-operator
OPERATOR_HELM_CHART ?= $(CHARTSDIR)/$(OPERATOR_HELM_CHART_NAME)
.PHONY: generate-helm-chart-operator
generate-helm-chart-operator: $(KUSTOMIZE) $(HELMIFY) generate-manifests-operator generate-manifests-operator-embedded ## Create a helm chart for the DPF Operator.
	$(KUSTOMIZE) build  config/operator-and-crds | $(HELMIFY) -image-pull-secrets -crd-dir -generate-defaults $(OPERATOR_HELM_CHART)

.PHONY: generate-operator-bundle
generate-operator-bundle: $(OPERATOR_SDK) $(KUSTOMIZE) ## Generate bundle manifests and metadata, then validate generated files.
	$(KUSTOMIZE) build config/operator-operatorsdk-bundle | $(OPERATOR_SDK) generate bundle \
	--overwrite --package dpf-operator --version $(BUNDLE_VERSION) --default-channel=$(BUNDLE_VERSION) --channels=$(BUNDLE_VERSION)
	# Remove the createdAt field to prevent rebasing issues.
	# TODO: Currently the clusterserviceversion is not being correctly generated e.g. metadata is missing.
	$Q sed -i '/  createdAt:/d'  bundle/manifests/dpf-operator.clusterserviceversion.yaml
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: clean-generated-yaml
clean-generated-yaml: ## Remove files generated by controller-tools from the mentioned dirs.
	(IFS=','; for i in $(SRC_DIRS); do find $$i -type f -name '*.yaml' -exec rm -f {} \;; done)

.PHONY: generate-go-deepcopy
generate-go-deepcopy: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(MAKE) clean-generated-deepcopy SRC_DIRS="./api"
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

.PHONY: clean-generated-deepcopy
clean-generated-deepcopy: ## Remove files generated by golang from the mentioned dirs.
	(IFS=','; for i in $(SRC_DIRS); do find $$i -type f -name 'zz_generated.deepcopy*' -exec rm -f {} \;; done)

##@ Testing

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test $$(go list ./... | grep -v /e2e)

.PHONY: test-report $(GOTESTSUM)
test-report: envtest gotestsum ## Run tests and generate a junit style report
	set +o errexit; KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test -count 1 -race -json $$(go list ./... | grep -v /e2e) > junit.stdout; echo $$? > junit.exitcode;
	$(GOTESTSUM) --junitfile junit.xml --raw-command cat junit.stdout
	exit $$(cat junit.exitcode)

TEST_CLUSTER_NAME := dpf-test
test-env-e2e: $(KAMAJI) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(MINIKUBE) $(ENVSUBST) ## Setup a Kubernetes environment to run tests.
	# Create a minikube cluster to host the test.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

	$(KUBECTL) create namespace dpf-operator-system

	# Create secrets required for using artefacts if required.
	$(CURDIR)/hack/scripts/create-artefact-secrets.sh

	# Deploy cert manager to provide certificates for webhooks.
	$Q $(KUBECTL) apply -f $(CERT_MANAGER_YAML)

	# Mirror images for e2e tests from docker hub and push them in the test registry to avoid docker pull limits.
	$Q eval $$($(MINIKUBE) -p $(TEST_CLUSTER_NAME) docker-env); \
	$(MAKE) test-build-and-push-artifacts

	echo "Waiting for cert-manager deployment to be ready."
	$(KUBECTL) wait --for=condition=ready pod -l app=webhook --timeout=180s -n cert-manager

	# Deploy Kamaji as the underlying control plane provider.
	$Q $(HELM) upgrade --set image.pullPolicy=IfNotPresent --set cfssl.image.tag=v1.6.5 --install kamaji $(KAMAJI)


.PHONY: test-build-and-push-artifacts
test-build-and-push-artifacts: $(KUSTOMIZE) ## Build and push DPF artifacts (images, charts, bundle) for e2e tests.
	# Build and push the sfcset, dpuservice, operator and operator-bundle images.
	$Q eval $$($(MINIKUBE) -p $(TEST_CLUSTER_NAME) docker-env); \
	$(MAKE) docker-build-sfc-controller docker-push-sfc-controller
	$(MAKE) docker-build-dpuservice docker-push-dpuservice; \
	$(MAKE) docker-build-operator docker-push-operator ; \
	$(MAKE) docker-build-operator-bundle docker-push-operator-bundle; \
	$(MAKE) docker-build-sfcset docker-push-sfcset; \
	$(MAKE) docker-build-dpf-provisioning docker-push-dpf-provisioning;

	# Build and push all the helm charts
	$(MAKE) helm-package-all helm-push-all

OPERATOR_NAMESPACE ?= dpf-operator-system

.PHONY: test-deploy-operator-kustomize
test-deploy-operator-kustomize: $(KUSTOMIZE) ## Deploy the DPF Operator using kustomize
	cd config/operator-and-crds/ && $(KUSTOMIZE) edit set namespace $(OPERATOR_NAMESPACE)
	cd config/operator/manager && $(KUSTOMIZE) edit set image controller=$(DPFOPERATOR_IMAGE):$(TAG)
	$(KUSTOMIZE) build config/operator-and-crds/ | $(KUBECTL) apply -f -

OLM_VERSION ?= v0.28.0
OPERATOR_REGISTRY_VERSION ?= v1.43.1
OLM_DIR = $(REPOSDIR)/olm
OLM_DOWNLOAD_URL = https://github.com/operator-framework/operator-lifecycle-manager/releases/download
.PHONY: test-install-operator-lifecycle-manager
test-install-operator-lifecycle-manager:
	mkdir -p $(OLM_DIR)
	curl -L $(OLM_DOWNLOAD_URL)/$(OLM_VERSION)/crds.yaml -o $(OLM_DIR)/crds.yaml
	curl -L $(OLM_DOWNLOAD_URL)/$(OLM_VERSION)/olm.yaml -o $(OLM_DIR)/olm.yaml

	# Operator SDK always installs the `latest` image for the configmap-operator. We need to replace that for this installation.
	$Q sed -i 's/configmap-operator-registry:latest/configmap-operator-registry:$(OLM_VERSION)/g' $(OLM_DIR)/olm.yaml

	$(KUBECTL) create -f "$(OLM_DIR)/crds.yaml"
	$(KUBECTL) wait --for=condition=Established -f "$(OLM_DIR)/crds.yaml"
	$(KUBECTL) create -f "$(OLM_DIR)/olm.yaml"
	$(KUBECTL) rollout status -w deployment/olm-operator --namespace=olm
	$(KUBECTL) rollout status -w deployment/catalog-operator --namespace=olm


OPERATOR_SDK_RUN_BUNDLE_EXTRA_ARGS ?= ""
.PHONY: test-deploy-operator-operator-sdk
test-deploy-operator-operator-sdk: $(KUSTOMIZE) test-install-operator-lifecycle-manager ## Deploy the DPF Operator using operator-sdk
	# Create the namespace for the operator to be installed.
	$(KUBECTL) create namespace $(OPERATOR_NAMESPACE)
	# TODO: This flow does not work on MacOS dues to some issue pulling images. Should be enabled to make local testing equivalent to CI.
	$(OPERATOR_SDK) run bundle --namespace $(OPERATOR_NAMESPACE) --index-image quay.io/operator-framework/opm:$(OPERATOR_REGISTRY_VERSION) $(OPERATOR_SDK_RUN_BUNDLE_EXTRA_ARGS) $(OPERATOR_BUNDLE_IMAGE):$(BUNDLE_VERSION)

.PHONY: test-undeploy-operator-kustomize
test-undeploy-operator-kustomize: $(KUSTOMIZE) ## Undeploy the DPF Operator using kustomize
	$(KUSTOMIZE) build config/operator-and-crds/ | $(KUBECTL) delete -f -

ARTIFACTS_DIR ?= $(shell pwd)/artifacts
.PHONY: test-cache-images
test-cache-images: $(MINIKUBE) ## Add images to the minikube cache based on the artifacts directory created in e2e.
	# Run a script which will cache images which were pulled in the test run to the minikube cache.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) ARTIFACTS_DIR=${ARTIFACTS_DIR} $(CURDIR)/hack/scripts/add-images-to-minikube-cache.sh

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  ## Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e: ## Run e2e tests
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: clean-test-env
clean-test-env: $(MINIKUBE) ## Clean test environment (teardown minikube cluster)
	$(MINIKUBE) delete -p $(TEST_CLUSTER_NAME)

##@ lint and verify
.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: verify-generate
verify-generate: generate ## Verify auto-generated code did not change
	$(info checking for git diff after running 'make generate')
	# Use intent-to-add to check for untracked files after generation.
	git add -N .
	$Q git diff --quiet ':!bundle' ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make generate'"; exit 1 ; fi
	# Files under `bundle` are verified here. The createdAt field is excluded as it is always updated at generation time and is not relevant to the bundle.
	$Q git diff --quiet -I'^    createdAt: ' bundle ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make generate'"; exit 1 ; fi

.PHONY: verify-copyright
verify-copyright: ## Verify copyrights for project files
	$Q $(CURDIR)/hack/scripts/copyright-validation.sh

.PHONY: lint-helm
lint-helm: $(HELM) lint-helm-sfcset lint-helm-multus lint-helm-sriov-dp lint-helm-nvidia-k8s-ipam lint-helm-ovs-cni lint-helm-sfc-controller lint-helm-ovnkubernetes-operator

.PHONY: lint-helm-sfcset
lint-helm-sfcset: $(HELM) ## Run helm lint for sfcset chart
	$Q $(HELM) lint $(SERVICECHAIN_CONTROLLER_HELM_CHART)

.PHONY: lint-helm-multus
lint-helm-multus: $(HELM) ## Run helm lint for multus chart
	$Q $(HELM) lint $(MULTUS_HELM_CHART)

.PHONY: lint-helm-sriov-dp
lint-helm-sriov-dp: $(HELM) ## Run helm lint for sriov device plugin chart
	$Q $(HELM) lint $(SRIOV_DP_HELM_CHART)

.PHONY: lint-helm-nvidia-k8s-ipam
lint-helm-nvidia-k8s-ipam: $(HELM) ## Run helm lint for nvidia-k8s-ipam chart
	$Q $(HELM) lint $(NVIDIA_K8S_IPAM_HELM_CHART)

.PHONY: lint-helm-ovs-cni
lint-helm-ovs-cni: $(HELM) ## Run helm lint for ovs-cni chart
	$Q $(HELM) lint $(OVS_CNI_HELM_CHART)

.PHONY: lint-helm-sfc-controller
lint-helm-sfc-controller: $(HELM) ## Run helm lint for sfc controller chart
	$Q $(HELM) lint $(SFC_CONTOLLER_HELM_CHART)

.PHONY: lint-helm-ovnkubernetes-operator
lint-helm-ovnkubernetes-operator: $(HELM) ## Run helm lint for OVN Kubernetes Operator chart
	$Q $(HELM) lint $(DPFOVNKUBERNETESOPERATOR_HELM_CHART)

##@ Release

.PHONY: release
release: generate # Build and push helm and container images for release.
	# Build arm64 images which will run on DPUs.
	$(MAKE) ARCH=$(DPU_ARCH) $(addprefix docker-build-,$(DPU_ARCH_DOCKER_BUILD_TARGETS))
	# Build amd64 images which will run on x86 hosts.
	$(MAKE) ARCH=$(HOST_ARCH) $(addprefix docker-build-,$(HOST_ARCH_DOCKER_BUILD_TARGETS))
	# Push all of the images
	$(MAKE) docker-push-all

	# Package and push the helm charts.
	$(MAKE) helm-package-all helm-push-all

##@ Build

GO_GCFLAGS=""
GO_LDFLAGS="-extldflags '-static'"
BUILD_TARGETS ?= $(DPU_ARCH_BUILD_TARGETS) $(HOST_ARCH_BUILD_TARGETS)
DPU_ARCH_BUILD_TARGETS ?= dpucniprovisioner sfcset
HOST_ARCH_BUILD_TARGETS ?= operator dpuservice hostcniprovisioner ovnkubernetes-operator
BUILD_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

# The BUNDLE_VERSION is the same as the TAG but the first character is stripped. This is used to strip a leading `v` which is invalid for Bundle versions.
$(eval BUNDLE_VERSION := $$$(TAG))

HOST_ARCH = amd64
# Note: If you make this variable configurable, ensure that the custom base image that is built in
# docker-build-base-image-ovs is fetching binaries with the correct architecture.
DPU_ARCH = arm64

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
BASE_IMAGE = gcr.io/distroless/static:nonroot
ALPINE_IMAGE = alpine:3.19

.PHONY: binaries
binaries: $(addprefix binary-,$(BUILD_TARGETS)) ## Build all binaries

.PHONY: binary-sfcset
binary-sfcset: ## Build the sfcset controller binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/sfcset-manager gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/servicechainset

.PHONY: binary-operator
binary-operator: generate-manifests-operator-embedded ## Build the operator controller binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/operator-manager gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/operator

.PHONY: binary-dpuservice
binary-dpuservice: ## Build the dpuservice controller binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpuservice-manager gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/dpuservice

.PHONY: binary-dpucniprovisioner
binary-dpucniprovisioner: ## Build the DPU CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpucniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/dpucniprovisioner

.PHONY: binary-hostcniprovisioner
binary-hostcniprovisioner: ## Build the Host CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/hostcniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/hostcniprovisioner

.PHONY: binary-sfc-controller
binary-sfc-controller: ## Build the Host CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/sfc-controller gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/sfc-controller

.PHONY: binary-ovnkubernetes-operator
binary-binary-ovnkubernetes-operator: generate-manifests-ovnkubernetes-operator-embedded ## Build the OVN Kubernetes operator.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/ovnkubernetesoperator gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/ovnkubernetesoperator

DOCKER_BUILD_TARGETS=$(HOST_ARCH_DOCKER_BUILD_TARGETS) $(DPU_ARCH_DOCKER_BUILD_TARGETS)
HOST_ARCH_DOCKER_BUILD_TARGETS=$(HOST_ARCH_BUILD_TARGETS) ovnkubernetes-dpu ovnkubernetes-non-dpu operator-bundle dpf-provisioning hostnetwork parprouted dms dhcrelay ovnkubernetes-operator hbn
DPU_ARCH_DOCKER_BUILD_TARGETS=$(DPU_ARCH_BUILD_TARGETS) sfc-controller hbn-sidecar ovs-cni

.PHONY: docker-build-all
docker-build-all: $(addprefix docker-build-,$(DOCKER_BUILD_TARGETS)) ## Build docker images for all DOCKER_BUILD_TARGETS. Architecture defaults to build system architecture unless overridden or hardcoded.

OVS_BASE_IMAGE_NAME = base-image-ovs
OVS_BASE_IMAGE = $(REGISTRY)/$(OVS_BASE_IMAGE_NAME)

SYSTEMD_BASE_IMAGE_NAME = base-image-systemd
SYSTEMD_BASE_IMAGE = $(REGISTRY)/$(SYSTEMD_BASE_IMAGE_NAME)

# Images that are running on the DPU enabled host cluster nodes (workers)
OVNKUBERNETES_DPU_IMAGE_NAME = ovn-kubernetes-dpu
export OVNKUBERNETES_DPU_IMAGE = $(REGISTRY)/$(OVNKUBERNETES_DPU_IMAGE_NAME)

# Images that are running on the non DPU host cluster nodes (control plane)
OVNKUBERNETES_NON_DPU_IMAGE_NAME = ovn-kubernetes-non-dpu
export OVNKUBERNETES_NON_DPU_IMAGE = $(REGISTRY)/$(OVNKUBERNETES_NON_DPU_IMAGE_NAME)

DPFOPERATOR_IMAGE_NAME ?= dpf-operator-controller-manager
DPFOPERATOR_IMAGE ?= $(REGISTRY)/$(DPFOPERATOR_IMAGE_NAME)

SFCSET_IMAGE_NAME ?= sfcset-controller-manager
export SFCSET_IMAGE ?= $(REGISTRY)/$(SFCSET_IMAGE_NAME)

DPUSERVICE_IMAGE_NAME ?= dpuservice-controller-manager
DPUSERVICE_IMAGE ?= $(REGISTRY)/$(DPUSERVICE_IMAGE_NAME)

DPFPROVISIONING_IMAGE_NAME ?= dpf-provisioning-controller-manager
export DPFPROVISIONING_IMAGE ?= $(REGISTRY)/$(DPFPROVISIONING_IMAGE_NAME)

SFC_CONTROLLER_IMAGE_NAME ?= sfc-controller-manager
export SFC_CONTROLLER_IMAGE ?= $(REGISTRY)/$(SFC_CONTROLLER_IMAGE_NAME)

OVS_CNI_IMAGE_NAME ?= ovs-cni-plugin
export OVS_CNI_IMAGE ?= $(REGISTRY)/$(OVS_CNI_IMAGE_NAME)

## TODO: Cleanup image building and versioning for dhcrelay, parprouted and hostnetwork.
DHCRELAY_VERSION ?= 0.1
export DHCRELAY_IMAGE ?= $(REGISTRY)/dhcrelay:v$(DHCRELAY_VERSION)

export PARPROUTED_IMAGE ?= $(REGISTRY)/parprouted:$(TAG)

export HOSTNETWORK_IMAGE ?= $(REGISTRY)/hostnetworksetup:v$(HOSTNETWORK_VERSION)
HOSTNETWORK_VERSION ?= 0.1

export DMS_IMAGE ?= $(REGISTRY)/dms-server:v$(DMS_VERSION)
DMS_VERSION ?= 2.7

HOSTCNIPROVISIONER_IMAGE_NAME ?= host-cni-provisioner
HOSTCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(HOSTCNIPROVISIONER_IMAGE_NAME)

OPERATOR_BUNDLE_NAME ?= dpf-operator-bundle
OPERATOR_BUNDLE_REGISTRY ?= $(REGISTRY)
OPERATOR_BUNDLE_IMAGE ?= $(OPERATOR_BUNDLE_REGISTRY)/$(OPERATOR_BUNDLE_NAME)

# Images that are running on DPU worker nodes (arm64)
DPUCNIPROVISIONER_IMAGE_NAME ?= dpu-cni-provisioner
DPUCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(DPUCNIPROVISIONER_IMAGE_NAME)

DPFOVNKUBERNETESOPERATOR_IMAGE_NAME ?= dpf-ovn-kubernetes-operator-controller-manager
export DPFOVNKUBERNETESOPERATOR_IMAGE ?= $(REGISTRY)/$(DPFOVNKUBERNETESOPERATOR_IMAGE_NAME)

HBN_DPUSERVICE_DIR ?= deploy/dpuservices/hbn
HBN_IMAGE_NAME ?= hbn
export HBN_IMAGE ?= $(REGISTRY)/$(HBN_IMAGE_NAME)
export HBN_NVCR_TAG ?= 2.2.0-doca2.7.0
export HBN_TAG ?= $(HBN_NVCR_TAG)-dpf

HBN_SIDECAR_IMAGE_NAME ?= hbn-sidecar
export HBN_SIDECAR_IMAGE ?= $(REGISTRY)/$(HBN_SIDECAR_IMAGE_NAME)

.PHONY: docker-build-sfcset
docker-build-sfcset: ## Build docker images for the sfcset-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/servicechainset \
		. \
		-t $(SFCSET_IMAGE):$(TAG)

.PHONY: docker-build-sfc-controller
docker-build-sfc-controller: docker-build-base-image-ovs ## Build docker images for the sfc-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(OVS_BASE_IMAGE):$(TAG) \
		--build-arg target_arch=$(DPU_ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/sfc-controller \
		. \
		-t $(SFC_CONTROLLER_IMAGE):$(TAG)

.PHONY: docker-build-operator
docker-build-operator: generate-manifests-operator-embedded ## Build docker images for the operator-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/operator \
		. \
		-t $(DPFOPERATOR_IMAGE):$(TAG)

.PHONY: docker-build-dpuservice
docker-build-dpuservice: ## Build docker images for the dpuservice-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/dpuservice \
		. \
		-t $(DPUSERVICE_IMAGE):$(TAG)

.PHONY: docker-build-dpf-provisioning
docker-build-dpf-provisioning: $(DPF_PROVISIONING_DIR) ## Build docker images for the dpf-provisioning-controller
	cd $(DPF_PROVISIONING_DIR) && docker build -t $(DPFPROVISIONING_IMAGE):$(TAG) . -f dockerfile/Dockerfile.controller

.PHONY: docker-build-dpucniprovisioner
docker-build-dpucniprovisioner: docker-build-base-image-ovs ## Build docker images for the DPU CNI Provisioner
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(OVS_BASE_IMAGE):$(TAG) \
		--build-arg target_arch=$(DPU_ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/dpucniprovisioner \
		. \
		-t $(DPUCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-build-hostcniprovisioner
docker-build-hostcniprovisioner: docker-build-base-image-systemd ## Build docker images for the HOST CNI Provisioner
	# Base image can't be distroless because of the readiness probe that is using cat which doesn't exist in distroless
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(SYSTEMD_BASE_IMAGE):$(TAG) \
		--build-arg target_arch=$(HOST_ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/hostcniprovisioner \
		. \
		-t $(HOSTCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-build-base-image-ovs
docker-build-base-image-ovs: ## Build base docker image with OVS dependencies
	docker buildx build \
		--load \
		--platform linux/${DPU_ARCH} \
		-f Dockerfile.ovs \
		. \
		-t $(OVS_BASE_IMAGE):$(TAG)

.PHONY: docker-build-base-image-systemd
docker-build-base-image-systemd: ## Build base docker image with systemd dependencies
	docker buildx build \
		--load \
		--platform linux/${HOST_ARCH} \
		-f Dockerfile.systemd \
		. \
		-t $(SYSTEMD_BASE_IMAGE):$(TAG)

.PHONY: docker-build-hbn-sidecar
docker-build-hbn-sidecar: $(HBN_SIDECAR_DIR) ## Build HBN sidecar DPU service image
	docker buildx build \
		--load \
		--platform linux/${DPU_ARCH} \
		--build-arg hbn_sidecar_dir=$(shell realpath --relative-to $(CURDIR) $(HBN_SIDECAR_DIR)) \
		-f $(HBN_SIDECAR_DIR)/Dockerfile \
		. \
		-t $(HBN_SIDECAR_IMAGE):$(TAG)

.PHONY: docker-build-ovs-cni
docker-build-ovs-cni: $(OVS_CNI_DIR) ## Builds the OVS CNI image
	cd $(OVS_CNI_DIR) && \
	$(OVS_CNI_DIR)/hack/get_version.sh > .version && \
	docker buildx build . \
	--load \
	--build-arg goarch=$(DPU_ARCH) \
	--platform linux/${DPU_ARCH} \
	-f ./cmd/Dockerfile \
	-t $(OVS_CNI_IMAGE):${TAG}


.PHONY: docker-build-ovnkubernetes-dpu
docker-build-ovnkubernetes-dpu: $(OVNKUBERNETES_DPU_DIR) $(OVN_DIR) ## Builds the custom OVN Kubernetes image that is used for the DPU (worker) nodes
	docker buildx build \
		--load \
		--platform linux/${HOST_ARCH} \
		--build-arg base_image=${OVNKUBERNETES_BASE_IMAGE} \
		--build-arg ovn_dir=$(shell realpath --relative-to $(CURDIR) $(OVN_DIR)) \
		--build-arg ovn_kubernetes_dir=$(shell realpath --relative-to $(CURDIR) $(OVNKUBERNETES_DPU_DIR)) \
		-f Dockerfile.ovn-kubernetes-dpu \
		. \
		-t $(OVNKUBERNETES_DPU_IMAGE):$(TAG)

.PHONY: docker-build-ovnkubernetes-non-dpu
docker-build-ovnkubernetes-non-dpu: $(OVNKUBERNETES_NON_DPU_DIR) ## Builds the custom OVN Kubernetes image that is used for the non DPU (control plane) nodes
	docker buildx build \
		--load \
		--platform linux/${HOST_ARCH} \
		--build-arg base_image=${OVNKUBERNETES_BASE_IMAGE} \
		--build-arg ovn_kubernetes_dir=$(shell realpath --relative-to $(CURDIR) $(OVNKUBERNETES_NON_DPU_DIR)) \
		-f Dockerfile.ovn-kubernetes-non-dpu \
		. \
		-t $(OVNKUBERNETES_NON_DPU_IMAGE):$(TAG)

.PHONY: docker-build-ovnkubernetes-operator
docker-build-ovnkubernetes-operator: generate-manifests-ovnkubernetes-operator-embedded ## Build docker images for the operator-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/ovnkubernetesoperator \
		. \
		-t $(DPFOVNKUBERNETESOPERATOR_IMAGE):$(TAG)

.PHONY: docker-build-parprouted
docker-build-parprouted: $(PARPROUTED_DIR) ## Build docker image with the parprouted.
	cd $(PARPROUTED_DIR) && $(MAKE) parprouted && docker build --platform linux/${HOST_ARCH} -t ${PARPROUTED_IMAGE} .

.PHONY: docker-build-dhcrelay
docker-build-dhcrelay: ## Build docker image with the dhcrelay.
	cd $(DPF_PROVISIONING_DIR) && docker build --platform linux/${HOST_ARCH} -t ${DHCRELAY_IMAGE} . -f dockerfile/Dockerfile.dhcrelay

.PHONY: docker-build-hostnetwork
docker-build-hostnetwork: ## Build docker image with the hostnetwork.
	cd $(DPF_PROVISIONING_DIR) && docker build --platform linux/${HOST_ARCH} -t ${HOSTNETWORK_IMAGE} . -f dockerfile/Dockerfile.hostnetwork

.PHONY: docker-build-dms
docker-build-dms: ## Build docker image with the hostnetwork.
	cd $(DPF_PROVISIONING_DIR) && docker build --platform linux/${HOST_ARCH} -t ${DMS_IMAGE} . -f dockerfile/Dockerfile.dms

.PHONY: docker-build-hbn
docker-build-hbn: ## Build docker image for HBN.
	## Note this image only ever builds for arm64.
	cd $(HBN_DPUSERVICE_DIR) && docker build --build-arg hbn_nvcr_tag=$(HBN_NVCR_TAG) -t ${HBN_IMAGE}:${HBN_TAG} . -f Dockerfile

.PHONY: docker-push-all
docker-push-all: $(addprefix docker-push-,$(DOCKER_BUILD_TARGETS))  ## Push the docker images for all DOCKER_BUILD_TARGETS.

.PHONY: docker-push-sfcset
docker-push-sfcset: ## Push the docker image for sfcset.
	docker push $(SFCSET_IMAGE):$(TAG)

.PHONY: docker-push-sfc-controller
docker-push-sfc-controller:
	docker push $(SFC_CONTROLLER_IMAGE):$(TAG)

.PHONY: docker-push-operator
docker-push-operator: ## Push the docker image for operator.
	docker push $(DPFOPERATOR_IMAGE):$(TAG)

.PHONY: docker-push-dpuservice
docker-push-dpuservice: ## Push the docker image for dpuservice.
	docker push $(DPUSERVICE_IMAGE):$(TAG)

.PHONY: docker-push-dpf-provisioning
docker-push-dpf-provisioning: ## Push the docker image for dpf provisioning controller.
	docker push $(DPFPROVISIONING_IMAGE):$(TAG)

.PHONY: docker-push-hbn-sidecar
docker-push-hbn-sidecar: ## Push the docker image for HBN sidecar.
	docker push $(HBN_SIDECAR_IMAGE):$(TAG)

.PHONY: docker-push-ovs-cni
docker-push-ovs-cni: ## Push the docker image for ovs-cni
	docker push $(OVS_CNI_IMAGE):$(TAG)

.PHONY: docker-push-dhcrelay
docker-push-dhcrelay: ## Push the docker image for dhcrelate.
	cd $(DPF_PROVISIONING_DIR) && docker push ${DHCRELAY_IMAGE}

.PHONY: docker-push-parprouted
docker-push-parprouted: ## Push the docker image for parprouted.
	cd $(DPF_PROVISIONING_DIR) && docker push ${PARPROUTED_IMAGE}

.PHONY: docker-push-hostnetwork
docker-push-hostnetwork: ## Push the docker image for the hostnetwork.
	cd $(DPF_PROVISIONING_DIR) && docker push ${HOSTNETWORK_IMAGE}

.PHONY: docker-push-dms
docker-push-dms: ## Push the docker image for DMS.
	cd $(DPF_PROVISIONING_DIR) && docker push ${DMS_IMAGE}

.PHONY: docker-push-dpucniprovisioner
docker-push-dpucniprovisioner: ## Push the docker image for DPU CNI Provisioner.
	docker push $(DPUCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-push-hostcniprovisioner
docker-push-hostcniprovisioner: ## Push the docker image for Host CNI Provisioner.
	docker push $(HOSTCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-push-ovnkubernetes-dpu
docker-push-ovnkubernetes-dpu: ## Push the custom OVN Kubernetes image that is used for the DPU (worker) nodes
	docker push $(OVNKUBERNETES_DPU_IMAGE):$(TAG)

.PHONY: docker-push-ovnkubernetes-non-dpu
docker-push-ovnkubernetes-non-dpu: ## Push the custom OVN Kubernetes image that is used for the non DPU (control plane) nodes
	docker push $(OVNKUBERNETES_NON_DPU_IMAGE):$(TAG)

# TODO: Consider whether this should be part of the docker-build-all- build targets.
.PHONY: docker-build-operator-bundle # Build the docker image for the Operator bundle. Not included in docker-build-all.
docker-build-operator-bundle: generate-operator-bundle
	docker build -f bundle.Dockerfile -t $(OPERATOR_BUNDLE_IMAGE):$(BUNDLE_VERSION) .

# TODO: Consider whether this should be part of the docker-push-all- push targets.
.PHONY: docker-push-operator-bundle # Push the docker image for the Operator bundle. Not included in docker-build-all.
docker-push-operator-bundle: ## Push the bundle image.
	docker push $(OPERATOR_BUNDLE_IMAGE):$(BUNDLE_VERSION)

.PHONY: docker-push-ovnkubernetes-operator
docker-push-ovnkubernetes-operator: ## Push the docker image for the OVN Kubernetes operator.
	docker push $(DPFOVNKUBERNETESOPERATOR_IMAGE):$(TAG)

.PHONY: docker-push-hbn
docker-push-hbn: ## Push the docker image for HBN
	docker push $(HBN_IMAGE):$(HBN_TAG)

# helm charts

HELM_TARGETS ?= servicechain-controller multus sriov-device-plugin flannel nvidia-k8s-ipam ovs-cni sfc-controller ovnkubernetes-operator operator hbn-dpuservice
HELM_REGISTRY ?= oci://$(REGISTRY)

## used in templating the DPUService
HELM_REPO := $(shell if echo $(HELM_REGISTRY) | grep -q '^http'; then echo $(HELM_REGISTRY); else echo $(REGISTRY); fi)
export HELM_REPO

## metadata for servicechain controller.
export SERVICECHAIN_CONTROLLER_HELM_CHART_NAME = servicechain
SERVICECHAIN_CONTROLLER_HELM_CHART ?= $(HELMDIR)/$(SERVICECHAIN_CONTROLLER_HELM_CHART_NAME)
SERVICECHAIN_CONTROLLER_HELM_CHART_VER ?= $(TAG)

## metadata for multus.
export MULTUS_HELM_CHART_NAME = multus
MULTUS_HELM_CHART ?= $(HELMDIR)/$(MULTUS_HELM_CHART_NAME)
MULTUS_HELM_CHART_VER ?= $(TAG)

## metadata for sriov device plugin.
export SRIOV_DP_HELM_CHART_NAME = sriov-device-plugin
SRIOV_DP_HELM_CHART ?= $(HELMDIR)/$(SRIOV_DP_HELM_CHART_NAME)
SRIOV_DP_HELM_CHART_VER ?= $(TAG)

## metadata for nvidia-k8s-ipam.
export NVIDIA_K8S_IPAM_HELM_CHART_NAME = nvidia-k8s-ipam
NVIDIA_K8S_IPAM_HELM_CHART ?= $(HELMDIR)/$(NVIDIA_K8S_IPAM_HELM_CHART_NAME)
NVIDIA_K8S_IPAM_HELM_CHART_VER ?= $(TAG)

## metadata for ovs-cni.
export OVS_CNI_HELM_CHART_NAME = ovs-cni
OVS_CNI_HELM_CHART ?= $(HELMDIR)/$(OVS_CNI_HELM_CHART_NAME)
OVS_CNI_HELM_CHART_VER ?= $(TAG)

# metadata for flannel - using the chart published with flannel github releases.
export FLANNEL_HELM_CHART_NAME ?= flannel
export FLANNEL_VERSION ?= v0.25.1
FLANNEL_HELM_CHART ?= $(abspath $(CHARTSDIR)/$(FLANNEL_HELM_CHART_NAME)-$(FLANNEL_VERSION).tgz)

## metadata for sfc-controller.
export SFC_CONTOLLER_HELM_CHART_NAME = sfc-controller
SFC_CONTOLLER_HELM_CHART ?= $(HELMDIR)/$(SFC_CONTOLLER_HELM_CHART_NAME)
SFC_CONTOLLER_HELM_CHART_VER ?= $(TAG)

## metadata for dpf-ovn-kubernetes-operator.
export DPFOVNKUBERNETESOPERATOR_HELM_CHART_NAME = dpf-ovn-kubernetes-operator
DPFOVNKUBERNETESOPERATOR_HELM_CHART ?= $(HELMDIR)/$(DPFOVNKUBERNETESOPERATOR_HELM_CHART_NAME)
DPFOVNKUBERNETESOPERATOR_HELM_CHART_VER ?= $(TAG)

## metadata for hbn dpuservice.
export HBN_HELM_CHART_NAME = hbn
HBN_HELM_CHART ?= $(DPUSERVICESDIR)/$(HBN_HELM_CHART_NAME)/chart
HBN_HELM_CHART_VER ?= $(TAG)

.PHONY: helm-package-all
helm-package-all: $(addprefix helm-package-,$(HELM_TARGETS))  ## Package the helm charts for all components.

.PHONY: helm-package-servicechain-controller
helm-package-servicechain-controller: $(CHARTSDIR) $(HELM) ## Package helm chart for service chain controller
	$(HELM) package $(SERVICECHAIN_CONTROLLER_HELM_CHART) --version $(SERVICECHAIN_CONTROLLER_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-multus
helm-package-multus: $(CHARTSDIR) $(HELM) ## Package helm chart for multus CNIs
	$(HELM) package $(MULTUS_HELM_CHART) --version $(MULTUS_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-sriov-device-plugin
helm-package-sriov-device-plugin: $(CHARTSDIR) $(HELM) ## Package helm chart for sriov-network-device-plugin
	$(HELM) package $(SRIOV_DP_HELM_CHART) --version $(SRIOV_DP_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-nvidia-k8s-ipam
helm-package-nvidia-k8s-ipam: $(CHARTSDIR) $(HELM) ## Package helm chart for nvidia-k8s-ipam
	$(HELM) package $(NVIDIA_K8S_IPAM_HELM_CHART) --version $(NVIDIA_K8S_IPAM_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-flannel
helm-package-flannel: $(CHARTSDIR) $(HELM) ## Package helm chart for flannel CNI
	$Q curl -v -fSsL https://github.com/flannel-io/flannel/releases/download/$(FLANNEL_VERSION)/flannel.tgz -o $(FLANNEL_HELM_CHART)

.PHONY: helm-package-ovs-cni
helm-package-ovs-cni: $(CHARTSDIR) $(HELM) ## Package helm chart for OVS CNI
	$(HELM) package $(OVS_CNI_HELM_CHART) --version $(OVS_CNI_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-sfc-controller
helm-package-sfc-controller: $(CHARTSDIR) $(HELM) ## Package helm chart for SFC controller
	$(HELM) package $(SFC_CONTOLLER_HELM_CHART) --version $(SFC_CONTOLLER_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-operator
helm-package-operator: $(CHARTSDIR) $(HELM) ## Package helm chart for DPF Operator
	$(HELM) package $(OPERATOR_HELM_CHART) --version $(TAG) --destination $(CHARTSDIR)

.PHONY: helm-package-ovnkubernetes-operator
helm-package-ovnkubernetes-operator: $(CHARTSDIR) $(HELM) ## Package helm chart for OVN Kubernetes Operator
	$(HELM) package $(DPFOVNKUBERNETESOPERATOR_HELM_CHART) --version $(DPFOVNKUBERNETESOPERATOR_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-hbn-dpuservice
helm-package-hbn-dpuservice: $(DPUSERVICESDIR) $(HELM) generate-manifests-hbn-dpuservice ## Package helm chart for HBN
	$(HELM) package $(HBN_HELM_CHART) --version $(HBN_HELM_CHART_VER) --destination $(CHARTSDIR)

helm-cm-push: $(HELM)
	# installs the helm chartmuseum push plugin which is used to push to NGC.
	$(HELM) plugin list | grep cm-push || $(HELM) plugin install https://github.com/chartmuseum/helm-push

HELM_PUSH_CMD ?= push
.PHONY: helm-push-all
helm-push-all: $(addprefix helm-push-,$(HELM_TARGETS))  ## Push the helm charts for all components.

.PHONY: helm-push-servicechain-controller
helm-push-servicechain-controller: $(CHARTSDIR) helm-cm-push ## Push helm chart for service chain controller
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS) $(CHARTSDIR)/$(SERVICECHAIN_CONTROLLER_HELM_CHART_NAME)-$(SERVICECHAIN_CONTROLLER_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-multus
helm-push-multus: $(CHARTSDIR) helm-cm-push ## Push helm chart for multus CNI
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS) $(CHARTSDIR)/$(MULTUS_HELM_CHART_NAME)-$(MULTUS_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-sriov-device-plugin
helm-push-sriov-device-plugin: $(CHARTSDIR) helm-cm-push ## Push helm chart for sriov-network-device-plugin
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS) $(CHARTSDIR)/$(SRIOV_DP_HELM_CHART_NAME)-$(SRIOV_DP_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-nvidia-k8s-ipam
helm-push-nvidia-k8s-ipam: $(CHARTSDIR) helm-cm-push ## Push helm chart for nvidia-k8s-ipam
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS) $(CHARTSDIR)/$(NVIDIA_K8S_IPAM_HELM_CHART_NAME)-$(NVIDIA_K8S_IPAM_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-flannel
helm-push-flannel: $(CHARTSDIR) helm-cm-push ## Push helm chart for flannel CNI
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS)  $(CHARTSDIR)/$(FLANNEL_HELM_CHART_NAME)-$(FLANNEL_VERSION).tgz $(HELM_REGISTRY)

.PHONY: helm-push-ovs-cni
helm-push-ovs-cni: $(CHARTSDIR) helm-cm-push ## Push helm chart for OVS CNI
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS)  $(CHARTSDIR)/$(OVS_CNI_HELM_CHART_NAME)-$(OVS_CNI_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-sfc-controller
helm-push-sfc-controller: $(CHARTSDIR) helm-cm-push ## Push helm chart for sfc-controller
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS) $(CHARTSDIR)/$(SFC_CONTOLLER_HELM_CHART_NAME)-$(SFC_CONTOLLER_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-ovnkubernetes-operator
helm-push-ovnkubernetes-operator: $(CHARTSDIR) helm-cm-push ## Push helm chart for DPF OVN Kubernetes Operator
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS) $(CHARTSDIR)/$(DPFOVNKUBERNETESOPERATOR_HELM_CHART_NAME)-$(DPFOVNKUBERNETESOPERATOR_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-operator
helm-push-operator: $(CHARTSDIR) helm-cm-push ## Push helm chart for dpf-operator
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS)  $(CHARTSDIR)/$(OPERATOR_HELM_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)

.PHONY: helm-push-hbn-dpuservice
helm-push-hbn-dpuservice: $(CHARTSDIR) helm-cm-push ## Push helm chart for HBN
	$(HELM) $(HELM_PUSH_CMD) $(HELM_PUSH_OPTS)  $(CHARTSDIR)/$(HBN_HELM_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)

##@ Development Environment

DEV_CLUSTER_NAME ?= dpf-dev
dev-minikube: $(MINIKUBE) ## Create a minikube cluster for development.
	CLUSTER_NAME=$(DEV_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

clean-minikube: $(MINIKUBE)  ## Delete the development minikube cluster.
	$(MINIKUBE) delete -p $(DEV_CLUSTER_NAME)

dev-prereqs-dpuservice: $(KUSTOMIZE) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(HELM) $(KAMAJI) ## Install pre-requisites for dpuservice controller on minikube dev cluster
	# Deploy the dpuservice CRD
	$(KUSTOMIZE) build config/dpuservice/crd | $(KUBECTL) apply -f -

    # Deploy cert manager to provide certificates for webhooks
	$Q $(KUBECTL) apply -f $(CERT_MANAGER_YAML) \
	&& echo "Waiting for cert-manager deployment to be ready."\
	&& $(KUBECTL) wait --for=condition=ready pod -l app=webhook --timeout=180s -n cert-manager

	# Deploy argoCD as the underlying application provider.
	$Q $(KUBECTL) create namespace argocd --dry-run=client -o yaml | $(KUBECTL) apply -f - && $(KUBECTL) apply -f $(ARGOCD_YAML)

	$Q $(HELM) upgrade --set image.pullPolicy=IfNotPresent --set cfssl.image.tag=v1.6.5 --install kamaji $(KAMAJI)

SKAFFOLD_REGISTRY=localhost:5000
dev-dpuservice: $(MINIKUBE) $(SKAFFOLD) ## Deploy dpuservice controller to dev cluster using skaffold
	# Use minikube for docker build and deployment and run skaffold
	$Q eval $$($(MINIKUBE) -p $(DEV_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug -p dpuservice --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

dev-operator:  $(MINIKUBE) $(SKAFFOLD) generate-manifests-operator-embedded ## Deploy operator controller to dev cluster using skaffold
	# Ensure the manager's kustomization has the correct image name and has not been changed by generation.
	git restore config/operator/manager/kustomization.yaml
	$Q eval $$($(MINIKUBE) -p $(DEV_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug -p operator --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false --cleanup=false

dev-ovnkubernetes-operator:  $(MINIKUBE) $(SKAFFOLD) generate-manifests-ovnkubernetes-operator-embedded ## Deploy operator controller to dev cluster using skaffold
	# Ensure the manager's kustomization has the correct image name and has not been changed by generation.
	git restore config/ovnkubernetesoperator/manager/kustomization.yaml
	$Q eval $$($(MINIKUBE) -p $(DEV_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug -p ovnkubernetes-operator --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

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
