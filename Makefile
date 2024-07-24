NAMESPACE ?= hmc-system
VERSION ?= $(shell git describe --tags --always)
# Image URL to use all building/pushing image targets
IMG ?= hmc/controller:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0

os = $(shell uname|tr DL dl)
OS := $(strip ${os})

ARCH := $(shell uname -m)

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

.PHONY: hmc-chart-generate
hmc-chart-generate: kustomize helmify yq ## Generate hmc helm chart
	rm -rf templates/hmc/values.yaml templates/hmc/templates/*.yaml
	$(KUSTOMIZE) build config/default | $(HELMIFY) templates/hmc
	$(YQ) eval -iN '' templates/hmc/values.yaml config/default/hmc_values.yaml

.PHONY: set-hmc-version
set-hmc-version:
	$(YQ) eval '.version = "$(VERSION)"' -i templates/hmc/Chart.yaml
	$(YQ) eval '.version = "$(VERSION)"' -i templates/hmc-templates/Chart.yaml

.PHONY: hmc-chart-release
hmc-chart-release: kustomize helmify yq set-hmc-version templates-generate ## Generate hmc helm chart
	rm -rf templates/hmc/values.yaml templates/hmc/templates/*.yaml
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(HELMIFY) templates/hmc
	$(YQ) eval -iN '' templates/hmc/values.yaml config/default/hmc_values.yaml

.PHONY: hmc-dist-release
hmc-dist-release: $(HELM) $(YQ)
	@mkdir -p dist
	@printf "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: $(NAMESPACE)\n" > dist/install.yaml
	$(HELM) template -n $(NAMESPACE) hmc templates/hmc >> dist/install.yaml
	$(YQ) eval -i '.metadata.namespace = "hmc-system"' dist/install.yaml
	$(YQ) eval -i '.metadata.annotations."meta.helm.sh/release-name" = "hmc"' dist/install.yaml
	$(YQ) eval -i '.metadata.annotations."meta.helm.sh/release-namespace" = "hmc-system"' dist/install.yaml
	$(YQ) eval -i '.metadata.labels."app.kubernetes.io/managed-by" = "Helm"' dist/install.yaml

.PHONY: templates-generate
templates-generate:
	@hack/templates.sh

.PHONY: generate-all
generate-all: generate manifests hmc-chart-generate templates-generate add-license

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: test
test: generate-all fmt vet envtest tidy ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

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

.PHONY: add-license
add-license: addlicense
	$(ADDLICENSE) -c "" -ignore ".github/**" -ignore "config/**" -ignore "templates/**" -ignore ".*" -y 2024 .

##@ Build

TEMPLATES_DIR := templates
CHARTS_PACKAGE_DIR ?= $(LOCALBIN)/charts
$(CHARTS_PACKAGE_DIR): | $(LOCALBIN)
	rm -rf $(CHARTS_PACKAGE_DIR)
	mkdir -p $(CHARTS_PACKAGE_DIR)

CHARTS = $(patsubst $(TEMPLATES_DIR)/%,%,$(wildcard $(TEMPLATES_DIR)/*))

.PHONY: helm-package
helm-package: helm
	@make $(patsubst %,package-chart-%,$(CHARTS))

lint-chart-%:
	$(HELM) dependency update $(TEMPLATES_DIR)/$*
	$(HELM) lint --strict $(TEMPLATES_DIR)/$*

package-chart-%: $(CHARTS_PACKAGE_DIR) lint-chart-%
	$(HELM) package --destination $(CHARTS_PACKAGE_DIR) $(TEMPLATES_DIR)/$*

LD_FLAGS?= -s -w
LD_FLAGS += -X github.com/Mirantis/hmc/internal/build.Version=$(VERSION)
LD_FLAGS += -X github.com/Mirantis/hmc/internal/telemetry.segmentToken=$(SEGMENT_TOKEN)

.PHONY: build
build: generate-all fmt vet ## Build manager binary.
	go build -ldflags="${LD_FLAGS}" -o bin/manager cmd/main.go

.PHONY: run
run: generate-all fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build \
	-t ${IMG} \
	--build-arg LD_FLAGS="${LD_FLAGS}" \
	.

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
build-installer: generate-all kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

KIND_CLUSTER_NAME ?= hmc-dev
KIND_NETWORK ?= kind
REGISTRY_NAME ?= hmc-local-registry
REGISTRY_PORT ?= 5001
REGISTRY_REPO ?= oci://127.0.0.1:$(REGISTRY_PORT)/charts

AWS_CREDENTIALS=${AWS_B64ENCODED_CREDENTIALS}

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: kind-deploy
kind-deploy: kind
	@if ! $(KIND) get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		kind create cluster -n $(KIND_CLUSTER_NAME); \
	fi

.PHONY: kind-undeploy
kind-undeploy: kind
	@if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		kind delete cluster --name $(KIND_CLUSTER_NAME); \
	fi

.PHONY: registry-deploy
registry-deploy:
	@if [ ! "$$($(CONTAINER_TOOL) ps -aq -f name=$(REGISTRY_NAME))" ]; then \
		echo "Starting new local registry container $(REGISTRY_NAME)"; \
		$(CONTAINER_TOOL) run -d --restart=always -p "127.0.0.1:$(REGISTRY_PORT):5000" --network bridge --name "$(REGISTRY_NAME)" registry:2; \
	fi; \
	if [ "$$($(CONTAINER_TOOL) inspect -f='{{json .NetworkSettings.Networks.$(KIND_NETWORK)}}' $(REGISTRY_NAME))" = 'null' ]; then \
		$(CONTAINER_TOOL) network connect $(KIND_NETWORK) $(REGISTRY_NAME); \
	fi

.PHONY: registry-undeploy
registry-undeploy:
	@if [ "$$($(CONTAINER_TOOL) ps -aq -f name=$(REGISTRY_NAME))" ]; then \
  		echo "Removing local registry container $(REGISTRY_NAME)"; \
		$(CONTAINER_TOOL) rm -f "$(REGISTRY_NAME)"; \
	fi

.PHONY: hmc-deploy
hmc-deploy: helm
	$(HELM) dependency update templates/hmc
	$(HELM) upgrade --values $(HMC_VALUES) --install --create-namespace hmc templates/hmc -n $(NAMESPACE)

.PHONY: deploy
deploy: generate-all kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: dev-deploy
dev-deploy: hmc-chart-generate ## Deploy HMC helm chart to the K8s cluster specified in ~/.kube/config.
	make hmc-deploy HMC_VALUES=config/dev/hmc_values.yaml
	$(KUBECTL) rollout restart -n $(NAMESPACE) deployment/hmc-controller-manager

.PHONY: dev-undeploy
dev-undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/dev | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: helm-push
helm-push: helm-package
	@for chart in $(CHARTS_PACKAGE_DIR)/*.tgz; do \
		base=$$(basename $$chart .tgz); \
		chart_version=$$(echo $$base | grep -o "v\{0,1\}[0-9]\+\.[0-9]\+\.[0-9].*"); \
		chart_name="$${base%-"$$chart_version"}"; \
		echo "Verifying if chart $$chart_name, version $$chart_version already exists in $(REGISTRY_REPO)"; \
		chart_exists=$$($(HELM) pull $(REGISTRY_REPO)/$$chart_name --version $$chart_version --destination /tmp 2>&1 | grep "not found" || true); \
		if [ -z "$$chart_exists" ]; then \
			echo "Chart $$chart_name version $$chart_version already exists in the repository."; \
		else \
			echo "Pushing $$chart to $(REGISTRY_REPO)"; \
			$(HELM) push "$$chart" $(REGISTRY_REPO); \
		fi; \
	done

.PHONY: dev-push
dev-push: docker-build helm-push
	$(KIND) load docker-image $(IMG) -n $(KIND_CLUSTER_NAME)

.PHONY: dev-templates
dev-templates: templates-generate
	$(KUBECTL) -n $(NAMESPACE) apply -f templates/hmc-templates/files/templates

.PHONY: dev-management
dev-management: yq
	$(YQ) '.spec.core.hmc.config += (load("config/dev/hmc_values.yaml"))' config/dev/management.yaml | $(KUBECTL) -n $(NAMESPACE) apply -f -

.PHONY: dev-aws
dev-aws: yq
	@$(YQ) e ".data.credentials = \"${AWS_CREDENTIALS}\"" config/dev/awscredentials.yaml | $(KUBECTL) -n $(NAMESPACE) apply -f -

.PHONY: dev-apply
dev-apply: kind-deploy registry-deploy dev-push dev-deploy dev-templates dev-management dev-aws

.PHONY: dev-destroy
dev-destroy: kind-undeploy registry-undeploy

.PHONY: dev-aws-apply
dev-aws-apply:
	$(KUBECTL) -n $(NAMESPACE) apply -f config/dev/deployment.yaml

.PHONY: dev-aws-destroy
dev-aws-destroy:
	$(KUBECTL) -n $(NAMESPACE) delete -f config/dev/deployment.yaml

.PHONY: cli-install
cli-install: clusterawsadm clusterctl

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
HELM ?= $(LOCALBIN)/helm-$(HELM_VERSION)
HELMIFY ?= $(LOCALBIN)/helmify-$(HELMIFY_VERSION)
KIND ?= $(LOCALBIN)/kind-$(KIND_VERSION)
YQ ?= $(LOCALBIN)/yq-$(YQ_VERSION)
CLUSTERAWSADM ?= $(LOCALBIN)/clusterawsadm
CLUSTERCTL ?= $(LOCALBIN)/clusterctl
ADDLICENSE ?= $(LOCALBIN)/addlicense-$(ADDLICENSE_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= release-0.17
GOLANGCI_LINT_VERSION ?= v1.57.2
HELM_VERSION ?= v3.15.1
HELMIFY_VERSION ?= v0.4.13
KIND_VERSION ?= v0.23.0
YQ_VERSION ?= v4.44.2
CLUSTERAWSADM_VERSION ?= v2.5.2
CLUSTERCTL_VERSION ?= v1.7.3
ADDLICENSE_VERSION ?= v1.1.1

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): | $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): | $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): | $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): | $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
HELM_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3"
$(HELM): | $(LOCALBIN)
	rm -f $(LOCALBIN)/helm-*
	curl -s $(HELM_INSTALL_SCRIPT) | USE_SUDO=false HELM_INSTALL_DIR=$(LOCALBIN) DESIRED_VERSION=$(HELM_VERSION) BINARY_NAME=helm-$(HELM_VERSION) PATH="$(LOCALBIN):$(PATH)" bash

.PHONY: helmify
helmify: $(HELMIFY) ## Download helmify locally if necessary.
$(HELMIFY): | $(LOCALBIN)
	$(call go-install-tool,$(HELMIFY),github.com/arttor/helmify/cmd/helmify,${HELMIFY_VERSION})

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): | $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,${KIND_VERSION})

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): | $(LOCALBIN)
	$(call go-install-tool,$(YQ),github.com/mikefarah/yq/v4,${YQ_VERSION})

.PHONY: clusterawsadm
clusterawsadm: $(CLUSTERAWSADM) ## Download clusterawsadm locally if necessary.
$(CLUSTERAWSADM): | $(LOCALBIN)
	curl -sL https://github.com/kubernetes-sigs/cluster-api-provider-aws/releases/download/$(CLUSTERAWSADM_VERSION)/clusterawsadm_$(CLUSTERAWSADM_VERSION)_$(OS)_$(ARCH) -o $(CLUSTERAWSADM)
	chmod +x $(CLUSTERAWSADM)

.PHONY: clusterctl
clusterctl: $(CLUSTERCTL) ## Download clusterctl locally if necessary.
$(CLUSTERCTL): | $(LOCALBIN)
	$(call go-install-tool,$(CLUSTERCTL),sigs.k8s.io/cluster-api/cmd/clusterctl,${CLUSTERCTL_VERSION})

.PHONY: addlicense
addlicense: $(ADDLICENSE) ## Download addlicense locally if necessary.
$(ADDLICENSE): | $(LOCALBIN)
	$(call go-install-tool,$(ADDLICENSE),github.com/google/addlicense,${ADDLICENSE_VERSION})

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
