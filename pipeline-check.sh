#!/bin/bash

# Pipeline Check Script
# This script performs pipeline checks including Go environment validation
# and running build/test for each project

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
PASSED=0
FAILED=0

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}$1${NC}"
    echo -e "${YELLOW}========================================${NC}"
}

# Check Go environment
check_go_env() {
    log_section "Checking Go Environment"
    
    # Check if go is installed
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        return 1
    fi
    
    log_info "Go version: $(go version)"
    
    # Check GO111MODULE
    local go_module=$(go env GO111MODULE)
    if [ "$go_module" != "on" ]; then
        log_error "GO111MODULE is not enabled (current: $go_module)"
        return 1
    fi
    log_info "GO111MODULE is enabled"
    
    # Check GOPROXY
    local goproxy=$(go env GOPROXY)
    if [[ "$goproxy" != *"https://goproxy.cn"* ]]; then
        log_error "GOPROXY is not set to https://goproxy.cn (current: $goproxy)"
        return 1
    fi
    log_info "GOPROXY is correctly set to: $goproxy"
    
    return 0
}

# Run build and test for a project
run_project_checks() {
    local project=$1
    local makefile_target=$2
    local skip_test=${3:-false}
    # Create temporary files for build and test output
    local build_output=$(mktemp)
    local test_output=$(mktemp)

    log_section "Processing Project: $project"

    # Run build
    log_info "Running build for $project..."
    if make "${makefile_target}-build" > "$build_output" 2>&1; then
        log_info "✓ Build passed for $project"
        ((PASSED++))
    else
        log_error "✗ Build failed for $project"
        cat "$build_output" >&2
        ((FAILED++))
        return 1
    fi
    rm -f "$build_output"

    # Run test (skip if requested)
    if [ "$skip_test" = true ]; then
        log_info "Skipping tests for $project"
        return 0
    fi

    log_info "Running tests for $project..."
    if make "${makefile_target}-test" > "$test_output" 2>&1; then
        log_info "✓ Tests passed for $project"
        ((PASSED++))
    else
        log_error "✗ Tests failed for $project"
        cat "$test_output" >&2
        ((FAILED++))
        return 1
    fi
    rm -f "$test_output"
}

# Main execution
main() {
    log_section "Pipeline Check Started"
    
    # Check Go environment
    if ! check_go_env; then
        log_error "Go environment check failed"
        exit 1
    fi
    
    # Get the directory where the script is located
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$SCRIPT_DIR"
    
    # Run checks for each project
    local failed_projects=()

    # Build and test kunpeng-tap and kae-device-plugin
    for project in "kunpeng-tap" "kae-device-plugin" "kunpeng-perf-monitor" "kunpeng-qos-controller"; do
        if ! run_project_checks "$project" "$project" false; then
            failed_projects+=("$project")
        fi
    done

    
    # Summary
    log_section "Pipeline Check Summary"
    log_info "Passed checks: $PASSED"
    log_error "Failed checks: $FAILED"
    
    if [ ${#failed_projects[@]} -gt 0 ]; then
        log_error "Failed projects: ${failed_projects[*]}"
        exit 1
    else
        log_info "All pipeline checks passed!"
        exit 0
    fi
}

main "$@"

