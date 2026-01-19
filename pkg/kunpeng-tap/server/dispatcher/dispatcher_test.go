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

package dispatcher_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/dispatcher"
	"kunpeng.huawei.com/kunpeng-cloud-computing/test/kunpeng-tap/fake"
)

var _ = Describe("Dispatcher", func() {
	var (
		fakeHookManager *fake.FakeRuntimeHookServiceClient
		fakeCache       *fake.FakeCache
		d               dispatcher.Dispatcher
		ctx             context.Context
	)

	BeforeEach(func() {
		fakeHookManager = &fake.FakeRuntimeHookServiceClient{}
		fakeCache = &fake.FakeCache{}
		d = dispatcher.NewDispatcher(fakeHookManager, fakeCache)
		ctx = context.TODO()
	})

	Describe("InterceptRuntimeRequest", func() {
		It("should handle a successful hook and runtime request", func() {
			hookType := policy.PreCreateContainer
			request := &v1alpha1.ContainerResourceHookRequest{}
			labels := map[string]string{}
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return "runtime response", nil
			}

			fakeHookManager.PreCreateContainerHookStub = func(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
				return &v1alpha1.ContainerResourceHookResponse{}, nil
			}

			resp, err := d.InterceptRuntimeRequest(ctx, hookType, request, labels, handler)
			Expect(err).To(BeNil())
			Expect(resp).To(Equal("runtime response"))
		})

		It("should return an error if the runtime handler fails", func() {
			hookType := policy.PreCreateContainer
			request := &v1alpha1.ContainerResourceHookRequest{}
			labels := map[string]string{}
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, errors.New("runtime error")
			}

			resp, err := d.InterceptRuntimeRequest(ctx, hookType, request, labels, handler)
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
		})
	})

	Describe("Dispatch", func() {
		It("should call the appropriate hook manager method", func() {
			hookType := policy.PreCreateContainer
			request := &v1alpha1.ContainerResourceHookRequest{}

			fakeHookManager.PreCreateContainerHookStub = func(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
				return &v1alpha1.ContainerResourceHookResponse{}, nil
			}

			resp := d.Dispatch(ctx, hookType, request, nil)
			Expect(resp).ToNot(BeNil())
		})

		It("should return an error if the hook manager fails", func() {
			hookType := policy.PreCreateContainer
			request := &v1alpha1.ContainerResourceHookRequest{}

			fakeHookManager.PreCreateContainerHookStub = func(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
				return nil, status.Errorf(codes.Internal, "hook error")
			}

			resp := d.Dispatch(ctx, hookType, request, nil)
			Expect(resp).To(BeNil())
		})

		It("should skip dispatch if SkipTAPHandle returns true", func() {
			labels := map[string]string{"tap.kunpeng.huawei.com/skip": "true"}
			resp := d.Dispatch(ctx, policy.PreRunPodSandbox, nil, labels)
			Expect(resp).To(BeNil())
		})
	})

	Describe("InsertIntoCacheIfNeed", func() {
		It("should insert a container into the cache", func() {
			containerResp := &runtimeapi.CreateContainerResponse{ContainerId: "container123"}
			hookReq := &v1alpha1.ContainerResourceHookRequest{}

			fakeCache.InsertContainerStub = func(containerId string, req interface{}) (cache.Container, error) {
				Expect(containerId).To(Equal("container123"))
				Expect(req).To(Equal(hookReq))
				return nil, nil
			}

			d.InsertIntoCacheIfNeed(containerResp, hookReq)
			Expect(fakeCache.InsertContainerCallCount()).To(Equal(1))
		})

		It("should insert a pod into the cache", func() {
			podResp := &runtimeapi.RunPodSandboxResponse{PodSandboxId: "pod123"}
			hookReq := &v1alpha1.PodSandboxHookRequest{}

			fakeCache.InsertPodStub = func(podId string, req interface{}, _ *cache.PodStatus) (cache.Pod, error) {
				Expect(podId).To(Equal("pod123"))
				Expect(req).To(Equal(hookReq))
				return nil, nil
			}

			d.InsertIntoCacheIfNeed(podResp, hookReq)
			Expect(fakeCache.InsertPodCallCount()).To(Equal(1))
		})
	})

	Describe("DeleteFromCacheIfNeed", func() {
		It("should delete a container from the cache", func() {
			request := &runtimeapi.StopContainerRequest{ContainerId: "container123"}

			fakeCache.DeleteContainerStub = func(containerId string) cache.Container {
				Expect(containerId).To(Equal("container123"))
				return nil
			}

			d.DeleteFromCacheIfNeed(request)
			Expect(fakeCache.DeleteContainerCallCount()).To(Equal(1))
		})

		It("should delete a pod from the cache", func() {
			request := &runtimeapi.StopPodSandboxRequest{PodSandboxId: "pod123"}

			fakeCache.DeletePodStub = func(podId string) cache.Pod {
				Expect(podId).To(Equal("pod123"))
				return nil
			}

			d.DeleteFromCacheIfNeed(request)
			Expect(fakeCache.DeletePodCallCount()).To(Equal(1))
		})
	})

	Describe("BackfillRequest", func() {
		It("should backfill a pod request", func() {
			proxyReq := &runtimeapi.RunPodSandboxRequest{}
			hookReq := &v1alpha1.PodSandboxHookRequest{}
			hookResp := &v1alpha1.PodSandboxHookResponse{
				Annotations: map[string]string{"key": "value"},
				Labels:      map[string]string{"label": "value"},
			}

			d.BackfillRequest(proxyReq, hookReq, hookResp)
			Expect(proxyReq.Config.Annotations).To(Equal(hookResp.Annotations))
			Expect(proxyReq.Config.Labels).To(Equal(hookResp.Labels))
		})

		It("should backfill a container request", func() {
			proxyReq := &runtimeapi.CreateContainerRequest{}
			hookReq := &v1alpha1.ContainerResourceHookRequest{}
			hookResp := &v1alpha1.ContainerResourceHookResponse{
				ContainerAnnotations: map[string]string{"key": "value"},
			}

			d.BackfillRequest(proxyReq, hookReq, hookResp)
			Expect(proxyReq.Config.Annotations).To(Equal(hookResp.ContainerAnnotations))
		})
	})

	Describe("ParseContainerRequest", func() {
		It("should parse a valid CreateContainerRequest", func() {
			podID := "pod123"
			containerName := "container123"
			pod := &fake.FakePod{}
			pod.GetNameReturns("test-pod")
			fakeCache.LookupPodStub = func(id string) (cache.Pod, bool) {
				Expect(id).To(Equal(podID))
				return pod, true
			}

			request := &runtimeapi.CreateContainerRequest{
				PodSandboxId: podID,
				Config: &runtimeapi.ContainerConfig{
					Metadata: &runtimeapi.ContainerMetadata{
						Name:    containerName,
						Attempt: 1,
					},
					Annotations: map[string]string{"container-annotation": "value"},
				},
			}

			hookReq := d.ParseContainerRequest(request)
			Expect(hookReq).ToNot(BeNil())
			hookRequest, ok := hookReq.(*v1alpha1.ContainerResourceHookRequest)
			Expect(ok).To(BeTrue())
			Expect(hookRequest.PodMeta.Name).To(Equal("test-pod"))
			Expect(hookRequest.ContainerMeta.Name).To(Equal(containerName))
		})

		It("should return nil if pod is not found in cache", func() {
			fakeCache.LookupPodStub = func(id string) (cache.Pod, bool) {
				return nil, false
			}

			request := &runtimeapi.CreateContainerRequest{
				PodSandboxId: "nonexistent-pod",
			}

			hookReq := d.ParseContainerRequest(request)
			Expect(hookReq).To(BeNil())
		})
	})

	Describe("ParsePodRequest", func() {
		It("should parse a valid RunPodSandboxRequest", func() {
			request := &runtimeapi.RunPodSandboxRequest{
				Config: &runtimeapi.PodSandboxConfig{
					Metadata: &runtimeapi.PodSandboxMetadata{
						Name:      "test-pod",
						Namespace: "default",
						Uid:       "uid123",
					},
					Annotations: map[string]string{"key": "value"},
					Labels:      map[string]string{"label": "value"},
					Linux: &runtimeapi.LinuxPodSandboxConfig{
						CgroupParent: "cgroup-parent",
					},
				},
				RuntimeHandler: "runc",
			}

			hookReq := d.ParsePodRequest(request)
			Expect(hookReq).ToNot(BeNil())
			hookRequest, ok := hookReq.(*v1alpha1.PodSandboxHookRequest)
			Expect(ok).To(BeTrue())
			Expect(hookRequest.PodMeta.Name).To(Equal("test-pod"))
			Expect(hookRequest.Annotations["key"]).To(Equal("value"))
		})
	})
})
