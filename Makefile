# Kunpeng Cloud Computing - Top Level Makefile
# This Makefile manages all sub-projects in the repository

# Project configuration
PROJECT_NAME := kunpeng-cloud-computing
VERSION ?= 0.1.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse HEAD)

# Sub-projects
SUBPROJECTS := k8s-mpam-controller kunpeng-tap

# Build directories
BUILD_DIR := build
DIST_DIR := dist

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@echo "Kunpeng Cloud Computing - Multi-Project Build System"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""
	@echo "Available sub-projects: $(SUBPROJECTS)"
	@echo ""
	@echo "Sub-project specific commands:"
	@echo "  make <project>-<command>    Run command for specific project"
	@echo "  Examples:"
	@echo "    make kunpeng-tap-build    Build kunpeng-tap project"
	@echo "    make mpam-build           Build k8s-mpam-controller project"
	@echo "    make kunpeng-tap-test     Test kunpeng-tap project"

.PHONY: version
version: ## Show version information.
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

##@ Development

.PHONY: fmt
fmt: ## Format code for all projects.
	@echo "Formatting all projects..."
	$(MAKE) -C Boostkit_CloudNative/K8S -f Makefile.kunpeng-tap fmt
	@echo "Code formatting completed"

.PHONY: vet
vet: ## Run go vet for all projects.
	@echo "Running go vet for all projects..."
	$(MAKE) -C Boostkit_CloudNative/K8S -f Makefile.kunpeng-tap vet
	@echo "Go vet completed"

.PHONY: test
test: ## Run tests for all projects.
	@echo "Running tests for all projects..."
	$(MAKE) -C Boostkit_CloudNative/K8S -f Makefile.kunpeng-tap test
	@echo "All tests completed"

.PHONY: tidy
tidy: ## Tidy go modules for all projects.
	@echo "Tidying go modules for all projects..."
	$(MAKE) -C Boostkit_CloudNative/K8S -f Makefile.kunpeng-tap tidy
	@echo "Go mod tidy completed"

##@ Build

.PHONY: build
build: ## Build all projects.
	@echo "Building all projects..."
	@mkdir -p $(BUILD_DIR)
	$(MAKE) -C Boostkit_CloudNative/K8S mpam-build
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-build
	@echo "All projects built successfully"

.PHONY: clean
clean: ## Clean build artifacts for all projects.
	@echo "Cleaning all projects..."
	$(MAKE) -C Boostkit_CloudNative/K8S mpam-clean
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-clean
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	@echo "Clean completed"

##@ K8S MPAM Controller

.PHONY: mpam-build
mpam-build: ## Build k8s-mpam-controller project.
	@echo "Building k8s-mpam-controller..."
	$(MAKE) -C Boostkit_CloudNative/K8S mpam-build

.PHONY: mpam-build-local
mpam-build-local: ## Build k8s-mpam-controller project.
	@echo "Building k8s-mpam-controller locally..."
	$(MAKE) -C Boostkit_CloudNative/K8S mpam-build-local

.PHONY: mpam-clean
mpam-clean: ## Clean k8s-mpam-controller build artifacts.
	@echo "Cleaning k8s-mpam-controller..."
	$(MAKE) -C Boostkit_CloudNative/K8S mpam-clean

.PHONY: mpam-docker
mpam-docker: ## Build k8s-mpam-controller docker image.
	@echo "Building k8s-mpam-controller docker image..."
	$(MAKE) -C Boostkit_CloudNative/K8S mpam-docker

##@ Kunpeng TAP

.PHONY: kunpeng-tap-build
kunpeng-tap-build: ## Build kunpeng-tap project.
	@echo "Building kunpeng-tap..."
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-build

.PHONY: kunpeng-tap-build-manager
kunpeng-tap-build-manager: ## Build kunpeng-tap manager.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-build-manager

.PHONY: kunpeng-tap-build-proxy
kunpeng-tap-build-proxy: ## Build kunpeng-tap proxy.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-build-proxy

.PHONY: kunpeng-tap-test
kunpeng-tap-test: ## Test kunpeng-tap project.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-test

.PHONY: kunpeng-tap-clean
kunpeng-tap-clean: ## Clean kunpeng-tap build artifacts.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-clean

.PHONY: kunpeng-tap-install-service
kunpeng-tap-install-service: ## Install kunpeng-tap service.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-install-service

.PHONY: kunpeng-tap-install-service-docker
kunpeng-tap-install-service-docker: ## Install kunpeng-tap service for Docker.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-install-service-docker

.PHONY: kunpeng-tap-install-service-containerd
kunpeng-tap-install-service-containerd: ## Install kunpeng-tap service for Containerd.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-install-service-containerd

.PHONY: kunpeng-tap-start-service
kunpeng-tap-start-service: ## Start kunpeng-tap service.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-start-service

.PHONY: kunpeng-tap-stop-service
kunpeng-tap-stop-service: ## Stop kunpeng-tap service.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-stop-service

.PHONY: kunpeng-tap-status-service
kunpeng-tap-status-service: ## Show kunpeng-tap service status.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-status-service

.PHONY: kunpeng-tap-uninstall-service
kunpeng-tap-uninstall-service: ## Uninstall kunpeng-tap service.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-uninstall-service

##@ Distribution

.PHONY: dist
dist: build ## Create distribution packages.
	@echo "Creating distribution packages..."
	@mkdir -p $(DIST_DIR)
	@cp -r Boostkit_CloudNative/K8S/bin/* $(DIST_DIR)/ 2>/dev/null || true
	@echo "Distribution packages created in $(DIST_DIR)/"

.PHONY: install
install: dist ## Install all binaries to system.
	@echo "Installing binaries to system..."
	@sudo cp $(DIST_DIR)/* /usr/local/bin/ 2>/dev/null || true
	@echo "Installation completed"

##@ Docker

.PHONY: docker-build
docker-build: ## Build docker images for all projects.
	@echo "Building docker images for all projects..."
	$(MAKE) -C Boostkit_CloudNative/K8S mpam-docker
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-docker-build

##@ CI/CD

.PHONY: ci
ci: fmt vet test build ## Run CI pipeline (format, vet, test, build).
	@echo "CI pipeline completed successfully"

.PHONY: release
release: ci dist ## Create a release (CI + distribution).
	@echo "Release $(VERSION) created successfully"
	@echo "Artifacts available in $(DIST_DIR)/" 

.PHONY: kunpeng-tap-rpm-build
kunpeng-tap-rpm-build: ## Build kunpeng-tap RPM package.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-rpm-build

.PHONY: kunpeng-tap-rpm-build-docker
kunpeng-tap-rpm-build-docker: ## Build kunpeng-tap RPM package using Docker.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-rpm-build-docker

.PHONY: kunpeng-tap-rpm-install
kunpeng-tap-rpm-install: ## Install kunpeng-tap RPM package.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-rpm-install

.PHONY: kunpeng-tap-rpm-uninstall
kunpeng-tap-rpm-uninstall: ## Uninstall kunpeng-tap RPM package.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-rpm-uninstall

.PHONY: kunpeng-tap-rpm-test
kunpeng-tap-rpm-test: ## Test kunpeng-tap RPM package.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-rpm-test

.PHONY: kunpeng-tap-rpm-clean
kunpeng-tap-rpm-clean: ## Clean kunpeng-tap RPM build artifacts.
	$(MAKE) -C Boostkit_CloudNative/K8S kunpeng-tap-rpm-clean
