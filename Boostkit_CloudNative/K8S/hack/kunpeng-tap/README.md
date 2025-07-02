# Kunpeng TAP RPM Packaging

This directory contains the RPM packaging configuration for the Kunpeng TAP (Topology-Affinity Plugin) project.

## Overview

The RPM package includes:
- `kunpeng-tap` binary executable
- Systemd service files for Docker and Containerd runtimes
- Installation and uninstallation scripts
- Runtime directory creation

## Files

- `kunpeng-tap.spec` - RPM spec file defining package structure and scripts
- `Makefile` - Build automation for RPM packaging
- `RUNTIME_SELECTION.md` - Detailed documentation for runtime selection

## Prerequisites

To build RPM packages, you need:
- `rpm-build` package installed
- Standard RPM build environment (`~/rpmbuild` directory structure)

## Usage

### Build RPM Package

```bash
# Build RPM package locally
make rpm-build

# Build RPM package using Docker (recommended for consistency)
make rpm-build-docker
```

### Install/Uninstall RPM Package

```bash
# Install RPM package
make rpm-install

# Uninstall RPM package
make rpm-uninstall
```

### Test RPM Package

```bash
# Test complete RPM workflow (build, install, start, stop, uninstall)
make rpm-test
```

### Clean Build Artifacts

```bash
# Clean RPM build artifacts
make rpm-clean
```

## Service Management

After installing the RPM package, the service will be automatically enabled. You can manage it using standard systemctl commands:

```bash
# Start the service
sudo systemctl start kunpeng-tap.service

# Stop the service
sudo systemctl stop kunpeng-tap.service

# Check service status
sudo systemctl status kunpeng-tap.service

# Enable/disable service
sudo systemctl enable kunpeng-tap.service
sudo systemctl disable kunpeng-tap.service
```

## Package Contents

The RPM package installs:

- `/usr/local/bin/kunpeng-tap` - Main binary executable
- `/etc/systemd/system/kunpeng-tap.service` - Systemd service file
- `/var/run/kunpeng/` - Runtime directory

## Service Configuration

The service is configured to:
- Start automatically on boot
- Restart automatically on failure
- Use Docker runtime by default
- Apply topology-aware resource policy

## Runtime Modes

The service supports two container runtime modes:
- **Docker**: Uses `/var/run/docker.sock`
- **Containerd**: Uses `/var/run/containerd/containerd.sock`

### Runtime Selection During Installation

The RPM package now supports selecting the runtime during installation:

```bash
# Install with default runtime (Docker)
rpm -ivh kunpeng-tap-0.1.0-1.aarch64.rpm

# Install with Containerd runtime
rpm -ivh --define "runtime_type containerd" kunpeng-tap-0.1.0-1.aarch64.rpm

# Install with Docker runtime (explicit)
rpm -ivh --define "runtime_type docker" kunpeng-tap-0.1.0-1.aarch64.rpm
```

### Manual Runtime Switching

If you need to switch runtime after installation, manually replace the service file:

```bash
# Switch to Containerd
sudo cp /usr/local/share/kunpeng-tap/kunpeng-tap.service.containerd /etc/systemd/system/kunpeng-tap.service
sudo systemctl daemon-reload
sudo systemctl restart kunpeng-tap.service

# Switch to Docker
sudo cp /usr/local/share/kunpeng-tap/kunpeng-tap.service.docker /etc/systemd/system/kunpeng-tap.service
sudo systemctl daemon-reload
sudo systemctl restart kunpeng-tap.service
```

## Troubleshooting

### Build Issues

1. Ensure `rpm-build` is installed:
   ```bash
   sudo yum install rpm-build
   ```

2. Check RPM build environment:
   ```bash
   rpmbuild --showrc | grep _topdir
   ```

### Installation Issues

1. Check for conflicts with existing installations:
   ```bash
   rpm -qa | grep kunpeng-tap
   ```

2. Verify service file installation:
   ```bash
   ls -la /etc/systemd/system/kunpeng-tap.service
   ```

### Service Issues

1. Check service logs:
   ```bash
   sudo journalctl -u kunpeng-tap.service
   ```

2. Verify binary permissions:
   ```bash
   ls -la /usr/local/bin/kunpeng-tap
   ```

## Development

To modify the RPM package:

1. Edit `kunpeng-tap.spec` for package configuration changes
2. Update `Makefile` for build process changes
3. Test changes with `make rpm-test`

## Integration with Main Build System

The RPM packaging is integrated into the main build system:

```bash
# From project root
make kunpeng-tap-rpm-build

# From K8S directory
make kunpeng-tap-rpm-build

# From kunpeng-tap directory
make rpm-build
```
