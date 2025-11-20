# K8S Projects Makefile
# This Makefile manages k8s-mpam-controller and kunpeng-tap projects

# Project configuration
VERSION ?= 0.1.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse HEAD)

# Build directories
BUILD_DIR := bin
GOPATH_BIN := $(shell go env GOPATH)/bin

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: mpam-build kunpeng-tap-build kae-device-plugin-build kunpeng-perf-monitor-build

##@ General

.PHONY: help
help: ## Display this help.
	@echo "K8S Projects Build System"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ K8S MPAM Controller

.PHONY: mpam-build
mpam-build: ## Build k8s-mpam-controller project.
	$(MAKE) -f Makefile.mpam build

.PHONY: mpam-build-local
mpam-build-local: ## Build k8s-mpam-controller locally.
	$(MAKE) -f Makefile.mpam build-local

.PHONY: mpam-clean
mpam-clean: ## Clean k8s-mpam-controller build artifacts.
	$(MAKE) -f Makefile.mpam clean

.PHONY: mpam-docker
mpam-docker: ## Build k8s-mpam-controller docker image.
	$(MAKE) -f Makefile.mpam docker-build

.PHONY: mpam-docker-push
mpam-docker-push: ## Push k8s-mpam-controller docker image.
	$(MAKE) -f Makefile.mpam docker-push

.PHONY: mpam-docker-run
mpam-docker-run: ## Run k8s-mpam-controller docker container.
	$(MAKE) -f Makefile.mpam docker-run

.PHONY: mpam-install
mpam-install: ## Install k8s-mpam-controller to system.
	$(MAKE) -f Makefile.mpam install

.PHONY: mpam-uninstall
mpam-uninstall: ## Uninstall k8s-mpam-controller from system.
	$(MAKE) -f Makefile.mpam uninstall

.PHONY: mpam-run
mpam-run: ## Run k8s-mpam-controller locally.
	$(MAKE) -f Makefile.mpam run

.PHONY: mpam-fmt
mpam-fmt: ## Format k8s-mpam-controller code.
	$(MAKE) -f Makefile.mpam fmt

.PHONY: mpam-vet
mpam-vet: ## Run go vet for k8s-mpam-controller.
	$(MAKE) -f Makefile.mpam vet

.PHONY: mpam-test
mpam-test: ## Test k8s-mpam-controller.
	$(MAKE) -f Makefile.mpam test

.PHONY: mpam-tidy
mpam-tidy: ## Tidy k8s-mpam-controller go modules.
	$(MAKE) -f Makefile.mpam tidy

##@ Kunpeng TAP

.PHONY: kunpeng-tap-build
kunpeng-tap-build: ## Build kunpeng-tap project.
	$(MAKE) -f Makefile.kunpeng-tap build

.PHONY: kunpeng-tap-build-manager
kunpeng-tap-build-manager: ## Build kunpeng-tap manager.
	$(MAKE) -f Makefile.kunpeng-tap build-manager

.PHONY: kunpeng-tap-build-proxy
kunpeng-tap-build-proxy: ## Build kunpeng-tap proxy.
	$(MAKE) -f Makefile.kunpeng-tap build-proxy

.PHONY: kunpeng-tap-test
kunpeng-tap-test: ## Test kunpeng-tap project.
	$(MAKE) -f Makefile.kunpeng-tap test

.PHONY: kunpeng-tap-clean
kunpeng-tap-clean: ## Clean kunpeng-tap build artifacts.
	$(MAKE) -f Makefile.kunpeng-tap clean

.PHONY: kunpeng-tap-fmt
kunpeng-tap-fmt: ## Format kunpeng-tap code.
	$(MAKE) -f Makefile.kunpeng-tap fmt

.PHONY: kunpeng-tap-vet
kunpeng-tap-vet: ## Run go vet for kunpeng-tap.
	$(MAKE) -f Makefile.kunpeng-tap vet

.PHONY: kunpeng-tap-tidy
kunpeng-tap-tidy: ## Tidy kunpeng-tap go modules.
	$(MAKE) -f Makefile.kunpeng-tap tidy

.PHONY: kunpeng-tap-run-manager
kunpeng-tap-run-manager: ## Run kunpeng-tap manager.
	$(MAKE) -f Makefile.kunpeng-tap run-manager

.PHONY: kunpeng-tap-run-proxy
kunpeng-tap-run-proxy: ## Run kunpeng-tap proxy.
	$(MAKE) -f Makefile.kunpeng-tap run-proxy

##@ Kunpeng TAP Service Management

.PHONY: kunpeng-tap-install-service
kunpeng-tap-install-service: ## Install kunpeng-tap service (default: docker).
	$(MAKE) -f Makefile.kunpeng-tap install-service

.PHONY: kunpeng-tap-install-service-docker
kunpeng-tap-install-service-docker: ## Install kunpeng-tap service for Docker runtime.
	$(MAKE) -f Makefile.kunpeng-tap install-service-docker

.PHONY: kunpeng-tap-install-service-containerd
kunpeng-tap-install-service-containerd: ## Install kunpeng-tap service for Containerd runtime.
	$(MAKE) -f Makefile.kunpeng-tap install-service-containerd

.PHONY: kunpeng-tap-start-service
kunpeng-tap-start-service: ## Start kunpeng-tap service.
	$(MAKE) -f Makefile.kunpeng-tap start-service

.PHONY: kunpeng-tap-stop-service
kunpeng-tap-stop-service: ## Stop kunpeng-tap service.
	$(MAKE) -f Makefile.kunpeng-tap stop-service

.PHONY: kunpeng-tap-restart-service
kunpeng-tap-restart-service: ## Restart kunpeng-tap service.
	$(MAKE) -f Makefile.kunpeng-tap restart-service

.PHONY: kunpeng-tap-status-service
kunpeng-tap-status-service: ## Show kunpeng-tap service status.
	$(MAKE) -f Makefile.kunpeng-tap status-service

.PHONY: kunpeng-tap-uninstall-service
kunpeng-tap-uninstall-service: ## Uninstall kunpeng-tap service.
	$(MAKE) -f Makefile.kunpeng-tap uninstall-service

##@ Kunpeng TAP Docker

.PHONY: kunpeng-tap-docker-build
kunpeng-tap-docker-build: ## Build kunpeng-tap docker image.
	$(MAKE) -f Makefile.kunpeng-tap docker-build

.PHONY: kunpeng-tap-docker-push
kunpeng-tap-docker-push: ## Push kunpeng-tap docker image.
	$(MAKE) -f Makefile.kunpeng-tap docker-push

.PHONY: kunpeng-tap-docker-buildx
kunpeng-tap-docker-buildx: ## Build and push kunpeng-tap docker image for cross-platform.
	$(MAKE) -f Makefile.kunpeng-tap docker-buildx

##@ KAE Device Plugin

.PHONY: kae-device-plugin-build
kae-device-plugin-build: ## Build kae-device-plugin project.
	$(MAKE) -f Makefile.kae-device-plugin build

.PHONY: kae-device-plugin-build-local
kae-device-plugin-build-local: ## Build kae-device-plugin locally.
	$(MAKE) -f Makefile.kae-device-plugin build-local

.PHONY: kae-device-plugin-clean
kae-device-plugin-clean: ## Clean kae-device-plugin build artifacts.
	$(MAKE) -f Makefile.kae-device-plugin clean

.PHONY: kae-device-plugin-docker
kae-device-plugin-docker: ## Build kae-device-plugin docker image.
	$(MAKE) -f Makefile.kae-device-plugin docker-build

.PHONY: kae-device-plugin-docker-push
kae-device-plugin-docker-push: ## Push kae-device-plugin docker image.
	$(MAKE) -f Makefile.kae-device-plugin docker-push

.PHONY: kae-device-plugin-install
kae-device-plugin-install: ## Install kae-device-plugin to system.
	$(MAKE) -f Makefile.kae-device-plugin install

.PHONY: kae-device-plugin-uninstall
kae-device-plugin-uninstall: ## Uninstall kae-device-plugin from system.
	$(MAKE) -f Makefile.kae-device-plugin uninstall

.PHONY: kae-device-plugin-run
kae-device-plugin-run: ## Run kae-device-plugin locally.
	$(MAKE) -f Makefile.kae-device-plugin run

.PHONY: kae-device-plugin-fmt
kae-device-plugin-fmt: ## Format kae-device-plugin code.
	$(MAKE) -f Makefile.kae-device-plugin fmt

.PHONY: kae-device-plugin-vet
kae-device-plugin-vet: ## Run go vet for kae-device-plugin.
	$(MAKE) -f Makefile.kae-device-plugin vet

.PHONY: kae-device-plugin-test
kae-device-plugin-test: ## Test kae-device-plugin.
	$(MAKE) -f Makefile.kae-device-plugin test

.PHONY: kae-device-plugin-tidy
kae-device-plugin-tidy: ## Tidy kae-device-plugin go modules.
	$(MAKE) -f Makefile.kae-device-plugin tidy

##@ Combined Operations

.PHONY: build
build: mpam-build kunpeng-tap-build kae-device-plugin-build kunpeng-perf-monitor-build ## Build all projects.

.PHONY: clean
clean: mpam-clean kunpeng-tap-clean kae-device-plugin-clean kunpeng-perf-monitor-clean## Clean all projects.

.PHONY: docker
docker: mpam-docker kunpeng-tap-docker-build kae-device-plugin-docker kunpeng-perf-monitor-docker ## Build all docker images.

.PHONY: fmt
fmt: mpam-fmt kunpeng-tap-fmt kae-device-plugin-fmt ## Format code for all projects.

.PHONY: vet
vet: mpam-vet kunpeng-tap-vet kae-device-plugin-vet ## Run go vet for all projects.

.PHONY: test
test: mpam-test kunpeng-tap-test kae-device-plugin-test kunpeng-perf-monitor-test ## Run tests for all projects.

.PHONY: tidy
tidy: mpam-tidy kunpeng-tap-tidy kae-device-plugin-tidy ## Tidy go modules for all projects.

##@ Kunpeng TAP RPM Packaging

.PHONY: kunpeng-tap-rpm-build
kunpeng-tap-rpm-build: ## Build kunpeng-tap RPM package.
	$(MAKE) -f Makefile.kunpeng-tap rpm-build

.PHONY: kunpeng-tap-rpm-build-docker
kunpeng-tap-rpm-build-docker: ## Build kunpeng-tap RPM package using Docker.
	$(MAKE) -f Makefile.kunpeng-tap rpm-build-docker

.PHONY: kunpeng-tap-rpm-install
kunpeng-tap-rpm-install: ## Install kunpeng-tap RPM package.
	$(MAKE) -f Makefile.kunpeng-tap rpm-install

.PHONY: kunpeng-tap-rpm-uninstall
kunpeng-tap-rpm-uninstall: ## Uninstall kunpeng-tap RPM package.
	$(MAKE) -f Makefile.kunpeng-tap rpm-uninstall

.PHONY: kunpeng-tap-rpm-test
kunpeng-tap-rpm-test: ## Test kunpeng-tap RPM package.
	$(MAKE) -f Makefile.kunpeng-tap rpm-test

.PHONY: kunpeng-tap-rpm-clean
kunpeng-tap-rpm-clean: ## Clean kunpeng-tap RPM build artifacts.
	$(MAKE) -f Makefile.kunpeng-tap rpm-clean

##@ Kunpeng Performance Monitor

.PHONY: kunpeng-perf-monitor-build
kunpeng-perf-monitor-build: ## Build kunpeng-perf-monitor project.
	$(MAKE) -f Makefile.kunpeng-perf-monitor build

.PHONY: kunpeng-perf-monitor-test
kunpeng-perf-monitor-test: ## Test kunpeng-perf-monitor project.
	$(MAKE) -f Makefile.kunpeng-perf-monitor test

.PHONY: kunpeng-perf-monitor-clean
kunpeng-perf-monitor-clean: ## Clean kunpeng-perf-monitor build artifacts.
	$(MAKE) -f Makefile.kunpeng-perf-monitor clean

.PHONY: kunpeng-perf-monitor-docker
kunpeng-perf-monitor-docker: ## Build kunpeng-perf-monitor docker image.
	$(MAKE) -f Makefile.kunpeng-perf-monitor docker-build