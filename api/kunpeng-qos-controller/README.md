# MPAM API (kubebuilder style)

This directory stores CRD Go types managed by kubebuilder/controller-gen markers.

## Prerequisite

Install `controller-gen` (from `sigs.k8s.io/controller-tools`).

## Generate deepcopy code

```bash
make -f api/kunpeng-qos-controller/Makefile generate
```

## Generate CRD YAML

```bash
make -f api/kunpeng-qos-controller/Makefile manifests
```

Generated CRDs are written to:

- `config/kunpeng-qos-controller-config/crd/bases`
