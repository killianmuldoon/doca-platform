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

## Include Make modules which are split up in this repo for better structure.
include hack/tools/tools.mk

PROJECT_NAME="DOCA Platform Framework"
PROJECT_REPO="https://github.com/NVIDIA/doca-platform"
DATE="$(shell date --rfc-3339=seconds)"
FULL_COMMIT=$(shell git rev-parse HEAD)

# Export is needed here so that the envsubst used in make targets has access to those variables even when they are not
# explicitly set when calling make.
# The tag must have three digits with a leading v - i.e. v9.9.1
export TAG ?= v0.1.0
# Note: Registry defaults to non-existing registry intentionally to avoid overriding useful images.
export REGISTRY ?= example.com

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

GO_VERSION ?= $(shell awk '/^go /{print $$2}' go.mod)

# Allows for defining additional Go test args, e.g. '-tags integration'.
GO_TEST_ARGS ?= -race

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

LOCALBIN ?= $(CURDIR)/bin
CHARTSDIR ?= $(CURDIR)/hack/charts
DPUSERVICESDIR ?= $(CURDIR)/deploy/dpuservices
REPOSDIR ?= $(CURDIR)/hack/repos
HELMDIR ?= $(CURDIR)/deploy/helm
THIRDPARTYDIR ?= $(CURDIR)/third_party/forked
EXAMPLE ?= $(CURDIR)/example

$(LOCALBIN) $(CHARTSDIR) $(DPUSERVICESDIR) $(REPOSDIR):
	@mkdir -p $@

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Clean non-essential files from the repo
	@rm -rf $(CHARTSDIR)
	@rm -rf $(TOOLSDIR)
	@rm -rf $(REPOSDIR)

# Note: This helps resolve errors with `docker manifest create`
.PHONY: clean-images-for-registry
clean-images-for-registry: ## Clean release deletes local images with the $REGISTRY
	for image in $$(docker images $$REGISTRY/* --format "{{.ID}}"); do \
	docker rmi -f $$image ; \
	done

##@ Dependencies

# OVS CNI
# A third party import to the repo. In future this will be further integrated.
OVS_CNI_DIR=$(THIRDPARTYDIR)/ovs-cni

# OVN Kubernetes dependencies to be able to build its docker image
OVNKUBERNETES_REF=d7ce0d0b9799bfbd9bb4cb7be7f025ed9a181ff0
OVNKUBERNETES_DIR=$(REPOSDIR)/ovn-kubernetes-$(OVNKUBERNETES_REF)
$(OVNKUBERNETES_DIR): | $(REPOSDIR)
	git clone https://github.com/aserdean/ovn-kubernetes $(OVNKUBERNETES_DIR)-tmp
	cd $(OVNKUBERNETES_DIR)-tmp && git reset --hard $(OVNKUBERNETES_REF)
	mv $(OVNKUBERNETES_DIR)-tmp $(OVNKUBERNETES_DIR)

DOCA_SOSREPORT_REPO_URL=https://github.com/NVIDIA/doca-sosreport/archive/$(DOCA_SOSREPORT_REF).tar.gz
DOCA_SOSREPORT_REF=6b4289b9f0d9f26af177b0d1c4c009ca74bb514a
SOS_REPORT_DIR=$(REPOSDIR)/doca-sosreport-$(DOCA_SOSREPORT_REF)
$(SOS_REPORT_DIR): | $(REPOSDIR)
	curl -sL ${DOCA_SOSREPORT_REPO_URL} | tar -xz -C ${REPOSDIR}

##@ GRPC

# go package for generated code
API_PKG_GO_MOD ?= github.com/nvidia/doca-platform/api/grpc

## Temporary location for GRPC files
GRPC_TMP_DIR  ?= $(CURDIR)/_tmp
$(GRPC_TMP_DIR):
	@mkdir -p $@

# GRPC DIRs
GRPC_DIR ?= $(CURDIR)/api/grpc
PROTO_DIR ?= $(GRPC_DIR)/proto
GENERATED_CODE_DIR ?= $(GRPC_DIR)

.PHONY: grpc-generate
grpc-generate: protoc protoc-gen-go protoc-gen-go-grpc ## Generate GO client and server GRPC code
	@echo "generate GRPC API"; \
	echo "   go module: $(API_PKG_GO_MOD)"; \
	echo "   output dir: $(GENERATED_CODE_DIR) "; \
	echo "   proto dir: $(PROTO_DIR) "; \
	cd $(PROTO_DIR) && \
	TARGET_FILES=""; \
	PROTOC_OPTIONS="--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
					--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
					--go_out=$(GENERATED_CODE_DIR) \
					--go_opt=module=$(API_PKG_GO_MOD) \
					--proto_path=$(PROTO_DIR) \
					--go-grpc_out=$(GENERATED_CODE_DIR) \
					--go-grpc_opt=module=$(API_PKG_GO_MOD)"; \
	echo "discovered proto files:"; \
	for proto_file in $$(find . -name "*.proto"); do \
		proto_file=$$(echo $$proto_file | cut -d'/' -f2-); \
		proto_dir=$$(dirname $$proto_file); \
		pkg_name=M$$proto_file=$(API_PKG_GO_MOD)/$$proto_dir; \
		echo "    $$proto_file"; \
		TARGET_FILES="$$TARGET_FILES $$proto_file"; \
		PROTOC_OPTIONS="$$PROTOC_OPTIONS \
						--go_opt=$$pkg_name \
						--go-grpc_opt=$$pkg_name" ; \
	done; \
	$(PROTOC) $$PROTOC_OPTIONS $$TARGET_FILES

.PHONY: grpc-check
grpc-check: grpc-format grpc-lint protoc protoc-gen-go protoc-gen-go-grpc $(GRPC_TMP_DIR)  ## Check that generated GO client code match proto files
	@rm -rf $(GRPC_TMP_DIR)/nvidia/
	@$(MAKE) GENERATED_CODE_DIR=$(GRPC_TMP_DIR) grpc-generate
	@diff -Naur $(GRPC_TMP_DIR)/nvidia/ $(GENERATED_CODE_DIR)/nvidia/ || \
		(printf "\n\nOutdated files detected!\nPlease, run 'make generate' to regenerate GO code\n\n" && exit 1)
	@echo "generated files are up to date"

.PHONY: grpc-lint
grpc-lint: buf  ## Lint GRPC files
	@echo "lint protobuf files";
	cd $(PROTO_DIR) && \
	$(BUF) lint --config ../buf.yaml .

.PHONY: grpc-format
grpc-format: buf  ## Format GRPC files
	@echo "format protobuf files";
	cd $(PROTO_DIR) && \
	$(BUF) format -w --exit-code

##@ Development
GENERATE_TARGETS ?= dpuservice provisioning servicechainset sfc-controller operator \
	operator-embedded release-defaults kamaji-cluster-manager static-cluster-manager \
	ovn-kubernetes storage-snap mock-dms

.PHONY: generate
generate: ## Run all generate-* targets: generate-modules generate-manifests-* and generate-go-deepcopy-*.
	$(MAKE) generate-mocks generate-modules generate-manifests generate-go-deepcopy generate-docs

.PHONY: generate-mocks
generate-mocks: mockgen ## Generate mocks
	## Prepend the TOOLSDIR to the path for this command as `mockgen` is called from the $PATH inline in the code.
	## The DPF TOOLSDIR should be first in the path to ensure user tools are not used.
	## See go:generate comments for examples.
	export PATH="$(TOOLSDIR):$(PATH)"; go generate ./...

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to update go modules
	go mod tidy

.PHONY: generate-manifests
generate-manifests: $(addprefix generate-manifests-,$(GENERATE_TARGETS)) ## Run all generate-manifests-* targets

.PHONY: generate-manifests-operator
generate-manifests-operator: controller-gen kustomize ## Generate manifests e.g. CRD, RBAC. for the operator controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./deploy/helm/dpf-operator/templates/crds/"
	$(CONTROLLER_GEN) \
	paths="./cmd/operator/..." \
	paths="./cmd/kamaji-cluster-manager/..." \
	paths="./cmd/static-cluster-manager/..." \
	paths="./internal/operator/..." \
	paths="./internal/clustermanager/..." \
	paths="./api/operator/..." \
	crd:crdVersions=v1 \
	rbac:roleName="dpf-operator-manager-role" \
	output:crd:dir=./deploy/helm/dpf-operator/templates/crds \
	output:rbac:dir=./deploy/helm/dpf-operator/templates
	## Copy all other CRD definitions to the operator helm directory
	$(KUSTOMIZE) build config/operator-additional-crds -o  deploy/helm/dpf-operator/templates/crds/;

.PHONE: generate-manifests-mock-dms
generate-manifests-mock-dms: controller-gen
	$(CONTROLLER_GEN) \
	paths="./test/mock/dms/..." \
	rbac:roleName=mock-dms-manager-role \
	output:rbac:dir=./test/mock/dms/chart/templates/

.PHONY: generate-manifests-dpuservice
generate-manifests-dpuservice: controller-gen ## Generate manifests e.g. CRD, RBAC. for the dpuservice controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/dpuservice/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/dpuservice/..." \
	paths="./internal/dpuservice/..." \
	paths="./internal/dpuservicechain/..." \
	paths="./api/dpuservice/..." \
	crd:crdVersions=v1 \
	rbac:roleName=manager-role \
	output:crd:dir=./config/dpuservice/crd/bases \
	output:rbac:dir=./config/dpuservice/rbac \
	output:webhook:dir=./config/dpuservice/webhook \
	webhook

.PHONY: generate-manifests-servicechainset
generate-manifests-servicechainset: controller-gen kustomize envsubst ## Generate manifests e.g. CRD, RBAC. for the servicechainset controller.
	# TODO: Clean up pod-ipam-injector generation
	$(CONTROLLER_GEN) \
	paths="./cmd/servicechainset/..." \
	paths="./internal/servicechainset/..." \
	paths="./internal/pod-ipam-injector/..." \
	rbac:roleName=dpf-servicechain-controller-manager \
	output:rbac:dir=deploy/helm/dpu-networking/charts/servicechainset-controller/templates;
	find config/dpuservice/crd/bases/ -type f -not -name '*_dpu*' -exec cp {} deploy/helm/dpu-networking/charts/servicechainset-controller/templates/crds/ \;

.PHONY: generate-manifests-storage-snap
generate-manifests-storage-snap: controller-gen kustomize ## Generate CRDs for SNAP storage in DPU cluster 
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/snap/crd"
	$(CONTROLLER_GEN) \
	paths="./api/storage/..." \
	crd:crdVersions=v1,generateEmbeddedObjectMeta=true \
	output:crd:dir=./config/snap/crd
	## Set the image names and tags for storage-related charts
	$(ENVSUBST) < deploy/helm/storage/snap-controller/values.yaml.tmpl > deploy/helm/storage/snap-controller/values.yaml
	$(ENVSUBST) < deploy/helm/storage/snap-csi-plugin/values.yaml.tmpl > deploy/helm/storage/snap-csi-plugin/values.yaml
	$(ENVSUBST) < deploy/helm/storage/snap-dpu/values.yaml.tmpl > deploy/helm/storage/snap-dpu/values.yaml

.PHONY: generate-manifests-ovn-kubernetes-resource-injector
generate-manifests-ovn-kubernetes-resource-injector: envsubst ## Generate manifests e.g. CRD, RBAC. for the OVN Kubernetes Resource Injector
	$(ENVSUBST) < deploy/helm/ovn-kubernetes-resource-injector/values.yaml.tmpl > deploy/helm/ovn-kubernetes-resource-injector/values.yaml

RELEASE_FILE = ./internal/release/manifests/defaults.yaml

.PHONY: generate-manifests-release-defaults
generate-manifests-release-defaults: envsubst ## Generates manifests that contain the default values that should be used by the operators
	$(ENVSUBST) <  ./internal/release/templates/defaults.yaml.tmpl > $(RELEASE_FILE)

TEMPLATES_DIR ?= $(CURDIR)/internal/operator/inventory/templates
EMBEDDED_MANIFESTS_DIR ?= $(CURDIR)/internal/operator/inventory/manifests
.PHONY: generate-manifests-operator-embedded
generate-manifests-operator-embedded: kustomize envsubst generate-manifests-dpuservice generate-manifests-provisioning generate-manifests-release-defaults generate-manifests-kamaji-cluster-manager generate-manifests-static-cluster-manager ## Generates manifests that are embedded into the operator binary.
	# Reorder none here ensure that we generate the kustomize files in a specific order to be consumed by the DPF Operator.
	$(KUSTOMIZE) build --reorder=none config/provisioning/default > $(EMBEDDED_MANIFESTS_DIR)/provisioning-controller.yaml
	$(KUSTOMIZE) build --reorder=none config/dpu-detector > $(EMBEDDED_MANIFESTS_DIR)/dpu-detector.yaml
	$(KUSTOMIZE) build --reorder=none config/dpuservice/default > $(EMBEDDED_MANIFESTS_DIR)/dpuservice-controller.yaml
	$(KUSTOMIZE) build --reorder=none config/kamaji-cluster-manager/default > $(EMBEDDED_MANIFESTS_DIR)/kamaji-cluster-manager.yaml
	$(KUSTOMIZE) build --reorder=none config/static-cluster-manager/default > $(EMBEDDED_MANIFESTS_DIR)/static-cluster-manager.yaml

.PHONY: generate-manifests-sfc-controller
generate-manifests-sfc-controller: envsubst generate-manifests-servicechainset
	cp deploy/helm/dpu-networking/charts/servicechainset-controller/templates/crds/svc.dpu.nvidia.com_servicechains.yaml deploy/helm/dpu-networking/charts/sfc-controller/templates/crds/
	cp deploy/helm/dpu-networking/charts/servicechainset-controller/templates/crds/svc.dpu.nvidia.com_serviceinterfaces.yaml deploy/helm/dpu-networking/charts/sfc-controller/templates/crds/

.PHONY: generate-manifests-provisioning
generate-manifests-provisioning: controller-gen kustomize ## Generate manifests e.g. CRD, RBAC. for the DPF provisioning controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/provisioning/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/provisioning/..." \
	paths="./internal/provisioning/..." \
	paths="./api/provisioning/..." \
	crd:crdVersions=v1,generateEmbeddedObjectMeta=true \
	rbac:roleName=manager-role \
	output:crd:dir=./config/provisioning/crd/bases \
	output:rbac:dir=./config/provisioning/rbac \
	output:webhook:dir=./config/provisioning/webhook \
	webhook

.PHONY: generate-manifests-kamaji-cluster-manager
generate-manifests-kamaji-cluster-manager: controller-gen kustomize ## Generate manifests e.g. CRD, RBAC. for the DPF provisioning controller.
	$(CONTROLLER_GEN) \
	paths="./cmd/kamaji-cluster-manager/..." \
	paths="./internal/clustermanager/controller/..." \
	paths="./internal/clustermanager/kamaji/..." \
	rbac:roleName=manager-role \
	output:rbac:dir=./config/kamaji-cluster-manager/rbac

.PHONY: generate-manifests-static-cluster-manager
generate-manifests-static-cluster-manager: controller-gen kustomize ## Generate manifests e.g. CRD, RBAC. for the DPF provisioning controller.
	$(CONTROLLER_GEN) \
	paths="./cmd/static-cluster-manager/..." \
	paths="./internal/clustermanager/controller/..." \
	paths="./internal/clustermanager/static/..." \
	rbac:roleName=manager-role \
	output:rbac:dir=./config/static-cluster-manager/rbac

.PHONY: generate-manifests-ovn-kubernetes
generate-manifests-ovn-kubernetes: $(OVNKUBERNETES_DIR) envsubst ## Generate manifests for ovn-kubernetes
	$(ENVSUBST) < $(OVNKUBERNETES_HELM_CHART)/values.yaml.tmpl > $(OVNKUBERNETES_HELM_CHART)/values.yaml

.PHONY: generate-manifests-dummydpuservice
generate-manifests-dummydpuservice: envsubst ## Generate values for dummydpuservice helm chart.
	$(ENVSUBST) < deploy/dpuservices/dummydpuservice/chart/values.yaml.tmpl > deploy/dpuservices/dummydpuservice/chart/values.yaml

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

##@ Documentation
GENERATE_DOC_TARGETS ?= mdtoc api helm embedmd
.PHONY: generate-docs
generate-docs: $(addprefix generate-docs-,$(GENERATE_DOC_TARGETS))
	$(MAKE)

.PHONY: generate-docs-mdtoc
generate-docs-mdtoc: mdtoc ## Generate table of contents for our documentation.
	grep -rl -e '<!-- toc -->' docs | grep '\.md$$' | xargs $(MDTOC) --inplace

.PHONY: generate-docs-api
generate-docs-api: gen-crd-api-reference-docs ## Generate docs for the API.
	$(GEN_CRD_API_REFERENCE_DOCS) --renderer=markdown --source-path=api --config=hack/tools/api-docs/config.yaml --output-path=docs/api.md

.PHONY: generate-docs-helm
generate-docs-helm: helm-docs ## Generate helm chart documentation.
	$(HELM_DOCS) --ignore-file=.helmdocsignore

.PHONY: generate-docs-embedmd
generate-docs-embedmd: embedmd ## Embed additional files into markdown docs.
	grep -rl --include \*.md -e '\[embedmd\]' docs | xargs $(EMBEDMD) -w

.PHONY: verify-md-links
verify-md-links: $(LYCHEE) ## Check links in markdown docs are working
	$(LYCHEE) --accept 200,429 . *.md --exclude-path third_party --exclude-path ./deploy/helm # Exclude the external `third_party` docs and the generate `deploy/helm` docs.

##@ Testing

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test $$(go list ./... | grep -v /e2e) $(GO_TEST_ARGS)

.PHONY: test-report
test-report: envtest gotestsum ## Run tests and generate a junit style report
	set +o errexit; KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test -count 1 -race -json $$(go list ./... | grep -v /e2e) > junit.stdout; echo $$? > junit.exitcode;
	$(GOTESTSUM) --junitfile junit.xml --raw-command cat junit.stdout
	exit $$(cat junit.exitcode)

.PHONY: test-release-e2e-quick
test-release-e2e-quick: # Build images required for the quick DPF e2e test.
	$(MAKE) docker-build-dpf-system-for-$(ARCH) docker-push-dpf-system-for-$(ARCH)

	# Build and push all the helm charts
	$(MAKE) helm-package-all helm-push-all

TEST_CLUSTER_NAME := dpf-test
ADD_CONTROL_PLANE_TAINTS ?= true
test-env-e2e: minikube helm ## Setup a Kubernetes environment to run tests.

	# Create a minikube cluster to host the test.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) ADD_CONTROL_PLANE_TAINTS=$(ADD_CONTROL_PLANE_TAINTS) $(CURDIR)/hack/scripts/minikube-install.sh

	$(KUBECTL) get namespace dpf-operator-system || $(KUBECTL) create namespace dpf-operator-system

	# Create secrets required for using artefacts if required.
	$(CURDIR)/hack/scripts/create-artefact-secrets.sh

.PHONY: test-env-dpf-standalone
test-env-dpf-standalone:
	# Run the setup script.
	echo "Running the provisioning script."
	$(CURDIR)/hack/scripts/e2e-provision-standalone-cluster.sh

OPERATOR_NAMESPACE ?= dpf-operator-system
DEPLOY_KSM ?= false
DEPLOY_GRAFANA ?= false
DEPLOY_PROMETHEUS ?= false

.PHONY: test-deploy-operator-helm
test-deploy-operator-helm: helm helm-package-operator ## Deploy the DPF Operator using helm
	$(HELM) upgrade --install --create-namespace --namespace $(OPERATOR_NAMESPACE) \
		--set controllerManager.image.repository=$(DPF_SYSTEM_IMAGE)\
		--set controllerManager.image.tag=$(TAG) \
		--set imagePullSecrets[0].name=dpf-pull-secret \
		--set kube-state-metrics.enabled=$(DEPLOY_KSM) \
		--set grafana.enabled=$(DEPLOY_GRAFANA) \
		--set prometheus.enabled=$(DEPLOY_PROMETHEUS) \
		dpf-operator $(OPERATOR_HELM_CHART)

.PHONY: test-deploy-mock-dms
test-deploy-mock-dms: helm
	$(HELM) upgrade --install --create-namespace --namespace $(OPERATOR_NAMESPACE) \
		--set controllerManager.manager.image.repository=$(MOCK_DMS_IMAGE)\
		--set controllerManager.manager.image.tag=$(TAG) \
		--set imagePullSecrets[0].name=dpf-pull-secret \
		mock-dms $(MOCK_DMS_HELM_CHART)

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

ARTIFACTS_DIR ?= $(CURDIR)/artifacts
.PHONY: test-cache-images
test-cache-images: minikube ## Add images to the minikube cache based on the artifacts directory created in e2e.
	# Run a script which will cache images which were pulled in the test run to the minikube cache.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) ARTIFACTS_DIR=${ARTIFACTS_DIR} $(CURDIR)/hack/scripts/add-images-to-minikube-cache.sh

E2E_TEST_ARGS ?= -v -ginkgo.v
# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e ## Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e: stern ## Run e2e tests
	STERN=$(STERN) ARTIFACTS=$(ARTIFACTS_DIR) $(CURDIR)/hack/scripts/stern-log-collector.sh \
	  go test -timeout 0 ./test/e2e/ $(E2E_TEST_ARGS)

.PHONY: clean-test-env
clean-test-env: minikube ## Clean test environment (teardown minikube cluster)
	$(MINIKUBE) delete -p $(TEST_CLUSTER_NAME)


##@ validate commit
.PHONY: commit-check
commit-check: conform ## Run conform to validate commit message
	$(CONFORM) enforce

##@ lint and verify
GOLANGCI_LINT_GOGC ?= "100"
.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	GOOS=linux GOGC=$(GOLANGCI_LINT_GOGC) $(GOLANGCI_LINT) run --timeout 5m

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	GOOS=linux $(GOLANGCI_LINT) run --fix

.PHONY: verify-generate
verify-generate: generate ## Verify auto-generated code did not change
	$(info checking for git diff after running 'make generate')
	# Use intent-to-add to check for untracked files after generation.
	git add -N .
	$Q git diff --quiet ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make generate'"; exit 1 ; fi

.PHONY: verify-copyright
verify-copyright: ## Verify copyrights for project files
	$Q $(CURDIR)/hack/scripts/copyright-validation.sh

.PHONY: lint-helm
lint-helm: lint-helm-dpu-networking lint-helm-ovn-kubernetes lint-helm-dummydpuservice

.PHONY: lint-helm-dpu-networking
lint-helm-dpu-networking: helm ## Run helm lint for servicechainset chart
	$Q $(HELM) lint $(DPU_NETWORKING_HELM_CHART)

.PHONY: lint-helm-ovn-kubernetes
lint-helm-ovn-kubernetes: generate-manifests-ovn-kubernetes helm ## Run helm lint for ovn-kubernetes chart
	$Q $(HELM) lint $(OVNKUBERNETES_HELM_CHART)

.PHONY: lint-helm-dummydpuservice
lint-helm-dummydpuservice: helm ## Run helm lint for dummydpuservice chart
	$Q $(HELM) lint $(DUMMYDPUSERVICE_HELM_CHART)

.PHONY: lint-helm-snap-csi-plugin
lint-helm-snap-csi-plugin: helm ## Run helm lint for snap-csi-plugin chart
	$Q $(HELM) lint $(SNAP_CSI_PLUGIN_CHART)

.PHONY: lint-helm-snap-controller
lint-helm-snap-controller: helm ## Run helm lint for snap controller chart
	$Q $(HELM) lint $(SNAP_CONTROLLER_CHART)

.PHONY: lint-helm-spdk-csi-controller
lint-helm-spdk-csi-controller: helm ## Run helm lint for spdk csi controller chart
	$Q $(HELM) lint $(SPDK_CSI_CONTROLLER_CHART)

.PHONY: lint-helm-snap-dpu
lint-helm-snap-dpu: helm ## Run helm lint for snap dpu chart
	$Q $(HELM) lint $(SNAP_DPU_CHART)

##@ Release

.PHONY: release-build
release-build: generate ## Build helm and container images for release.
	# Build multiarch images which will run on both DPUs and x86 hosts.
	$(MAKE) $(addprefix docker-build-,$(MULTI_ARCH_DOCKER_BUILD_TARGETS))
	# Build arm64 images which will run on DPUs.
	$(MAKE) ARCH=$(DPU_ARCH) $(addprefix docker-build-,$(DPU_ARCH_DOCKER_BUILD_TARGETS))
	# Build amd64 images which will run on x86 hosts.
	$(MAKE) ARCH=$(HOST_ARCH) $(addprefix docker-build-,$(HOST_ARCH_DOCKER_BUILD_TARGETS))

	# Package the helm charts.
	$(MAKE) helm-package-all

.PHONY: release
release: release-build ## Build and push helm and container images for release.

	# Push all of the images
	$(MAKE) docker-push-all

	# Package and push the helm charts.
	$(MAKE) helm-push-all

.PHONY: warm-cache
warm-cache: ## Warm the cache for the tests.

	$(MAKE) release-build test lint

##@ Build

GO_GCFLAGS ?= ""
GO_LDFLAGS ?= "-extldflags '-static'"

STORAGE_SNAP_CSI_DRIVER_GO_LDFLAGS ?= "$(shell echo $(GO_LDFLAGS)) -X github.com/nvidia/doca-platform/internal/storage/snap/csi-plugin/common.VendorVersion=$(TAG)"

BUILD_TARGETS ?= $(DPU_ARCH_BUILD_TARGETS)
DPF_SYSTEM_BUILD_TARGETS ?= operator provisioning dpuservice servicechainset kamaji-cluster-manager static-cluster-manager sfc-controller ovs-helper snap-controller dpfctl dpfctl-darwin
DPU_ARCH_BUILD_TARGETS ?= storage-snap-node-driver storage-vendor-dpu-plugin
BUILD_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

HOST_ARCH = amd64
DPU_ARCH = arm64

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
BASE_IMAGE = gcr.io/distroless/base:nonroot
ALPINE_IMAGE = alpine:3.19

.PHONY: binaries
binaries: $(addprefix binary-,$(BUILD_TARGETS)) ## Build all binaries

.PHONY: binaries-dpf-system
binaries-dpf-system: $(addprefix binary-,$(DPF_SYSTEM_BUILD_TARGETS)) ## Build binaries for the dpf-system image.

.PHONY: binary-operator
binary-operator: ## Build the operator controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/operator github.com/nvidia/doca-platform/cmd/operator

.PHONY: binary-provisioning
binary-provisioning: ## Build the provisioning controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/provisioning github.com/nvidia/doca-platform/cmd/provisioning

.PHONY: binary-kamaji-cluster-manager
binary-kamaji-cluster-manager: ## Build the kamaji-cluster-manager binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/kamaji-cluster-manager github.com/nvidia/doca-platform/cmd/kamaji-cluster-manager

.PHONY: binary-static-cluster-manager
binary-static-cluster-manager: ## Build the static-cluster-manager binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/static-cluster-manager github.com/nvidia/doca-platform/cmd/static-cluster-manager

.PHONY: binary-dpuservice
binary-dpuservice: ## Build the dpuservice controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpuservice github.com/nvidia/doca-platform/cmd/dpuservice

.PHONY: binary-servicechainset
binary-servicechainset: ## Build the servicechainset controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/servicechainset github.com/nvidia/doca-platform/cmd/servicechainset

.PHONY: binary-dpucniprovisioner
binary-dpucniprovisioner: ## Build the DPU CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpucniprovisioner github.com/nvidia/doca-platform/cmd/dpucniprovisioner

.PHONY: binary-ovs-helper
binary-ovs-helper: ## Build the OVS Helper binary
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/ovshelper github.com/nvidia/doca-platform/cmd/ovshelper

.PHONY: binary-sfc-controller
binary-sfc-controller: ## Build the Host CNI Provisioner binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/sfc-controller github.com/nvidia/doca-platform/cmd/sfc-controller

.PHONY: binary-ipallocator
binary-ipallocator: ## Build the IP allocator binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/ipallocator github.com/nvidia/doca-platform/cmd/ipallocator

.PHONY: binary-detector
binary-detector: ## Build the DPU detector binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpu-detector github.com/nvidia/doca-platform/cmd/dpudetector

.PHONY: binary-ovn-kubernetes-resource-injector
binary-ovn-kubernetes-resource-injector: ## Build the OVN Kubernetes Resource Injector.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/ovnkubernetesresourceinjector github.com/nvidia/doca-platform/cmd/ovnkubernetesresourceinjector

.PHONY: binary-snap-controller
binary-snap-controller: ## Build the snap controller controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/snap-controller github.com/nvidia/doca-platform/cmd/storage/snap-controller

.PHONY: binary-snap-node-driver
binary-snap-node-driver: ## Build the snap node driver controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/provisioning github.com/nvidia/doca-platform/cmd/provisioning

.PHONY: binary-storage-vendor-dpu-plugin
binary-storage-vendor-dpu-plugin: ## Build the storage vendor DPU plugin controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/provisioning github.com/nvidia/doca-platform/cmd/provisioning

.PHONY: binary-snap-csi-plugin
binary-snap-csi-plugin: ## Build the snap-csi-plugin binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build \
		-ldflags=$(STORAGE_SNAP_CSI_DRIVER_GO_LDFLAGS) \
		-gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/snap-csi-plugin github.com/nvidia/doca-platform/cmd/storage/snap-csi-plugin

.PHONY: binary-dpfctl
binary-dpfctl: ## Build the dpfctl binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build \
		-ldflags="$(shell echo $(GO_LDFLAGS)) -X main.version=$(TAG)" \
		-gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpfctl github.com/nvidia/doca-platform/cmd/dpfctl

.PHONY: binary-dpfctl-darwin
binary-dpfctl-darwin: ## Build the dpfctl binary.
	CGO_ENABLED=0 GOOS=darwin GOARCH=$(ARCH) go build \
		-ldflags="$(shell echo $(GO_LDFLAGS)) -X main.version=$(TAG)" \
		-gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpfctl-darwin github.com/nvidia/doca-platform/cmd/dpfctl

.PHONY: install-dpfctl
install-dpfctl: binary-dpfctl ## Install the dpfctl binary.
	install -m 755 $(LOCALBIN)/dpfctl $(GOPATH)/bin/dpfctl

DOCKER_BUILD_TARGETS=$(HOST_ARCH_DOCKER_BUILD_TARGETS) $(DPU_ARCH_DOCKER_BUILD_TARGETS) $(MULTI_ARCH_DOCKER_BUILD_TARGETS)
HOST_ARCH_DOCKER_BUILD_TARGETS=hostdriver
DPU_ARCH_DOCKER_BUILD_TARGETS=$(DPU_ARCH_BUILD_TARGETS) ovs-cni
MULTI_ARCH_DOCKER_BUILD_TARGETS= dpf-system ovn-kubernetes dpf-tools snap-csi-plugin snap-controller mock-dms

.PHONY: docker-build-all
docker-build-all: $(addprefix docker-build-,$(DOCKER_BUILD_TARGETS)) ## Build docker images for all DOCKER_BUILD_TARGETS. Architecture defaults to build system architecture unless overridden or hardcoded.

DPF_SYSTEM_IMAGE_NAME ?= dpf-system
export DPF_SYSTEM_IMAGE ?= $(REGISTRY)/$(DPF_SYSTEM_IMAGE_NAME)

OVNKUBERNETES_IMAGE_NAME = ovn-kubernetes
export OVNKUBERNETES_IMAGE = $(REGISTRY)/$(OVNKUBERNETES_IMAGE_NAME)

OVNKUBERNETES_RESOURCE_INJECTOR_IMAGE_NAME = ovn-kubernetes-resource-injector
export OVNKUBERNETES_RESOURCE_INJECTOR_IMAGE = $(REGISTRY)/$(OVNKUBERNETES_RESOURCE_INJECTOR_IMAGE_NAME)

OVS_CNI_IMAGE_NAME ?= ovs-cni-plugin
export OVS_CNI_IMAGE ?= $(REGISTRY)/$(OVS_CNI_IMAGE_NAME)

export HOSTDRIVER_IMAGE ?= $(REGISTRY)/hostdriver

IPALLOCATOR_IMAGE_NAME ?= ip-allocator
export IPALLOCATOR_IMAGE ?= $(REGISTRY)/$(IPALLOCATOR_IMAGE_NAME)


# Images that are running on DPU worker nodes (arm64)
DPUCNIPROVISIONER_IMAGE_NAME ?= dpu-cni-provisioner
DPUCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(DPUCNIPROVISIONER_IMAGE_NAME)

DUMMYDPUSERVICE_IMAGE_NAME ?= dummydpuservice
export DUMMYDPUSERVICE_IMAGE ?= $(REGISTRY)/$(DUMMYDPUSERVICE_IMAGE_NAME)

MOCK_DMS_IMAGE_NAME ?= mock-dms
MOCK_DMS_IMAGE ?= $(REGISTRY)/$(MOCK_DMS_IMAGE_NAME)

DPF_TOOLS_IMAGE_NAME ?= dpf-tools
export DPF_TOOLS_IMAGE ?= $(REGISTRY)/$(DPF_TOOLS_IMAGE_NAME)

STORAGE_SNAP_CONTROLLER_NAME ?= snap-controller
export STORAGE_SNAP_CONTROLLER_IMAGE ?= $(REGISTRY)/$(STORAGE_SNAP_CONTROLLER_NAME)

STORAGE_SNAP_NODE_DRIVER_NAME ?= snap-node-driver
export STORAGE_SNAP_NODE_DRIVER_IMAGE ?= $(REGISTRY)/$(STORAGE_SNAP_NODE_DRIVER_NAME)

STORAGE_VENDOR_DPU_PLUGIN_NAME ?= storage-vendor-dpu-plugin
export STORAGE_VENDOR_DPU_PLUGIN_IMAGE ?= $(REGISTRY)/$(STORAGE_VENDOR_DPU_PLUGIN_NAME)

STORAGE_SNAP_CSI_DRIVER_IMAGE_NAME = snap-csi-plugin
export STORAGE_SNAP_CSI_DRIVER_IMAGE ?= $(REGISTRY)/$(STORAGE_SNAP_CSI_DRIVER_IMAGE_NAME)

DPF_SYSTEM_ARCH ?= $(HOST_ARCH) $(DPU_ARCH)
.PHONY: docker-build-dpf-system # Build a multi-arch image for DPF System. The variable DPF_SYSTEM_ARCH defines which architectures this target builds for.
docker-build-dpf-system: $(addprefix docker-build-dpf-system-for-,$(DPF_SYSTEM_ARCH))

docker-build-dpf-system-for-%:
	# Provenance false ensures this target builds an image rather than a manifest when using buildx.
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--platform=linux/$* \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg TAG=$(TAG) \
		-f Dockerfile.dpf-system \
		. \
		-t $(DPF_SYSTEM_IMAGE):$(TAG)-$*

.PHONY: docker-push-dpf-system # Push a multi-arch image for DPF System using `docker manifest`. The variable DPF_SYSTEM_ARCH defines which architectures this target pushes for.
docker-push-dpf-system: $(addprefix docker-push-dpf-system-for-,$(DPF_SYSTEM_ARCH))
	docker manifest push --purge $(DPF_SYSTEM_IMAGE):$(TAG)

docker-push-dpf-system-for-%:
	# Tag and push the arch-specific image with the single arch-agnostic tag.
	docker tag $(DPF_SYSTEM_IMAGE):$(TAG)-$* $(DPF_SYSTEM_IMAGE):$(TAG)
	docker push $(DPF_SYSTEM_IMAGE):$(TAG)
	# This must be called in a separate target to ensure the shell command is called in the correct order.
	$(MAKE) docker-create-manifest-for-dpf-system

docker-create-manifest-for-dpf-system:
	# Note: If you tag an image with multiple registries this push might fail. This can be fixed by pruning existing docker images.
	docker manifest create --amend $(DPF_SYSTEM_IMAGE):$(TAG) $(shell docker inspect --format='{{index .RepoDigests 0}}' $(DPF_SYSTEM_IMAGE):$(TAG))


# additional build args for the dpf-tools image.
DPF_TOOLS_BUILD_ARGS ?=
.PHONY: docker-build-dpf-tools
docker-build-dpf-tools: $(addprefix docker-build-dpf-tools-for-,$(DPF_SYSTEM_ARCH))

docker-build-dpf-tools-for-%: $(SOS_REPORT_DIR)
	# Copy DPF specific files to the SOS report repo in the hack directory
	cp -Rp ./hack/tools/dpf-tools/* $(REPOSDIR)/doca-sosreport-${DOCA_SOSREPORT_REF}/
	cd $(REPOSDIR)/doca-sosreport-${DOCA_SOSREPORT_REF} && \
	docker buildx build \
	--load \
	--label=org.opencontainers.image.created=$(DATE) \
	--label=org.opencontainers.image.name=$(PROJECT_NAME) \
	--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
	--label=org.opencontainers.image.version=$(TAG) \
	--label=org.opencontainers.image.source=$(PROJECT_REPO) \
	--provenance=false \
	--platform=linux/$* \
	. \
	-t ${DPF_TOOLS_IMAGE}:${TAG}-$* \
	${DPF_TOOLS_BUILD_ARGS}

.PHONY: docker-push-dpf-tools # Push a multi-arch image for DPF tools using `docker manifest`. The variable DPF_SYSTEM_ARCH defines which architectures this target pushes for.
docker-push-dpf-tools: $(addprefix docker-push-dpf-tools-for-,$(DPF_SYSTEM_ARCH))
	docker manifest push --purge $(DPF_TOOLS_IMAGE):$(TAG)

docker-push-dpf-tools-for-%:
	# Tag and push the arch-specific image with the single arch-agnostic tag.
	docker tag $(DPF_TOOLS_IMAGE):$(TAG)-$* $(DPF_TOOLS_IMAGE):$(TAG)
	docker push $(DPF_TOOLS_IMAGE):$(TAG)
	# This must be called in a separate target to ensure the shell command is called in the correct order.
	$(MAKE) docker-create-manifest-for-dpf-tools

docker-create-manifest-for-dpf-tools:
	# Note: If you tag an image with multiple registries this push might fail. This can be fixed by pruning existing docker images.
	docker manifest create --amend $(DPF_TOOLS_IMAGE):$(TAG) $(shell docker inspect --format='{{json .RepoDigests}}' $(DPF_TOOLS_IMAGE):$(TAG) \
	| sed 's/[][]//g; s/,/\n/g' |grep $(REGISTRY))

.PHONY: docker-build-ipallocator
docker-build-ipallocator: ## Build docker image for the IP Allocator
	# Base image can't be distroless because of the readiness probe that is using cat which doesn't exist in distroless
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--platform=linux/$(ARCH) \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(ALPINE_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/ipallocator \
		  -f Dockerfile \
		. \
		-t $(IPALLOCATOR_IMAGE):$(TAG)

.PHONY: docker-build-ovs-cni
docker-build-ovs-cni: $(OVS_CNI_DIR) ## Builds the OVS CNI image
	cd $(OVS_CNI_DIR) && \
	$(OVS_CNI_DIR)/hack/get_version.sh > .version && \
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--build-arg goarch=$(DPU_ARCH) \
		--platform linux/${DPU_ARCH} \
		-f ./cmd/Dockerfile \
		-t $(OVS_CNI_IMAGE):${TAG} \
		.

.PHONY: docker-build-ovn-kubernetes # Build a multi-arch image for DPF System. The variable DPF_SYSTEM_ARCH defines which architectures this target builds for.
docker-build-ovn-kubernetes: $(addprefix docker-build-ovn-kubernetes-for-,$(DPF_SYSTEM_ARCH))

docker-build-ovn-kubernetes-for-%: $(OVNKUBERNETES_DIR)
	# Provenance false ensures this target builds an image rather than a manifest when using buildx.
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--platform=linux/$* \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg ovn_kubernetes_dir=$(subst $(CURDIR)/,,$(OVNKUBERNETES_DIR)) \
		-f Dockerfile.ovn-kubernetes \
		. \
		-t $(OVNKUBERNETES_IMAGE):$(TAG)-$*

.PHONY: docker-push-ovn-kubernetes # Push a multi-arch image for ovn-kubernetes using `docker manifest`. The variable DPF_SYSTEM_ARCH defines which architectures this target pushes for.
docker-push-ovn-kubernetes: $(addprefix docker-push-ovn-kubernetes-for-,$(DPF_SYSTEM_ARCH))
	docker manifest push --purge $(OVNKUBERNETES_IMAGE):$(TAG)

docker-push-ovn-kubernetes-for-%:
	# Tag and push the arch-specific image with the single arch-agnostic tag.
	docker tag $(OVNKUBERNETES_IMAGE):$(TAG)-$* $(OVNKUBERNETES_IMAGE):$(TAG)
	docker push $(OVNKUBERNETES_IMAGE):$(TAG)
	# This must be called in a separate target to ensure the shell command is called in the correct order.
	$(MAKE) docker-create-manifest-for-ovn-kubernetes

docker-create-manifest-for-ovn-kubernetes:
	# Note: If you tag an image with multiple registries this push might fail. This can be fixed by pruning existing docker images.
	docker manifest create --amend $(OVNKUBERNETES_IMAGE):$(TAG) $(shell docker inspect --format='{{index .RepoDigests 0}}' $(OVNKUBERNETES_IMAGE):$(TAG))

.PHONY: docker-build-hostdriver
docker-build-hostdriver: ## Build docker image for DMS and hostnetwork.
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--platform linux/${HOST_ARCH} \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		-t $(HOSTDRIVER_IMAGE):$(TAG) \
		-f Dockerfile.hostdriver \
		.

.PHONY: docker-build-dummydpuservice
docker-build-dummydpuservice: ## Build docker images for the dummydpuservice
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--platform=linux/$(DPU_ARCH) \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/dummydpuservice \
		-f Dockerfile \
		. \
		-t $(DUMMYDPUSERVICE_IMAGE):$(TAG)

.PHONY: docker-build-mock-dms
docker-build-mock-dms: ## Build docker images for the mock-dms
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--platform=linux/$(ARCH) \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./test/mock/dms \
		-f Dockerfile \
		. \
		-t $(MOCK_DMS_IMAGE):$(TAG)

.PHONY: docker-build-ovn-kubernetes-resource-injector
docker-build-ovn-kubernetes-resource-injector: ## Build docker image for the OVN Kubernetes Resource Injector
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--label=org.opencontainers.image.source=$(PROJECT_REPO) \
		--provenance=false \
		--platform=linux/$(ARCH) \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/ovnkubernetesresourceinjector \
		-f Dockerfile \
		. \
		-t $(OVNKUBERNETES_RESOURCE_INJECTOR_IMAGE):$(TAG)

.PHONY: docker-build-snap-controller # Build a multi-arch image for snap controller. The variable DPF_SYSTEM_ARCH defines which architectures this target builds for.
docker-build-snap-controller: $(addprefix docker-build-snap-controller-for-,$(DPF_SYSTEM_ARCH))

docker-build-snap-controller-for-%:
	docker buildx build \
	--load \
	--label=org.opencontainers.image.created=$(DATE) \
	--label=org.opencontainers.image.name=$(PROJECT_NAME) \
	--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
	--label=org.opencontainers.image.version=$(TAG) \
	--label=org.opencontainers.image.source=$(PROJECT_REPO) \
	--provenance=false \
	--platform=linux/$* \
	--build-arg builder_image=$(BUILD_IMAGE) \
	--build-arg base_image=$(BASE_IMAGE) \
	--build-arg ldflags=$(GO_LDFLAGS) \
	--build-arg gcflags=$(GO_GCFLAGS) \
	--build-arg package=./cmd/storage/snap-controller \
	-f Dockerfile \
	. \
	-t ${STORAGE_SNAP_CONTROLLER_IMAGE}:$(TAG)-$*

.PHONY: docker-push-snap-controller # Push a multi-arch image for snap controller using `docker manifest`. The variable DPF_SYSTEM_ARCH defines which architectures this target pushes for.
docker-push-snap-controller: $(addprefix docker-push-snap-controller-for-,$(DPF_SYSTEM_ARCH))
	docker manifest push --purge $(STORAGE_SNAP_CONTROLLER_IMAGE):$(TAG)

docker-push-snap-controller-for-%:
	# Tag and push the arch-specific image with the single arch-agnostic tag.
	docker tag $(STORAGE_SNAP_CONTROLLER_IMAGE):$(TAG)-$* $(STORAGE_SNAP_CONTROLLER_IMAGE):$(TAG)
	docker push $(STORAGE_SNAP_CONTROLLER_IMAGE):$(TAG)
	# This must be called in a separate target to ensure the shell command is called in the correct order.
	$(MAKE) docker-create-manifest-for-snap-controller

docker-create-manifest-for-snap-controller:
	# Note: If you tag an image with multiple registries this push might fail. This can be fixed by pruning existing docker images.
	docker manifest create --amend $(STORAGE_SNAP_CONTROLLER_IMAGE):$(TAG) $(shell docker inspect --format='{{index .RepoDigests 0}}' $(STORAGE_SNAP_CONTROLLER_IMAGE):$(TAG))

.PHONY: docker-build-storage-snap-node-driver # Build a arm64 image for snap-node-driver
docker-build-storage-snap-node-driver:
	docker buildx build \
	--load \
	--label=org.opencontainers.image.created=$(DATE) \
	--label=org.opencontainers.image.name=$(PROJECT_NAME) \
	--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
	--label=org.opencontainers.image.version=$(TAG) \
	--label=org.opencontainers.image.source=$(PROJECT_REPO) \
	--provenance=false \
	--platform=linux/$(DPU_ARCH) \
	--build-arg builder_image=$(BUILD_IMAGE) \
	--build-arg base_image=$(BASE_IMAGE) \
	--build-arg ldflags=$(GO_LDFLAGS) \
	--build-arg gcflags=$(GO_GCFLAGS) \
	--build-arg package=./cmd/storage/snap-node-driver \
	-f Dockerfile \
	. \
	-t ${STORAGE_SNAP_NODE_DRIVER_IMAGE}:$(TAG)

.PHONY: docker-build-storage-vendor-dpu-plugin # Build a arm64 image for storage-vendor-dpu-plugin
docker-build-storage-vendor-dpu-plugin:
	docker buildx build \
	--load \
	--label=org.opencontainers.image.created=$(DATE) \
	--label=org.opencontainers.image.name=$(PROJECT_NAME) \
	--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
	--label=org.opencontainers.image.version=$(TAG) \
	--provenance=false \
	--platform=linux/$(DPU_ARCH) \
	--build-arg builder_image=$(BUILD_IMAGE) \
	--build-arg base_image=$(BASE_IMAGE) \
	--build-arg ldflags=$(GO_LDFLAGS) \
	--build-arg gcflags=$(GO_GCFLAGS) \
	--build-arg package=./cmd/storage/storage-vendor-dpu-plugin \
	-f Dockerfile \
	. \
	-t ${STORAGE_VENDOR_DPU_PLUGIN_IMAGE}:$(TAG)

.PHONY: docker-build-snap-csi-plugin # Build a multi-arch image for DPF System. The variable DPF_SYSTEM_ARCH defines which architectures this target builds for.
docker-build-snap-csi-plugin: $(addprefix docker-build-snap-csi-plugin-for-,$(DPF_SYSTEM_ARCH))

docker-build-snap-csi-plugin-for-%:
	# Provenance false ensures this target builds an image rather than a manifest when using buildx.
	docker buildx build \
		--load \
		--label=org.opencontainers.image.created=$(DATE) \
		--label=org.opencontainers.image.name=$(PROJECT_NAME) \
		--label=org.opencontainers.image.revision=$(FULL_COMMIT) \
		--label=org.opencontainers.image.version=$(TAG) \
		--provenance=false \
		--platform=linux/$* \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg ldflags=$(STORAGE_SNAP_CSI_DRIVER_GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		-f Dockerfile.snap-csi-plugin \
		. \
		-t $(STORAGE_SNAP_CSI_DRIVER_IMAGE):$(TAG)-$*

.PHONY: docker-push-snap-csi-plugin # Push a multi-arch image for snap-csi-plugin using `docker manifest`. The variable DPF_SYSTEM_ARCH defines which architectures this target pushes for.
docker-push-snap-csi-plugin: $(addprefix docker-push-snap-csi-plugin-for-,$(DPF_SYSTEM_ARCH))
	docker manifest push --purge $(STORAGE_SNAP_CSI_DRIVER_IMAGE):$(TAG)

docker-push-snap-csi-plugin-for-%:
	# Tag and push the arch-specific image with the single arch-agnostic tag.
	docker tag $(STORAGE_SNAP_CSI_DRIVER_IMAGE):$(TAG)-$* $(STORAGE_SNAP_CSI_DRIVER_IMAGE):$(TAG)
	docker push $(STORAGE_SNAP_CSI_DRIVER_IMAGE):$(TAG)
	# This must be called in a separate target to ensure the shell command is called in the correct order.
	$(MAKE) docker-create-manifest-for-snap-csi-plugin

docker-create-manifest-for-snap-csi-plugin:
	# Note: If you tag an image with multiple registries this push might fail. This can be fixed by pruning existing docker images.
	docker manifest create --amend $(STORAGE_SNAP_CSI_DRIVER_IMAGE):$(TAG) $(shell docker inspect --format='{{index .RepoDigests 0}}' $(STORAGE_SNAP_CSI_DRIVER_IMAGE):$(TAG))

.PHONY: docker-push-all
docker-push-all: $(addprefix docker-push-,$(DOCKER_BUILD_TARGETS))  ## Push the docker images for all DOCKER_BUILD_TARGETS.

.PHONY: docker-push-dpf-system
docker-push-dpf-system: ## This is a no-op to allow using DOCKER_BUILD_TARGETS.

.PHONY: docker-push-ovs-cni
docker-push-ovs-cni: ## Push the docker image for ovs-cni
	docker push $(OVS_CNI_IMAGE):$(TAG)

.PHONY: docker-push-hostdriver
docker-push-hostdriver: ## Push the docker image for DMS and hostnetwork.
	docker push $(HOSTDRIVER_IMAGE):$(TAG)

.PHONY: docker-push-dpucniprovisioner
docker-push-dpucniprovisioner: ## Push the docker image for DPU CNI Provisioner.
	docker push $(DPUCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-push-ipallocator
docker-push-ipallocator: ## Push the docker image for IP Allocator.
	docker push $(IPALLOCATOR_IMAGE):$(TAG)

.PHONY: docker-push-dummydpuservice
docker-push-dummydpuservice: ## Push the docker image for dummydpuservice
	docker push $(DUMMYDPUSERVICE_IMAGE):$(TAG)

.PHONY: docker-push-mock-dms
docker-push-mock-dms: ## Push the docker image for dummydpuservice
	docker push $(MOCK_DMS_IMAGE):$(TAG)

.PHONY: docker-push-ovn-kubernetes-resource-injector
docker-push-ovn-kubernetes-resource-injector: ## Push the docker image for the OVN Kubernetes Resource Injector
	docker push $(OVNKUBERNETES_RESOURCE_INJECTOR_IMAGE):$(TAG)

.PHONY: docker-push-storage-snap-node-driver
docker-push-storage-snap-node-driver: ## Push the docker image for snap node driver
	docker push ${STORAGE_SNAP_NODE_DRIVER_IMAGE}:$(TAG)

.PHONY: docker-push-storage-vendor-dpu-plugin
docker-push-storage-vendor-dpu-plugin: ## Push the docker image for storage vendor dpu plugin
	docker push ${STORAGE_VENDOR_DPU_PLUGIN_IMAGE}:$(TAG)

# helm charts

# By default the helm registry is assumed to be an OCI registry. This variable should be overwritten when using a https helm repository.
export HELM_REGISTRY ?= oci://$(REGISTRY)

HELM_TARGETS ?= dpu-networking operator ovn-kubernetes snap-csi-plugin snap-controller snap-dpu

# metadata for the operator helm chart
OPERATOR_HELM_CHART_NAME ?= dpf-operator
OPERATOR_HELM_CHART ?= $(HELMDIR)/$(OPERATOR_HELM_CHART_NAME)

## metadata for dpu-networking helm chart.
export DPU_NETWORKING_HELM_CHART_NAME = dpu-networking
DPU_NETWORKING_HELM_CHART ?= $(HELMDIR)/$(DPU_NETWORKING_HELM_CHART_NAME)
DPU_NETWORKING_HELM_CHART_VER ?= $(TAG)

## metadata for ovn-kubernetes
export OVNKUBERNETES_HELM_CHART_NAME = ovn-kubernetes-chart
OVNKUBERNETES_HELM_CHART ?= $(OVNKUBERNETES_DIR)/helm/ovn-kubernetes
OVNKUBERNETES_HELM_CHART_VER ?= $(TAG)

## metadata for ovn-kubernetes-resource-injector
export OVNKUBERNETES_RESOURCE_INJECTOR_HELM_CHART_NAME = ovn-kubernetes-resource-injector-chart
OVNKUBERNETES_RESOURCE_INJECTOR_HELM_CHART ?= $(HELMDIR)/ovn-kubernetes-resource-injector
OVNKUBERNETES_RESOURCE_INJECTOR_HELM_CHART_VER ?= $(TAG)

# metadata for dummydpuservice.
DUMMYDPUSERVICE_HELM_CHART_NAME = dummydpuservice-chart
DUMMYDPUSERVICE_HELM_CHART ?= $(DPUSERVICESDIR)/dummydpuservice/chart

# metadata for snap-csi-plugin.
SNAP_CSI_PLUGIN_CHART_NAME = snap-csi-plugin-chart
SNAP_CSI_PLUGIN_CHART ?= $(HELMDIR)/storage/snap-csi-plugin
SNAP_CSI_PLUGIN_CHART_VER ?= $(TAG)

# metadata for snap controller.
SNAP_CONTROLLER_CHART_NAME = snap-controller-chart
SNAP_CONTROLLER_CHART ?= $(HELMDIR)/storage/snap-controller
SNAP_CONTROLLER_CHART_VER ?= $(TAG)

# metadata for spdk csi controller.
SPDK_CSI_CONTROLLER_CHART_NAME = spdk-csi-controller-chart
SPDK_CSI_CONTROLLER_CHART ?= $(EXAMPLE)/spdk-csi/helm
SPDK_CSI_CONTROLLER_CHART_VER ?= $(TAG)

# metadata for snap dpu.
SNAP_DPU_CHART_NAME = snap-dpu-chart
SNAP_DPU_CHART ?= $(HELMDIR)/storage/snap-dpu
SNAP_DPU_CHART_VER ?= $(TAG)

# metadata for mock dms.
MOCK_DMS_HELM_CHART ?=test/mock/dms/chart

.PHONY: helm-package-all
helm-package-all: $(addprefix helm-package-,$(HELM_TARGETS))  ## Package the helm charts for all components.

.PHONY: helm-package-dpu-networking
helm-package-dpu-networking: $(CHARTSDIR) helm ## Package helm chart for service chain controller
	$(HELM) dependency update $(DPU_NETWORKING_HELM_CHART)
	$(HELM) package $(DPU_NETWORKING_HELM_CHART) --version $(DPU_NETWORKING_HELM_CHART_VER) --destination $(CHARTSDIR)

OPERATOR_CHART_TAGS ?=$(TAG)
.PHONY: helm-package-operator
helm-package-operator: $(CHARTSDIR) helm yq ## Package helm chart for DPF Operator
	## Set the default images used in the chart.
	$(YQ) e -i '.controllerManager.image.repository = env(DPF_SYSTEM_IMAGE)'  deploy/helm/dpf-operator/values.yaml
	$(YQ) e -i '.controllerManager.image.tag = env(TAG)'  deploy/helm/dpf-operator/values.yaml

	## Update the helm dependencies for the chart.
	$(HELM) repo add argo https://argoproj.github.io/argo-helm
	$(HELM) repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts
	$(HELM) repo add prometheus https://prometheus-community.github.io/helm-charts
	$(HELM) repo add grafana https://grafana.github.io/helm-charts
	$(HELM) repo add clastix https://clastix.github.io/charts
	$(HELM) dependency build $(OPERATOR_HELM_CHART)
	for tag in $(OPERATOR_CHART_TAGS); do \
		$(HELM) package $(OPERATOR_HELM_CHART) --version $$tag --destination $(CHARTSDIR); \
	done

.PHONY: helm-package-ovn-kubernetes
helm-package-ovn-kubernetes: $(OVNKUBERNETES_DIR) $(CHARTSDIR) helm ## Package helm chart for ovn-kubernetes
	$(HELM) package $(OVNKUBERNETES_HELM_CHART) --version $(OVNKUBERNETES_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-ovn-kubernetes-resource-injector
helm-package-ovn-kubernetes-resource-injector: $(CHARTSDIR) helm generate-manifests-ovn-kubernetes-resource-injector ## Package helm chart for OVN Kubernetes Resource Injector
	$(HELM) package $(OVNKUBERNETES_RESOURCE_INJECTOR_HELM_CHART) --version $(OVNKUBERNETES_RESOURCE_INJECTOR_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-dummydpuservice
helm-package-dummydpuservice: $(DPUSERVICESDIR) helm generate-manifests-dummydpuservice ## Package helm chart for dummydpuservice
	$(HELM) package $(DUMMYDPUSERVICE_HELM_CHART) --version $(TAG) --destination $(CHARTSDIR)

.PHONY: helm-package-snap-csi-plugin
helm-package-snap-csi-plugin: $(CHARTSDIR) helm generate-manifests-storage-snap
	$(HELM) package $(SNAP_CSI_PLUGIN_CHART) --version $(SNAP_CSI_PLUGIN_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-snap-controller
helm-package-snap-controller: $(CHARTSDIR) helm generate-manifests-storage-snap
	$(HELM) package $(SNAP_CONTROLLER_CHART) --version $(SNAP_CONTROLLER_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-spdk-csi-controller
helm-package-spdk-csi-controller: $(CHARTSDIR) helm
	$(HELM) package $(SPDK_CSI_CONTROLLER_CHART) --version $(SPDK_CSI_CONTROLLER_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-snap-dpu
helm-package-snap-dpu: $(CHARTSDIR) helm generate-manifests-storage-snap
	cp -r config/snap/crd $(SNAP_DPU_CHART)/templates
	$(HELM) package $(SNAP_DPU_CHART) --version $(SNAP_DPU_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-push-all
helm-push-all: $(addprefix helm-push-,$(HELM_TARGETS))  ## Push the helm charts for all components.

.PHONY: helm-push-operator
helm-push-operator: $(CHARTSDIR) helm ## Push helm chart for dpf-operator
	for tag in $(OPERATOR_CHART_TAGS); do \
		$(HELM) push $(CHARTSDIR)/$(OPERATOR_HELM_CHART_NAME)-$$tag.tgz $(HELM_REGISTRY); \
	done

.PHONY: helm-push-dpu-networking
helm-push-dpu-networking: $(CHARTSDIR) helm ## Push helm chart for service chain controller
	$(HELM) push $(CHARTSDIR)/$(DPU_NETWORKING_HELM_CHART_NAME)-$(DPU_NETWORKING_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-ovn-kubernetes
helm-push-ovn-kubernetes: $(CHARTSDIR) helm ## Push helm chart for ovn-kubernetes
	$(HELM) push $(CHARTSDIR)/$(OVNKUBERNETES_HELM_CHART_NAME)-$(OVNKUBERNETES_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-ovn-kubernetes-resource-injector
helm-push-ovn-kubernetes-resource-injector: $(CHARTSDIR) helm ## Push helm chart for OVN Kubernetes Resource Injector
	$(HELM) push $(CHARTSDIR)/$(OVNKUBERNETES_RESOURCE_INJECTOR_HELM_CHART_NAME)-$(OVNKUBERNETES_RESOURCE_INJECTOR_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-dummydpuservice
helm-push-dummydpuservice: $(CHARTSDIR) helm ## Push helm chart for dummydpuservice
	$(HELM) push $(CHARTSDIR)/$(DUMMYDPUSERVICE_HELM_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)

.PHONY: helm-push-snap-csi-plugin
helm-push-snap-csi-plugin: $(CHARTSDIR) helm ## Push helm chart for snap-csi-plugin
	$(HELM) push $(CHARTSDIR)/$(SNAP_CSI_PLUGIN_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)

.PHONY: helm-push-snap-controller
helm-push-snap-controller: $(CHARTSDIR) helm ## Push helm chart for snap controller
	$(HELM) push $(CHARTSDIR)/$(SNAP_CONTROLLER_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)

.PHONY: helm-push-spdk-csi-controller
helm-push-spdk-csi-controller: $(CHARTSDIR) helm ## Push helm chart for spdk csi controller
	$(HELM) push $(CHARTSDIR)/$(SPDK_CSI_CONTROLLER_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)

.PHONY: helm-push-snap-dpu
helm-push-snap-dpu: $(CHARTSDIR) helm ## Push helm chart for snap dpu
	$(HELM) push $(CHARTSDIR)/$(SNAP_DPU_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)
