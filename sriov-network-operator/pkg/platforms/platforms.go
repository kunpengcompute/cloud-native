package platforms

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/host"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/platforms/openshift"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/platforms/openstack" // 新增openstack包
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/platforms/qemu"      // 新增qemu包
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"
)

//go:generate ../../bin/mockgen -destination mock/mock_platforms.go -source platforms.go
type Interface interface {
	openshift.OpenshiftContextInterface
	openstack.OpenstackInterface
	qemu.QemuKvmInterface // 新增QEMU接口
}

type platformHelper struct {
	openshift.OpenshiftContextInterface
	openstack.OpenstackInterface
	qemu.QemuKvmInterface // 嵌入QEMU实现
}

func NewDefaultPlatformHelper() (Interface, error) {
	openshiftContext, err := openshift.New()
	if err != nil {
		return nil, err
	}
	utilsHelper := utils.New()
	hostManager, err := host.NewHostManager(utilsHelper)
	if err != nil {
		log.Log.Error(err, "failed to create host manager")
		return nil, err
	}
	openstackContext := openstack.New(hostManager)

	// 新增QEMU上下文初始化
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	k8sClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %v", err)
	}
	qemukvmContext := qemu.New(hostManager, k8sClientset)

	return &platformHelper{
		openshiftContext,
		openstackContext,
		qemukvmContext, // 添加QEMU实现
	}, nil
}
