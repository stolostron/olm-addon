IMAGE ?= olm-addon-controller
CLEANER_IMAGE ?= olm-addon-cleaner
IMAGE_REGISTRY ?= quay.io/fgiloux
IMAGE_TAG ?= latest
IMG ?= $(IMAGE_REGISTRY)/$(IMAGE):$(IMAGE_TAG)
CLEANER_IMG ?= $(IMAGE_REGISTRY)/$(CLEANER_IMAGE):$(IMAGE_TAG)

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

# Helper software versions
GOLANGCI_VERSION := v1.50.0

.PHONY: build
build: ## Build the project binaries
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 go build $(BUILDFLAGS) -o bin/olm-addon-controller

.PHONY: docker-build
docker-build: ## Build docker image
	docker build -t ${IMG} .
	docker build -t ${CLEANER_IMG} -f cleaner-Dockerfile

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}
	docker push ${CLEANER_IMG}

.PHONY: deploy
deploy:
	kubectl apply -k deploy

## Location to install dependencies to
LOCALBIN ?= $(PROJECT_DIR)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: lint
lint: golangci-lint ## Lint source code
	$(GOLANGCILINT) run --timeout 4m0s ./...

.PHONY: golangci-lint
GOLANGCILINT := $(LOCALBIN)/golangci-lint
GOLANGCI_URL := https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh
golangci-lint: $(GOLANGCILINT) ## Download golangci-lint
$(GOLANGCILINT): $(LOCALBIN)
	curl -sSfL $(GOLANGCI_URL) | sh -s -- -b $(LOCALBIN) $(GOLANGCI_VERSION)

.PHONY: unit
TEST_PACKAGES ?= ./pkg/...
unit: ## Run unit tests
	go test $(TEST_PACKAGES)

.PHONY: e2e
E2E_PACKAGES ?= ./test/e2e/...
e2e: ## Run e2e tests
	go test $(E2E_PACKAGES)

