# RPM Packaging Makefile for kunpeng-tap

# Package configuration
PACKAGE_NAME := kunpeng-tap
VERSION ?= 0.1.0
RELEASE ?= 1
DIST ?= 

# Build directories
BUILD_DIR := ../../bin
PACKAGE_DIR := $(PACKAGE_NAME)-$(VERSION)
RPM_BUILD_DIR := $(HOME)/rpmbuild
SPEC_FILE := $(PACKAGE_NAME).spec

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: rpm

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

.PHONY: prepare
prepare: ## Prepare RPM build environment.
	@echo "Preparing RPM build environment..."
	mkdir -p $(RPM_BUILD_DIR)/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	@echo "RPM build environment prepared"

.PHONY: build-binary
build-binary: ## Build kunpeng-tap binary.
	@echo "Building kunpeng-tap binary..."
	$(MAKE) -C ../.. -f Makefile.kunpeng-tap build
	@echo "Binary built successfully"

.PHONY: create-source
create-source: build-binary ## Create source tarball for RPM build.
	@echo "Creating source tarball..."
	rm -rf $(PACKAGE_DIR)
	mkdir -p $(PACKAGE_DIR)
	
	# Copy binary
	mkdir -p $(PACKAGE_DIR)/bin
	cp $(BUILD_DIR)/kunpeng-tap $(PACKAGE_DIR)/bin/
	
	# Copy service files
	mkdir -p $(PACKAGE_DIR)/hack/kunpeng-tap
	cp kunpeng-tap.service.docker $(PACKAGE_DIR)/hack/kunpeng-tap/
	cp kunpeng-tap.service.containerd $(PACKAGE_DIR)/hack/kunpeng-tap/
	
	# Copy documentation
	cp ../../README.kunpeng-tap.md $(PACKAGE_DIR)/README.md 2>/dev/null || echo "# Kunpeng TAP" > $(PACKAGE_DIR)/README.md
	cp ../../LICENSE $(PACKAGE_DIR)/ 2>/dev/null || echo "Apache License 2.0" > $(PACKAGE_DIR)/LICENSE
	
	# Create tarball
	tar -czf $(PACKAGE_DIR).tar.gz $(PACKAGE_DIR)
	rm -rf $(PACKAGE_DIR)
	
	# Copy to RPM sources
	cp $(PACKAGE_DIR).tar.gz $(RPM_BUILD_DIR)/SOURCES/
	@echo "Source tarball created: $(PACKAGE_DIR).tar.gz"

.PHONY: rpm
rpm: prepare create-source ## Build RPM package.
	@echo "Building RPM package..."
	rpmbuild -ba $(SPEC_FILE) \
		--nodebuginfo \
		--define "_topdir $(RPM_BUILD_DIR)" \
		--define "version $(VERSION)" \
		--define "release $(RELEASE)"
	@echo "RPM package built successfully"
	@echo "RPM files available in: $(RPM_BUILD_DIR)/RPMS/"

.PHONY: rpm-docker
rpm-docker: ## Build RPM package using Docker.
	@echo "Building RPM package using Docker..."
	docker run --rm -v $(PWD):/workspace -w /workspace \
		openeuler/openeuler:22.03-lts /bin/bash -c "\
		yum install -y rpm-build make && \
		make rpm"
	@echo "RPM package built successfully using Docker"

.PHONY: install-rpm
install-rpm: ## Install RPM package.
	@echo "Installing RPM package..."
	sudo rpm -ivh $(RPM_BUILD_DIR)/RPMS/*/$(PACKAGE_NAME)-$(VERSION)-$(RELEASE).*.rpm
	@echo "RPM package installed successfully"

.PHONY: uninstall-rpm
uninstall-rpm: ## Uninstall RPM package.
	@echo "Uninstalling RPM package..."
	sudo rpm -e $(PACKAGE_NAME)
	@echo "RPM package uninstalled successfully"

.PHONY: clean
clean: ## Clean build artifacts.
	@echo "Cleaning build artifacts..."
	rm -f $(PACKAGE_DIR).tar.gz
	rm -rf $(PACKAGE_DIR)
	rm -rf $(RPM_BUILD_DIR)
	@echo "Clean completed"

##@ Service Management

.PHONY: start-service
start-service: ## Start kunpeng-tap service.
	sudo systemctl start kunpeng-tap.service
	sudo systemctl status kunpeng-tap.service

.PHONY: stop-service
stop-service: ## Stop kunpeng-tap service.
	sudo systemctl stop kunpeng-tap.service

.PHONY: status-service
status-service: ## Show kunpeng-tap service status.
	sudo systemctl status kunpeng-tap.service

##@ Development

.PHONY: test-rpm
test-rpm: rpm ## Test RPM package installation and service.
	@echo "Testing RPM package..."
	# Install RPM
	$(MAKE) install-rpm
	# Start service
	$(MAKE) start-service
	# Check status
	$(MAKE) status-service
	# Stop service
	$(MAKE) stop-service
	# Uninstall RPM
	$(MAKE) uninstall-rpm
	@echo "RPM package test completed successfully"
