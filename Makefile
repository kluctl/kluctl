# Based on the work of Thomas Poignant (thomaspoignant)
# https://gist.github.com/thomaspoignant/5b72d579bd5f311904d973652180c705
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

EXE=
ifeq ($(GOOS), windows)
EXE=.exe
endif

RACE=
ifneq ($(GOOS), windows)
RACE=-race
endif

PATHCONF=cat
ifeq ($(GOOS), windows)
PATHCONF=cygpath -f- -u
endif

# Image URL to use all building/pushing image targets
IMG ?= kluctl/kluctl:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.0

# This is the last version that supported go 1.22. We can switch back to latest when we don't need to support 1.22 anymore.
SETUP_ENVTEST_VERSION = v0.0.0-20241011141221-469837099f73

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
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
manifests: controller-gen kustomize ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=kluctl-controller-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	@echo "# Warning, this file is generated via \"make manifests\", don't edit it directly but instead change the files in config/crd" > install/controller/controller/crd.yaml
	$(KUSTOMIZE) build config/crd >> install/controller/controller/crd.yaml
	@echo "# Warning, this file is generated via \"make manifests\", don't edit it directly but instead change the files in config/rbac" > install/controller/controller/rbac.yaml
	$(KUSTOMIZE) build config/rbac >> install/controller/controller/rbac.yaml
	@echo "# Warning, this file is generated via \"make manifests\", don't edit it directly but instead change the files in config/manager" > install/controller/controller/manager.yaml
	$(KUSTOMIZE) build config/manager >> install/controller/controller/manager.yaml

# Generate API reference documentation
api-docs: gen-crd-api-reference-docs
	$(GEN_CRD_API_REFERENCE_DOCS) -v=4 -api-dir=./api/v1beta1 -config=./hack/api-docs/config.json -template-dir=./hack/api-docs/template -out-file=./docs/gitops/api/kluctl-controller.md

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	go generate ./...

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate test-unit test-lib test-e2e fmt vet ## Run all tests.

.PHONY: test-unit
test-unit: envtest ## Run unit tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir=$(LOCALBIN) -p path | $(PATHCONF))" go test $(RACE) $(shell go list ./... | grep -v v2/e2e) -coverprofile cover.out -test.v

.PHONY: test-lib
test-lib: ## Run lib tests.
	cd lib && go test $(RACE) ./...  -coverprofile cover.out -test.v

.PHONY: test-e2e
test-e2e: envtest ## Run all e2e tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir=$(LOCALBIN) -p path | $(PATHCONF))" go test $(RACE) ./e2e -timeout 15m -coverprofile cover.out -test.v

.PHONY: test-e2e-non-gitops
test-e2e-non-gitops: envtest ## Run non-gitops e2e tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir=$(LOCALBIN) -p path | $(PATHCONF))" go test $(RACE) ./e2e -timeout 15m -coverprofile cover.out -test.v -skip 'TestGitOps.*'

.PHONY: test-e2e-gitops
test-e2e-gitops: envtest ## Run gitops e2e tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir=$(LOCALBIN) -p path | $(PATHCONF))" go test $(RACE) ./e2e -timeout 15m -coverprofile cover.out -test.v -run 'TestGitOps.*'

replace-commands-help: ## Replace commands help in docs
	go run ./internal/replace-commands-help --docs-dir ./docs/kluctl/commands

LYCHEE_VERSION=0.14
markdown-link-check: ## Check markdown files for dead links
	docker run --init -i --rm -w /input -v ${PWD}:/input:ro lycheeverse/lychee:$(LYCHEE_VERSION) -- README.md docs install

##@ Build

.PHONY: build
build: manifests generate fmt vet build-webui build-bin ## Build manager binary.

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: build-bin
build-bin:
	go build -o bin/kluctl$(EXE) cmd/main.go

.PHONY: build-webui
build-webui:
	cd pkg/webui/ui && npm ci && npm run build

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: deploy-kind
deploy-kind: manifests kustomize
	GOOS=linux GOARCH=amd64 make build
	docker build -t kluctl-kind:latest --build-arg BIN_PATH=./bin/kluctl .
	kind load docker-image kluctl-kind:latest
	cd config/manager && $(KUSTOMIZE) edit set image controller=kluctl-kind
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.3
CONTROLLER_TOOLS_VERSION ?= v0.14.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) --output install_kustomize.sh && bash install_kustomize.sh $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); rm install_kustomize.sh; }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

# Find or download gen-crd-api-reference-docs
GEN_CRD_API_REFERENCE_DOCS = $(LOCALBIN)/gen-crd-api-reference-docs
.PHONY: gen-crd-api-reference-docs
gen-crd-api-reference-docs:
	GOBIN=$(LOCALBIN) go install github.com/ahmetb/gen-crd-api-reference-docs@v0.3.0

.PHONY: envtest
envtest: $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)
