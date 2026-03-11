/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package docker_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/docker/docker/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/monitoring"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/dispatcher"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker/utils"
	"kunpeng.huawei.com/kunpeng-cloud-computing/test/kunpeng-tap/fake"
)

const (
	FakeCgroupParent = "/kubepods.slice/kubepods-podFakeCgroupParent.slice"
	ContentType      = "application/json"
)

var _ = Describe("Handler", Ordered, func() {
	var dockerClient *client.Client
	var dockerServer server.ProxyServer
	var unixHttpClient http.Client
	var proxyCache cache.Cache
	var fakeDockerRuntimerReceivedRequest []byte
	var err error

	BeforeAll(func() {
		By("create fake docker runtime")
		fakeDockerRuntimer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var data []byte
			var err error
			switch {
			case strings.Contains(r.URL.Path, "create"):
				if strings.Contains(r.URL.RawQuery, "pod") {
					w.WriteHeader(200)
					data, err = os.ReadFile("../../../../test/kunpeng-tap/fake/fake_docker_pod_http_create_resp.json")
					Expect(err).To(BeNil())
				} else {
					w.WriteHeader(200)
					data, err = os.ReadFile("../../../../test/kunpeng-tap/fake/fake_docker_http_create_resp.json")
					Expect(err).To(BeNil())
				}
				fakeDockerRuntimerReceivedRequest, err = io.ReadAll(r.Body)
				Expect(err).To(BeNil())

			case strings.Contains(r.URL.Path, "start"):
				w.WriteHeader(200)
				data, err = os.ReadFile("../../../../test/kunpeng-tap/fake/fake_docker_http_start_resp.json")
				Expect(err).To(BeNil())
			case strings.Contains(r.URL.Path, "stop"):
				w.WriteHeader(200)
				data, err = os.ReadFile("../../../../test/kunpeng-tap/fake/fake_docker_http_stop_resp.json")
				Expect(err).To(BeNil())
			case strings.Contains(r.URL.Path, "update"):
				w.WriteHeader(200)
				data, err = os.ReadFile("../../../../test/kunpeng-tap/fake/fake_docker_http_update_resp.json")
				Expect(err).To(BeNil())
			case r.Method == http.MethodDelete:
				w.WriteHeader(200)
			}

			w.Write(data)
		}))

		fakeContainerRuntimeSocketPath := path.Join(utSocketPathPrefix, "containerruntime.sock")
		if _, err := os.Stat(fakeContainerRuntimeSocketPath); err == nil {
			err := syscall.Unlink(fakeContainerRuntimeSocketPath)
			Expect(err).To(BeNil())
			os.Remove(fakeContainerRuntimeSocketPath)
		}

		By("create fake docker client")
		dockerClient, err = client.NewClientWithOpts(client.WithHost("unix://"+fakeContainerRuntimeSocketPath), client.WithVersion("1.39"))
		Expect(err).To(BeNil())

		l, err := net.Listen("unix", fakeContainerRuntimeSocketPath)
		Expect(err).To(BeNil())
		fakeDockerRuntimer.Listener = l
		fakeDockerRuntimer.Start()

		By("wait fake docker runtime start")
		Eventually(func() error {
			_, err := os.Stat(fakeContainerRuntimeSocketPath)
			return err
		}, timeout, interval).Should(BeNil())

		By("init fake RuntimeHookServiceClient")
		fakeRuntimeHookServiceClient := &fake.FakeRuntimeHookServiceClient{}
		fakeRuntimeHookServiceClient.PostStartContainerHookReturns(nil, nil)
		fakeRuntimeHookServiceClient.PostStopContainerHookReturns(nil, nil)
		fakeRuntimeHookServiceClient.PostStopPodSandboxHookReturns(nil, nil)
		fakeRuntimeHookServiceClient.PreCreateContainerHookReturns(&v1alpha1.ContainerResourceHookResponse{
			PodCgroupParent: FakeCgroupParent,
		}, nil)
		fakeRuntimeHookServiceClient.PreStartContainerHookReturns(nil, nil)
		fakeRuntimeHookServiceClient.PreRunPodSandboxHookReturns(&v1alpha1.PodSandboxHookResponse{
			CgroupParent: FakeCgroupParent,
		}, nil)
		fakeRuntimeHookServiceClient.PreUpdateContainerResourcesHookReturns(nil, nil)

		By("create dockerServer")
		proxyCache, err = cache.NewCache(cache.Options{CacheDir: path.Join(utSocketPathPrefix, "/topology_cache")})
		Expect(err).To(BeNil())

		dispatcher := dispatcher.NewDispatcher(
			fakeRuntimeHookServiceClient, proxyCache,
		)
		dispatcher.SetDockerCgroupDriver("cgroupDriver")
		dockerHandler := docker.NewDockerHandler(
			docker.ReverseProxy(fakeContainerRuntimeSocketPath), proxyCache, dockerClient,
			dispatcher,
		)

		options.RuntimeProxyEndpoint = path.Join(utSocketPathPrefix, "runtimeproxy.sock")
		if _, err := os.Stat(options.RuntimeProxyEndpoint); err == nil {
			err := syscall.Unlink(options.RuntimeProxyEndpoint)
			Expect(err).To(BeNil())
			os.Remove(options.RuntimeProxyEndpoint)
		}
		dockerServer = docker.NewDockerServer(dockerHandler, nil)
		go dockerServer.Run()

		By("wait dockerServer start")
		Eventually(func() error {
			_, err := os.Stat(options.RuntimeProxyEndpoint)
			return err
		}, timeout, interval).Should(BeNil())

		unixHttpClient = http.Client{
			Transport: &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					return net.Dial("unix", options.RuntimeProxyEndpoint)
				},
			},
		}
	})

	AfterAll(func() {
		dockerServer.Shutdown(context.Background())
		os.Remove(options.RuntimeProxyEndpoint)
	})

	Describe("Create pod", func() {
		It("should return 200", func() {
			file, err := os.Open("../../../../test/kunpeng-tap/fake/fake_docker_pod_http_create_request.json")
			Expect(err).To(BeNil())
			defer file.Close()
			requestInFile, err := io.ReadAll(file)
			Expect(err).To(BeNil())
			_, err = file.Seek(0, 0)
			Expect(err).To(BeNil())

			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/create?name=0_containername_podname_podnamespace_podid_5", ContentType, file)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("Id"))
			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal(FakeCgroupParent))

			Expect(CompareRequest(requestInFile, fakeDockerRuntimerReceivedRequest)).To(BeFalse())
		})
	})

	// This test simulates the scenario where the --metrics-bind-address flag is explicitly set (non-empty),
	// which enables the Prometheus metrics server (see startMonitor in cmd/kunpeng-tap/proxy/main.go).
	// Prerequisites:
	//   1. A free TCP port must be available on the host for binding the metrics HTTP server.
	//   2. The default Go HTTP mux (http.DefaultServeMux) must not have "/metrics" already registered,
	//      so only one such metrics server can be started per test process.
	Describe("Metrics endpoint when enabled", func() {
		BeforeAll(func() {
			By("find a free port and start metrics server on 127.0.0.1")
			l, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).To(BeNil())
			var port string
			_, port, err = net.SplitHostPort(l.Addr().String())
			Expect(err).To(BeNil())
			l.Close()
			options.MetricsAddr = "127.0.0.1:" + port
			go monitoring.ExportMetrics()

			By("wait for metrics server to be ready")
			Eventually(func() error {
				resp, err := http.Get("http://" + options.MetricsAddr + "/metrics")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, timeout, interval).Should(BeNil())
		})

		It("should expose prometheus metrics", func() {
			metricsResp, err := http.Get("http://" + options.MetricsAddr + "/metrics")
			Expect(err).NotTo(HaveOccurred())
			defer metricsResp.Body.Close()
			metricsBody, err := io.ReadAll(metricsResp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(metricsBody)).Should(ContainSubstring("tap_proxy_request_duration_seconds"))
		})
	})

	Describe("Create container", func() {
		It("should return 200", func() {
			file, err := os.Open("../../../../test/kunpeng-tap/fake/fake_docker_http_create_request.json")
			Expect(err).To(BeNil())
			defer file.Close()
			requestInFile, err := io.ReadAll(file)
			Expect(err).To(BeNil())
			_, err = file.Seek(0, 0)
			Expect(err).To(BeNil())

			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/create?name=0_containername_2_3_4_5", ContentType, file)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("Id"))
			containers := proxyCache.GetContainers()
			Expect(len(containers)).To(Equal(1))
			Expect(containers[0].GetID()).To(Equal("containerid"))

			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal(FakeCgroupParent))

			Expect(CompareRequest(requestInFile, fakeDockerRuntimerReceivedRequest)).To(BeFalse())
		})
	})

	Describe("Start container", func() {
		It("should return 200", func() {
			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/c2ada9df5af8/start", ContentType, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("message"))

			containers := proxyCache.GetContainers()
			Expect(len(containers)).To(Equal(1))
			Expect(containers[0].GetID()).To(Equal("containerid"))
			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal(FakeCgroupParent))
		})
	})
	Describe("Stop container", func() {
		It("should return 200", func() {
			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/c2ada9df5af8/stop", ContentType, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("message"))

			containers := proxyCache.GetContainers()
			Expect(len(containers)).To(Equal(1))
			Expect(containers[0].GetID()).To(Equal("containerid"))
			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal(FakeCgroupParent))
		})
	})
	Describe("Update container", func() {
		It("should return 200", func() {
			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/c2ada9df5af8/update", ContentType, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("Warnings"))

			containers := proxyCache.GetContainers()
			Expect(len(containers)).To(Equal(1))
			Expect(containers[0].GetID()).To(Equal("containerid"))
			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal(FakeCgroupParent))
		})
	})
	Describe("Delete container", func() {
		It("should return 200", func() {
			req, err := http.NewRequest("DELETE", "http://unix/v1.39/containers/containerid", nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err := unixHttpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			containers := proxyCache.GetContainers()
			Expect(len(containers)).To(Equal(0))

			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal(FakeCgroupParent))
		})
	})
	Describe("Delete pod", func() {
		It("should return 200", func() {
			req, err := http.NewRequest("DELETE", "http://unix/v1.39/containers/podid", nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err := unixHttpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			containers := proxyCache.GetContainers()
			Expect(len(containers)).To(Equal(0))

			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(0))
		})
	})

	Describe("Create pod with skip label", func() {
		It("should return 200", func() {
			file, err := os.Open("../../../../test/kunpeng-tap/fake/fake_docker_pod_skip_http_create_request.json")
			Expect(err).To(BeNil())
			defer file.Close()
			requestInFile, err := io.ReadAll(file)
			Expect(err).To(BeNil())
			_, err = file.Seek(0, 0)
			Expect(err).To(BeNil())

			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/create?name=0_containername_podname_podnamespace_podid_5", ContentType, file)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("Id"))
			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal("FakeCgroupParent"))

			Expect(CompareRequest(requestInFile, fakeDockerRuntimerReceivedRequest)).To(BeTrue())
		})
	})

	Describe("Create container with skip label", func() {
		It("should return 200", func() {
			file, err := os.Open("../../../../test/kunpeng-tap/fake/fake_docker_skip_http_create_request.json")
			Expect(err).To(BeNil())
			defer file.Close()
			requestInFile, err := io.ReadAll(file)
			Expect(err).To(BeNil())
			_, err = file.Seek(0, 0)
			Expect(err).To(BeNil())

			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/create?name=0_containername_2_3_4_5", ContentType, file)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("Id"))
			containers := proxyCache.GetContainers()
			Expect(len(containers)).To(Equal(1))
			Expect(containers[0].GetID()).To(Equal("containerid"))

			pods := proxyCache.GetPods()
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].GetID()).To(Equal("podid"))
			Expect(pods[0].GetCgroupParentDir()).To(Equal("FakeCgroupParent"))
			Expect(CompareRequest(requestInFile, fakeDockerRuntimerReceivedRequest)).To(BeTrue())

		})
	})

})

func CompareRequest(request1, request2 []byte) bool {
	request1Body := &utils.ConfigWrapper{}
	request2Body := &utils.ConfigWrapper{}

	err := json.Unmarshal(request1, request1Body)
	Expect(err).To(BeNil())
	err = json.Unmarshal(request2, request2Body)
	Expect(err).To(BeNil())

	formattedRequest1, err := json.Marshal(request1Body)
	Expect(err).To(BeNil())
	formattedrequest2, err := json.Marshal(request2Body)
	Expect(err).To(BeNil())

	return string(formattedRequest1) == string(formattedrequest2)
}
