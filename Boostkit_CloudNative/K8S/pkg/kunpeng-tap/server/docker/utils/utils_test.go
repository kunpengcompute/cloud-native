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

package utils_test

import (
	"bytes"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker/utils"
)

var _ = Describe("Utils", func() {
	Describe("EncodeBody", func() {
		It("should return nil when the input object is nil", func() {
			body, err := utils.EncodeBody(nil)
			Expect(err).To(BeNil())
			Expect(body).To(BeNil())
		})

		It("should encode a valid object into a JSON reader", func() {
			obj := map[string]string{"key": "value"}
			body, err := utils.EncodeBody(obj)
			Expect(err).To(BeNil())
			Expect(body).NotTo(BeNil())

			buf := new(bytes.Buffer)
			_, err = buf.ReadFrom(body)
			Expect(err).To(BeNil())
			Expect(buf.String()).To(ContainSubstring(`"key":"value"`))
		})
	})

	Describe("CalculateContentLength", func() {
		It("should return an error when the reader is nil", func() {
			length, err := utils.CalculateContentLength(nil)
			Expect(err).To(HaveOccurred())
			Expect(length).To(Equal(int64(-1)))
		})

		It("should calculate the correct content length", func() {
			body := strings.NewReader("test content")
			length, err := utils.CalculateContentLength(body)
			Expect(err).To(BeNil())
			Expect(length).To(Equal(int64(len("test content"))))
		})
	})

	Describe("SplitLabelsAndAnnotations", func() {
		It("should split labels and annotations correctly", func() {
			configs := map[string]string{
				"label1":          "value1",
				"annotation.key1": "value2",
				"annotation.key2": "value3",
				"label2":          "value4",
			}
			labels, annotations := utils.SplitLabelsAndAnnotations(configs)
			Expect(labels).To(HaveKeyWithValue("label1", "value1"))
			Expect(labels).To(HaveKeyWithValue("label2", "value4"))
			Expect(annotations).To(HaveKeyWithValue("key1", "value2"))
			Expect(annotations).To(HaveKeyWithValue("key2", "value3"))
		})
	})

	Describe("SplitDockerEnv", func() {
		It("should split Docker environment variables correctly", func() {
			dockerEnvs := []string{"KEY1=VALUE1", "KEY2=VALUE2", "INVALID"}
			result := utils.SplitDockerEnv(dockerEnvs)
			Expect(result).To(HaveKeyWithValue("KEY1", "VALUE1"))
			Expect(result).To(HaveKeyWithValue("KEY2", "VALUE2"))
			Expect(result).NotTo(HaveKey("INVALID"))
		})
	})

	Describe("DockerContainers2ContainerIds", func() {
		It("should extract container IDs from Docker containers", func() {
			containers := []dockertypes.Container{
				{ID: "container1"},
				{ID: "container2"},
			}
			result := utils.DockerContainers2ContainerIds(containers)
			Expect(result).To(ConsistOf("container1", "container2"))
		})
	})

	Describe("ToCriCgroupPath", func() {
		It("should return the correct cgroup path for systemd driver", func() {
			cgroupDriver := "systemd"
			cgroupParent := "kubepods-besteffort-pod1234.slice"
			result := utils.ToCriCgroupPath(cgroupDriver, cgroupParent)
			Expect(result).To(Equal("/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod1234.slice"))
		})

		It("should return the cgroup parent as-is for non-systemd driver", func() {
			cgroupDriver := "cgroupfs"
			cgroupParent := "kubepods/besteffort/pod1234"
			result := utils.ToCriCgroupPath(cgroupDriver, cgroupParent)
			Expect(result).To(Equal(cgroupParent))
		})
	})

	Describe("GenerateEnvList", func() {
		It("should generate a list of environment variables", func() {
			envs := map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"}
			result := utils.GenerateEnvList(envs)
			Expect(result).To(ConsistOf("KEY1=VALUE1", "KEY2=VALUE2"))
		})
	})

	Describe("HostConfigToResource", func() {
		It("should return nil when the input HostConfig is nil", func() {
			result := utils.HostConfigToResource(nil)
			Expect(result).To(BeNil())
		})

		It("should convert HostConfig to LinuxContainerResources correctly", func() {
			hostConfig := &container.HostConfig{
				Resources: container.Resources{
					CPUPeriod:  1000,
					CPUQuota:   2000,
					CPUShares:  512,
					Memory:     1024,
					CpusetCpus: "0-2",
					CpusetMems: "0",
					MemorySwap: 2048,
				},
				OomScoreAdj: 10,
			}
			result := utils.HostConfigToResource(hostConfig)
			Expect(result).NotTo(BeNil())
			Expect(result.CpuPeriod).To(Equal(int64(1000)))
			Expect(result.CpuQuota).To(Equal(int64(2000)))
			Expect(result.CpuShares).To(Equal(int64(512)))
			Expect(result.MemoryLimitInBytes).To(Equal(int64(1024)))
			Expect(result.OomScoreAdj).To(Equal(int64(10)))
			Expect(result.CpusetCpus).To(Equal("0-2"))
			Expect(result.CpusetMems).To(Equal("0"))
			Expect(result.MemorySwapLimitInBytes).To(Equal(int64(2048)))
		})
	})

	Describe("UpdateHostConfigByResource", func() {
		It("should return the original HostConfig when input is nil", func() {
			hostConfig := &container.HostConfig{}
			result := utils.UpdateHostConfigByResource(hostConfig, nil)
			Expect(result).To(Equal(hostConfig))
		})

		It("should update HostConfig based on LinuxContainerResources", func() {
			hostConfig := &container.HostConfig{}
			resources := &v1alpha1.LinuxContainerResources{
				CpuPeriod:              1000,
				CpuQuota:               2000,
				CpuShares:              512,
				MemoryLimitInBytes:     1024,
				OomScoreAdj:            10,
				CpusetCpus:             "0-2",
				CpusetMems:             "0",
				MemorySwapLimitInBytes: 2048,
			}
			result := utils.UpdateHostConfigByResource(hostConfig, resources)
			Expect(result.CPUPeriod).To(Equal(int64(1000)))
			Expect(result.CPUQuota).To(Equal(int64(2000)))
			Expect(result.CPUShares).To(Equal(int64(512)))
			Expect(result.Memory).To(Equal(int64(1024)))
			Expect(result.OomScoreAdj).To(Equal(10))
			Expect(result.CpusetCpus).To(Equal("0-2"))
			Expect(result.CpusetMems).To(Equal("0"))
			Expect(result.MemorySwap).To(Equal(int64(2048)))
		})
	})

	Describe("GenerateExpectedCgroupParent", func() {
		It("should return the base of cgroupParent for systemd driver", func() {
			cgroupDriver := "systemd"
			cgroupParent := "kubepods-besteffort-pod1234.slice"
			result := utils.GenerateExpectedCgroupParent(cgroupDriver, cgroupParent)
			Expect(result).To(Equal("kubepods-besteffort-pod1234.slice"))
		})

		It("should return the cgroupParent as-is for non-systemd driver", func() {
			cgroupDriver := "cgroupfs"
			cgroupParent := "kubepods/besteffort/pod1234"
			result := utils.GenerateExpectedCgroupParent(cgroupDriver, cgroupParent)
			Expect(result).To(Equal(cgroupParent))
		})
	})

	Describe("GetRuntimeResourceType", func() {
		It("should return RuntimePodResource for sandbox label", func() {
			labels := map[string]string{
				utils.ContainerTypeLabelKey: utils.ContainerTypeLabelSandbox,
			}
			result := utils.GetRuntimeResourceType(labels)
			Expect(result).To(Equal(server.RuntimePodResource))
		})

		It("should return RuntimeContainerResource for non-sandbox label", func() {
			labels := map[string]string{
				utils.ContainerTypeLabelKey: utils.ContainerTypeLabelContainer,
			}
			result := utils.GetRuntimeResourceType(labels)
			Expect(result).To(Equal(server.RuntimeContainerResource))
		})
	})
})
