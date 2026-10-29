// Copyright 2023 FoundryLabs, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hoststats

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SandboxHostStat represents a single host-level statistics sample
// for a Firecracker process running a sandbox
type SandboxHostStat struct {
	Timestamp          time.Time `ch:"timestamp"`
	SandboxID          string    `ch:"sandbox_id"`
	SandboxExecutionID string    `ch:"sandbox_execution_id"`
	SandboxTemplateID  string    `ch:"sandbox_template_id"`
	SandboxBuildID     string    `ch:"sandbox_build_id"`
	SandboxTeamID      uuid.UUID `ch:"sandbox_team_id"`

	SandboxVCPUCount int64 `ch:"sandbox_vcpu_count"` // number of virtual CPUs allocated to the sandbox
	SandboxMemoryMB  int64 `ch:"sandbox_memory_mb"`  // total memory allocated to the sandbox in megabytes

	FirecrackerCPUUserTime   float64 `ch:"firecracker_cpu_user_time"`   // cumulative user CPU time in seconds
	FirecrackerCPUSystemTime float64 `ch:"firecracker_cpu_system_time"` // cumulative system CPU time in seconds
	FirecrackerMemoryRSS     uint64  `ch:"firecracker_memory_rss"`      // Resident Set Size in bytes
	FirecrackerMemoryVMS     uint64  `ch:"firecracker_memory_vms"`      // Virtual Memory Size in bytes

	// Cgroup v2 accounting — cumulative CPU values, deltas calculated in queries
	CgroupCPUUsageUsec  uint64 `ch:"cgroup_cpu_usage_usec"`     // cumulative, microseconds
	CgroupCPUUserUsec   uint64 `ch:"cgroup_cpu_user_usec"`      // cumulative, microseconds
	CgroupCPUSystemUsec uint64 `ch:"cgroup_cpu_system_usec"`    // cumulative, microseconds
	CgroupMemoryUsage   uint64 `ch:"cgroup_memory_usage_bytes"` // current, bytes
	CgroupMemoryPeak    uint64 `ch:"cgroup_memory_peak_bytes"`  // lifetime peak, bytes
}

// Delivery is the interface for delivering host stats to storage backend
// This allows the orchestrator to depend on the interface rather than concrete implementation
type Delivery interface {
	Push(stat SandboxHostStat) error
	Close(ctx context.Context) error
}
