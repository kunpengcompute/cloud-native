package containerd_test

import (
	"context"
	"os"
	"path"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/containerd"
	"kunpeng.huawei.com/kunpeng-cloud-computing/test/kunpeng-tap/fake"
)

var _ = Describe("Server", func() {
	var (
		containerRuntimeConn     *grpc.ClientConn
		fakeDispatcher           *fake.FakeDispatcher
		fakeRuntimeServiceClient *fake.FakeRuntimeServiceClient
		server                   server.ProxyServer
	)

	BeforeEach(func() {

		options.RuntimeProxyEndpoint = path.Join(utSocketPathPrefix, "runtimeproxy.sock")
		if _, err := os.Stat(options.RuntimeProxyEndpoint); err == nil {
			err := syscall.Unlink(options.RuntimeProxyEndpoint)
			Expect(err).To(BeNil())
			os.Remove(options.RuntimeProxyEndpoint)
		}

		var err error
		containerRuntimeConn, err = grpc.NewClient(
			options.GRPCPassthroughScheme+options.ContainerRuntimeEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(containerd.Dialer),
		)
		Expect(err).To(BeNil())

		fakeDispatcher = &fake.FakeDispatcher{}
		fakeRuntimeServiceClient = &fake.FakeRuntimeServiceClient{}
		fakeRuntimeServiceClient.VersionReturns(&runtimeapi.VersionResponse{
			RuntimeName:       "containerd",
			RuntimeVersion:    "1.0.0",
			RuntimeApiVersion: "v1",
		}, nil)

		server = containerd.NewContainerdServer(containerd.NewCriServer(
			fakeRuntimeServiceClient, fakeDispatcher,
		), containerRuntimeConn)

	})

	AfterEach(func() {
		containerRuntimeConn.Close()
	})

	Describe("Run", func() {
		It("should start the server and accept connections", func() {
			go func() {
				err := server.Run()
				Expect(err).To(BeNil())
				Eventually(func() error {
					_, err := os.Stat(options.RuntimeProxyEndpoint)
					return err
				}, timeout, interval).Should(BeNil())
			}()

			// Wait for the server to start
			time.Sleep(100 * time.Millisecond)

			conn, err := grpc.Dial("unix://"+options.RuntimeProxyEndpoint, grpc.WithInsecure())
			Expect(err).To(BeNil())
			defer conn.Close()

			client := runtimeapi.NewRuntimeServiceClient(conn)
			versionResp, err := client.Version(context.Background(), &runtimeapi.VersionRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(versionResp.RuntimeVersion).To(Equal("1.0.0"))
			server.Shutdown(context.Background())
		})
	})
})
