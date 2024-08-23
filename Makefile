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
OVNKUBERNETES_BASE_IMAGE=gitlab-master.nvidia.com:5005/doca-platform-foundation/doca-platform-foundation/ovn-kubernetes-base:ca01b0a7e924b17765df8145d8669611d513e3edb2ac6f3cd518d04b6d01de6e
# Points to branch dpf-23.09.0
OVN_REVISION=89fb67b6222d1e6a48fed3ae6d6ac486326c6ab2
# Points to branch dpf-4.14
OVNKUBERNETES_DPU_REVISION=33f3f2bd91cc68305a39add3dfdaa0142121b69e
# Points to branch dpf-4.14-non-dpu
OVNKUBERNETES_NON_DPU_REVISION=f73cdc1f764b36a1f4df9849e15f32eb0e0082c1

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Clean non-essential files from the repo
	@rm -rf $(CHARTSDIR)
	@rm -rf $(TOOLSDIR)
	@rm -rf $(REPOSDIR)

##@ Dependencies

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
OVS_CNI_REVISION ?= 89cfce926068419184c0bc547028397aebea4e30
OVS_CNI_DIR=$(REPOSDIR)/ovs-cni-$(OVS_CNI_REVISION)
$(OVS_CNI_DIR): | $(REPOSDIR)
	GITLAB_TOKEN=$(GITLAB_TOKEN) $(CURDIR)/hack/scripts/git-clone-repo.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/dpf-sfc-cni.git $(OVS_CNI_DIR) $(OVS_CNI_REVISION)

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

##@ Development
GENERATE_TARGETS ?= operator dpuservice provisioning hostcniprovisioner dpucniprovisioner servicechainset operator-embedded ovnkubernetes-operator ovnkubernetes-operator-embedded sfc-controller release-defaults hbn-dpuservice ovs-cni dummydpuservice

.PHONY: generate
generate: ## Run all generate-* targets: generate-modules generate-manifests-* and generate-go-deepcopy-*.
	$(MAKE) generate-mocks generate-modules generate-manifests generate-go-deepcopy generate-operator-bundle

.PHONY: generate-mocks
generate-mocks: $(MOCKGEN) ## Generate mocks
	## Add the TOOLSDIR to the path for this command as `mockgen` is called from the $PATH inline in the code.
	## See go:generate comments for examples.
	export PATH="$(PATH):$(TOOLSDIR)"; go generate ./...

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to update go modules
	go mod tidy

.PHONY: generate-manifests
generate-manifests: controller-gen kustomize $(addprefix generate-manifests-,$(GENERATE_TARGETS)) ## Run all generate-manifests-* targets

.PHONY: generate-manifests-operator
generate-manifests-operator: $(KUSTOMIZE) $(CONTROLLER_GEN) $(ENVSUBST) $(HELM) ## Generate manifests e.g. CRD, RBAC. for the operator controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./deploy/helm/dpf-operator/crds/"
	$(CONTROLLER_GEN) \
	paths="./cmd/operator/..." \
	paths="./internal/operator/..." \
	paths="./api/operator/..." \
	crd:crdVersions=v1 \
	rbac:roleName="dpf-operator-manager-role" \
	output:crd:dir=./deploy/helm/dpf-operator/crds \
	output:rbac:dir=./deploy/helm/dpf-operator/templates
	## Copy all other CRD definitions to the operator helm directory
	$(KUSTOMIZE) build config/operator-additional-crds -o  deploy/helm/dpf-operator/crds/;
	## Set the image name and tag in the operator helm chart values
	$(ENVSUBST) < deploy/helm/dpf-operator/values.yaml.tmpl > deploy/helm/dpf-operator/values.yaml
	## Update the helm dependencies for the chart.
	$(HELM) repo add argo https://argoproj.github.io/argo-helm
	$(HELM) repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts
	$(HELM) repo add prometheus https://prometheus-community.github.io/helm-charts
	$(HELM) repo add grafana https://grafana.github.io/helm-charts
	$(HELM) dependency build $(OPERATOR_HELM_CHART)


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
	output:rbac:dir=./config/dpuservice/rbac \
	output:webhook:dir=./config/dpuservice/webhook \
	webhook
	cd config/dpuservice/manager && $(KUSTOMIZE) edit set image controller=$(DPF_SYSTEM_IMAGE):$(TAG)

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
generate-manifests-operator-embedded: $(ENVSUBST) generate-manifests-dpuservice generate-manifests-provisioning generate-manifests-release-defaults ## Generates manifests that are embedded into the operator binary.
	$(KUSTOMIZE) build config/provisioning/default > $(EMBEDDED_MANIFESTS_DIR)/provisioning-controller.yaml
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

.PHONY: generate-manifests-servicechainset
generate-manifests-servicechainset: $(KUSTOMIZE) $(ENVSUBST) ## Generate manifests e.g. CRD, RBAC. for the servicechainset controller.
	$(MAKE) clean-generated-yaml SRC_DIRS="./config/servicechainset/crd/bases"
	$(CONTROLLER_GEN) \
	paths="./cmd/servicechainset/..." \
	paths="./internal/servicechainset/..." \
	paths="./internal/pod-ipam-injector/..." \
	paths="./api/servicechain/..." \
	crd:crdVersions=v1,generateEmbeddedObjectMeta=true \
	rbac:roleName=manager-role \
	output:crd:dir=./config/servicechainset/crd/bases \
	output:rbac:dir=./config/servicechainset/rbac
	cd config/servicechainset/manager && $(KUSTOMIZE) edit set image controller=$(DPF_SYSTEM_IMAGE):$(TAG)
	find config/servicechainset/crd/bases/ -type f -not -name '*dpu*' -exec cp {} deploy/helm/servicechain/crds/ \;
	# Template the image name and tag used in the helm templates.
	$(ENVSUBST) < deploy/helm/servicechain/values.yaml.tmpl > deploy/helm/servicechain/values.yaml

.PHONY: generate-manifests-sfc-controller
generate-manifests-sfc-controller: generate-manifests-servicechainset $(ENVSUBST)
	cp deploy/helm/servicechain/crds/sfc.dpf.nvidia.com_servicechains.yaml deploy/helm/sfc-controller/crds/
	cp deploy/helm/servicechain/crds/sfc.dpf.nvidia.com_serviceinterfaces.yaml deploy/helm/sfc-controller/crds/
	# Template the image name and tag used in the helm templates.
	$(ENVSUBST) < deploy/helm/sfc-controller/values.yaml.tmpl > deploy/helm/sfc-controller/values.yaml

.PHONY: generate-manifests-provisioning
generate-manifests-provisioning: $(KUSTOMIZE) $(ENVTEST) ## Generate manifests e.g. CRD, RBAC. for the DPF provisioning controller.
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
	cd config/provisioning/manager && $(KUSTOMIZE) edit set image controller=$(DPF_SYSTEM_IMAGE):$(TAG)

.PHONY: generate-manifests-hbn-dpuservice
generate-manifests-hbn-dpuservice: $(ENVSUBST)
	$(ENVSUBST) < deploy/dpuservices/hbn/chart/values.yaml.tmpl > deploy/dpuservices/hbn/chart/values.yaml

.PHONY: generate-manifests-ovs-cni
generate-manifests-ovs-cni: $(ENVSUBST) ## Generate values for OVS helm chart.
	$(ENVSUBST) < deploy/helm/ovs-cni/values.yaml.tmpl > deploy/helm/ovs-cni/values.yaml

.PHONY: generate-operator-bundle
generate-operator-bundle: $(OPERATOR_SDK) $(HELM) generate-manifests-operator ## Generate bundle manifests and metadata, then validate generated files.
	# First template the actual manifests to include using helm.
	mkdir -p hack/charts/dpf-operator/
	$(HELM) template --namespace $(OPERATOR_NAMESPACE) \
		--set image=$(DPF_SYSTEM_IMAGE):$(TAG) $(OPERATOR_HELM_CHART) \
		--set argo-cd.enabled=false \
		--set node-feature-discovery.enabled=false \
		--set kube-state-metrics.enabled=false \
		--set grafana.enabled=false \
		--set prometheus.enabled=false \
		--set templateOperatorBundle=true > hack/charts/dpf-operator/manifests.yaml
	# Next generate the operator bundle.
    # Note we need to explicitly set stdin to null using < /dev/null.
	$(OPERATOR_SDK) generate bundle \
	--overwrite --package dpf-operator --version $(BUNDLE_VERSION) --default-channel=$(BUNDLE_VERSION) --channels=$(BUNDLE_VERSION) \
	--deploy-dir hack/charts/dpf-operator --crds-dir deploy/helm/dpf-operator/crds 	</dev/null

	# We need to ensure operator-sdk receives nothing on stdin by explicitly redirecting null there.
	# Remove the createdAt field to prevent rebasing issues.
	# TODO: Currently the clusterserviceversion is not being correctly generated e.g. metadata is missing.
	# MacOS: We have to ensure that we are using gnu sed. Install gnu-sed via homebrew and put it somewhere in your PATH.
	#   e.g.: ln -s /opt/homebrew/bin/gsed $HOME/bin/sed
	$Q sed -i '/  createdAt:/d'  bundle/manifests/dpf-operator.clusterserviceversion.yaml
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: generate-manifests-dummydpuservice
generate-manifests-dummydpuservice: $(ENVSUBST) ## Generate values for dummydpuservice helm chart.
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

##@ Testing

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test $$(go list ./... | grep -v /e2e)

.PHONY: test-report $(GOTESTSUM)
test-report: envtest gotestsum ## Run tests and generate a junit style report
	set +o errexit; KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLSDIR) -p path)" go test -count 1 -race -json $$(go list ./... | grep -v /e2e) > junit.stdout; echo $$? > junit.exitcode;
	$(GOTESTSUM) --junitfile junit.xml --raw-command cat junit.stdout
	exit $$(cat junit.exitcode)

.PHONY: test-release-e2e-quick
test-release-e2e-quick: # Build images required for the quick DPF e2e test.
	# Build and push the dpuservice, provisioning, operator and operator-bundle images.
    # The quick test will only run on amd64 nodes.
	$(MAKE) docker-build-dpf-system-for-amd64 docker-push-dpf-system-for-amd64

	# Build and push all the helm charts
	$(MAKE) helm-package-all helm-push-all

TEST_CLUSTER_NAME := dpf-test
test-env-e2e: $(KAMAJI) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(MINIKUBE) $(ENVSUBST) ## Setup a Kubernetes environment to run tests.

	# Create a minikube cluster to host the test.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

	$(KUBECTL) get namespace dpf-operator-system || $(KUBECTL) create namespace dpf-operator-system

	# Create secrets required for using artefacts if required.
	$(CURDIR)/hack/scripts/create-artefact-secrets.sh

	# Deploy cert manager to provide certificates for webhooks.
	$Q $(KUBECTL) apply -f $(CERT_MANAGER_YAML)

	echo "Waiting for cert-manager deployment to be ready."
	$(KUBECTL) -n cert-manager rollout status deploy cert-manager-webhook --timeout=180s

	# Deploy Kamaji as the underlying control plane provider.
	$Q $(HELM) upgrade --set image.pullPolicy=IfNotPresent --set cfssl.image.tag=v1.6.5 --install kamaji $(KAMAJI)

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
test-deploy-operator-helm: $(HELM) ## Deploy the DPF Operator using helm
	$(HELM) upgrade --install --create-namespace --namespace $(OPERATOR_NAMESPACE) \
		--set image=$(DPF_SYSTEM_IMAGE):$(TAG) \
		--set imagePullSecrets[0].name=dpf-pull-secret \
		--set kube-state-metrics.enabled=$(DEPLOY_KSM) \
		--set grafana.enabled=$(DEPLOY_GRAFANA) \
		--set prometheus.enabled=$(DEPLOY_PROMETHEUS) \
		dpf-operator $(OPERATOR_HELM_CHART)

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

ARTIFACTS_DIR ?= $(shell pwd)/artifacts
.PHONY: test-cache-images
test-cache-images: $(MINIKUBE) ## Add images to the minikube cache based on the artifacts directory created in e2e.
	# Run a script which will cache images which were pulled in the test run to the minikube cache.
	CLUSTER_NAME=$(TEST_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) ARTIFACTS_DIR=${ARTIFACTS_DIR} $(CURDIR)/hack/scripts/add-images-to-minikube-cache.sh

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  ## Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e: ## Run e2e tests
	go test -timeout 0 ./test/e2e/ -v -ginkgo.v

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
lint-helm: $(HELM) lint-helm-servicechainset lint-helm-multus lint-helm-sriov-dp lint-helm-nvidia-k8s-ipam lint-helm-ovs-cni lint-helm-sfc-controller lint-helm-ovnkubernetes-operator lint-helm-dummydpuservice

.PHONY: lint-helm-servicechainset
lint-helm-servicechainset: $(HELM) ## Run helm lint for servicechainset chart
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

.PHONY: lint-helm-dummydpuservice
lint-helm-dummydpuservice: $(HELM) ## Run helm lint for dummydpuservice chart
	$Q $(HELM) lint $(DUMMYDPUSERVICE_HELM_CHART)

##@ Release

.PHONY: release
release: generate ## Build and push helm and container images for release.
	# Build multiarch images which will run on both DPUs and x86 hosts.
	$(MAKE) $(addprefix docker-build-,$(MULTI_ARCH_DOCKER_BUILD_TARGETS))
	# Build arm64 images which will run on DPUs.
	$(MAKE) ARCH=$(DPU_ARCH) $(addprefix docker-build-,$(DPU_ARCH_DOCKER_BUILD_TARGETS))
	# Build amd64 images which will run on x86 hosts.
	$(MAKE) ARCH=$(HOST_ARCH) $(addprefix docker-build-,$(HOST_ARCH_DOCKER_BUILD_TARGETS))
	# Push all of the images
	$(MAKE) docker-push-all

	# Package and push the helm charts.
	$(MAKE) helm-package-all helm-push-all

##@ Build

GO_GCFLAGS ?= ""
GO_LDFLAGS ?= "-extldflags '-static'"
BUILD_TARGETS ?= $(DPU_ARCH_BUILD_TARGETS) $(HOST_ARCH_BUILD_TARGETS)
DPF_SYSTEM_BUILD_TARGETS ?= operator provisioning dpuservice servicechainset
DPU_ARCH_BUILD_TARGETS ?= dpucniprovisioner ipallocator
HOST_ARCH_BUILD_TARGETS ?=  hostcniprovisioner ovnkubernetes-operator
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

.PHONY: binaries-dpf-system
binaries-dpf-system: $(addprefix binary-,$(DPF_SYSTEM_BUILD_TARGETS)) ## Build binaries for the dpf-system image.

.PHONY: binary-operator
binary-operator: ## Build the operator controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/operator gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/operator

.PHONY: binary-provisioning
binary-provisioning: ## Build the provisioning controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/provisioning gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/provisioning

.PHONY: binary-dpuservice
binary-dpuservice: ## Build the dpuservice controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpuservice gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/dpuservice

.PHONY: binary-servicechainset
binary-servicechainset: ## Build the servicechainset controller binary.
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/servicechainset gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/servicechainset

.PHONY: binary-dpucniprovisioner
binary-dpucniprovisioner: ## Build the DPU CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/dpucniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/dpucniprovisioner

.PHONY: binary-hostcniprovisioner
binary-hostcniprovisioner: ## Build the Host CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/hostcniprovisioner gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/hostcniprovisioner

.PHONY: binary-sfc-controller
binary-sfc-controller: ## Build the Host CNI Provisioner binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/sfc-controller gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/sfc-controller

.PHONY: binary-ovnkubernetes-operator
binary-binary-ovnkubernetes-operator: generate-manifests-ovnkubernetes-operator-embedded ## Build the OVN Kubernetes operator.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/ovnkubernetesoperator gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/ovnkubernetesoperator

.PHONY: binary-ipallocator
binary-ipallocator: ## Build the IP allocator binary.
	go build -ldflags=$(GO_LDFLAGS) -gcflags=$(GO_GCFLAGS) -trimpath -o $(LOCALBIN)/ipallocator gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/cmd/ipallocator

DOCKER_BUILD_TARGETS=$(HOST_ARCH_DOCKER_BUILD_TARGETS) $(DPU_ARCH_DOCKER_BUILD_TARGETS) $(MULTI_ARCH_DOCKER_BUILD_TARGETS)
HOST_ARCH_DOCKER_BUILD_TARGETS=$(HOST_ARCH_BUILD_TARGETS) ovnkubernetes-dpu ovnkubernetes-non-dpu operator-bundle hostnetwork dms  ovnkubernetes-operator
DPU_ARCH_DOCKER_BUILD_TARGETS=$(DPU_ARCH_BUILD_TARGETS) sfc-controller hbn hbn-sidecar ovs-cni ipallocator
MULTI_ARCH_DOCKER_BUILD_TARGETS= dpf-system

.PHONY: docker-build-all
docker-build-all: $(addprefix docker-build-,$(DOCKER_BUILD_TARGETS)) ## Build docker images for all DOCKER_BUILD_TARGETS. Architecture defaults to build system architecture unless overridden or hardcoded.

DPF_SYSTEM_IMAGE_NAME ?= dpf-system
export DPF_SYSTEM_IMAGE ?= $(REGISTRY)/$(DPF_SYSTEM_IMAGE_NAME)

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

SFC_CONTROLLER_IMAGE_NAME ?= sfc-controller-manager
export SFC_CONTROLLER_IMAGE ?= $(REGISTRY)/$(SFC_CONTROLLER_IMAGE_NAME)

OVS_CNI_IMAGE_NAME ?= ovs-cni-plugin
export OVS_CNI_IMAGE ?= $(REGISTRY)/$(OVS_CNI_IMAGE_NAME)

export HOSTNETWORK_IMAGE ?= $(REGISTRY)/hostnetworksetup
export DMS_IMAGE ?= $(REGISTRY)/dms-server

HOSTCNIPROVISIONER_IMAGE_NAME ?= host-cni-provisioner
HOSTCNIPROVISIONER_IMAGE ?= $(REGISTRY)/$(HOSTCNIPROVISIONER_IMAGE_NAME)

IPALLOCATOR_IMAGE_NAME ?= ip-allocator
IPALLOCATOR_IMAGE ?= $(REGISTRY)/$(IPALLOCATOR_IMAGE_NAME)

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
export HBN_TAG ?= $(HBN_NVCR_TAG)-dpf-$(TAG)

HBN_SIDECAR_IMAGE_NAME ?= hbn-sidecar
export HBN_SIDECAR_IMAGE ?= $(REGISTRY)/$(HBN_SIDECAR_IMAGE_NAME)

DUMMYDPUSERVICE_IMAGE_NAME ?= dummydpuservice
export DUMMYDPUSERVICE_IMAGE ?= $(REGISTRY)/$(DUMMYDPUSERVICE_IMAGE_NAME)

DPF_SYSTEM_ARCH ?= $(HOST_ARCH) $(DPU_ARCH)
.PHONY: docker-build-dpf-system # Build a multi-arch image for DPF System. The variable DPF_SYSTEM_ARCH defines which architectures this target builds for.
docker-build-dpf-system: $(addprefix docker-build-dpf-system-for-,$(DPF_SYSTEM_ARCH))

docker-build-dpf-system-for-%:
	# Provenance false ensures this target builds an image rather than a manifest when using buildx.
	docker build \
		--provenance=false \
		--platform=linux/$* \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		-f dpf-system.Dockerfile \
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
	$(MAKE) docker-create-manifest

docker-create-manifest:
	# Note: If you tag an image with multiple registries this push might fail. This can be fixed by pruning existing docker images.
	docker manifest create --amend $(DPF_SYSTEM_IMAGE):$(TAG) $(shell docker inspect --format='{{index .RepoDigests 0}}' $(DPF_SYSTEM_IMAGE):$(TAG))

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

.PHONY: docker-build-ipallocator
docker-build-ipallocator: ## Build docker image for the IP Allocator
	# Base image can't be distroless because of the readiness probe that is using cat which doesn't exist in distroless
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(ALPINE_IMAGE) \
		--build-arg target_arch=$(ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/ipallocator \
		. \
		-t $(IPALLOCATOR_IMAGE):$(TAG)

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
docker-build-hbn-sidecar: docker-build-base-image-ovs ## Build HBN sidecar DPU service image
	cd $(HBN_DPUSERVICE_DIR) && \
	docker buildx build \
		--load \
		--platform linux/${DPU_ARCH} \
		--build-arg base_image=$(OVS_BASE_IMAGE):$(TAG) \
		-f Dockerfile.sidecar \
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

.PHONY: docker-build-hostnetwork
docker-build-hostnetwork: ## Build docker image with the hostnetwork.
	docker build --platform linux/${HOST_ARCH} -t $(HOSTNETWORK_IMAGE):$(TAG) . -f Dockerfile.hostnetwork

.PHONY: docker-build-dms
docker-build-dms: ## Build docker image with the hostnetwork.
	docker build --platform linux/${HOST_ARCH} -t $(DMS_IMAGE):$(TAG) . -f Dockerfile.dms

.PHONY: docker-build-hbn
docker-build-hbn: ## Build docker image for HBN.
	## Note this image only ever builds for arm64.
	cd $(HBN_DPUSERVICE_DIR) && docker build --build-arg hbn_nvcr_tag=$(HBN_NVCR_TAG) -t $(HBN_IMAGE):$(HBN_TAG) . -f Dockerfile

.PHONY: docker-build-dummydpuservice
docker-build-dummydpuservice: ## Build docker images for the dummydpuservice
	docker build \
		--build-arg builder_image=$(BUILD_IMAGE) \
		--build-arg base_image=$(BASE_IMAGE) \
		--build-arg target_arch=$(DPU_ARCH) \
		--build-arg ldflags=$(GO_LDFLAGS) \
		--build-arg gcflags=$(GO_GCFLAGS) \
		--build-arg package=./cmd/dummydpuservice \
		. \
		-t $(DUMMYDPUSERVICE_IMAGE):$(TAG)

.PHONY: docker-push-all
docker-push-all: $(addprefix docker-push-,$(DOCKER_BUILD_TARGETS))  ## Push the docker images for all DOCKER_BUILD_TARGETS.

.PHONY: docker-push-dpf-system
docker-push-dpf-system: ## This is a no-op to allow using DOCKER_BUILD_TARGETS.

.PHONY: docker-push-sfc-controller
docker-push-sfc-controller:
	docker push $(SFC_CONTROLLER_IMAGE):$(TAG)

.PHONY: docker-push-hbn-sidecar
docker-push-hbn-sidecar: ## Push the docker image for HBN sidecar.
	docker push $(HBN_SIDECAR_IMAGE):$(TAG)

.PHONY: docker-push-ovs-cni
docker-push-ovs-cni: ## Push the docker image for ovs-cni
	docker push $(OVS_CNI_IMAGE):$(TAG)

.PHONY: docker-push-hostnetwork
docker-push-hostnetwork: ## Push the docker image for the hostnetwork.
	docker push $(HOSTNETWORK_IMAGE):$(TAG)

.PHONY: docker-push-dms
docker-push-dms: ## Push the docker image for DMS.
	docker push $(DMS_IMAGE):$(TAG)

.PHONY: docker-push-dpucniprovisioner
docker-push-dpucniprovisioner: ## Push the docker image for DPU CNI Provisioner.
	docker push $(DPUCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-push-hostcniprovisioner
docker-push-hostcniprovisioner: ## Push the docker image for Host CNI Provisioner.
	docker push $(HOSTCNIPROVISIONER_IMAGE):$(TAG)

.PHONY: docker-push-ipallocator
docker-push-ipallocator: ## Push the docker image for IP Allocator.
	docker push $(IPALLOCATOR_IMAGE):$(TAG)

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

.PHONY: docker-push-dummydpuservice
docker-push-dummydpuservice: ## Push the docker image for dummydpuservice
	docker push $(DUMMYDPUSERVICE_IMAGE):$(TAG)

# helm charts
HELM_TARGETS ?= servicechain-controller multus sriov-device-plugin flannel nvidia-k8s-ipam ovs-cni sfc-controller ovnkubernetes-operator operator hbn-dpuservice
HELM_REGISTRY ?= oci://$(REGISTRY)

# metadata for the operator helm chart
OPERATOR_HELM_CHART_NAME ?= dpf-operator
OPERATOR_HELM_CHART ?= $(HELMDIR)/$(OPERATOR_HELM_CHART_NAME)

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

# metadata for dummydpuservice.
DUMMYDPUSERVICE_HELM_CHART_NAME = dummydpuservice-chart
DUMMYDPUSERVICE_HELM_CHART ?= $(DPUSERVICESDIR)/dummydpuservice/chart

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

OPERATOR_CHART_TAGS ?=$(TAG)
.PHONY: helm-package-operator
helm-package-operator: $(CHARTSDIR) $(HELM) ## Package helm chart for DPF Operator
	for tag in $(OPERATOR_CHART_TAGS); do \
		$(HELM) package $(OPERATOR_HELM_CHART) --version $$tag --destination $(CHARTSDIR); \
	done

.PHONY: helm-package-ovnkubernetes-operator
helm-package-ovnkubernetes-operator: $(CHARTSDIR) $(HELM) ## Package helm chart for OVN Kubernetes Operator
	$(HELM) package $(DPFOVNKUBERNETESOPERATOR_HELM_CHART) --version $(DPFOVNKUBERNETESOPERATOR_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-hbn-dpuservice
helm-package-hbn-dpuservice: $(DPUSERVICESDIR) $(HELM) generate-manifests-hbn-dpuservice ## Package helm chart for HBN
	$(HELM) package $(HBN_HELM_CHART) --version $(HBN_HELM_CHART_VER) --destination $(CHARTSDIR)

.PHONY: helm-package-dummydpuservice
helm-package-dummydpuservice: $(DPUSERVICESDIR) $(HELM) generate-manifests-dummydpuservice ## Package helm chart for dummydpuservice
	$(HELM) package $(DUMMYDPUSERVICE_HELM_CHART) --version $(TAG) --destination $(CHARTSDIR)

.PHONY: helm-push-all
helm-push-all: $(addprefix helm-push-,$(HELM_TARGETS))  ## Push the helm charts for all components.

.PHONY: helm-push-operator
helm-push-operator: $(CHARTSDIR) ## Push helm chart for dpf-operator
	for tag in $(OPERATOR_CHART_TAGS); do \
		$(HELM) push  $(CHARTSDIR)/$(OPERATOR_HELM_CHART_NAME)-$$tag.tgz $(HELM_REGISTRY); \
	done

.PHONY: helm-push-servicechain-controller
helm-push-servicechain-controller: $(CHARTSDIR) ## Push helm chart for service chain controller
	$(HELM) push $(CHARTSDIR)/$(SERVICECHAIN_CONTROLLER_HELM_CHART_NAME)-chart-$(SERVICECHAIN_CONTROLLER_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-multus
helm-push-multus: $(CHARTSDIR) ## Push helm chart for multus CNI
	$(HELM) push $(CHARTSDIR)/$(MULTUS_HELM_CHART_NAME)-chart-$(MULTUS_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-sriov-device-plugin
helm-push-sriov-device-plugin: $(CHARTSDIR) ## Push helm chart for sriov-network-device-plugin
	$(HELM) push $(CHARTSDIR)/$(SRIOV_DP_HELM_CHART_NAME)-chart-$(SRIOV_DP_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-nvidia-k8s-ipam
helm-push-nvidia-k8s-ipam: $(CHARTSDIR) ## Push helm chart for nvidia-k8s-ipam
	$(HELM) push $(CHARTSDIR)/$(NVIDIA_K8S_IPAM_HELM_CHART_NAME)-chart-$(NVIDIA_K8S_IPAM_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-flannel
helm-push-flannel: $(CHARTSDIR) ## Push helm chart for flannel CNI
	$(HELM) push $(CHARTSDIR)/$(FLANNEL_HELM_CHART_NAME)-$(FLANNEL_VERSION).tgz $(HELM_REGISTRY)

.PHONY: helm-push-ovs-cni
helm-push-ovs-cni: $(CHARTSDIR) ## Push helm chart for OVS CNI
	$(HELM) push $(CHARTSDIR)/$(OVS_CNI_HELM_CHART_NAME)-chart-$(OVS_CNI_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-sfc-controller
helm-push-sfc-controller: $(CHARTSDIR) ## Push helm chart for sfc-controller
	$(HELM) push $(CHARTSDIR)/$(SFC_CONTOLLER_HELM_CHART_NAME)-chart-$(SFC_CONTOLLER_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-ovnkubernetes-operator
helm-push-ovnkubernetes-operator: $(CHARTSDIR) ## Push helm chart for DPF OVN Kubernetes Operator
	$(HELM) push $(CHARTSDIR)/$(DPFOVNKUBERNETESOPERATOR_HELM_CHART_NAME)-chart-$(DPFOVNKUBERNETESOPERATOR_HELM_CHART_VER).tgz $(HELM_REGISTRY)

.PHONY: helm-push-hbn-dpuservice
helm-push-hbn-dpuservice: $(CHARTSDIR) ## Push helm chart for HBN
	$(HELM) push $(CHARTSDIR)/$(HBN_HELM_CHART_NAME)-chart-$(TAG).tgz $(HELM_REGISTRY)

.PHONY: helm-push-dummydpuservice
helm-push-dummydpuservice: $(CHARTSDIR) ## Push helm chart for HBN
	$(HELM) push $(CHARTSDIR)/$(DUMMYDPUSERVICE_HELM_CHART_NAME)-$(TAG).tgz $(HELM_REGISTRY)

##@ Development Environment

DEV_CLUSTER_NAME ?= dpf-dev
dev-minikube: $(MINIKUBE) ## Create a minikube cluster for development.
	CLUSTER_NAME=$(DEV_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

clean-dev-env: $(MINIKUBE)
	$(MINIKUBE) delete -p $(DEV_CLUSTER_NAME)

clean-minikube: $(MINIKUBE)  ## Delete the development minikube cluster.
	$(MINIKUBE) delete -p $(DEV_CLUSTER_NAME)

dev-prereqs-dpuservice: $(KUSTOMIZE) $(CERT_MANAGER_YAML) $(ARGOCD_YAML) $(HELM) $(KAMAJI) ## Install pre-requisites for dpuservice controller on minikube dev cluster
	# Deploy the dpuservice CRD
	$(KUSTOMIZE) build config/dpuservice/crd | $(KUBECTL) apply -f -

    # Deploy cert manager to provide certificates for webhooks
	$Q $(KUBECTL) apply -f $(CERT_MANAGER_YAML) \
	&& echo "Waiting for cert-manager deployment to be ready." \
	&& $(KUBECTL) -n cert-manager rollout status deploy cert-manager-webhook --timeout=180s

	$Q $(KUBECTL) create namespace argocd --dry-run=client -o yaml | $(KUBECTL) apply -f - && $(KUBECTL) apply -f $(ARGOCD_YAML)

	$Q $(HELM) upgrade --set image.pullPolicy=IfNotPresent --set cfssl.image.tag=v1.6.5 --install kamaji $(KAMAJI)

SKAFFOLD_REGISTRY=localhost:5000
dev-dpuservice: $(MINIKUBE) $(SKAFFOLD) ## Deploy dpuservice controller to dev cluster using skaffold
	# Use minikube for docker build and deployment and run skaffold
	$(SKAFFOLD) debug -p dpuservice --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

dev-operator:  $(MINIKUBE) $(SKAFFOLD) generate-manifests-operator-embedded ## Deploy operator controller to dev cluster using skaffold
	$(SKAFFOLD) debug -p operator --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false --cleanup=false

dev-ovnkubernetes-operator:  $(MINIKUBE) $(SKAFFOLD) generate-manifests-ovnkubernetes-operator-embedded ## Deploy operator controller to dev cluster using skaffold
	# Ensure the manager's kustomization has the correct image name and has not been changed by generation.
	git restore config/ovnkubernetesoperator/manager/kustomization.yaml
	$(SKAFFOLD) debug -p ovnkubernetes-operator --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

