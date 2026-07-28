package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/helper"
	helperMock "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/helper/mock"
	hosttypes "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/host/types"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/platforms"
	platformsMock "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/platforms/mock"
	plugin "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins/generic"
	pluginsMock "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins/mock"
)

func restoreOrigFuncs() {
	origNewGenericPluginFunc := newGenericPluginFunc
	origNewVirtualPluginFunc := newVirtualPluginFunc
	origNewHostHelpersFunc := newHostHelpersFunc
	origNewPlatformHelperFunc := newPlatformHelperFunc
	DeferCleanup(func() {
		newGenericPluginFunc = origNewGenericPluginFunc
		newVirtualPluginFunc = origNewVirtualPluginFunc
		newHostHelpersFunc = origNewHostHelpersFunc
		newPlatformHelperFunc = origNewPlatformHelperFunc
	})
}

func getTestSriovInterfaceConfig(platform consts.PlatformTypes) *hosttypes.SriovConfig {
	return &hosttypes.SriovConfig{
		Spec: sriovnetworkv1.SriovNetworkNodeStateSpec{
			Interfaces: sriovnetworkv1.Interfaces{
				{
					PciAddress:  "0000:d8:00.0",
					NumVfs:      4,
					Mtu:         1500,
					Name:        "enp216s0f0np0",
					LinkType:    "",
					EswitchMode: "legacy",
					VfGroups: []sriovnetworkv1.VfGroup{
						{
							ResourceName: "legacy",
							DeviceType:   "",
							VfRange:      "0-3",
							PolicyName:   "test-legacy",
							Mtu:          1500,
							IsRdma:       false,
							VdpaType:     "",
						},
					},
					ExternallyManaged: false,
				},
			},
		},
		PlatformType:          platform,
		UnsupportedNics:       false,
		ManageSoftwareBridges: true,
	}
}

var testSriovSupportedNicIDs = []string{"8086 1583 154c", "8086 0d58 154c", "8086 10c9 10ca", "19e5 1822 375e", "19e5 0222 375f"}

func getTestResultFileContent(syncStatus, errMsg string) *hosttypes.SriovResult {
	return &hosttypes.SriovResult{SyncStatus: syncStatus, LastSyncError: errMsg}
}

// checks if NodeState contains deviceName in spec and status fields
func newNodeStateContainsDeviceMatcher(deviceName string) gomock.Matcher {
	return &nodeStateContainsDeviceMatcher{deviceName: deviceName}
}

type nodeStateContainsDeviceMatcher struct {
	deviceName string
}

func (ns *nodeStateContainsDeviceMatcher) Matches(x interface{}) bool {
	s, ok := x.(*sriovnetworkv1.SriovNetworkNodeState)
	if !ok {
		return false
	}
	specFound := false
	for _, i := range s.Spec.Interfaces {
		if i.Name == ns.deviceName {
			specFound = true
			break
		}
	}
	if !specFound {
		return false
	}
	statusFound := false
	for _, i := range s.Status.Interfaces {
		if i.Name == ns.deviceName {
			statusFound = true
			break
		}
	}
	return statusFound
}

func (ns *nodeStateContainsDeviceMatcher) String() string {
	return "node state contains match: " + ns.deviceName
}

var _ = Describe("Service", func() {
	var (
		hostHelpers    *helperMock.MockHostHelpersInterface
		platformHelper *platformsMock.MockInterface
		genericPlugin  *pluginsMock.MockVendorPlugin
		virtualPlugin  *pluginsMock.MockVendorPlugin

		testCtrl  *gomock.Controller
		testError = fmt.Errorf("test")
	)

	BeforeEach(func() {
		restoreOrigFuncs()

		testCtrl = gomock.NewController(GinkgoT())
		hostHelpers = helperMock.NewMockHostHelpersInterface(testCtrl)
		platformHelper = platformsMock.NewMockInterface(testCtrl)
		genericPlugin = pluginsMock.NewMockVendorPlugin(testCtrl)
		virtualPlugin = pluginsMock.NewMockVendorPlugin(testCtrl)

		newGenericPluginFunc = func(_ helper.HostHelpersInterface, _ ...generic.Option) (plugin.VendorPlugin, error) {
			return genericPlugin, nil
		}
		newVirtualPluginFunc = func(_ helper.HostHelpersInterface) (plugin.VendorPlugin, error) {
			return virtualPlugin, nil
		}
		newHostHelpersFunc = func() (helper.HostHelpersInterface, error) {
			return hostHelpers, nil
		}
		newPlatformHelperFunc = func() (platforms.Interface, error) {
			return platformHelper, nil
		}

	})
	AfterEach(func() {
		phaseArg = ""
		testCtrl.Finish()
	})

	It("Pre phase - baremetal cluster", func() {
		phaseArg = PhasePre
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.0").Return("enp216s0f0np0")
		hostHelpers.EXPECT().WaitUdevEventsProcessed(60).Return(nil)
		hostHelpers.EXPECT().CheckRDMAEnabled().Return(true, nil)
		hostHelpers.EXPECT().TryEnableTun().Return()
		hostHelpers.EXPECT().TryEnableVhostNet().Return()
		hostHelpers.EXPECT().DiscoverSriovDevices(gomock.Any()).Return([]sriovnetworkv1.InterfaceExt{{
			Name: "enp216s0f0np0",
		}}, nil)
		hostHelpers.EXPECT().ReadConfFile().Return(getTestSriovInterfaceConfig(0), nil)
		hostHelpers.EXPECT().ReadSriovSupportedNics().Return(testSriovSupportedNicIDs, nil)
		hostHelpers.EXPECT().RemoveSriovResult().Return(nil)
		hostHelpers.EXPECT().WriteSriovResult(&hosttypes.SriovResult{SyncStatus: consts.SyncStatusInProgress})

		genericPlugin.EXPECT().OnNodeStateChange(newNodeStateContainsDeviceMatcher("enp216s0f0np0")).Return(true, false, nil)
		genericPlugin.EXPECT().Apply().Return(nil)

		Expect(runServiceCmd(&cobra.Command{}, []string{})).NotTo(HaveOccurred())
		Expect(testCtrl.Satisfied()).To(BeTrue())
	})

	It("Pre phase - virtual cluster", func() {
		phaseArg = PhasePre
		hostHelpers.EXPECT().CheckRDMAEnabled().Return(true, nil)
		hostHelpers.EXPECT().TryEnableTun().Return()
		hostHelpers.EXPECT().TryEnableVhostNet().Return()
		hostHelpers.EXPECT().ReadConfFile().Return(getTestSriovInterfaceConfig(1), nil)
		hostHelpers.EXPECT().ReadSriovSupportedNics().Return(testSriovSupportedNicIDs, nil)
		hostHelpers.EXPECT().RemoveSriovResult().Return(nil)
		hostHelpers.EXPECT().WriteSriovResult(&hosttypes.SriovResult{SyncStatus: consts.SyncStatusInProgress})

		platformHelper.EXPECT().CreateOpenstackDevicesInfo().Return(nil)
		platformHelper.EXPECT().DiscoverSriovDevicesVirtual().Return([]sriovnetworkv1.InterfaceExt{{
			Name: "enp216s0f0np0",
		}}, nil)

		virtualPlugin.EXPECT().OnNodeStateChange(newNodeStateContainsDeviceMatcher("enp216s0f0np0")).Return(true, false, nil)
		virtualPlugin.EXPECT().Apply().Return(nil)

		Expect(runServiceCmd(&cobra.Command{}, []string{})).NotTo(HaveOccurred())
		Expect(testCtrl.Satisfied()).To(BeTrue())
	})

	It("Pre phase - qemu-kvm cluster", func() {
		phaseArg = PhasePre
		hostHelpers.EXPECT().CheckRDMAEnabled().Return(true, nil)
		hostHelpers.EXPECT().TryEnableTun().Return()
		hostHelpers.EXPECT().TryEnableVhostNet().Return()
		hostHelpers.EXPECT().ReadConfFile().Return(getTestSriovInterfaceConfig(2), nil) // 2 对应 consts.VirtualQemuKVM
		hostHelpers.EXPECT().ReadSriovSupportedNics().Return(testSriovSupportedNicIDs, nil)
		hostHelpers.EXPECT().RemoveSriovResult().Return(nil)
		hostHelpers.EXPECT().WriteSriovResult(&hosttypes.SriovResult{SyncStatus: consts.SyncStatusInProgress})

		platformHelper.EXPECT().CreateQemuKvmDevicesInfo().Return(nil) // 新增 QEMU 方法调用
		platformHelper.EXPECT().DiscoverSriovDevicesVirtualInQemuKvm().Return([]sriovnetworkv1.InterfaceExt{{
			Name: "enp216s0f0np0",
		}}, nil)

		virtualPlugin.EXPECT().OnNodeStateChange(newNodeStateContainsDeviceMatcher("enp216s0f0np0")).Return(true, false, nil)
		virtualPlugin.EXPECT().Apply().Return(nil)

		Expect(runServiceCmd(&cobra.Command{}, []string{})).NotTo(HaveOccurred())
		Expect(testCtrl.Satisfied()).To(BeTrue())
	})

	It("Pre phase - apply failed", func() {
		phaseArg = PhasePre
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.0").Return("enp216s0f0np0")
		hostHelpers.EXPECT().WaitUdevEventsProcessed(60).Return(nil)
		hostHelpers.EXPECT().CheckRDMAEnabled().Return(true, nil)
		hostHelpers.EXPECT().TryEnableTun().Return()
		hostHelpers.EXPECT().TryEnableVhostNet().Return()
		hostHelpers.EXPECT().DiscoverSriovDevices(gomock.Any()).Return([]sriovnetworkv1.InterfaceExt{{
			Name: "enp216s0f0np0",
		}}, nil)
		hostHelpers.EXPECT().ReadConfFile().Return(getTestSriovInterfaceConfig(0), nil)
		hostHelpers.EXPECT().ReadSriovSupportedNics().Return(testSriovSupportedNicIDs, nil)
		hostHelpers.EXPECT().RemoveSriovResult().Return(nil)
		hostHelpers.EXPECT().WriteSriovResult(&hosttypes.SriovResult{SyncStatus: consts.SyncStatusFailed, LastSyncError: "pre: failed to apply configuration: test"})

		genericPlugin.EXPECT().OnNodeStateChange(newNodeStateContainsDeviceMatcher("enp216s0f0np0")).Return(true, false, nil)
		genericPlugin.EXPECT().Apply().Return(testError)

		Expect(runServiceCmd(&cobra.Command{}, []string{})).To(MatchError(ContainSubstring("test")))
		Expect(testCtrl.Satisfied()).To(BeTrue())
	})

	It("Post phase - baremetal cluster", func() {
		phaseArg = PhasePost
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.0").Return("enp216s0f0np0")
		hostHelpers.EXPECT().WaitUdevEventsProcessed(60).Return(nil)
		hostHelpers.EXPECT().DiscoverSriovDevices(gomock.Any()).Return([]sriovnetworkv1.InterfaceExt{{
			Name: "enp216s0f0np0",
		}}, nil)
		hostHelpers.EXPECT().DiscoverBridges().Return(sriovnetworkv1.Bridges{}, nil)
		hostHelpers.EXPECT().ReadSriovResult().Return(getTestResultFileContent("InProgress", ""), nil)
		hostHelpers.EXPECT().ReadConfFile().Return(getTestSriovInterfaceConfig(0), nil)
		hostHelpers.EXPECT().ReadSriovSupportedNics().Return(testSriovSupportedNicIDs, nil)
		hostHelpers.EXPECT().WriteSriovResult(&hosttypes.SriovResult{SyncStatus: consts.SyncStatusSucceeded})

		genericPlugin.EXPECT().OnNodeStateChange(newNodeStateContainsDeviceMatcher("enp216s0f0np0")).Return(true, false, nil)
		genericPlugin.EXPECT().Apply().Return(nil)
		Expect(runServiceCmd(&cobra.Command{}, []string{})).NotTo(HaveOccurred())
		Expect(testCtrl.Satisfied()).To(BeTrue())
	})

	It("Post phase - virtual cluster", func() {
		phaseArg = PhasePost
		hostHelpers.EXPECT().ReadConfFile().Return(getTestSriovInterfaceConfig(1), nil)
		hostHelpers.EXPECT().ReadSriovSupportedNics().Return(testSriovSupportedNicIDs, nil)
		hostHelpers.EXPECT().ReadSriovResult().Return(getTestResultFileContent("InProgress", ""), nil)
		hostHelpers.EXPECT().WriteSriovResult(&hosttypes.SriovResult{SyncStatus: consts.SyncStatusSucceeded})

		Expect(runServiceCmd(&cobra.Command{}, []string{})).NotTo(HaveOccurred())
		Expect(testCtrl.Satisfied()).To(BeTrue())
	})

	It("Post phase - wrong result of the pre phase", func() {
		phaseArg = PhasePost
		hostHelpers.EXPECT().ReadConfFile().Return(getTestSriovInterfaceConfig(1), nil)
		hostHelpers.EXPECT().ReadSriovSupportedNics().Return(testSriovSupportedNicIDs, nil)
		hostHelpers.EXPECT().ReadSriovResult().Return(getTestResultFileContent("Failed", "pretest"), nil)
		hostHelpers.EXPECT().WriteSriovResult(&hosttypes.SriovResult{SyncStatus: consts.SyncStatusFailed, LastSyncError: "post: unexpected result of the pre phase: Failed, syncError: pretest"})

		Expect(runServiceCmd(&cobra.Command{}, []string{})).To(HaveOccurred())
	})
	It("waitForDevicesInitialization", func() {
		cfg := &hosttypes.SriovConfig{Spec: sriovnetworkv1.SriovNetworkNodeStateSpec{
			Interfaces: []sriovnetworkv1.Interface{
				{Name: "name1", PciAddress: "0000:d8:00.0"},
				{Name: "name2", PciAddress: "0000:d8:00.1"}}}}
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.0").Return("other")
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.1").Return("")
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.0").Return("name1")
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.1").Return("")
		hostHelpers.EXPECT().TryGetInterfaceName("0000:d8:00.1").Return("name2")
		hostHelpers.EXPECT().WaitUdevEventsProcessed(60).Return(nil)
		sc, err := newServiceConfig(logr.Discard())
		Expect(err).ToNot(HaveOccurred())
		sc.sriovConfig = cfg
		sc.waitForDevicesInitialization()
	})
})

type MockHostHelper struct {
	mock.Mock
	helper.HostHelpersInterface
}

type MockVendorPlugin struct {
	mock.Mock
	plugin.VendorPlugin
}

func TestGetPlugin(t *testing.T) {
	tests := []struct {
		name          string
		platformType  consts.PlatformTypes
		phase         string
		expectedError error
		setupMock     func(*MockHostHelper)
		mockPlugin    func() (plugin.VendorPlugin, error)
	}{
		{
			name:         "Baremetal Pre phase - success",
			platformType: consts.Baremetal,
			phase:        PhasePre,
			setupMock:    func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return &MockVendorPlugin{}, nil
			},
		},
		{
			name:         "Baremetal Post phase - success",
			platformType: consts.Baremetal,
			phase:        PhasePost,
			setupMock:    func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return &MockVendorPlugin{}, nil
			},
		},
		{
			name:         "VirtualOpenStack Pre phase - success",
			platformType: consts.VirtualOpenStack,
			phase:        PhasePre,
			setupMock:    func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return &MockVendorPlugin{}, nil
			},
		},
		{
			name:          "VirtualOpenStack Post phase - nil plugin",
			platformType:  consts.VirtualOpenStack,
			phase:         PhasePost,
			expectedError: nil,
			setupMock:     func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return nil, nil
			},
		},
		{
			name:         "VirtualQemuKVM Pre phase - success",
			platformType: consts.VirtualQemuKVM,
			phase:        PhasePre,
			setupMock:    func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return &MockVendorPlugin{}, nil
			},
		},
		{
			name:          "VirtualQemuKVM Post phase - nil plugin",
			platformType:  consts.VirtualQemuKVM,
			phase:         PhasePost,
			expectedError: nil,
			setupMock:     func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return nil, nil
			},
		},
		{
			name:          "Unknown platform type - error",
			platformType:  consts.PlatformTypes(99),
			phase:         PhasePre,
			expectedError: errors.New("unknown platform type"),
			setupMock:     func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return nil, nil
			},
		},
		{
			name:          "Generic plugin creation error",
			platformType:  consts.Baremetal,
			phase:         PhasePre,
			expectedError: errors.New("failed to create generic plugin"),
			setupMock:     func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return nil, errors.New("generic plugin error")
			},
		},
		{
			name:          "Virtual plugin creation error",
			platformType:  consts.VirtualOpenStack,
			phase:         PhasePre,
			expectedError: errors.New("failed to create virtual plugin"),
			setupMock:     func(m *MockHostHelper) {},
			mockPlugin: func() (plugin.VendorPlugin, error) {
				return nil, errors.New("virtual plugin error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockHostHelper := new(MockHostHelper)
			if tt.setupMock != nil {
				tt.setupMock(mockHostHelper)
			}

			// Mock plugin creation functions
			originalNewGenericPluginFunc := newGenericPluginFunc
			newGenericPluginFunc = func(helper helper.HostHelpersInterface, opts ...generic.Option) (plugin.VendorPlugin, error) {
				return tt.mockPlugin()
			}
			defer func() { newGenericPluginFunc = originalNewGenericPluginFunc }()

			originalNewVirtualPluginFunc := newVirtualPluginFunc
			newVirtualPluginFunc = func(helper helper.HostHelpersInterface) (plugin.VendorPlugin, error) {
				return tt.mockPlugin()
			}
			defer func() { newVirtualPluginFunc = originalNewVirtualPluginFunc }()

			serviceConfig := &ServiceConfig{
				hostHelper: mockHostHelper,
				log:        logr.Discard(),
				sriovConfig: &hosttypes.SriovConfig{
					PlatformType: tt.platformType,
				},
			}

			// Execute
			plugin, err := serviceConfig.getPlugin(tt.phase)

			// Verify
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, plugin)
			} else {
				assert.NoError(t, err)
				if tt.phase == PhasePost && (tt.platformType == consts.VirtualOpenStack || tt.platformType == consts.VirtualQemuKVM) {
					assert.Nil(t, plugin)
				} else {
					assert.NotNil(t, plugin)
				}
			}

			mockHostHelper.AssertExpectations(t)
		})
	}
}

func TestGetPluginEdgeCases(t *testing.T) {
	t.Run("Nil service config", func(t *testing.T) {
		plugin, err := (*ServiceConfig)(nil).getPlugin(PhasePre)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil service config")
		assert.Nil(t, plugin)
	})

	t.Run("Nil sriovConfig", func(t *testing.T) {
		serviceConfig := &ServiceConfig{
			hostHelper:  new(MockHostHelper),
			log:         logr.Discard(),
			sriovConfig: nil,
		}

		plugin, err := serviceConfig.getPlugin(PhasePre)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil sriov config")
		assert.Nil(t, plugin)
	})

	t.Run("Invalid phase", func(t *testing.T) {
		serviceConfig := &ServiceConfig{
			hostHelper: new(MockHostHelper),
			log:        logr.Discard(),
			sriovConfig: &hosttypes.SriovConfig{
				PlatformType: consts.Baremetal,
			},
		}

		plugin, err := serviceConfig.getPlugin("invalid-phase")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid phase")
		assert.Nil(t, plugin)
	})
}
