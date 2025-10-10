package collector

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMPAMCollector(t *testing.T) {

	// 保存resctlMountPath原始值并在测试结束后恢复
	origResctlPath := *resctlMountPath
	defer func() { *resctlMountPath = origResctlPath }()

	// 测试正常初始化
	// 创建临时测试目录结构
	tmpDir := t.TempDir()
	*resctlMountPath = tmpDir
	c, err := NewMPAMCollector(slog.Default())
	assert.NoError(t, err)
	assert.IsType(t, &mpamCollector{}, c)

	// 测试错误路径（需要mock文件系统）
	*resctlMountPath = "/invalid/path"

	_, err = NewMPAMCollector(slog.Default())
	assert.Error(t, err)
}

func TestGetMPAMGroups(t *testing.T) {
	// 创建临时测试目录
	testDir := t.TempDir()
	// 在测试开始前添加
	originalPath := *resctlMountPath
	*resctlMountPath = testDir // 指向临时目录
	defer func() { *resctlMountPath = originalPath }()

	// 模拟正常情况
	t.Run("three_mpam_groups", func(t *testing.T) {

		// 为了避免测试之间的干扰，每个测试都创建一个新的临时目录
		// 创建临时测试目录
		testDir := t.TempDir()
		// 在测试开始前添加
		originalPath := *resctlMountPath
		*resctlMountPath = testDir // 指向临时目录
		defer func() { *resctlMountPath = originalPath }()

		// 创建测试用目录结构
		groupDirs := []string{"groupA", "groupB", "groupC"}
		for _, dir := range groupDirs {
			path := filepath.Join(testDir, dir, DirNameInAGroup)
			os.MkdirAll(path, 0755)
		}

		// 执行测试
		c := &mpamCollector{}
		result, err := c.getMPAMGroups()

		// 验证结果
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(result) != len(groupDirs) {
			t.Errorf("Expected %d groups, got %d", len(groupDirs), len(result))
		}
	})

	t.Run("one_mpam_group", func(t *testing.T) {
		// 为了避免测试之间的干扰，每个测试都创建一个新的临时目录
		// 创建临时测试目录
		testDir := t.TempDir()
		// 在测试开始前添加
		originalPath := *resctlMountPath
		*resctlMountPath = testDir // 指向临时目录
		defer func() { *resctlMountPath = originalPath }()

		groupDirs := []string{"groupA", "groupB", "groupC"}
		for _, dir := range groupDirs {
			path := filepath.Join(testDir, DirNameInAGroup, dir)
			os.MkdirAll(path, 0755)
		}

		// 执行测试
		c := &mpamCollector{}
		result, err := c.getMPAMGroups()

		// 验证结果
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("wrong result, expect only one result")
		}
	})

	// 模拟错误情况
	t.Run("empty_directory", func(t *testing.T) {
		// 为了避免测试之间的干扰，每个测试都创建一个新的临时目录
		// 创建临时测试目录
		testDir := t.TempDir()
		// 在测试开始前添加
		originalPath := *resctlMountPath
		*resctlMountPath = testDir // 指向临时目录
		defer func() { *resctlMountPath = originalPath }()

		c := &mpamCollector{}
		_, err := c.getMPAMGroups()
		if err == nil {
			t.Error("Expected error but got nil")
		}
	})

	// wrong path
	t.Run("non_exist_path", func(t *testing.T) {
		// 在测试开始前添加
		originalPath := *resctlMountPath
		*resctlMountPath = "wrong" // 指向临时目录
		defer func() { *resctlMountPath = originalPath }()

		c := &mpamCollector{}
		_, err := c.getMPAMGroups()
		if err == nil {
			t.Error("Expected error but got nil")
		}
		assert.ErrorContains(t, err, "failed to get mpam groups")
	})
}

func TestGetLabels(t *testing.T) {
	// 保存原始全局变量
	originalPath := *resctlMountPath
	defer func() { *resctlMountPath = originalPath }()

	tests := []struct {
		name        string
		groupPath   string
		mockFiles   map[string]string
		wantLabels  mpamMetricsCommonLabels
		wantError   bool
		logContains string
	}{
		{
			name:      "normal case",
			groupPath: "group1",
			mockFiles: map[string]string{
				"group1/cpus_list": "0-3",
				"group1/mode":      "exclusive",
			},
			wantLabels: mpamMetricsCommonLabels{
				groupName: "group1",
				cpuList:   "0-3",
				mode:      "exclusive",
			},
		},
		{
			name:      "empty cpus_list",
			groupPath: "group2",
			mockFiles: map[string]string{
				"group2/cpus_list": "",
				"group2/mode":      "shared",
			},
			wantLabels: mpamMetricsCommonLabels{
				groupName: "group2",
				cpuList:   "null",
				mode:      "shared",
			},
			logContains: "cpus_list is empty",
		},
		{
			name:      "file not found",
			groupPath: "",
			mockFiles: map[string]string{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置临时目录
			tmpDir := t.TempDir()
			*resctlMountPath = tmpDir

			// 创建模拟文件
			for path, content := range tt.mockFiles {
				fullPath := filepath.Join(tmpDir, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// 初始化collector
			c := &mpamCollector{
				logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
			}

			// 执行测试
			got, err := c.getLabels(tt.groupPath, tt.groupPath)

			// 验证错误
			if (err != nil) != tt.wantError {
				t.Fatalf("getLabels() error = %v, wantErr %v", err, tt.wantError)
			}

			// 验证返回值
			if !reflect.DeepEqual(got, tt.wantLabels) {
				t.Errorf("getLabels() = %v, want %v", got, tt.wantLabels)
			}
		})
	}
}

func TestUpdateResUsageMetrics(t *testing.T) {
	origPath := *resctlMountPath
	defer func() { *resctlMountPath = origPath }()

	t.Run("normal_cache_usage", func(t *testing.T) {
		testDir := t.TempDir()
		*resctlMountPath = testDir

		// 创建测试目录结构
		mpamPath := filepath.Join(*resctlMountPath, "groupA", dirPathForMPAMGroupData, nameKeyForCacheUsageDirs+"_test")
		os.MkdirAll(mpamPath, 0755)
		os.WriteFile(filepath.Join(mpamPath, fileNameForCache), []byte("1024"), 0644)

		c := &mpamCollector{
			logger:     slog.Default(), // 必须有
			cacheUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"), "MPAM L3 cache usage.", []string{"group", "id", "cpu_list", "mode"}, nil),
		}

		ch := make(chan prometheus.Metric, 1)
		labels := mpamMetricsCommonLabels{
			groupName: "groupA",
		}
		err := c.updateResUsageMetrics(ch, "groupA", labels, c.cacheUsage, []string{nameKeyForCacheUsageDirs + "_test"})

		close(ch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ch))

	})

	// 日志捕获逻辑
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	t.Run("invalid_file_content", func(t *testing.T) {
		testDir := t.TempDir()
		*resctlMountPath = testDir

		mpamPath := filepath.Join(*resctlMountPath, "groupB", dirPathForMPAMGroupData, nameKeyForCacheUsageDirs+"_test")
		os.MkdirAll(mpamPath, 0755)
		os.WriteFile(filepath.Join(mpamPath, fileNameForCache), []byte("invalid"), 0644)

		c := &mpamCollector{
			logger:     logger, // 使用可捕获的logger
			cacheUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"), "MPAM L3 cache usage.", []string{"group", "id", "cpu_list", "mode"}, nil),
		}
		ch := make(chan prometheus.Metric)
		labels := mpamMetricsCommonLabels{
			groupName: "groupB",
		}
		err := c.updateResUsageMetrics(ch, "groupB", labels, c.cacheUsage, []string{nameKeyForCacheUsageDirs + "_test"})

		assert.NoError(t, err)
		assert.Contains(t, logBuf.String(), "failed to parse resource usage data")
		assert.Contains(t, logBuf.String(), "invalid")
	})

	t.Run("dir_not_exist", func(t *testing.T) {
		testDir := t.TempDir()
		*resctlMountPath = testDir

		c := &mpamCollector{
			logger:     slog.Default(), // 必须有
			cacheUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"), "MPAM L3 cache usage.", []string{"group", "id", "cpu_list", "mode"}, nil),
		}
		ch := make(chan prometheus.Metric)
		labels := mpamMetricsCommonLabels{
			groupName: "non_exist",
		}
		err := c.updateResUsageMetrics(ch, "non_exist", labels, c.cacheUsage, []string{nameKeyForCacheUsageDirs + "_test"})

		assert.NoError(t, err) // 预期跳过不存在的目录
	})

	t.Run("empty_target_dir", func(t *testing.T) {
		testDir := t.TempDir()
		*resctlMountPath = testDir

		c := &mpamCollector{
			logger: slog.Default(), // 必须有
			cacheUsage: prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"),
				"MPAM L3 cache usage.",
				[]string{"group", "id", "cpu_list", "mode"},
				nil),
		}
		ch := make(chan prometheus.Metric)
		labels := mpamMetricsCommonLabels{
			groupName: "empty_dir",
		}
		os.MkdirAll(filepath.Join(*resctlMountPath, "empty_dir", dirPathForMPAMGroupData), 0644)
		err := c.updateResUsageMetrics(ch, "empty_dir", labels, c.cacheUsage, []string{})

		// 预期返回错误，包含"non fatal error, targetDir is empty"
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non fatal error, targetDir is empty")
	})
}

func TestUpdateMPAMResUsageMetrics(t *testing.T) {
	origPath := *resctlMountPath
	defer func() { *resctlMountPath = origPath }()

	t.Run("both_metrics_success", func(t *testing.T) {
		testDir := t.TempDir()
		*resctlMountPath = testDir
		// 创建缓存和内存测试数据
		groupDir := filepath.Join(testDir, "groupX")

		cacheDir := filepath.Join(groupDir, dirPathForMPAMGroupData, nameKeyForCacheUsageDirs+"test1")
		memDir := filepath.Join(groupDir, dirPathForMPAMGroupData, nameKeyForMemUsageDirs+"test1")
		// 权限必须是07XX
		os.MkdirAll(cacheDir, 0744)
		os.MkdirAll(memDir, 0744)
		os.WriteFile(filepath.Join(cacheDir, fileNameForCache), []byte("1024"), 0644)
		os.WriteFile(filepath.Join(memDir, fileNameForMem), []byte("2048"), 0644)
		c := &mpamCollector{
			logger: slog.Default(), // 必须有
			cacheUsage: prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"),
				"MPAM L3 cache usage.",
				[]string{"group", "id", "cpu_list", "mode"},
				nil),
			memUsage: prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mpam", "mem_usage"),
				"MPAM memory bw usage.",
				[]string{"group", "id", "cpu_list", "mode"},
				nil,
			),
		}

		ch := make(chan prometheus.Metric, 2)
		err := c.updateMPAMResUsageMetrics(ch, "groupX", mpamMetricsCommonLabels{groupName: "groupX"})

		close(ch)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ch))
	})

	t.Run("wrong_permission", func(t *testing.T) {
		testDir := t.TempDir()
		*resctlMountPath = testDir
		// 创建缓存和内存测试数据
		groupDir := filepath.Join(testDir, "groupX")

		cacheDir := filepath.Join(groupDir, dirPathForMPAMGroupData, nameKeyForCacheUsageDirs+"test1")
		memDir := filepath.Join(groupDir, dirPathForMPAMGroupData, nameKeyForMemUsageDirs+"test1")
		// 权限必须是07XX
		os.MkdirAll(cacheDir, 0644)
		os.MkdirAll(memDir, 0644)
		os.WriteFile(filepath.Join(cacheDir, fileNameForCache), []byte("1024"), 0644)
		os.WriteFile(filepath.Join(memDir, fileNameForMem), []byte("2048"), 0644)
		c := &mpamCollector{
			logger: slog.Default(), // 必须有
			cacheUsage: prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"),
				"MPAM L3 cache usage.",
				[]string{"group", "id", "cpu_list", "mode"},
				nil),
			memUsage: prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mpam", "mem_usage"),
				"MPAM memory bw usage.",
				[]string{"group", "id", "cpu_list", "mode"},
				nil,
			),
		}

		ch := make(chan prometheus.Metric, 2)
		err := c.updateMPAMResUsageMetrics(ch, "groupX", mpamMetricsCommonLabels{groupName: "groupX"})

		close(ch)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("partial_metrics_failure", func(t *testing.T) {
		testDir := t.TempDir()
		*resctlMountPath = testDir

		// 只创建缓存目录（不创建内存目录）
		cacheDir := filepath.Join(testDir, "groupY", dirPathForMPAMGroupData, nameKeyForCacheUsageDirs+"test1")
		os.MkdirAll(cacheDir, 0755)
		os.WriteFile(filepath.Join(cacheDir, fileNameForCache), []byte("1024"), 0644)

		c := &mpamCollector{
			logger: slog.Default(), // 必须有
			cacheUsage: prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"),
				"MPAM L3 cache usage.",
				[]string{"group", "id", "cpu_list", "mode"},
				nil),
			memUsage: prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "mpam", "mem_usage"),
				"MPAM memory bw usage.",
				[]string{"group", "id", "cpu_list", "mode"},
				nil,
			),
		}

		ch := make(chan prometheus.Metric, 1)
		err := c.updateMPAMResUsageMetrics(ch, "groupY", mpamMetricsCommonLabels{groupName: "groupY"})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(ch))

	})
}

func TestUpdateResConfigMetrics(t *testing.T) {
	c := &mpamCollector{}
	labels := mpamMetricsCommonLabels{
		groupName: "test-group",
		cpuList:   "0-3",
		mode:      "balanced",
	}
	metricType := prometheus.NewDesc(
		"test_metric",
		"Test metric",
		[]string{"group", "cpu", "mode", "config", "id"},
		nil,
	)

	// 测试用例1：正常数据
	t.Run("valid_config_data", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 3)
		defer close(ch)
		configData := &ConfigData{
			"cache":  {"L3": 2048},
			"memory": {"DDR": 4096},
		}

		err := c.updateResConfigMetrics(ch, labels, metricType, configData)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ch))
	})

	// 测试用例2：空配置数据
	t.Run("empty_config_data", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 3)
		defer close(ch)
		configData := &ConfigData{}

		err := c.updateResConfigMetrics(ch, labels, metricType, configData)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ch))
	})

	// 测试用例3：指标标签验证
	t.Run("metric_labels", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 3)
		defer close(ch)
		configData := &ConfigData{"cache": {"L2": 1024}}
		err := c.updateResConfigMetrics(ch, labels, metricType, configData)

		metric := <-ch
		assert.NoError(t, err)
		assert.Equal(t, metricType.String(), metric.Desc().String())
	})
}

func TestUpdateMPAMResConfigMetrics(t *testing.T) {
	c := &mpamCollector{
		cacheConfig: prometheus.NewDesc(
			"cache_config",
			"Cache config",
			[]string{"group", "cpu", "mode", "config", "id"},
			nil,
		),
		memConfig: prometheus.NewDesc(
			"mem_config",
			"Memory config",
			[]string{"group", "cpu", "mode", "config", "id"},
			nil,
		),
	}

	labels := mpamMetricsCommonLabels{
		groupName: "test-group",
		cpuList:   "0-3",
		mode:      "balanced",
	}

	t.Run("both_configs", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 10)
		defer close(ch)

		// 创建临时测试文件
		testFile := createTestConfigFile(t, "L3:0=256\nMB:0=1024")
		defer os.Remove(testFile)

		// 替换全局变量便于测试
		oldPath := *resctlMountPath
		*resctlMountPath = filepath.Dir(testFile)
		defer func() { *resctlMountPath = oldPath }()

		err := c.updateMPAMResConfigMetrics(ch, filepath.Base(testFile), labels)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ch)) // 预期生成2个指标
	})

	t.Run("config_file_error", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 10)
		defer close(ch)

		// 使用不存在的路径
		err := c.updateMPAMResConfigMetrics(ch, "invalid/path", labels)
		assert.ErrorContains(t, err, "failed to get mpam config data")
	})

	t.Run("partial_configs", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 10)
		defer close(ch)

		testFile := createTestConfigFile(t, "L3:0=512")
		defer os.Remove(testFile)

		oldPath := *resctlMountPath
		*resctlMountPath = filepath.Dir(testFile)
		defer func() { *resctlMountPath = oldPath }()

		err := c.updateMPAMResConfigMetrics(ch, filepath.Base(testFile), labels)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ch)) // 只生成cache指标
	})
}

// 创建临时配置文件
func createTestConfigFile(t *testing.T, content string) string {
	dir := t.TempDir()
	filePath := filepath.Join(dir, configDataFile)
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
	return dir
}
