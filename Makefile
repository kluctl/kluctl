# Based on the work of Thomas Poignant (thomaspoignant)
# https://gist.github.com/thomaspoignant/5b72d579bd5f311904d973652180c705
GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
BINARY_NAME=kluctl
TEST_BINARY_NAME=kluctl-e2e
REQUIRED_ENV_VARS=GOOS GOARCH
EXPORT_RESULT?=false

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: all test build vendor check-kubectl check-helm check-kind

all: help

## Check:

check-kubectl: ## Checks if kubectl is installed
	kubectl version --client=true

check-helm: ## Checks if helm is installed
	helm version

check-kind: ## Checks if kind is installed
	kind version

## Build:
build: vendor python generate build-go ## Run the complete build pipeline

build-go:  ## Build your project and put the output binary in out/bin/
	mkdir -p out/bin
	CGO_ENBALED=0 GO111MODULE=on $(GOCMD) build -mod vendor -o out/bin/$(BINARY_NAME) ./cmd/kluctl

clean: ## Remove build related file
	rm -fr ./bin
	rm -fr ./out
	rm -fr ./reports
	rm -fr ./download-python

vendor: ## Copy of all packages needed to support builds and tests in the vendor directory
	$(GOCMD) mod vendor

python:  ## Download python for Jinja2 support
	./hack/download-python.sh linux
	./hack/download-python.sh windows
	./hack/download-python.sh darwin

generate: ## Generating Jinja2 support
	$(GOCMD) generate ./...
	$(GOCMD) generate ./pkg/python

## Test:
test: test-unit test-e2e ## Runs the complete test suite

test-e2e: check-kubectl check-helm check-kind ## Runs the end to end tests
	CGO_ENBALED=0 GO111MODULE=on $(GOCMD) test -o out/bin/$(TEST_BINARY_NAME) ./e2e

test-unit: ## Run the unit tests of the project
ifeq ($(EXPORT_RESULT), true)
	mkdir -p reports/test-unit
	GO111MODULE=off $(GOCMD) get -u github.com/jstemmer/go-junit-report
	$(eval OUTPUT_OPTIONS = | tee /dev/tty | go-junit-report -set-exit-code > reports/test-unit/junit-report.xml)
endif
	$(GOTEST) -v -race $(shell go list ./... | grep -v /e2e/ | grep -v /vendor/) $(OUTPUT_OPTIONS)

coverage-unit: ## Run the unit tests of the project and export the coverage
	$(GOTEST) -cover -covermode=count -coverprofile=reports/coverage-unit/profile.cov $(shell go list ./... | grep -v /e2e/ | grep -v /vendor/)
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