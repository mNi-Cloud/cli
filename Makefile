# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN = $(shell go env GOPATH)/bin
else
GOBIN = $(shell go env GOBIN)
endif

## Location to write build output to
LOCALBIN ?= $(shell pwd)/bin

BINARY = mni

# urfave/cli only offers --version once the string is not empty, so a build that
# does not stamp this in ships a binary that cannot say what it is.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo unknown)
LDFLAGS = -X main.version=$(VERSION)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'.

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

.PHONY: lint
lint: ## Run golangci-lint linter.
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint linter and perform fixes.
	golangci-lint run --fix

##@ Build

.PHONY: build
build: fmt vet ## Build the mni binary into bin/.
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o "$(LOCALBIN)/$(BINARY)" ./cmd

.PHONY: install
install: ## Build the mni binary into $GOBIN.
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o "$(GOBIN)/$(BINARY)" ./cmd

.PHONY: clean
clean: ## Remove what a build leaves behind.
	rm -rf "$(LOCALBIN)" cover.out result
