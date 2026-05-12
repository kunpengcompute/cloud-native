# K8S Projects Makefile
# This Makefile manages kunpeng-qos-controller and kunpeng-tap projects

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
all: qos-build kunpeng-tap-build kae-device-plugin-build kunpeng-perf-monitor-build

##@ General

.PHONY: help
help: ## Display this help.
	@echo "K8S Projects Build System"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Kunpeng QoS Controller

.PHONY: qos-build
qos-build: ## Build kunpeng-qos-controller project.
	$(MAKE) -f Makefile.kunpeng-qos-controller build

.PHONY: qos-clean
qos-clean: ## Clean kunpeng-qos-controller build artifacts.
	$(MAKE) -f Makefile.kunpeng-qos-controller clean

.PHONY: qos-docker
qos-docker: ## Build kunpeng-qos-controller docker image.
	$(MAKE) -f Makefile.kunpeng-qos-controller docker-build

.PHONY: qos-docker-push
qos-docker-push: ## Push kunpeng-qos-controller docker image.
	$(MAKE) -f Makefile.kunpeng-qos-controller docker-push

.PHONY: qos-docker-run
qos-docker-run: ## Run kunpeng-qos-controller docker container.
	$(MAKE) -f Makefile.kunpeng-qos-controller docker-run

.PHONY: qos-install
qos-install: ## Install kunpeng-qos-controller to system.
	$(MAKE) -f Makefile.kunpeng-qos-controller install

.PHONY: qos-uninstall
qos-uninstall: ## Uninstall kunpeng-qos-controller from system.
	$(MAKE) -f Makefile.kunpeng-qos-controller uninstall

.PHONY: qos-run
qos-run: ## Run kunpeng-qos-controller locally.
	$(MAKE) -f Makefile.kunpeng-qos-controller run


.PHONY: qos-test
qos-test: ## Test kunpeng-qos-controller.
	$(MAKE) -f Makefile.kunpeng-qos-controller test

.PHONY: qos-tidy
qos-tidy: ## Tidy kunpeng-qos-controller go modules.
	$(MAKE) -f Makefile.kunpeng-qos-controller tidy

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

.PHONY: kae-device-plugin-test
kae-device-plugin-test: ## Test kae-device-plugin.
	$(MAKE) -f Makefile.kae-device-plugin test

.PHONY: kae-device-plugin-tidy
kae-device-plugin-tidy: ## Tidy kae-device-plugin go modules.
	$(MAKE) -f Makefile.kae-device-plugin tidy

##@ Combined Operations

.PHONY: build
build: qos-build kunpeng-tap-build kae-device-plugin-build kunpeng-perf-monitor-build ## Build all projects.

.PHONY: clean
clean: qos-clean kunpeng-tap-clean kae-device-plugin-clean kunpeng-perf-monitor-clean## Clean all projects.

.PHONY: docker
docker: qos-docker kunpeng-tap-docker-build kae-device-plugin-docker kunpeng-perf-monitor-docker ## Build all docker images.

.PHONY: test
test: qos-test kunpeng-tap-test kae-device-plugin-test kunpeng-perf-monitor-test ## Run tests for all projects.

.PHONY: tidy
tidy: qos-tidy kunpeng-tap-tidy kae-device-plugin-tidy ## Tidy go modules for all projects.

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

##@ Kunpeng TAP NRI Container Deployment

.PHONY: kunpeng-tap-build-nri
kunpeng-tap-build-nri: ## Build NRI container image using standard Dockerfile.
	$(MAKE) -f Makefile.kunpeng-tap nri-build-image

.PHONY: kunpeng-tap-nri-deploy
kunpeng-tap-nri-deploy: ## Deploy NRI plugin as DaemonSet to Kubernetes.
	$(MAKE) -f Makefile.kunpeng-tap nri-deploy

.PHONY: kunpeng-tap-nri-status
kunpeng-tap-nri-status: ## Check NRI plugin deployment status.
	$(MAKE) -f Makefile.kunpeng-tap nri-status

.PHONY: kunpeng-tap-nri-logs
kunpeng-tap-nri-logs: ## Show NRI plugin logs.
	$(MAKE) -f Makefile.kunpeng-tap nri-logs

.PHONY: kunpeng-tap-nri-logs-follow
kunpeng-tap-nri-logs-follow: ## Follow NRI plugin logs.
	$(MAKE) -f Makefile.kunpeng-tap nri-logs-follow

.PHONY: kunpeng-tap-nri-restart
kunpeng-tap-nri-restart: ## Restart NRI plugin pods.
	$(MAKE) -f Makefile.kunpeng-tap nri-restart

.PHONY: kunpeng-tap-nri-undeploy
kunpeng-tap-nri-undeploy: ## Remove NRI plugin from Kubernetes.
	$(MAKE) -f Makefile.kunpeng-tap nri-undeploy

##@ Kunpeng Performance Monitor

.PHONY: kunpeng-perf-monitor-build
kunpeng-perf-monitor-build: ## Build kunpeng-perf-monitor project.
	$(MAKE) -f Makefile.kunpeng-perf-monitor build-static

.PHONY: kunpeng-perf-monitor-test
kunpeng-perf-monitor-test: ## Test kunpeng-perf-monitor project.
	$(MAKE) -f Makefile.kunpeng-perf-monitor test

.PHONY: kunpeng-perf-monitor-clean
kunpeng-perf-monitor-clean: ## Clean kunpeng-perf-monitor build artifacts.
	$(MAKE) -f Makefile.kunpeng-perf-monitor clean

.PHONY: kunpeng-perf-monitor-docker
kunpeng-perf-monitor-docker: ## Build kunpeng-perf-monitor docker image.
	$(MAKE) -f Makefile.kunpeng-perf-monitor docker-build

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.20.0

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.7.2
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
