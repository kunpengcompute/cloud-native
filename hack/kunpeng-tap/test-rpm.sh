#!/bin/bash

# Test script for kunpeng-tap RPM packaging
set -e

echo "=== Testing Kunpeng TAP RPM Packaging ==="

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

# Check prerequisites
print_status "Checking prerequisites..."

# Check if rpm-build is available
if ! command -v rpmbuild &> /dev/null; then
    print_warning "rpmbuild not found. This is required for local RPM building."
    print_warning "You can install it with: sudo yum install rpm-build"
    print_warning "Or use Docker-based building: make rpm-build-docker"
fi

# Check if Docker is available for Docker-based building
if ! command -v docker &> /dev/null; then
    print_warning "Docker not found. Docker-based RPM building will not be available."
fi

# Check if we're in the right directory
if [ ! -f "kunpeng-tap.spec" ]; then
    print_error "kunpeng-tap.spec not found. Please run this script from the packaging directory."
    exit 1
fi

print_status "Prerequisites check completed."

# Test 1: Prepare RPM build environment
print_status "Test 1: Preparing RPM build environment..."
make prepare
print_status "RPM build environment prepared successfully."

# Test 2: Build binary (if source is available)
print_status "Test 2: Building kunpeng-tap binary..."
if [ -f "../cmd/kunpeng-tap/proxy/main.go" ]; then
    make build-binary
    print_status "Binary built successfully."
else
    print_warning "Source code not found. Skipping binary build test."
    print_warning "This is expected if you're testing the packaging system independently."
fi

# Test 3: Create source tarball
print_status "Test 3: Creating source tarball..."
if [ -f "../bin/kunpeng-tap" ] || [ -f "../hack/kunpeng-tap/kunpeng-tap.service.docker" ]; then
    make create-source
    print_status "Source tarball created successfully."
else
    print_warning "Binary or service files not found. Creating minimal source tarball..."
    # Create minimal test structure
    mkdir -p kunpeng-tap-0.1.0/bin
    mkdir -p kunpeng-tap-0.1.0/hack/kunpeng-tap
    echo "#!/bin/bash" > kunpeng-tap-0.1.0/bin/kunpeng-tap
    echo "echo 'kunpeng-tap test binary'" >> kunpeng-tap-0.1.0/bin/kunpeng-tap
    chmod +x kunpeng-tap-0.1.0/bin/kunpeng-tap
    echo "[Unit]" > kunpeng-tap-0.1.0/hack/kunpeng-tap/kunpeng-tap.service.docker
    echo "Description=Kunpeng TAP Test Service" >> kunpeng-tap-0.1.0/hack/kunpeng-tap/kunpeng-tap.service.docker
    echo "# Kunpeng TAP" > kunpeng-tap-0.1.0/README.md
    echo "Apache License 2.0" > kunpeng-tap-0.1.0/LICENSE
    tar -czf kunpeng-tap-0.1.0.tar.gz kunpeng-tap-0.1.0
    rm -rf kunpeng-tap-0.1.0
    mkdir -p ~/rpmbuild/SOURCES
    cp kunpeng-tap-0.1.0.tar.gz ~/rpmbuild/SOURCES/
    print_status "Minimal source tarball created successfully."
fi

# Test 4: Validate RPM spec file
print_status "Test 4: Validating RPM spec file..."
if rpmbuild --nobuild --nodeps kunpeng-tap.spec; then
    print_status "RPM spec file validation passed."
else
    print_error "RPM spec file validation failed."
    exit 1
fi

# Test 5: Check RPM build environment
print_status "Test 5: Checking RPM build environment..."
if [ -d "$HOME/rpmbuild" ]; then
    print_status "RPM build environment exists."
    ls -la ~/rpmbuild/
else
    print_error "RPM build environment not found."
    exit 1
fi

print_status "=== All tests completed successfully! ==="
print_status ""
print_status "Next steps:"
print_status "1. To build RPM package: make rpm"
print_status "2. To build RPM package using Docker: make rpm-build-docker"
print_status "3. To test complete workflow: make test-rpm"
print_status ""
print_status "Note: Actual RPM building requires the complete source code and binary."
print_status "This test script validates the packaging infrastructure only."
