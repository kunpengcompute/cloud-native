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

package policymanager

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"

	runtimeapi "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

type PolicyManager interface {
	Setup() error
	Start() error
	Stop()
}

type policyManager struct {
	listener net.Listener // socket our gRPC server listens on
	server   *grpc.Server // our gRPC server
	endpoint string
	Hooks    policy.HookManager
	runtimeapi.UnimplementedRuntimeHookServiceServer
}

func NewPolicyManager(endpoint string, hooks policy.HookManager) PolicyManager {
	return &policyManager{
		endpoint: endpoint,
		Hooks:    hooks,
	}
}

func (p *policyManager) Setup() error {
	if err := p.createRPCServer(); err != nil {
		return err
	}
	runtimeapi.RegisterRuntimeHookServiceServer(p.server, p)
	return nil
}

func (p *policyManager) Start() error {
	klog.InfoS("Starting runtime hook server", "endpoint", p.endpoint)
	go func() {
		p.server.Serve(p.listener)
	}()
	return nil
}

func (p *policyManager) Stop() {
	klog.InfoS("Stopping runtime hook server")
	p.server.Stop()
}

func (p *policyManager) createRPCServer() error {
	if p.server != nil {
		return nil
	}

	l, err := net.Listen("unix", p.endpoint)
	if err != nil {
		return fmt.Errorf("failed to create runtime hook server, error: %w", err)
	}
	p.listener = l
	p.server = grpc.NewServer()
	reflection.Register(p.server)
	return nil
}
