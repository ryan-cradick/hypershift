PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
GOLANGCI_LINT = go run ${PROJECT_DIR}/vendor/github.com/golangci/golangci-lint/cmd/golangci-lint

VERSION     ?= $(shell git describe --always --abbrev=7)

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

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: vendor
vendor: ## Ensure the vendor directory is up to date.
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: lint
lint: ## Run golangci-lint over the codebase.
	${GOLANGCI_LINT} run ./... --timeout 5m -v

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Run golangci-lint over the codebase and run auto-fixers if supported by the linter
	${GOLANGCI_LINT} run ./... --timeout 5m -v --fix

.PHONY: test
test: fmt vet unit ## Run tests.

.PHONY: unit
unit: ## Run unit tests.
	go test ./...

##@ Build

.PHONY: build
build: ## Build the golangci-lint custom plugin binary.
	${GOLANGCI_LINT} custom

.PHONY: verify-%
verify-%:
	make $*
	git diff --exit-code
