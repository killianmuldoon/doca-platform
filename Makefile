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

GO_VERSION ?= 1.22.0

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

TOOLSDIR?= $(shell pwd)/hack/tools/bin
$(TOOLSDIR):
	@mkdir -p $@

CHARTSDIR?= $(shell pwd)/hack/charts
$(CHARTSDIR):
	@mkdir -p $@

## Tool Binaries

KUBECTL ?= kubectl
KUSTOMIZE ?= $(TOOLSDIR)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(TOOLSDIR)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(TOOLSDIR)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(TOOLSDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)
MOCKGEN = $(TOOLSDIR)/mockgen-$(MOCKGEN_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= v0.0.0-20240110160329-8f8247fdc1c3
GOLANGCI_LINT_VERSION ?= v1.54.2
MOCKGEN_VERSION ?= v0.4.0

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
	curl -fSsL "https://raw.githubusercontent.com/argoproj/argo-cd/$(ARGOCD_VER)/manifests/core-install.yaml" -o $(ARGOCD_YAML)

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Cleanup everything
	@rm -rf $(TOOLSDIR)
	@rm -rf $(CHARTSDIR)

##@ Development
GENERATE_TARGETS ?= dpuservice controlplane

.PHONY: generate
generate: ## Run all generate-* targets: generate-modules generate-manifests-* and generate-go-deepcopy-*.
	$(MAKE) generate-mocks generate-modules generate-manifests generate-go-deepcopy

.PHONY: generate-mocks
generate-mocks: mockgen
	go generate ./...

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to update go modules
	go mod tidy

.PHONY: generate-manifests
generate-manifests: controller-gen $(addprefix generate-manifests-,$(GENERATE_TARGETS)) ## Run all generate-manifests-* targets

.PHONY: generate-manifests-dpuservice
generate-manifests-dpuservice: ## Generate manifests e.g. CRD, RBAC. for the dpuservice controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/dpuservice/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./internal/dpuservice/..." \
	paths="./api/dpuservice/..." \
	crd:crdVersions=v1 \
	rbac:roleName=manager-role \
	output:crd:dir=./config/dpuservice/crd/bases \
	output:rbac:dir=./config/dpuservice/rbac

.PHONY: generate-manifests-controlplane
generate-manifests-controlplane: ## Generate manifests e.g. CRD, RBAC. for the controlplane controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/controlplane/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./internal/controlplane/..." \
	paths="./api/controlplane/..." \
	crd:crdVersions=v1 \
	rbac:roleName=manager-role \
	output:crd:dir=./config/controlplane/crd/bases \
	output:rbac:dir=./config/controlplane/rbac

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
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

TEST_CLUSTER_NAME := dpf-test
test-env-e2e: $(KAMAJI) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(MINIKUBE) ## Setup a Kubernetes environment to run tests.
	# Create a minikube cluster to host the test.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

	# Deploy cert manager to provide certificates for webhooks.
	$Q kubectl apply -f $(CERT_MANAGER_YAML) \
	&& echo "Waiting for cert-manager deployment to be ready."\
	&& kubectl wait --for=condition=ready pod -l app=webhook --timeout=180s -n cert-manager

	# Deploy Kamaji as the underlying control plane provider. This values file is required due to a bug in a dependency.
	$Q $(HELM) upgrade --install kamaji $(KAMAJI) -f ./hack/values/kamaji-values.yaml

	# Deploy argoCD as the underlying application provider.
	$Q kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f - && kubectl apply -f $(ARGOCD_YAML)

.PHONY: test-deploy-dpuservice
test-deploy-dpuservice: $(KUSTOMIZE)
	# Build and push the dpuservice controller images to the minikube registry
	$Q eval $$($(MINIKUBE) -p $(TEST_CLUSTER_NAME) docker-env); \
	$(MAKE) docker-build-dpuservice docker-push-dpuservice

	# Deploy the DPUService CRDs to the test env
	$Q kubectl apply -f config/dpuservice/crd/bases

	# Deploy the dpuservice controller to the cluster
	cd config/dpuservice/manager && $(KUSTOMIZE) edit set image controller=$(DPUSERVICE_IMAGE):$(TAG)
	$(KUSTOMIZE) build config/dpuservice/default | kubectl apply -f -

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  ## Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: clean-test-env
clean-test-env:
	$(MINIKUBE) delete -p $(TEST_CLUSTER_NAME)

##@ lint and verify
.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: verify-generate
verify-generate: generate
	$(info checking for git diff after running 'make generate')
	git diff --quiet ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make generate'"; exit 1 ; fi

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

BUILD_TARGETS ?= dpuservice controlplane dpucniprovisioner hostcniprovisioner
REGISTRY ?= harbor.mellanox.com/cloud-orchestration-dev/dpf
BUILD_IMAGE ?= docker.io/library/golang:$(GO_VERSION)
TAG ?= v0.0.1

HOST_ARCH = amd64
# Note: If you make this variable configurable, ensure that the custom base image that is built in
# docker-build-base-image-ovs is fetching binaries with the correct architecture.
DPU_ARCH = arm64

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
BASE_IMAGE = gcr.io/distroless/static:nonroot

.PHONY: binaries
binaries: $(addprefix binary-,$(BUILD_TARGETS)) ## Build all binaries

.PHONY: binary-dpuservice
binary-dpuservice: ## Build the dpuservice controller binary.
	go build -trimpath -o $(LOCALBIN)/dpuservice-manager gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/dpuservice

.PHONY: binary-controlplane
binary-controlplane: ## Build the controlplane controller binary.
	go build -trimpath -o $(LOCALBIN)/controlplane-manager gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/controlplane

.PHONY: binary-dpucniprovisioner
binary-dpucniprovisioner: ## Build the DPU CNI Provisioner binary.
	go build -trimpath -o $(LOCALBIN)/dpucniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/dpucniprovisioner

.PHONY: binary-hostcniprovisioner
binary-hostcniprovisioner: ## Build the Host CNI Provisioner binary.
	go build -trimpath -o $(LOCALBIN)/hostcniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/cmd/hostcniprovisioner

.PHONY: docker-build-all
docker-build-all: ## Build docker images for all BUILD_TARGETS
	$(MAKE) $(addprefix docker-build-,$(BUILD_TARGETS))

OVS_BASE_IMAGE_NAME = base-image-ovs
OVS_BASE_IMAGE = $(REGISTRY)/$(OVS_BASE_IMAGE_NAME)

DPUSERVICE_IMAGE_NAME ?= dpuservice-controller-manager
DPUSERVICE_IMAGE ?= $(REGISTRY)/$(DPUSERVICE_IMAGE_NAME)

CONTROLPLANE_IMAGE_NAME ?= controlplane-controller-manager
CONTROLPLANE_IMAGE ?= $(REGISTRY)/$(CONTROLPLANE_IMAGE_NAME)

DPUCNIPROVISIONER_IMAGE_NAME ?= dpu-cni-provisioner
DPUCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(DPUCNIPROVISIONER_IMAGE_NAME)

HOSTCNIPROVISIONER_IMAGE_NAME ?= host-cni-provisioner
HOSTCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(HOSTCNIPROVISIONER_IMAGE_NAME)

.PHONY: docker-build-dpuservice
docker-build-dpuservice: ## Build docker images for the dpuservice-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(HOST_ARCH) \
		--build-arg package=./cmd/dpuservice \
		. \
		-t $(DPUSERVICE_IMAGE):$(TAG)

.PHONY: docker-build-controlplane
docker-build-controlplane: ## Build docker images for the controlplane-controller
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(HOST_ARCH) \
		--build-arg package=./cmd/controlplane \
		. \
		-t $(CONTROLPLANE_IMAGE):$(TAG)

.PHONY: docker-build-dpucniprovisioner
docker-build-dpucniprovisioner: docker-build-base-image-ovs ## Build docker images for the DPU CNI Provisioner
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(OVS_BASE_IMAGE):$(TAG) \
		--build-arg target_arch=$(DPU_ARCH) \
		--build-arg package=./cmd/dpucniprovisioner \
		. \
		-t $(DPUCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-build-hostcniprovisioner
docker-build-hostcniprovisioner: ;## Build docker images for the HOST CNI Provisioner

.PHONY: docker-build-base-image-ovs
docker-build-base-image-ovs: ## Build base docker image with OVS dependencies
	docker buildx build \
		--load \
		--platform linux/${DPU_ARCH} \
		-f Dockerfile.ovs \
		. \
		-t $(OVS_BASE_IMAGE):$(TAG)

.PHONY: docker-push-all
docker-push-all: $(addprefix docker-push-,$(BUILD_TARGETS))  ## Push the docker images for all controllers.

.PHONY: docker-push-dpuservice
docker-push-dpuservice: ## Push the docker image for dpuservice.
	docker push $(DPUSERVICE_IMAGE):$(TAG)

.PHONY: docker-push-controlplane
docker-push-controlplane: ## Push the docker image for dpuservice.
	docker push $(CONTROLPLANE_IMAGE):$(TAG)

.PHONY: docker-push-dpucniprovisioner
docker-push-dpucniprovisioner: ## Push the docker image for DPU CNI Provisioner.
	docker push $(DPUCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-push-hostcniprovisioner
.PHONY: docker-push-hostcniprovisioner
docker-push-hostcniprovisioner: ## Push the docker image for Host CNI Provisioner.
	docker push $(HOSTCNIPROVISIONER_IMAGE):$(TAG)

# dev environment
MINIKUBE_CLUSTER_NAME = dpf-dev
dev-minikube: $(MINIKUBE) ## Create a minikube cluster for development.
	CLUSTER_NAME=$(MINIKUBE_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

clean-minikube: $(MINIKUBE)  ## Delete the development minikube cluster.
	$(MINIKUBE) delete -p $(MINIKUBE_CLUSTER_NAME)

dev-prereqs-dpuservice: $(KAMAJI) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(SKAFFOLD) dev-minikube ## Create a development minikube cluster and deploy the operator in debug mode.
	# Deploy the dpuservice CRD.
	$(KUSTOMIZE) build config/dpuservice/crd | $(KUBECTL) apply -f -

    # Deploy cert manager to provide certificates for webhooks.
	$Q kubectl apply -f $(CERT_MANAGER_YAML) \
	&& echo "Waiting for cert-manager deployment to be ready."\
	&& kubectl wait --for=condition=ready pod -l app=webhook --timeout=180s -n cert-manager

	# Deploy argoCD as the underlying application provider.
	$Q kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f - && kubectl apply -f $(ARGOCD_YAML)

	# Deploy Kamaji as the underlying control plane provider. This values file is required due to a bug in a dependency.
	$Q $(HELM) upgrade --install kamaji $(KAMAJI) -f ./hack/values/kamaji-values.yaml

SKAFFOLD_REGISTRY=localhost:5000
dev-dpuservice: kustomize generate
	# Use minikube for docker build and deployment and run skaffold
	$Q eval $$($(MINIKUBE) -p $(MINIKUBE_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug -p dpuservice --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

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
