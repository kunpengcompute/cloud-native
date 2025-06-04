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

package containerd

import (
	"context"
	"net"

	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
)

type ContainerdServer struct {
	grpcServer *grpc.Server
}

func Dialer(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", addr)
}

func NewContainerdServer(
	criServer *criServer,
	remoteContainerRuntimeConn *grpc.ClientConn,
) server.ProxyServer {
	klog.InfoS("Creating and Registering containerd server")

	// For unsupported requests, pass through directly to the backend
	director := func(ctx context.Context, fullName string) (context.Context, *grpc.ClientConn, error) {
		return ctx, remoteContainerRuntimeConn, nil
	}
	grpcServer := grpc.NewServer(
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
	)

	runtimeapi.RegisterRuntimeServiceServer(grpcServer, criServer)

	return &ContainerdServer{
		grpcServer: grpcServer,
	}
}

func (c *ContainerdServer) Run() error {
	klog.InfoS("Starting containerd server", "endpoint", options.RuntimeProxyEndpoint)

	lis, err := net.Listen("unix", options.RuntimeProxyEndpoint)
	if err != nil {
		klog.ErrorS(err, "Failed to create the listener")
		return err
	}

	if err := c.grpcServer.Serve(lis); err != nil {
		klog.ErrorS(err, "ListenAndServe failed")
		return err
	}

	return nil
}

func (c *ContainerdServer) Shutdown(ctx context.Context) {
	c.grpcServer.Stop()
}
