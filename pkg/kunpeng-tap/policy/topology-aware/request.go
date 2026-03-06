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

package topologyaware

import (
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

// Request represents a container's resource request.
type Request interface {
	GetContext() policy.ContainerContext
	CPULimit() int
	CPURequest() int
	MemoryLimit() int64
	MemoryRequest() int64
	HasGPURequest() bool
	GetRequestedGPUDevices() []string
	// String returns a printable representation of this request.
	String() string
}

// request implements the Request interface.
type request struct {
	container           policy.ContainerContext
	cpuLimit            int               // millicores
	cpuRequest          int               // millicores
	memLimit            int64             // KB
	memRequest          int64             // KB
	memType             system.MemoryType // memory type
	hasGPURequest       bool              // whether GPU resources are requested
	requestedGPUDevices []string          // list of requested GPU device IDs
}

func (r *request) GetContext() policy.ContainerContext {
	return r.container
}

func (r *request) CPULimit() int {
	return r.cpuLimit
}

func (r *request) CPURequest() int {
	return r.cpuRequest
}

func (r *request) MemoryLimit() int64 {
	return r.memLimit
}

func (r *request) MemoryRequest() int64 {
	return r.memRequest
}

func (r *request) HasGPURequest() bool {
	return r.hasGPURequest
}

func (r *request) GetRequestedGPUDevices() []string {
	return r.requestedGPUDevices
}

func (r *request) String() string {
	return fmt.Sprintf("cpu: %dm, gpu: %v, devices: %v",
		r.cpuLimit, r.hasGPURequest, r.requestedGPUDevices)
}

// DeviceType represents the type of accelerator device.
type DeviceType string

const (
	// GPU device type
	GPU DeviceType = "GPU"
	// NPU device type
	NPU DeviceType = "NPU"
	// FPGA device type
	FPGA DeviceType = "FPGA"
)

// DeviceEnvConfig defines the environment variable configuration for a device type.
type DeviceEnvConfig struct {
	AllocateEnvName string     // Environment variable name for allocated devices
	VisibleEnvName  string     // Environment variable name for visible devices
	DevicePrefix    string     // Device path prefix, e.g., "/dev/vacc"
	DeviceRegex     string     // Regex pattern for extracting device ID
	DeviceType      DeviceType // Device type
}

// knownDeviceConfigs contains known device environment variable configurations.
var knownDeviceConfigs = []DeviceEnvConfig{
	{
		AllocateEnvName: "VA_ALLOCATE_DEVICES",
		VisibleEnvName:  "VA_VISIBLE_DEVICES",
		DevicePrefix:    "/dev/vacc",
		DeviceRegex:     `/dev/vacc(\d+)`,
		DeviceType:      GPU,
	},
	// More device configurations can be added here
}

// parseDeviceID extracts the device ID from a device path.
func parseDeviceID(device, regex string) (string, bool) {
	device = strings.TrimSpace(device)
	if match := regexp.MustCompile(regex).FindStringSubmatch(device); len(match) > 1 {
		return match[1], true
	}
	return "", false
}

// checkDeviceRequest checks if a container requests a specific device type.
func checkDeviceRequest(containerEnvs map[string]string, config DeviceEnvConfig) (bool, []string) {
	if containerEnvs == nil {
		return false, nil
	}

	var deviceIDs []string
	hasRequest := false

	// Check allocate devices environment variable
	if allocDevices := containerEnvs[config.AllocateEnvName]; allocDevices != "" {
		hasRequest = true
		deviceIDs = append(deviceIDs, extractDeviceIDs(allocDevices, config.DeviceRegex)...)
	}

	// Check visible devices environment variable
	if visibleDevices := containerEnvs[config.VisibleEnvName]; visibleDevices != "" {
		hasRequest = true
		deviceIDs = append(deviceIDs, extractVisibleDeviceIDs(visibleDevices, config.DeviceRegex)...)
	}

	return hasRequest, deviceIDs
}

// extractDeviceIDs extracts device IDs from a device string.
func extractDeviceIDs(deviceStr, regex string) []string {
	if deviceStr == "" {
		return nil
	}

	var deviceIDs []string
	devices := strings.Split(deviceStr, ",")

	for _, device := range devices {
		if deviceID, ok := parseDeviceID(device, regex); ok {
			deviceIDs = append(deviceIDs, deviceID)
		}
	}

	return deviceIDs
}

// extractVisibleDeviceIDs extracts device IDs from a visible device string.
func extractVisibleDeviceIDs(deviceStr, regex string) []string {
	if deviceStr == "" {
		return nil
	}

	var deviceIDs []string
	devices := strings.Split(deviceStr, ",")

	for _, device := range devices {
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}

		// Try to extract device ID using regex
		if deviceID, ok := parseDeviceID(device, regex); ok {
			deviceIDs = append(deviceIDs, deviceID)
		} else {
			// If regex doesn't match, use the original string as device ID
			deviceIDs = append(deviceIDs, device)
		}
	}

	return deviceIDs
}

// parseResourceRequirements parses CPU and memory resources from resource lists.
func parseResourceRequirements(r *request, resourceReq, resourceLimit *corev1.ResourceList) {
	// Parse CPU resources
	if resourceLimit != nil && resourceLimit.Cpu() != nil {
		r.cpuLimit = int(resourceLimit.Cpu().MilliValue())
	}

	if resourceReq != nil && resourceReq.Cpu() != nil {
		r.cpuRequest = int(resourceReq.Cpu().MilliValue())
	}

	// Parse Memory request
	if resourceReq != nil && resourceReq.Memory() != nil {
		r.memRequest = resourceReq.Memory().Value()
	}

	if resourceLimit != nil && resourceLimit.Memory() != nil {
		r.memLimit = resourceLimit.Memory().Value()
	} else {
		r.memLimit = r.memRequest
	}

	r.memType = system.MemoryTypeDRAM
}

// processGPUDeviceRequests processes GPU device requests from container environment variables.
func processGPUDeviceRequests(r *request, containerEnvs map[string]string) {
	if containerEnvs == nil {
		return
	}

	// Iterate through all known device configurations
	for _, config := range knownDeviceConfigs {
		hasRequest, deviceIDs := checkDeviceRequest(containerEnvs, config)

		// Set fields based on device type
		switch config.DeviceType {
		case GPU:
			if hasRequest {
				r.hasGPURequest = true
				r.requestedGPUDevices = append(r.requestedGPUDevices, deviceIDs...)
			}
		case NPU:
			// TODO: Add NPU device type handling
		default:
			klog.ErrorS(nil, "Unknown device type", "deviceType", config.DeviceType)
		}
	}
}

// newRequest creates a new request from a container context.
func newRequest(containerCtx policy.ContainerContext) Request {
	r := &request{
		container:           containerCtx,
		cpuLimit:            0,
		cpuRequest:          0,
		hasGPURequest:       false,
		requestedGPUDevices: []string{},
	}
	request := containerCtx.Request

	resourceReq, resourceLimit := request.Resources.GetRequests(), request.Resources.GetLimits()
	if resourceReq == nil || resourceLimit == nil {
		klog.V(0).InfoS("Resource requirements or limits are nil")
		return nil
	}
	parseResourceRequirements(r, resourceReq, resourceLimit)
	// Check for GPU resource requests
	klog.InfoS("Claims done, Start to Check GPU")
	processGPUDeviceRequests(r, containerCtx.Request.ContainerEnvs)

	return r
}
