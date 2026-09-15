// Copyright (c) 2025 Huawei Technology corp.
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

//go:build linux

package collector

import "strings"

// sanitizedTopdownFixture 构造最小 TopDown 报文。测试数据仅保留解析合同要求的
// scope、四个顶层节点、必要的 Backend 子树和一条 PMU 事件，不携带真实环境信息。
func sanitizedTopdownFixture(scope string) string {
	return strings.ReplaceAll(`Command     : devkit tuner top-down -d 3

TOP-DOWN Summary Report-ALL

Top-down metrics of {{SCOPE}}:
Cycles              1,000
Instructions        800
IPC                 0.80

  Top-down Metrics                        Bound(%)    Preferred Sampling Event
  Bad Speculation                            10.00    --
  ├── Branch Mispredicts                       5.00    branch_event
  │   └── Indirect Branch                       2.00    --
  Frontend Bound                             20.00    --
  Retiring                                   30.00    inst_retired
  Backend Bound                              40.00    --
  ├── Core Bound                             15.00    --
  └── Memory Bound                           25.00    --

  PMU Event                                  Count
  r0001                                      1,000
3000 milliseconds time elapsed
`, "{{SCOPE}}", scope)
}

// sanitizedMemoryFixture 构造最小 Memory 报文。CPU 和 L1D miss 值由测试传入，
// 其余数据为虚构值，但覆盖 Access、L3 和 DDRC 的动态表头及复合单元格解析。
func sanitizedMemoryFixture(cpu, l1dMiss string) string {
	replacer := strings.NewReplacer(
		"{{CPU}}", cpu,
		"{{L1D_MISS}}", l1dMiss,
	)
	return replacer.Replace(`Command     : devkit tuner memory -d 3 -m 1

Memory Summary Report-ALL

Percentage of core Cache miss
L1D         {{L1D_MISS}}%
L1I        12.34%
L2D        34.56%
L2I        23.82%

DDR Bandwidth (system wide)
ddrc_write        154.49MB/s
ddrc_read         245.51MB/s

Memory metrics of the Cache

L1/L2/TLB Access Bandwidth and Hit Rate
CPU       L1D              L1I              L2D              L2I              L2D_TLB        L2I_TLB
{{CPU}}   10MB/s|90.00%    20MB/s|91.00%    30MB/s|92.00%    40MB/s|93.00%    N/A| 85.30%   N/A| N/A

L3 Read Bandwidth and Hit Rate
NODE CCL Read Hit Bandwidth Read Bandwidth Read Hit Rate
0    --  30.00MB/s         60.00MB/s      50.00%
0    0   30.00MB/s         60.00MB/s      50.00%

Memory metrics of the DDRC
DDRC_ACCESS_BANDWIDTH
NODE DDRC_0 Total
0    10.00MB/s| 20.00MB/s 10.00MB/s| 20.00MB/s
`)
}
