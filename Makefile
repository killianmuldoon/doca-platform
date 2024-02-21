
# Image URL to use all building/pushing image targets
IMG ?=  nvidia/dpf-controller
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

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	@if [ -d "config/crd" ]; then \
		$(KUSTOMIZE) build config/crd > dist/install.yaml; \
	fi
	echo "---" >> dist/install.yaml  # Add a document separator before appending
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default >> dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

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

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= latest
GOLANGCI_LINT_VERSION ?= v1.54.2

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

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Cleanup everything
	@rm -rf $(TOOLSDIR)
	@rm -rf $(CHARTSDIR)


# dev environment

MINIKUBE_CLUSTER_NAME = dpf-dev
dev-minikube: $(MINIKUBE) ## Create a minikube cluster for development.
	CLUSTER_NAME=$(MINIKUBE_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

clean-minikube: $(MINIKUBE)  ## Delete the development minikube cluster.
	$(MINIKUBE) delete -p $(MINIKUBE_CLUSTER_NAME)

SKAFFOLD_REGISTRY=localhost:5000
dev-prerequisites: $(KAMAJI) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(SKAFFOLD) kustomize manifests generate dev-minikube ## Create a development minikube cluster and deploy the operator in debug mode.
	# Deploy the network attachment definition CRD.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

    # Deploy cert manager to provide certificates for webhooks.
	$Q kubectl apply -f $(CERT_MANAGER_YAML) \
	&& echo "Waiting for cert-manager deployment to be ready."\
	&& kubectl wait --for=condition=ready pod -l app=webhook --timeout=180s -n cert-manager

	# Deploy argoCD as the underlying application provider.
	$Q kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f - && kubectl apply -f $(ARGOCD_YAML)

	# Deploy Kamaji as the underlying control plane provider. This values file is required due to a bug in a dependency.
	$Q $(HELM) upgrade --install kamaji $(KAMAJI) -f /hack/values/kamaji-values.yaml

dev-skaffold: dev-prerequisites
	# Use minikube for docker build and deployment and run skaffold
	$Q eval $$($(MINIKUBE) -p $(MINIKUBE_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

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
