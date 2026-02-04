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
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"syscall"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker"
	"kunpeng.huawei.com/kunpeng-cloud-computing/test/kunpeng-tap/fake"
)

var _ = Describe("Server", Ordered, func() {
	var server server.ProxyServer
	var unixHttpClient http.Client
	BeforeAll(func() {
		FakeDockerHandler := &fake.FakeDockerHandler{}
		FakeDockerHandler.HandleCreateContainerReturns(func(resp http.ResponseWriter, req *http.Request) {
			vars := mux.Vars(req)

			resp.WriteHeader(http.StatusOK)
			resp.Write([]byte("create container" + vars["dockerversion"]))
		})
		FakeDockerHandler.HandleStartContainerReturns(func(resp http.ResponseWriter, req *http.Request) {
			vars := mux.Vars(req)

			resp.WriteHeader(http.StatusOK)
			resp.Write([]byte("start container" + vars["dockerversion"] + vars["containerid"]))
		})
		FakeDockerHandler.HandleStopContainerReturns(func(resp http.ResponseWriter, req *http.Request) {
			vars := mux.Vars(req)

			resp.WriteHeader(http.StatusOK)
			resp.Write([]byte("stop container" + vars["dockerversion"] + vars["containerid"]))
		})
		FakeDockerHandler.HandleUpdateContainerReturns(func(resp http.ResponseWriter, req *http.Request) {
			vars := mux.Vars(req)

			resp.WriteHeader(http.StatusOK)
			resp.Write([]byte("update container" + vars["dockerversion"] + vars["containerid"]))
		})
		FakeDockerHandler.HandleDeleteContainerReturns(func(resp http.ResponseWriter, req *http.Request) {
			vars := mux.Vars(req)

			resp.WriteHeader(http.StatusOK)
			resp.Write([]byte("delete container" + vars["dockerversion"] + vars["containerid"]))
		})
		FakeDockerHandler.DirectReturns("")

		options.RuntimeProxyEndpoint = path.Join(utSocketPathPrefix, "runtimeproxy.sock")
		if _, err := os.Stat(options.RuntimeProxyEndpoint); err == nil {
			err := syscall.Unlink(options.RuntimeProxyEndpoint)
			Expect(err).To(BeNil())
			os.Remove(options.RuntimeProxyEndpoint)
		}

		server = docker.NewDockerServer(FakeDockerHandler, nil)
		Expect(server).NotTo(BeNil())

		go func() {
			server.Run()
		}()
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
		server.Shutdown(context.Background())
		os.Remove(options.RuntimeProxyEndpoint)
	})

	Describe("Call create container url", func() {
		It("should be pass", func() {
			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/create", "", nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("create container"))
			Expect(body).To(ContainSubstring("v1.39"))
		})
	})
	Describe("Call stop container url", func() {
		It("should be pass", func() {
			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/666/stop", "", nil)
			Expect(err).NotTo(HaveOccurred())
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("stop container"))
			Expect(body).To(ContainSubstring("v1.39"))
			Expect(body).To(ContainSubstring("666"))
		})
	})
	Describe("Call start container url", func() {
		It("should be pass", func() {
			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/666/start", "", nil)
			Expect(err).NotTo(HaveOccurred())
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("start container"))
			Expect(body).To(ContainSubstring("v1.39"))
			Expect(body).To(ContainSubstring("666"))
		})
	})
	Describe("Call update container url", func() {
		It("should be pass", func() {
			resp, err := unixHttpClient.Post("http://unix/v1.39/containers/666/update", "", nil)
			Expect(err).NotTo(HaveOccurred())
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("update container"))
			Expect(body).To(ContainSubstring("v1.39"))
			Expect(body).To(ContainSubstring("666"))

		})
	})
	Describe("Cal delete container url", func() {
		It("should be pass", func() {
			req, err := http.NewRequest("DELETE", "http://unix/v1.39/containers/666", nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err := unixHttpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("delete container"))
			Expect(body).To(ContainSubstring("v1.39"))
			Expect(body).To(ContainSubstring("666"))
		})
	})
	Describe("Call healthz url", func() {
		It("should be pass", func() {
			resp, err := unixHttpClient.Get("http://unix/healthz")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

		})
	})
	Describe("Call any other urls", func() {
		It("should be pass", func() {
			resp, err := unixHttpClient.Get("http://unix/any")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp, err = unixHttpClient.Get("http://unix/any/other")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp, err = unixHttpClient.Get("http://unix/")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
