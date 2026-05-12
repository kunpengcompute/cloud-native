CONTROLLER_GEN ?= controller-gen
API_PATH ?= ./pkg/k8s-mpam-controller/api/v1alpha1
CRD_OUT ?= ./config/kunpeng-qos-controller-config/crd/bases

.PHONY: generate manifests all

all: generate manifests

generate:
	$(CONTROLLER_GEN) object paths=$(API_PATH)

manifests:
	$(CONTROLLER_GEN) crd paths=$(API_PATH) output:crd:artifacts:config=$(CRD_OUT)
