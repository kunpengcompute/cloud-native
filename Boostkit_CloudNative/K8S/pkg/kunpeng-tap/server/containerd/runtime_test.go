package containerd_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/containerd"
	"kunpeng.huawei.com/kunpeng-cloud-computing/test/kunpeng-tap/fake"
)

var _ = Describe("CriServer", func() {
	var (
		fakeDispatcher           *fake.FakeDispatcher
		fakeRuntimeServiceClient *fake.FakeRuntimeServiceClient
		ctx                      context.Context
	)

	BeforeEach(func() {
		fakeDispatcher = &fake.FakeDispatcher{}
		fakeRuntimeServiceClient = &fake.FakeRuntimeServiceClient{}
		ctx = context.TODO()
	})

	Describe("Version", func() {
		It("should return the runtime version", func() {
			fakeRuntimeServiceClient.VersionReturns(&runtimeapi.VersionResponse{
				RuntimeName:       "containerd",
				RuntimeVersion:    "1.0.0",
				RuntimeApiVersion: "v1",
			}, nil)

			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)
			resp, err := criServer.Version(ctx, &runtimeapi.VersionRequest{})
			Expect(err).To(BeNil())
			Expect(resp.RuntimeName).To(Equal("containerd"))
			Expect(resp.RuntimeVersion).To(Equal("1.0.0"))
		})
	})

	Describe("RunPodSandbox", func() {
		It("should intercept and forward the RunPodSandbox request", func() {
			req := &runtimeapi.RunPodSandboxRequest{
				Config: &runtimeapi.PodSandboxConfig{
					Metadata: &runtimeapi.PodSandboxMetadata{
						Name: "test",
					},
				}}
			fakeDispatcher.InterceptRuntimeRequestReturns(&runtimeapi.RunPodSandboxResponse{PodSandboxId: "sandbox123"}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.RunPodSandbox(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp.PodSandboxId).To(Equal("sandbox123"))
		})
	})

	Describe("StopPodSandbox", func() {
		It("should intercept and forward the StopPodSandbox request", func() {
			req := &runtimeapi.StopPodSandboxRequest{PodSandboxId: "sandbox123"}
			fakeDispatcher.InterceptRuntimeRequestReturns(&runtimeapi.StopPodSandboxResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.StopPodSandbox(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("RemovePodSandbox", func() {
		It("should forward the RemovePodSandbox request", func() {
			req := &runtimeapi.RemovePodSandboxRequest{PodSandboxId: "sandbox123"}
			fakeRuntimeServiceClient.RemovePodSandboxReturns(&runtimeapi.RemovePodSandboxResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.RemovePodSandbox(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("PodSandboxStatus", func() {
		It("should forward the PodSandboxStatus request", func() {
			req := &runtimeapi.PodSandboxStatusRequest{PodSandboxId: "sandbox123"}
			fakeRuntimeServiceClient.PodSandboxStatusReturns(&runtimeapi.PodSandboxStatusResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.PodSandboxStatus(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("ListPodSandbox", func() {
		It("should forward the ListPodSandbox request", func() {
			req := &runtimeapi.ListPodSandboxRequest{}
			fakeRuntimeServiceClient.ListPodSandboxReturns(&runtimeapi.ListPodSandboxResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.ListPodSandbox(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("CreateContainer", func() {
		It("should intercept and forward the CreateContainer request", func() {
			req := &runtimeapi.CreateContainerRequest{
				Config: &runtimeapi.ContainerConfig{
					Metadata: &runtimeapi.ContainerMetadata{
						Name: "test",
					},
				},
				SandboxConfig: &runtimeapi.PodSandboxConfig{
					Metadata: &runtimeapi.PodSandboxMetadata{
						Name: "test",
					},
				},
			}
			fakeDispatcher.InterceptRuntimeRequestReturns(&runtimeapi.CreateContainerResponse{ContainerId: "container123"}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.CreateContainer(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp.ContainerId).To(Equal("container123"))
		})
	})

	Describe("StartContainer", func() {
		It("should forward the StartContainer request", func() {
			req := &runtimeapi.StartContainerRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.StartContainerReturns(&runtimeapi.StartContainerResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.StartContainer(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("StopContainer", func() {
		It("should intercept and forward the StopContainer request", func() {
			req := &runtimeapi.StopContainerRequest{ContainerId: "container123"}
			fakeDispatcher.InterceptRuntimeRequestReturns(&runtimeapi.StopContainerResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.StopContainer(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("RemoveContainer", func() {
		It("should forward the RemoveContainer request", func() {
			req := &runtimeapi.RemoveContainerRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.RemoveContainerReturns(&runtimeapi.RemoveContainerResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.RemoveContainer(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("ContainerStatus", func() {
		It("should forward the ContainerStatus request", func() {
			req := &runtimeapi.ContainerStatusRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.ContainerStatusReturns(&runtimeapi.ContainerStatusResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.ContainerStatus(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("ListContainers", func() {
		It("should forward the ListContainers request", func() {
			req := &runtimeapi.ListContainersRequest{}
			fakeRuntimeServiceClient.ListContainersReturns(&runtimeapi.ListContainersResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.ListContainers(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("UpdateContainerResources", func() {
		It("should forward the UpdateContainerResources request", func() {
			req := &runtimeapi.UpdateContainerResourcesRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.UpdateContainerResourcesReturns(&runtimeapi.UpdateContainerResourcesResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.UpdateContainerResources(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("ContainerStats", func() {
		It("should forward the ContainerStats request", func() {
			req := &runtimeapi.ContainerStatsRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.ContainerStatsReturns(&runtimeapi.ContainerStatsResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.ContainerStats(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("ListContainerStats", func() {
		It("should forward the ListContainerStats request", func() {
			req := &runtimeapi.ListContainerStatsRequest{}
			fakeRuntimeServiceClient.ListContainerStatsReturns(&runtimeapi.ListContainerStatsResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.ListContainerStats(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("ReopenContainerLog", func() {
		It("should forward the ReopenContainerLog request", func() {
			req := &runtimeapi.ReopenContainerLogRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.ReopenContainerLogReturns(&runtimeapi.ReopenContainerLogResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.ReopenContainerLog(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("ExecSync", func() {
		It("should forward the ExecSync request", func() {
			req := &runtimeapi.ExecSyncRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.ExecSyncReturns(&runtimeapi.ExecSyncResponse{Stdout: []byte("output")}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.ExecSync(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp.Stdout).To(Equal([]byte("output")))
		})
	})

	Describe("Exec", func() {
		It("should forward the Exec request", func() {
			req := &runtimeapi.ExecRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.ExecReturns(&runtimeapi.ExecResponse{Url: "http://localhost"}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.Exec(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp.Url).To(Equal("http://localhost"))
		})
	})

	Describe("Attach", func() {
		It("should forward the Attach request", func() {
			req := &runtimeapi.AttachRequest{ContainerId: "container123"}
			fakeRuntimeServiceClient.AttachReturns(&runtimeapi.AttachResponse{Url: "http://localhost"}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.Attach(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp.Url).To(Equal("http://localhost"))
		})
	})

	Describe("PortForward", func() {
		It("should forward the PortForward request", func() {
			req := &runtimeapi.PortForwardRequest{PodSandboxId: "sandbox123"}
			fakeRuntimeServiceClient.PortForwardReturns(&runtimeapi.PortForwardResponse{Url: "http://localhost"}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.PortForward(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp.Url).To(Equal("http://localhost"))
		})
	})

	Describe("RuntimeConfig", func() {
		It("should forward the RuntimeConfig request", func() {
			req := &runtimeapi.RuntimeConfigRequest{}
			fakeRuntimeServiceClient.RuntimeConfigReturns(&runtimeapi.RuntimeConfigResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.RuntimeConfig(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("PodSandboxStats", func() {
		It("should forward the PodSandboxStats request", func() {
			req := &runtimeapi.PodSandboxStatsRequest{PodSandboxId: "sandbox123"}
			fakeRuntimeServiceClient.PodSandboxStatsReturns(&runtimeapi.PodSandboxStatsResponse{}, nil)
			criServer := containerd.NewCriServer(fakeRuntimeServiceClient, fakeDispatcher)

			resp, err := criServer.PodSandboxStats(ctx, req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())
		})
	})
})
