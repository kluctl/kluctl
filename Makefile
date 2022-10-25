# Based on the work of Thomas Poignant (thomaspoignant)
# https://gist.github.com/thomaspoignant/5b72d579bd5f311904d973652180c705
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

EXE=
ifeq ($(GOOS), windows)
EXE=.exe
endif

RACE=
ifeq ($(GOOS), linux)
RACE=-race
endif

GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
BINARY_NAME=kluctl$(EXE)
TEST_BINARY_NAME=kluctl-e2e$(EXE)
EXPORT_RESULT?=false

# If gobin not set, create one on ./build and add to path.
ifeq (,$(shell go env GOBIN))
GOBIN=$(BUILD_DIR)/gobin
else
GOBIN=$(shell go env GOBIN)
endif
export PATH:=$(GOBIN):${PATH}

# Architecture to use envtest with
ENVTEST_ARCH ?= amd64

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: all test build

all: help

## Build:
build: build-go ## Run the complete build pipeline

build-go:  ## Build your project and put the output binary in ./bin/
	mkdir -p ./bin
	$(GOCMD) build -o ./bin/$(BINARY_NAME)

clean: ## Remove build related file
	rm -fr ./bin
	rm -fr ./out
	rm -fr ./reports
	rm -fr ./download-python

## Envtest setup
# Repository root based on Git metadata
REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)
BUILD_DIR := $(REPOSITORY_ROOT)/build
ENVTEST = $(GOBIN)/setup-envtest
.PHONY: envtest
setup-envtest: ## Download envtest-setup locally if necessary.
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# Download the envtest binaries to testbin
ENVTEST_KUBERNETES_VERSION?=1.25.0
ENVTEST_ASSETS_DIR=$(BUILD_DIR)/testbin

ifeq ($(OS),Windows_NT)
ENVTEST_ASSETS_DIR_WINDOWS=$(ENVTEST_ASSETS_DIR)/k8s/$(ENVTEST_KUBERNETES_VERSION)-windows-amd64
install-envtest: setup-envtest $(ENVTEST_ASSETS_DIR_WINDOWS)/etcd.exe
$(ENVTEST_ASSETS_DIR_WINDOWS)/etcd.exe:
	mkdir -p $(ENVTEST_ASSETS_DIR_WINDOWS)
	curl -o $(ENVTEST_ASSETS_DIR_WINDOWS)/kubebuilder-tools.tar.gz "https://raw.githubusercontent.com/kluctl/kubebuilder-tools-releases-windows/main/releases/kubebuilder-tools-$(ENVTEST_KUBERNETES_VERSION)_windows_amd64.tar.gz"
	cd $(ENVTEST_ASSETS_DIR_WINDOWS) && tar xzf kubebuilder-tools.tar.gz && mv kubebuilder/bin/* .
	rm $(ENVTEST_ASSETS_DIR_WINDOWS)/kubebuilder-tools.tar.gz
else
install-envtest: setup-envtest
	$(ENVTEST) use $(ENVTEST_KUBERNETES_VERSION) --arch=$(ENVTEST_ARCH) --bin-dir=$(ENVTEST_ASSETS_DIR)
endif

## Test:
test: test-unit test-e2e ## Runs the complete test suite

KUBEBUILDER_ASSETS?="$(shell $(ENVTEST) --arch=$(ENVTEST_ARCH) use -i $(ENVTEST_KUBERNETES_VERSION) --bin-dir=$(ENVTEST_ASSETS_DIR) -p path)"
test-e2e: test-e2e-build test-e2e-pre-built ## Runs the end to end tests

test-e2e-build: ## Builds the end to end tests
	$(GOCMD) test $(RACE) -c ./e2e -o ./bin/$(TEST_BINARY_NAME)

test-e2e-pre-built: install-envtest
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) ./bin/$(TEST_BINARY_NAME) -test.v

test-unit: ## Run the unit tests of the project
ifeq ($(EXPORT_RESULT), true)
	mkdir -p reports/test-unit
	GO111MODULE=off $(GOCMD) get -u github.com/jstemmer/go-junit-report
	$(eval OUTPUT_OPTIONS = | tee /dev/tty | go-junit-report -set-exit-code > reports/test-unit/junit-report.xml)
endif
	$(GOTEST) -v $(RACE) $(shell go list ./... | grep -v 'v2/e2e') $(OUTPUT_OPTIONS)

coverage-unit: ## Run the unit tests of the project and export the coverage
	$(GOTEST) -cover -covermode=count -coverprofile=reports/coverage-unit/profile.cov $(shell go list ./... | grep -v /e2e/)
	$(GOCMD) tool cover -func profile.cov
ifeq ($(EXPORT_RESULT), true)
	mkdir -p reports/coverage-unit
	GO111MODULE=off $(GOCMD) get -u github.com/AlekSi/gocov-xml
	GO111MODULE=off $(GOCMD) get -u github.com/axw/gocov/gocov
	gocov convert  reports/coverage-unit/profile.cov | gocov-xml > reports/coverage-unit/coverage.xml
endif

## Lint:
lint: lint-go ## Run all available linters

lint-go: ## Use golintci-lint on your project
ifeq ($(EXPORT_RESULT), true)
	mkdir -p reports/lint-go
	$(eval OUTPUT_OPTIONS = $(shell echo "--out-format checkstyle ./... | tee /dev/tty > reports/lint-go/checkstyle-report.xml" ))
else
	$(eval OUTPUT_OPTIONS = $(shell echo ""))
endif
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:latest-alpine golangci-lint run $(OUTPUT_OPTIONS)

replace-commands-help: ## Replace commands help in docs
	$(GOCMD) run ./internal/replace-commands-help --docs-dir ./docs/reference/commands

markdown-link-check: ## Check markdown files for dead links
	find . -name '*.md' | xargs docker run -v ${PWD}:/tmp:ro --rm -i -w /tmp ghcr.io/tcort/markdown-link-check:stable

## Release:
version: ## Write next version into version file
	$(GOCMD) install github.com/bvieira/sv4git/v2/cmd/git-sv@v2.7.0
	$(eval KLUCTL_VERSION:=$(shell git sv next-version))
	sed -ibak "s/0.0.0/$(KLUCTL_VERSION)/g" pkg/version/version.go

changelog: ## Generating changelog
	git sv changelog -n 1 > CHANGELOG.md

## Help:
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[0-9a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)

## Tools
# go-install-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(GOBIN) go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
