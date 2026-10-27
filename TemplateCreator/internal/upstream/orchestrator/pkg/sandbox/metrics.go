// Copyright (c) 2023-2026 e2b Devs Inc.
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

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/consts"
)

type Metrics struct {
	Timestamp int64 `json:"ts"` // Unix Timestamp in UTC

	CPUCount       int64   `json:"cpu_count"`    // Total CPU cores
	CPUUsedPercent float64 `json:"cpu_used_pct"` // Percent rounded to 2 decimal places

	MemTotal int64 `json:"mem_total"` // Total virtual memory in bytes
	MemUsed  int64 `json:"mem_used"`  // Used virtual memory in bytes

	DiskUsed  int64 `json:"disk_used"`  // Used disk space in bytes
	DiskTotal int64 `json:"disk_total"` // Total disk space in bytes

	// Deprecated
	MemTotalMiB int64 `json:"mem_total_mib"` // Total virtual memory in MiB

	// Deprecated
	MemUsedMiB int64 `json:"mem_used_mib"` // Used virtual memory in MiB
}

func (c *Checks) GetMetrics(ctx context.Context, timeout time.Duration) (*Metrics, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	address := fmt.Sprintf("http://%s:%d/metrics", c.sandbox.Slot.HostIPString(), consts.DefaultEnvdServerPort)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	if c.sandbox.Config.Envd.AccessToken != nil {
		request.Header.Set("X-Access-Token", *c.sandbox.Config.Envd.AccessToken)
	}

	response, err := sandboxHttpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected status code: %d", response.StatusCode)

		return nil, err
	}

	var m Metrics
	err = json.NewDecoder(response.Body).Decode(&m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}
