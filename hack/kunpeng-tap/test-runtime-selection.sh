#!/bin/bash

# Test script for kunpeng-tap runtime selection
# This script tests the RPM package installation with different runtime options

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if RPM is installed
check_rpm_installed() {
    rpm -q kunpeng-tap >/dev/null 2>&1
}

# Function to uninstall RPM if installed
uninstall_rpm() {
    if check_rpm_installed; then
        print_status "Uninstalling existing kunpeng-tap RPM..."
        rpm -e kunpeng-tap
    fi
}

# Function to build RPM
build_rpm() {
    print_status "Building kunpeng-tap RPM..."
    cd "$PROJECT_ROOT"
    make kunpeng-tap-rpm-build
}

# Function to install RPM with specific runtime
install_rpm_with_runtime() {
    local runtime_type=$1
    local rpm_file="kunpeng-tap-0.1.0-1.x86_64.rpm"
    
    print_status "Installing kunpeng-tap with $runtime_type runtime..."
    
    if [ "$runtime_type" = "containerd" ]; then
        rpm -ivh --define "runtime_type containerd" "$rpm_file"
    else
        rpm -ivh --define "runtime_type docker" "$rpm_file"
    fi
}

# Function to verify service file
verify_service_file() {
    local expected_runtime=$1
    local service_file="/etc/systemd/system/kunpeng-tap.service"
    
    if [ ! -f "$service_file" ]; then
        print_error "Service file not found: $service_file"
        return 1
    fi
    
    if [ "$expected_runtime" = "containerd" ]; then
        if grep -q "containerd.sock" "$service_file" && grep -q "Containerd" "$service_file"; then
            print_status "✓ Service file correctly configured for containerd"
        else
            print_error "✗ Service file not correctly configured for containerd"
            return 1
        fi
    else
        if grep -q "docker.sock" "$service_file" && grep -q "Docker" "$service_file"; then
            print_status "✓ Service file correctly configured for docker"
        else
            print_error "✗ Service file not correctly configured for docker"
            return 1
        fi
    fi
}

# Function to test service
test_service() {
    local runtime_type=$1
    
    print_status "Testing kunpeng-tap service with $runtime_type runtime..."
    
    # Check if service file exists and is correct
    verify_service_file "$runtime_type"
    
    # Check service status
    if systemctl is-enabled kunpeng-tap.service >/dev/null 2>&1; then
        print_status "✓ Service is enabled"
    else
        print_error "✗ Service is not enabled"
        return 1
    fi
    
    # Try to start service (this might fail if runtime is not available)
    if systemctl start kunpeng-tap.service 2>/dev/null; then
        print_status "✓ Service started successfully"
        systemctl stop kunpeng-tap.service
    else
        print_warning "⚠ Service could not be started (runtime might not be available)"
    fi
}

# Main test function
run_tests() {
    print_status "Starting kunpeng-tap runtime selection tests..."
    
    # Clean up any existing installation
    uninstall_rpm
    
    # Build RPM
    build_rpm
    
    # Test 1: Install with docker runtime (default)
    print_status "=== Test 1: Docker Runtime (Default) ==="
    install_rpm_with_runtime "docker"
    test_service "docker"
    uninstall_rpm
    
    # Test 2: Install with containerd runtime
    print_status "=== Test 2: Containerd Runtime ==="
    install_rpm_with_runtime "containerd"
    test_service "containerd"
    uninstall_rpm
    
    # Test 3: Install with explicit docker runtime
    print_status "=== Test 3: Explicit Docker Runtime ==="
    install_rpm_with_runtime "docker"
    test_service "docker"
    uninstall_rpm
    
    print_status "All tests completed successfully!"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_error "This script must be run as root (use sudo)"
    exit 1
fi

# Parse command line arguments
case "${1:-}" in
    "docker")
        uninstall_rpm
        build_rpm
        install_rpm_with_runtime "docker"
        test_service "docker"
        ;;
    "containerd")
        uninstall_rpm
        build_rpm
        install_rpm_with_runtime "containerd"
        test_service "containerd"
        ;;
    "clean")
        uninstall_rpm
        ;;
    "test")
        run_tests
        ;;
    *)
        echo "Usage: $0 {docker|containerd|clean|test}"
        echo "  docker     - Install with docker runtime"
        echo "  containerd - Install with containerd runtime"
        echo "  clean      - Uninstall kunpeng-tap"
        echo "  test       - Run all tests"
        exit 1
        ;;
esac
