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

GO_VERSION ?= 1.22.1

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

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


##@ Dependencies
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $@

TOOLSDIR ?= $(shell pwd)/hack/tools/bin
$(TOOLSDIR):
	@mkdir -p $@

CHARTSDIR ?= $(shell pwd)/hack/charts
$(CHARTSDIR):
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

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= v0.0.0-20240110160329-8f8247fdc1c3
GOLANGCI_LINT_VERSION ?= v1.54.2
MOCKGEN_VERSION ?= v0.4.0
GOTESTSUM_VERSION ?= v1.11.0
DPF_PROVISIONING_CONTROLLER_REV ?= eeb227e7dbec7a206bcb3ecdb02fc7bf368a9a3c

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

# kamaji is the underlying control plane provider
KAMAJI_REPO_URL=https://clastix.github.io/charts
KAMAJI_REPO_NAME=clastix
KAMAJI_CHART_VERSION=0.15.0
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
	$Q curl -fsSL https://storage.googleapis.com/skaffold/releases/latest/skaffold-$(OS)-$(ARCH) -o $(SKAFFOLD)
	$Q chmod +x $(SKAFFOLD)

# minikube is used to set-up a local kubernetes cluster for dev work.
MINIKUBE_VER := v0.0.0-20231012212722-e25aeebc7846
MINIKUBE_BIN := minikube
MINIKUBE := $(abspath $(TOOLSDIR)/$(MINIKUBE_BIN)-$(MINIKUBE_VER))
$(MINIKUBE): | $(TOOLSDIR)
	$Q echo "Installing minikube-$(MINIKUBE_VER) to $(TOOLSDIR)"
	$Q curl -fsSL https://storage.googleapis.com/minikube/releases/latest/minikube-$(OS)-$(ARCH) -o $(MINIKUBE)
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

# OVN Kubernetes dependencies to be able to build its docker image
OVNKUBERNETES_DIR=$(REPOSDIR)/ovn-kubernetes
OVN_DIR=$(REPOSDIR)/ovn
$(OVNKUBERNETES_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/ovn-kubernetes.git $(OVNKUBERNETES_DIR)

OVN_DIR=$(REPOSDIR)/ovn
$(OVN_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/ovn.git $(OVN_DIR)

PROV_DIR=$(REPOSDIR)/dpf-provisioning-controller
$(PROV_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/dpf-provisioning-controller.git $(PROV_DIR) $(DPF_PROVISIONING_CONTROLLER_REV)

# operator-sdk is used to generate operator-sdk bundles
OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download
OPERATOR_SDK_BIN = operator-sdk
OPERATOR_SDK_VER = v1.34.1
OPERATOR_SDK = $(abspath $(TOOLSDIR)/$(OPERATOR_SDK_BIN)-$(OPERATOR_SDK_VER))
$(OPERATOR_SDK): | $(TOOLSDIR)
	$Q echo "Installing $(OPERATOR_SDK_BIN)-$(OPERATOR_SDK_VER) to $(TOOLSDIR)"
	$Q curl -sSfL $(OPERATOR_SDK_DL_URL)/$(OPERATOR_SDK_VER)/operator-sdk_$(OS)_$(ARCH) -o $(OPERATOR_SDK)
	$Q chmod +x $(OPERATOR_SDK)

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Cleanup everything
	@rm -rf $(TOOLSDIR)
	@rm -rf $(CHARTSDIR)
	@rm -rf $(REPOSDIR)

##@ Development
GENERATE_TARGETS ?= operator dpuservice hostcniprovisioner dpucniprovisioner sfcset operator-embedded

.PHONY: generate
generate: ## Run all generate-* targets: generate-modules generate-manifests-* and generate-go-deepcopy-*.
	$(MAKE) generate-mocks generate-modules generate-manifests generate-go-deepcopy generate-operator-bundle

.PHONY: generate-mocks
generate-mocks: mockgen
	go generate ./...

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to update go modules
	go mod tidy

.PHONY: generate-manifests
generate-manifests: controller-gen kustomize $(addprefix generate-manifests-,$(GENERATE_TARGETS)) ## Run all generate-manifests-* targets

.PHONY: generate-manifests-operator
generate-manifests-operator: $(KUSTOMIZE) ## Generate manifests e.g. CRD, RBAC. for the operator controller.
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

.PHONY: generate-manifests-dpuservice
generate-manifests-dpuservice: $(KUSTOMIZE) ## Generate manifests e.g. CRD, RBAC. for the dpuservice controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/dpuservice/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/dpuservice/..." \
	paths="./internal/dpuservice/..." \
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

.PHONY: generate-manifests-provisioning
generate-manifests-provisioning: $(PROV_DIR)
	$(MAKE) -C $(PROV_DIR) kustomize-build

.PHONY: generate-manifests-operator-embedded
generate-manifests-operator-embedded: generate-manifests-dpucniprovisioner generate-manifests-hostcniprovisioner generate-manifests-dpuservice generate-manifests-provisioning ## Generates manifests that are embedded into the operator binary.
	$(KUSTOMIZE) build config/hostcniprovisioner/default > ./internal/operator/controllers/manifests/hostcniprovisioner.yaml
	$(KUSTOMIZE) build config/dpucniprovisioner/default > ./internal/operator/controllers/manifests/dpucniprovisioner.yaml
	$(KUSTOMIZE) build config/dpuservice/default > ./internal/operator/inventory/manifests/dpuservice.yaml
	cp $(PROV_DIR)/output/deploy.yaml ./internal/operator/inventory/manifests/provisioningctrl.yaml

.PHONY: generate-manifests-sfcset
generate-manifests-sfcset: $(KUSTOMIZE) ## Generate manifests e.g. CRD, RBAC. for the sfcset controller.
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


.PHONY: generate-operator-bundle
generate-operator-bundle: $(OPERATOR_SDK) $(KUSTOMIZE) ## Generate bundle manifests and metadata, then validate generated files.
	$(KUSTOMIZE) build config/bundle-operatorsdk | $(OPERATOR_SDK) generate bundle \
	--overwrite --package dpf-operator --version $(BUNDLE_VERSION) --default-channel=$(BUNDLE_VERSION) --channels=$(BUNDLE_VERSION)
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

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test $$(go list ./... | grep -v /e2e)

.PHONY: test-report $(GOTESTSUM)
test-report: envtest gotestsum ## Run tests and generate a junit style report
	set +o errexit; KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test -json $$(go list ./... | grep -v /e2e) > junit.stdout; echo $$? > junit.exitcode;
	$(GOTESTSUM) --junitfile junit.xml --raw-command cat junit.stdout
	exit $$(cat junit.exitcode)

TEST_CLUSTER_NAME := dpf-test
test-env-e2e: $(KAMAJI) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(MINIKUBE) ## Setup a Kubernetes environment to run tests.
	# Create a minikube cluster to host the test.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

	# Deploy cert manager to provide certificates for webhooks.
	$Q kubectl apply -f $(CERT_MANAGER_YAML)

	# Deploy argoCD as the underlying application provider.
	$Q kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f - && kubectl apply -f $(ARGOCD_YAML)

	# Mirror images for e2e tests from docker hub and push them in the test registry to avoid docker pull limits.
	$Q eval $$($(MINIKUBE) -p $(TEST_CLUSTER_NAME) docker-env); \
	$(MAKE) test-build-and-push-artifacts test-upload-external-images

	echo "Waiting for cert-manager deployment to be ready."
	kubectl wait --for=condition=ready pod -l app=webhook --timeout=180s -n cert-manager

	# Deploy Kamaji as the underlying control plane provider.
	# TODO: Disaggregate the kamaji apply and wait for ready to speed up environment creation.
	cat ./hack/values/kamaji-values.yaml | envsubst > ./hack/values/kamaji-values.yaml.tmp
	$Q $(HELM) upgrade --install kamaji $(KAMAJI) -f ./hack/values/kamaji-values.yaml.tmp


.PHONY: test-upload-external-images
test-upload-external-images: 	# Mirror images for e2e tests from docker hub and push them in the test registry. - this is done to avoid docker pull limits.
	docker pull clastix/kamaji:v0.4.1
	docker image tag clastix/kamaji:v0.4.1 $(REGISTRY)/clastix/kamaji:v0.4.1
	docker push $(REGISTRY)/clastix/kamaji:v0.4.1
	docker pull cfssl/cfssl:v1.6.5
	docker tag cfssl/cfssl:v1.6.5 $(REGISTRY)/cfssl/cfssl:v1.6.5
	docker push $(REGISTRY)/cfssl/cfssl:v1.6.5

.PHONY: test-build-and-push-artifacts
test-build-and-push-artifacts: $(KUSTOMIZE)
	# Build and push the sfcset, dpuservice, operator and operator-bundle images.
	$Q eval $$($(MINIKUBE) -p $(TEST_CLUSTER_NAME) docker-env); \
	$(MAKE) docker-build-dpuservice docker-push-dpuservice; \
	$(MAKE) docker-build-operator docker-push-operator ; \
	$(MAKE) docker-build-operator-bundle docker-push-operator-bundle; \
	$(MAKE) docker-build-sfcset docker-push-sfcset

	# Build and push all the helm charts
	$(MAKE) helm-package-all helm-push-all

OLM_VERSION ?= v0.27.0
OPERATOR_REGISTRY_VERSION ?= v1.39.0
OPERATOR_NAMESPACE ?= dpf-operator-system
.PHONY: test-deploy-operator
test-deploy-operator: $(KUSTOMIZE) # Deploy the DPF Operator using operator-sdk
	# Install OLM in the cluster
	$(OPERATOR_SDK) olm install --version $(OLM_VERSION)

	# Create the namespace for the operator to be installed.
	$(KUBECTL) create namespace $(OPERATOR_NAMESPACE)
	# TODO: This flow does not work on MacOS dues to some issue pulling images. Should be enabled to make local testing equivalent to CI.
	$(OPERATOR_SDK) run bundle --namespace $(OPERATOR_NAMESPACE) --index-image quay.io/operator-framework/opm:$(OPERATOR_REGISTRY_VERSION) $(OPERATOR_BUNDLE_IMAGE):$(BUNDLE_VERSION)

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  ## Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: clean-test-env
clean-test-env: $(MINIKUBE)
	$(MINIKUBE) delete -p $(TEST_CLUSTER_NAME)

##@ lint and verify
.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: verify-generate
verify-generate: generate
	$(info checking for git diff after running 'make generate')
	$Q git diff --quiet ':!bundle' ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make generate'"; exit 1 ; fi
	# Files under `bundle` are verified here. The createdAt field is excluded as it is always updated at generation time and is not relevant to the bundle.
	$Q git diff --quiet -I'^    createdAt: ' bundle ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make generate'"; exit 1 ; fi

.PHONY: verify-copyright
verify-copyright:
	$Q $(CURDIR)/hack/scripts/copyright-validation.sh

.PHONY: lint-helm-sfcset
lint-helm-sfcset: $(HELM) ; $(info  running lint for helm charts...) @ ## Run helm lint
	$Q $(HELM) lint $(SERVICECHAIN_CONTROLLER_HELM_CHART)

##@ Build

GO_GCFLAGS=""

GO_LDFLAGS="-extldflags '-static'"
DPFOPERATOR_GO_LDFLAGS="$(subst ",,$(GO_LDFLAGS)) -X 'main.defaultCustomOVNKubernetesDPUImage=${OVNKUBERNETES_DPU_IMAGE}:${TAG}' -X 'main.defaultCustomOVNKubernetesNonDPUImage=${OVNKUBERNETES_NON_DPU_IMAGE}:${TAG}'"

BUILD_TARGETS ?= operator dpuservice dpucniprovisioner hostcniprovisioner sfcset
# Note: Registry defaults to non-existing registry intentionally to avoid overriding useful images.
REGISTRY ?= nvidia.com
BUILD_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

# The tag must have three digits with a leading v - i.e. v9.9.1
TAG ?= v0.0.0
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
	go build -ldflags=$(DPFOPERATOR_GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/operator-manager gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/operator

.PHONY: binary-dpuservice
binary-dpuservice: ## Build the dpuservice controller binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpuservice-manager gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/dpuservice

.PHONY: binary-dpucniprovisioner
binary-dpucniprovisioner: ## Build the DPU CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpucniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/dpucniprovisioner

.PHONY: binary-hostcniprovisioner
binary-hostcniprovisioner: ## Build the Host CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/hostcniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/hostcniprovisioner

DOCKER_BUILD_TARGETS=$(BUILD_TARGETS) ovnkubernetes-dpu ovnkubernetes-non-dpu operator-bundle

.PHONY: docker-build-all
docker-build-all: $(addprefix docker-build-,$(DOCKER_BUILD_TARGETS)) ## Build docker images for all DOCKER_BUILD_TARGETS

OVS_BASE_IMAGE_NAME = base-image-ovs
OVS_BASE_IMAGE = $(REGISTRY)/$(OVS_BASE_IMAGE_NAME)

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
OVNKUBERNETES_BASE_IMAGE=harbor.mellanox.com/cloud-orchestration-dev/dpf/ovn-kubernetes-base:ca01b0a7e924b17765df8145d8669611d513e3edb2ac6f3cd518d04b6d01de6e
OVN_BRANCH=dpf-23.09.0
OVNKUBERNETES_DPU_BRANCH=dpf-4.14
OVNKUBERNETES_NON_DPU_BRANCH=dpf-4.14-non-dpu

# Image that is running on the DPU enabled host cluster nodes (workers)
OVNKUBERNETES_DPU_IMAGE_NAME = ovn-kubernetes-dpu
OVNKUBERNETES_DPU_IMAGE = $(REGISTRY)/$(OVNKUBERNETES_DPU_IMAGE_NAME)

# Image that is running on the non DPU host cluster nodes (control plane)
OVNKUBERNETES_NON_DPU_IMAGE_NAME = ovn-kubernetes-non-dpu
OVNKUBERNETES_NON_DPU_IMAGE = $(REGISTRY)/$(OVNKUBERNETES_NON_DPU_IMAGE_NAME)

DPFOPERATOR_IMAGE_NAME ?= operator-controller-manager
DPFOPERATOR_IMAGE ?= $(REGISTRY)/$(DPFOPERATOR_IMAGE_NAME)

SFCSET_IMAGE_NAME ?= sfcset-controller-manager
SFCSET_IMAGE ?= $(REGISTRY)/$(SFCSET_IMAGE_NAME)

DPUSERVICE_IMAGE_NAME ?= dpuservice-controller-manager
DPUSERVICE_IMAGE ?= $(REGISTRY)/$(DPUSERVICE_IMAGE_NAME)

DPUCNIPROVISIONER_IMAGE_NAME ?= dpu-cni-provisioner
DPUCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(DPUCNIPROVISIONER_IMAGE_NAME)

HOSTCNIPROVISIONER_IMAGE_NAME ?= host-cni-provisioner
HOSTCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(HOSTCNIPROVISIONER_IMAGE_NAME)

OPERATOR_BUNDLE_NAME ?= dpf-operator-bundle
OPERATOR_BUNDLE_REGISTRY ?= $(REGISTRY)
OPERATOR_BUNDLE_IMAGE ?= $(OPERATOR_BUNDLE_REGISTRY)/$(OPERATOR_BUNDLE_NAME)

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

.PHONY: docker-build-operator
docker-build-operator: generate-manifests-operator-embedded ## Build docker images for the operator-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(ARCH) \
		--build-arg ldflags=$(DPFOPERATOR_GO_LDFLAGS) \
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
docker-build-hostcniprovisioner: ## Build docker images for the HOST CNI Provisioner
	# Base image can't be distroless because of the readiness probe that is using cat which doesn't exist in distroless
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(ALPINE_IMAGE) \
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

.PHONY: docker-build-ovnkubernetes-dpu
docker-build-ovnkubernetes-dpu: $(OVNKUBERNETES_DIR) $(OVN_DIR) ## Builds the custom OVN Kubernetes image that is used for the DPU (worker) nodes
	docker buildx build \
		--load \
		--platform linux/${HOST_ARCH} \
		--build-arg base_image=${OVNKUBERNETES_BASE_IMAGE} \
		--build-arg ovn_branch=${OVN_BRANCH} \
		--build-arg ovn_kubernetes_branch=${OVNKUBERNETES_DPU_BRANCH} \
		-f Dockerfile.ovn-kubernetes-dpu \
		. \
		-t $(OVNKUBERNETES_DPU_IMAGE):$(TAG)

.PHONY: docker-build-ovnkubernetes-non-dpu
docker-build-ovnkubernetes-non-dpu: $(OVNKUBERNETES_DIR) $(OVN_DIR) ## Builds the custom OVN Kubernetes image that is used for the non DPU (control plane) nodes
	docker buildx build \
		--load \
		--platform linux/${HOST_ARCH} \
		--build-arg base_image=${OVNKUBERNETES_BASE_IMAGE} \
		--build-arg ovn_kubernetes_branch=${OVNKUBERNETES_NON_DPU_BRANCH} \
		-f Dockerfile.ovn-kubernetes-non-dpu \
		. \
		-t $(OVNKUBERNETES_NON_DPU_IMAGE):$(TAG)

.PHONY: docker-push-all
docker-push-all: $(addprefix docker-push-,$(DOCKER_BUILD_TARGETS))  ## Push the docker images for all controllers.

.PHONY: docker-push-sfcset
docker-push-sfcset: ## Push the docker image for sfcset.
	docker push $(SFCSET_IMAGE):$(TAG)

.PHONY: docker-push-operator
docker-push-operator: ## Push the docker image for operator.
	docker push $(DPFOPERATOR_IMAGE):$(TAG)

.PHONY: docker-push-dpuservice
docker-push-dpuservice: ## Push the docker image for dpuservice.
	docker push $(DPUSERVICE_IMAGE):$(TAG)

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

# helm charts

HELM_TARGETS ?= servicechain-controller
HELM_REGISTRY ?= oci://$(REGISTRY)

## metadata for servicechain controller.
SERVICECHAIN_CONTROLLER_HELM_CHART_NAME ?= servicechain
SERVICECHAIN_CONTROLLER_HELM_CHART ?= $(HELMDIR)/$(SERVICECHAIN_CONTROLLER_HELM_CHART_NAME)
SERVICECHAIN_CONTROLLER_HELM_CHART_VER ?= $(TAG)

.PHONY: helm-package-all
helm-package-all: $(addprefix helm-package-,$(HELM_TARGETS))  ## Package the helm charts for all components.

.PHONY: helm-package-servicechain-controller
helm-package-servicechain-controller:
	$(HELM) package $(SERVICECHAIN_CONTROLLER_HELM_CHART) --version $(SERVICECHAIN_CONTROLLER_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-push-all
helm-push-all: $(addprefix helm-push-,$(HELM_TARGETS))  ## Push the helm charts for all components.

.PHONY: helm-push-servicechain-controller
helm-push-servicechain-controller:
	$(HELM) push $(CHARTSDIR)/$(SERVICECHAIN_CONTROLLER_HELM_CHART_NAME)-$(SERVICECHAIN_CONTROLLER_HELM_CHART_VER).tgz $(HELM_REGISTRY)

# dev environment
MINIKUBE_CLUSTER_NAME ?= dpf-dev
dev-minikube: $(MINIKUBE) ## Create a minikube cluster for development.
	CLUSTER_NAME=$(MINIKUBE_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

clean-minikube: $(MINIKUBE)  ## Delete the development minikube cluster.
	$(MINIKUBE) delete -p $(MINIKUBE_CLUSTER_NAME)

dev-prereqs-dpuservice: $(KAMAJI) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(SKAFFOLD) $(KUSTOMIZE) dev-minikube ## Create a development minikube cluster and deploy the operator in debug mode.
	# Deploy the dpuservice CRD
	$(KUSTOMIZE) build config/dpuservice/crd | $(KUBECTL) apply -f -

    # Deploy cert manager to provide certificates for webhooks
	$Q kubectl apply -f $(CERT_MANAGER_YAML) \
	&& echo "Waiting for cert-manager deployment to be ready."\
	&& kubectl wait --for=condition=ready pod -l app=webhook --timeout=180s -n cert-manager

	# Deploy argoCD as the underlying application provider.
	$Q kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f - && kubectl apply -f $(ARGOCD_YAML)

	# Deploy Kamaji as the underlying control plane provider.
	# The values file is currently empty.
	touch ./hack/values/kamaji-values.yaml.tmp
	$Q $(HELM) upgrade --install kamaji $(KAMAJI) -f ./hack/values/kamaji-values.yaml

SKAFFOLD_REGISTRY=localhost:5000
dev-dpuservice: $(MINIKUBE) $(SKAFFOLD)
	# Use minikube for docker build and deployment and run skaffold
	$Q eval $$($(MINIKUBE) -p $(MINIKUBE_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug -p dpuservice --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

ENABLE_OVN_KUBERNETES?=true
dev-operator:  $(MINIKUBE) $(SKAFFOLD)	# Use minikube for docker build and deployment and run skaffold
	sed -i '' "s/reconcileOVNKubernetes=.*/reconcileOVNKubernetes=$(ENABLE_OVN_KUBERNETES)/" config/operator/manager/manager.yaml
	$Q eval $$($(MINIKUBE) -p $(MINIKUBE_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug -p operator --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

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
