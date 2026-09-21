.DEFAULT_GOAL := help

# Terraform discovers locally installed providers by registry address, version, and platform.
REGISTRY := registry.terraform.io
NAMESPACE := f5networks
NAME := f5os
VERSION ?= 0.0.1
INSTALL_VERSION := $(patsubst v%,%,$(VERSION))
BINARY_NAME := terraform-provider-$(NAME)
OUTPUT_DIR := bin
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
INSTALL_DIR := $(HOME)/.terraform.d/plugins/$(REGISTRY)/$(NAMESPACE)/$(NAME)/$(INSTALL_VERSION)/$(GOOS)_$(GOARCH)

help: ## Show this help menu
	@printf "Usage: make <target>\n\n"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "%-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the local provider binary
	@mkdir -p $(OUTPUT_DIR)
	go build -v -ldflags "-X main.version=$(INSTALL_VERSION)" -o $(OUTPUT_DIR)/$(BINARY_NAME) .

install: build ## Install the provider for local Terraform discovery
	@mkdir -p $(INSTALL_DIR)
	cp $(OUTPUT_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)_v$(INSTALL_VERSION)
	@echo "Installed $(BINARY_NAME) v$(INSTALL_VERSION) to $(INSTALL_DIR)"

uninstall: ## Remove the locally installed provider
	@case "$(INSTALL_DIR)" in \
		"$(HOME)/.terraform.d/plugins/"*) rm -rf "$(INSTALL_DIR)" ;; \
		*) echo "Refusing to remove unsafe path: $(INSTALL_DIR)"; exit 1 ;; \
	esac

# See https://golangci-lint.run/
lint: ## Run static analysis
	golangci-lint run --timeout=2m

generate: ## Generate documentation and examples
	go generate ./...

fmt: ## Format provider source code
	gofmt -s -w ./internal

test: ## Run unit tests
	go test -v -covermode=count -coverprofile cover.out -timeout=3600s -parallel=4 ./...

testacc: ## Run acceptance tests
	TF_ACC=1 go test -v -parallel=1 -cover -timeout 120m ./...

clean: ## Remove build artifacts and test cache
	go clean -testcache
	rm -rf $(OUTPUT_DIR)

.PHONY: help build install uninstall lint generate fmt test testacc clean
