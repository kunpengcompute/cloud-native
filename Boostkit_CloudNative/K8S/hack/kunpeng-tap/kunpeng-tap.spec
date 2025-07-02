Name:           kunpeng-tap
Version:        0.1.0
Release:        1
Summary:        Kunpeng Topology-Affinity Plugin for Kubernetes

Group:          System Environment/Daemons
License:        Apache License 2.0
URL:            https://github.com/kunpeng-cloud-computing
Source0:        %{name}-%{version}.tar.gz
BuildArch:      aarch64

Requires:       systemd

# Runtime selection option
%define runtime_type %{?runtime_type:runtime_type}%{!?runtime_type:docker}

%description
Kunpeng Topology-Affinity Plugin (TAP) is a Kubernetes runtime hook service
that provides topology-aware resource allocation for containers running on
Kunpeng processors.

%prep
%setup -q

%build
# Binary is pre-built, no compilation needed

%install
# Create directories
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/etc/systemd/system
mkdir -p %{buildroot}/var/run/kunpeng

# Install binary
install -m 755 bin/kunpeng-tap %{buildroot}/usr/local/bin/

# Install systemd service file based on runtime selection
%if "%{runtime_type}" == "containerd"
    install -m 644 hack/kunpeng-tap/kunpeng-tap.service.containerd %{buildroot}/etc/systemd/system/kunpeng-tap.service
%else
    install -m 644 hack/kunpeng-tap/kunpeng-tap.service.docker %{buildroot}/etc/systemd/system/kunpeng-tap.service
%endif

# Create runtime directory
mkdir -p %{buildroot}/var/run/kunpeng

%files
%license LICENSE
%doc README.md
/usr/local/bin/kunpeng-tap
/etc/systemd/system/kunpeng-tap.service
%dir /var/run/kunpeng

%pre
# Pre-installation script
if [ $1 -eq 1 ]; then
    # Fresh installation
    echo "Installing kunpeng-tap service with %{runtime_type} runtime..."
fi

%post
# Post-installation script
if [ $1 -eq 1 ]; then
    # Fresh installation
    systemctl daemon-reload
    systemctl enable kunpeng-tap.service
    echo "kunpeng-tap service installed and enabled successfully with %{runtime_type} runtime"
fi

%preun
# Pre-uninstallation script
if [ $1 -eq 0 ]; then
    # Complete removal
    echo "Stopping kunpeng-tap service..."
    systemctl stop kunpeng-tap.service || true
    systemctl disable kunpeng-tap.service || true
fi

%postun
# Post-uninstallation script
if [ $1 -eq 0 ]; then
    # Complete removal
    systemctl daemon-reload
    echo "kunpeng-tap service uninstalled successfully"
fi

%changelog
* Fri Jun 30 2025 Package Builder <builder@example.com> - 0.1.0-1
- Initial RPM package for kunpeng-tap
- Added runtime selection option (containerd/docker)
