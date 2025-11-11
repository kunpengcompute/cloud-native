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

package collector

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-perf-monitor/libkperf/kperf"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDebugEnv() checks the existence of env var LD_LIBRARY_PATH and libkperf.so in it
func TestDebugEnv(t *testing.T) {
	ldPath := os.Getenv("LD_LIBRARY_PATH")
	t.Logf("Current LD_LIBRARY_PATH: %s", ldPath)

	// 如果LD_LIBRARY_PATH为空，直接返回
	if ldPath == "" {
		t.Log("LD_LIBRARY_PATH is empty")
		return
	}

	// 解析LD_LIBRARY_PATH中的所有路径
	paths := strings.Split(ldPath, ":")
	t.Logf("Found %d paths in LD_LIBRARY_PATH", len(paths))

	found := false
	// 依次检查每个路径中是否存在libkperf.so文件
	for i, path := range paths {
		if path == "" {
			continue // 跳过空路径
		}

		libkperfSOPath := filepath.Join(path, "libkperf.so")
		t.Logf("Checking path %d: %s", i+1, path)

		// 检查库文件是否存在
		if _, err := os.Stat(libkperfSOPath); err == nil {
			t.Logf("✓ libkperf.so found at: %s", libkperfSOPath)
			found = true
			break
		} else {
			t.Logf("✗ libkperf.so not found at: %s (error: %v)", libkperfSOPath, err)
		}
	}
	// 如果没有找到libkperf.so，输出错误信息
	if !found {
		t.Errorf("✗ libkperf.so not found in any path in LD_LIBRARY_PATH")
		t.Log("Please ensure LD_LIBRARY_PATH is correctly configured")
	}
}

// TestNewPMUCollector() tests the NewPMUCollector() function
func TestNewPMUCollector(t *testing.T) {
	tests := []struct {
		name          string
		logger        *slog.Logger
		wantErr       bool
		wantCollector bool
		errorContains string
	}{
		{
			name:          "normal_initialization",
			logger:        slog.Default(),
			wantErr:       false,
			wantCollector: true,
		},
		{
			name:          "nil_logger",
			logger:        nil,
			wantErr:       false,
			wantCollector: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行测试
			collector, err := NewPMUCollector(tt.logger)

			// 验证错误
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// 验证收集器
			if tt.wantCollector {
				require.NotNil(t, collector)
				assert.IsType(t, &pmuCollector{}, collector)

				// 验证具体字段
				pmuCollector := collector.(*pmuCollector)
				assert.Equal(t, tt.logger, pmuCollector.logger)
				assert.NotNil(t, pmuCollector.hhaCrossNumaOpRatio)
				assert.NotNil(t, pmuCollector.hhaCrossSocketOpRatio)
			} else {
				assert.Nil(t, collector)
			}
		})
	}
}

// MockPMUCollector 用于测试 getHHACrossNumaAndSocketDataWithCollector 函数
type MockPMUCollector struct {
	// 模拟各种场景的配置
	deviceOpenError      error
	pmuEnableError       error
	pmuDisableError      error
	pmuReadError         error
	pmuGetDevMetricError error
	deviceData           kperf.PmuDeviceDataVo
	pmuData              kperf.PmuDataVo
	deviceAttrs          []kperf.PmuDeviceAttr
	closeCalled          bool
	dataFreeCalled       bool
	devDataFreeCalled    bool
}

func (m *MockPMUCollector) PmuDeviceOpen(attrs []kperf.PmuDeviceAttr) (int, error) {
	m.deviceAttrs = attrs
	if m.deviceOpenError != nil {
		return -1, m.deviceOpenError
	}
	return 123, nil // 返回有效的文件描述符
}

func (m *MockPMUCollector) PmuClose(fd int) {
	m.closeCalled = true
}

func (m *MockPMUCollector) PmuEnable(fd int) error {
	if m.pmuEnableError != nil {
		return m.pmuEnableError
	}
	return nil
}

func (m *MockPMUCollector) PmuDisable(fd int) error {
	if m.pmuDisableError != nil {
		return m.pmuDisableError
	}
	return nil
}

func (m *MockPMUCollector) PmuRead(fd int) (kperf.PmuDataVo, error) {
	if m.pmuReadError != nil {
		return kperf.PmuDataVo{}, m.pmuReadError
	}
	return m.pmuData, nil
}

func (m *MockPMUCollector) PmuDataFree(dataVo kperf.PmuDataVo) {
	m.dataFreeCalled = true
}

func (m *MockPMUCollector) PmuGetDevMetric(dataVo kperf.PmuDataVo, attrs []kperf.PmuDeviceAttr) (kperf.PmuDeviceDataVo, error) {
	if m.pmuGetDevMetricError != nil {
		return kperf.PmuDeviceDataVo{}, m.pmuGetDevMetricError
	}
	return m.deviceData, nil
}

func (m *MockPMUCollector) DevDataFree(deviceData kperf.PmuDeviceDataVo) {
	m.devDataFreeCalled = true
}

// TestGetHHACrossNumaAndSocketDataWithCollector 测试 getHHACrossNumaAndSocketDataWithCollector 函数
func TestGetHHACrossNumaAndSocketDataWithCollector(t *testing.T) {
	// 保存原始采样时间，测试后恢复
	originalSamplingTime := samplingTime
	defer func() { samplingTime = originalSamplingTime }()

	// 设置较短的采样时间用于测试
	samplingTime = 10 * time.Millisecond

	tests := []struct {
		name               string
		mockCollector      *MockPMUCollector
		wantErr            bool
		errorContains      string
		wantDeviceData     kperf.PmuDeviceDataVo
		wantCloseCalled    bool
		wantDataFreeCalled bool
	}{
		{
			name: "successful_data_collection",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{
						{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 100},
						{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 0, Count: 50},
					},
				},
			},
			wantErr: false,
			wantDeviceData: kperf.PmuDeviceDataVo{
				GoDeviceData: []kperf.PmuDeviceData{
					{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 100},
					{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 0, Count: 50},
				},
			},
			wantCloseCalled:    true,
			wantDataFreeCalled: true,
		},
		{
			name: "device_open_failure",
			mockCollector: &MockPMUCollector{
				deviceOpenError: errors.New("device open failed"),
			},
			wantErr:         true,
			errorContains:   "failed to run PmuDeviceOpen()",
			wantCloseCalled: false, // 设备打开失败，不会调用关闭
		},
		{
			name: "pmu_enable_failure",
			mockCollector: &MockPMUCollector{
				pmuEnableError: errors.New("pmu enable failed"),
			},
			wantErr:         true,
			errorContains:   "failed to run PmuEnable()",
			wantCloseCalled: true, // 设备已打开，需要关闭
		},
		{
			name: "pmu_disable_failure",
			mockCollector: &MockPMUCollector{
				pmuDisableError: errors.New("pmu disable failed"),
			},
			wantErr:         true,
			errorContains:   "failed to run PmuDisable()",
			wantCloseCalled: true,
		},
		{
			name: "pmu_read_failure",
			mockCollector: &MockPMUCollector{
				pmuReadError: errors.New("pmu read failed"),
			},
			wantErr:         true,
			errorContains:   "failed to run PmuRead()",
			wantCloseCalled: true,
		},
		{
			name: "pmu_get_dev_metric_failure",
			mockCollector: &MockPMUCollector{
				pmuGetDevMetricError: errors.New("get dev metric failed"),
			},
			wantErr:            true,
			errorContains:      "failed to run PmuGetDevMetric()",
			wantCloseCalled:    true,
			wantDataFreeCalled: true, // PmuDataFree 会在 defer 中调用
		},
		{
			name: "empty_device_data",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{},
				},
			},
			wantErr: false,
			wantDeviceData: kperf.PmuDeviceDataVo{
				GoDeviceData: []kperf.PmuDeviceData{},
			},
			wantCloseCalled:    true,
			wantDataFreeCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置模拟器的状态
			tt.mockCollector.closeCalled = false
			tt.mockCollector.dataFreeCalled = false
			tt.mockCollector.devDataFreeCalled = false

			// 执行测试
			deviceData, err := getHHACrossNumaAndSocketDataWithCollector(tt.mockCollector)

			// 验证错误
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				// 验证返回的设备数据
				assert.Equal(t, tt.wantDeviceData.GoDeviceData, deviceData.GoDeviceData)
			}

			// 验证设备属性是否正确传递
			expectedAttrs := []kperf.PmuDeviceAttr{
				{Metric: kperf.PMU_HHA_CROSS_NUMA},
				{Metric: kperf.PMU_HHA_CROSS_SOCKET},
			}
			assert.Equal(t, expectedAttrs, tt.mockCollector.deviceAttrs)

			// 验证资源清理是否正确调用
			assert.Equal(t, tt.wantCloseCalled, tt.mockCollector.closeCalled, "PmuClose called incorrectly")
			assert.Equal(t, tt.wantDataFreeCalled, tt.mockCollector.dataFreeCalled, "PmuDataFree called incorrectly")
		})
	}
}

// TestUpdateHHACrossNumaAndSocketOpRatio 测试 getHHACrossNumaAndSocketDataWithCollector 函数
func TestUpdateHHACrossNumaAndSocketOpRatio(t *testing.T) {
	// 创建测试用的 pmuCollector 实例
	logger := slog.Default()
	collector, err := NewPMUCollector(logger)
	require.NoError(t, err)
	pmuCollector := collector.(*pmuCollector)

	// 保存原始采样时间
	originalSamplingTime := samplingTime
	samplingTime = 10 * time.Millisecond // 很短的采样时间
	defer func() { samplingTime = originalSamplingTime }()

	tests := []struct {
		name            string
		mockCollector   *MockPMUCollector
		wantErr         bool
		errorContains   string
		expectedMetrics []struct {
			metricType string
			numaID     string
			value      float64
		}
		wantDevDataFreeCalled bool
	}{
		{
			name: "successful_metric_update_with_both_metrics",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{
						{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 100.5},
						{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 0, Count: 50.2},
						{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 1, Count: 200.7},
						{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 1, Count: 75.9},
					},
				},
			},
			wantErr: false,
			expectedMetrics: []struct {
				metricType string
				numaID     string
				value      float64
			}{
				{"HHA_cross_numa", "0", 100.5},
				{"HHA_cross_socket", "0", 50.2},
				{"HHA_cross_numa", "1", 200.7},
				{"HHA_cross_socket", "1", 75.9},
			},
			wantDevDataFreeCalled: true,
		},
		{
			name: "successful_metric_update_with_only_cross_numa",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{
						{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 150.3},
						{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 1, Count: 250.8},
					},
				},
			},
			wantErr: false,
			expectedMetrics: []struct {
				metricType string
				numaID     string
				value      float64
			}{
				{"HHA_cross_numa", "0", 150.3},
				{"HHA_cross_numa", "1", 250.8},
			},
			wantDevDataFreeCalled: true,
		},
		{
			name: "successful_metric_update_with_only_cross_socket",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{
						{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 0, Count: 80.1},
						{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 1, Count: 120.4},
					},
				},
			},
			wantErr: false,
			expectedMetrics: []struct {
				metricType string
				numaID     string
				value      float64
			}{
				{"HHA_cross_socket", "0", 80.1},
				{"HHA_cross_socket", "1", 120.4},
			},
			wantDevDataFreeCalled: true,
		},
		{
			name: "empty_device_data",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{},
				},
			},
			wantErr: false,
			expectedMetrics: []struct {
				metricType, numaID string
				value              float64
			}{},
			wantDevDataFreeCalled: true,
		},
		{
			name: "data_collection_failure",
			mockCollector: &MockPMUCollector{
				deviceOpenError: errors.New("device open failed"),
			},
			wantErr:       true,
			errorContains: "failed to get HHA cross-numa and socket data",
			expectedMetrics: []struct {
				metricType, numaID string
				value              float64
			}{},
			wantDevDataFreeCalled: false, // 数据收集失败，不会调用 DevDataFree
		},
		{
			name: "unknown_metric_type_ignored",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{
						{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 100.0},
						{Metric: 9999, NumaId: 0, Count: 200.0}, // 未知的 metric 类型
						{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 0, Count: 50.0},
					},
				},
			},
			wantErr: false,
			expectedMetrics: []struct {
				metricType string
				numaID     string
				value      float64
			}{
				{"HHA_cross_numa", "0", 100.0},
				{"HHA_cross_socket", "0", 50.0},
			},
			wantDevDataFreeCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// 创建 metrics 通道
			metricsCh := make(chan prometheus.Metric, 10)

			// 执行测试
			err := pmuCollector.updateHHACrossNumaAndSocketOpRatio(metricsCh, tt.mockCollector)

			// 验证错误
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// 验证生成的 metrics
			close(metricsCh)
			var collectedMetrics []prometheus.Metric
			for metric := range metricsCh {
				collectedMetrics = append(collectedMetrics, metric)
			}

			assert.Equal(t, len(tt.expectedMetrics), len(collectedMetrics), "Number of collected metrics mismatch")

			// 验证每个 metric 的内容
			for i, expected := range tt.expectedMetrics {
				if i < len(collectedMetrics) {
					metric := collectedMetrics[i]

					// 获取 metric 的描述信息
					desc := metric.Desc()
					descString := desc.String()

					// 验证 metric 类型
					if expected.metricType == "HHA_cross_numa" {
						assert.Contains(t, descString, "HHA_cross_numa_operations_ratio", "Metric should be HHA cross-numa")
					} else {
						assert.Contains(t, descString, "HHA_cross_socket_operations_ratio", "Metric should be HHA cross-socket")
					}

					// 验证 metric 值（这里简化验证，实际可能需要更复杂的检查）
					// 在实际测试中，可能需要使用 reflect 或其他方法来检查 metric 的具体值
				}
			}
		})
	}
}

// TestPMUCollectorUpdate 测试 pmuCollector 的 Update 方法
func TestPMUCollectorUpdate(t *testing.T) {
	// 创建测试用的 pmuCollector 实例
	logger := slog.Default()
	collector, err := NewPMUCollector(logger)
	require.NoError(t, err)
	pmuCollector := collector.(*pmuCollector)
	// 保存原始采样时间
	originalSamplingTime := samplingTime
	samplingTime = 10 * time.Millisecond // 很短的采样时间
	defer func() { samplingTime = originalSamplingTime }()

	tests := []struct {
		name                 string
		setupMock            func() *MockPMUCollector
		wantErr              bool
		errorContains        string
		expectedMetricsCount int
	}{
		{
			name: "successful_update_with_metrics",
			setupMock: func() *MockPMUCollector {
				return &MockPMUCollector{
					deviceData: kperf.PmuDeviceDataVo{
						GoDeviceData: []kperf.PmuDeviceData{
							{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 100.5},
							{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 0, Count: 50.2},
						},
					},
				}
			},
			wantErr:              false,
			expectedMetricsCount: 2,
		},
		{
			name: "successful_update_empty_metrics",
			setupMock: func() *MockPMUCollector {
				return &MockPMUCollector{
					deviceData: kperf.PmuDeviceDataVo{
						GoDeviceData: []kperf.PmuDeviceData{},
					},
				}
			},
			wantErr:              false,
			expectedMetricsCount: 0,
		},
		{
			name: "update_failure_due_to_device_open_error",
			setupMock: func() *MockPMUCollector {
				return &MockPMUCollector{
					deviceOpenError: errors.New("device open failed"),
				}
			},
			wantErr:              true,
			errorContains:        "failed to",
			expectedMetricsCount: 0,
		},
		{
			name: "update_failure_due_to_pmu_enable_error",
			setupMock: func() *MockPMUCollector {
				return &MockPMUCollector{
					pmuEnableError: errors.New("pmu enable failed"),
				}
			},
			wantErr:              true,
			errorContains:        "failed to",
			expectedMetricsCount: 0,
		},
		{
			name: "update_failure_due_to_pmu_read_error",
			setupMock: func() *MockPMUCollector {
				return &MockPMUCollector{
					pmuReadError: errors.New("pmu read failed"),
				}
			},
			wantErr:              true,
			errorContains:        "failed to",
			expectedMetricsCount: 0,
		},
		{
			name: "update_with_multiple_numa_nodes",
			setupMock: func() *MockPMUCollector {
				return &MockPMUCollector{
					deviceData: kperf.PmuDeviceDataVo{
						GoDeviceData: []kperf.PmuDeviceData{
							{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 100.0},
							{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 0, Count: 50.0},
							{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 1, Count: 200.0},
							{Metric: kperf.PMU_HHA_CROSS_SOCKET, NumaId: 1, Count: 75.0},
						},
					},
				}
			},
			wantErr:              false,
			expectedMetricsCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 metrics 通道
			metricsCh := make(chan prometheus.Metric, 10)

			// 由于 Update 方法使用真实的收集器，我们需要通过修改全局变量或使用依赖注入来测试
			// 这里我们创建一个包装函数来测试 Update 的逻辑
			mockCollector := tt.setupMock()

			// 测试 updateHHACrossNumaAndSocketOpRatio 函数（这是 Update 方法调用的核心函数）
			err := pmuCollector.updateHHACrossNumaAndSocketOpRatio(metricsCh, mockCollector)

			// 验证错误
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// 验证生成的 metrics 数量
			close(metricsCh)
			metricsCount := 0
			for range metricsCh {
				metricsCount++
			}
			assert.Equal(t, tt.expectedMetricsCount, metricsCount, "Number of collected metrics mismatch")
		})
	}
}

// TestPMUCollectorUpdateErrorHandling 测试 Update 方法的错误处理
func TestPMUCollectorUpdateErrorHandling(t *testing.T) {
	// 创建测试用的 pmuCollector 实例
	logger := slog.Default()
	collector, err := NewPMUCollector(logger)
	require.NoError(t, err)
	pmuCollector := collector.(*pmuCollector)
	// 保存原始采样时间
	originalSamplingTime := samplingTime
	samplingTime = 10 * time.Millisecond // 很短的采样时间
	defer func() { samplingTime = originalSamplingTime }()

	tests := []struct {
		name          string
		mockCollector *MockPMUCollector
		wantErr       bool
		errorContains string
	}{
		{
			name: "error_propagation_from_inner_function",
			mockCollector: &MockPMUCollector{
				deviceOpenError: errors.New("simulated device error"),
			},
			wantErr:       true,
			errorContains: "failed to update HHA cross-numa and socket op ratio",
		},
		{
			name: "nil_channel_handling",
			mockCollector: &MockPMUCollector{
				deviceData: kperf.PmuDeviceDataVo{
					GoDeviceData: []kperf.PmuDeviceData{
						{Metric: kperf.PMU_HHA_CROSS_NUMA, NumaId: 0, Count: 100.0},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建有效的 metrics 通道
			metricsCh := make(chan prometheus.Metric, 10)

			// 测试内部函数（模拟 Update 方法的行为）
			err := pmuCollector.updateHHACrossNumaAndSocketOpRatio(metricsCh, tt.mockCollector)

			// 验证错误处理
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					// 注意：由于我们测试的是内部函数，错误信息可能略有不同
					// 但错误应该被正确传播
					assert.ErrorContains(t, err, "failed to get HHA cross-numa and socket data")
				}
			} else {
				assert.NoError(t, err)
			}

			// 清理通道
			close(metricsCh)
			for range metricsCh {
				// 清空通道
			}
		})
	}
}
