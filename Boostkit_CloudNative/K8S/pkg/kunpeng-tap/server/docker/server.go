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

package docker

import (
	"context"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/httputil"
)

func ReverseProxy(containerRuntimeEndpoint string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			param := ""
			if len(req.URL.RawQuery) > 0 {
				param = "?" + req.URL.RawQuery
			}
			u, err := url.Parse("http://docker" + req.URL.Path + param)
			if err != nil {
				klog.ErrorS(err, "Failed to parse URL", "url", "http://docker"+req.URL.Path+param)
			}
			*req.URL = *u
		},
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", containerRuntimeEndpoint)
			}},
	}
}

type DockerServer struct {
	httpServer http.Server
}

func NewDockerServer(
	dockerHandler DockerHandler,
) server.ProxyServer {
	klog.InfoS("Creating and Registering docker server")
	router := mux.NewRouter()

	router.HandleFunc("/{dockerversion}/containers/create", dockerHandler.HandleCreateContainer()).Methods("POST")
	router.HandleFunc("/{dockerversion}/containers/{containerid}/update", dockerHandler.HandleUpdateContainer()).Methods("POST")
	router.HandleFunc("/{dockerversion}/containers/{containerid}/start", dockerHandler.HandleStartContainer()).Methods("POST")
	router.HandleFunc("/{dockerversion}/containers/{containerid}/stop", dockerHandler.HandleStopContainer()).Methods("POST")
	router.HandleFunc("/{dockerversion}/containers/{containerid}", dockerHandler.HandleDeleteContainer()).Methods("DELETE")
	router.HandleFunc("/healthz", func(wr http.ResponseWriter, req *http.Request) {
		wr.WriteHeader(http.StatusOK)
	})

	router.PathPrefix("/").HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		dockerHandler.Direct(wr, req)
	})
	return &DockerServer{
		httpServer: http.Server{
			Handler: router,
		},
	}
}

func (d *DockerServer) Run() error {
	klog.InfoS("Starting docker server", "endpoint", options.RuntimeProxyEndpoint)

	lis, err := net.Listen("unix", options.RuntimeProxyEndpoint)
	if err != nil {
		klog.ErrorS(err, "Failed to create the listener")
		return err
	}

	if err := d.httpServer.Serve(lis); err != nil {
		klog.ErrorS(err, "ListenAndServe failed")
		return err
	}

	return nil
}

func (d *DockerServer) Shutdown(ctx context.Context) {
	if err := d.httpServer.Shutdown(ctx); err != nil {
		klog.ErrorS(err, "Failed to shutdown docker server gracefully")
	}
}
