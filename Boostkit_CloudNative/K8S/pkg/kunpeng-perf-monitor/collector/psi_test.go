package collector

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckCgroupVersion 测试checkCgroupVersion函数的各种场景
func TestCheckCgroupVersion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name           string
		cgroupPath     string
		mockOutput     []byte
		mockError      error
		expectedResult int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "cgroup v2 filesystem - cgroup2",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("cgroup2\n"),
			mockError:      nil,
			expectedResult: cgroupV2,
			expectError:    false,
		},
		{
			name:           "cgroup v2 filesystem - cgroup2fs",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("cgroup2fs\n"),
			mockError:      nil,
			expectedResult: cgroupV2,
			expectError:    false,
		},
		{
			name:           "cgroup v1 filesystem - tmpfs",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("tmpfs\n"),
			mockError:      nil,
			expectedResult: cgroupV1,
			expectError:    false,
		},
		{
			name:           "unknown filesystem - ext4",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("ext4\n"),
			mockError:      nil,
			expectedResult: unknownCgroupVersion,
			expectError:    true,
			errorContains:  "unknown cgroup filesystem",
		},
		{
			name:           "unknown filesystem - xfs",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("xfs\n"),
			mockError:      nil,
			expectedResult: unknownCgroupVersion,
			expectError:    true,
			errorContains:  "unknown cgroup filesystem",
		},
		{
			name:           "stat command fails with exit error",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     nil,
			mockError:      &exec.ExitError{Stderr: []byte("stat: cannot stat '/sys/fs/cgroup': No such file or directory")},
			expectedResult: unknownCgroupVersion,
			expectError:    true,
			errorContains:  "stat cgroupPath",
		},
		{
			name:           "stat command fails with generic error",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     nil,
			mockError:      assert.AnError,
			expectedResult: unknownCgroupVersion,
			expectError:    true,
			errorContains:  "stat cgroupPath",
		},
		{
			name:           "empty path",
			cgroupPath:     "",
			mockOutput:     nil,
			mockError:      nil,
			expectedResult: unknownCgroupVersion,
			expectError:    true,
			errorContains:  "cgroupPath is empty",
		},
		{
			name:           "cgroup2 with newline",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("cgroup2\n"),
			mockError:      nil,
			expectedResult: cgroupV2,
			expectError:    false,
		},
		{
			name:           "empty output",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte(""),
			mockError:      nil,
			expectedResult: unknownCgroupVersion,
			expectError:    true,
			errorContains:  "unknown cgroup filesystem",
		},
		{
			name:           "whitespace only output",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("   \n"),
			mockError:      nil,
			expectedResult: unknownCgroupVersion,
			expectError:    true,
			errorContains:  "unknown cgroup filesystem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始的execCommand函数
			originalExecCommand := execCommand

			// 设置mock的execCommand
			execCommand = func(command string, args ...string) ([]byte, error) {
				require.Equal(t, "stat", command)
				require.Equal(t, []string{"-fc", "%T", tt.cgroupPath}, args)
				return tt.mockOutput, tt.mockError
			}

			// 测试结束后恢复原始的execCommand函数
			defer func() {
				execCommand = originalExecCommand
			}()

			version, err := checkCgroupVersion(tt.cgroupPath, logger)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Equal(t, tt.expectedResult, version)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, version)
			}
		})
	}
}

// TestGetFinalPath 测试getFinalPath函数的各种场景
func TestGetFinalPath(t *testing.T) {
	tests := []struct {
		name           string
		rootPath       string
		fileCandidates []string
		setupFunc      func() func()
		expectedPath   string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "empty root path",
			rootPath:       "",
			fileCandidates: []string{"file1", "file2"},
			expectedPath:   "",
			expectError:    true,
			errorContains:  "rootPath is empty",
		},
		{
			name:           "empty file candidates",
			rootPath:       "/tmp",
			fileCandidates: []string{},
			expectedPath:   "",
			expectError:    true,
			errorContains:  "target file list is empty",
		},
		{
			name:           "first file exists",
			rootPath:       "/tmp",
			fileCandidates: []string{"existing_file", "non_existing_file"},
			setupFunc: func() func() {
				// 创建临时文件
				filePath := "/tmp/existing_file"
				f, err := os.Create(filePath)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				f.Close()

				return func() {
					os.Remove(filePath)
				}
			},
			expectedPath: "/tmp/existing_file",
			expectError:  false,
		},
		{
			name:           "second file exists",
			rootPath:       "/tmp",
			fileCandidates: []string{"non_existing_file1", "existing_file", "non_existing_file2"},
			setupFunc: func() func() {
				// 创建临时文件
				filePath := "/tmp/existing_file"
				f, err := os.Create(filePath)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				f.Close()

				return func() {
					os.Remove(filePath)
				}
			},
			expectedPath: "/tmp/existing_file",
			expectError:  false,
		},
		{
			name:           "no files exist",
			rootPath:       "/tmp",
			fileCandidates: []string{"non_existing_file1", "non_existing_file2"},
			expectedPath:   "",
			expectError:    true,
			errorContains:  "no target file found",
		},
		{
			name:           "directory exists",
			rootPath:       "/tmp",
			fileCandidates: []string{"existing_dir"},
			setupFunc: func() func() {
				// 创建临时目录
				dirPath := "/tmp/existing_dir"
				err := os.Mkdir(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}

				return func() {
					os.RemoveAll(dirPath)
				}
			},
			expectedPath: "/tmp/existing_dir",
			expectError:  false,
		},
		{
			name:           "file with subdirectory path",
			rootPath:       "/tmp",
			fileCandidates: []string{"subdir/file"},
			setupFunc: func() func() {
				// 创建临时目录和文件
				dirPath := "/tmp/subdir"
				err := os.Mkdir(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}

				filePath := "/tmp/subdir/file"
				f, err := os.Create(filePath)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				f.Close()

				return func() {
					os.RemoveAll(dirPath)
				}
			},
			expectedPath: "/tmp/subdir/file",
			expectError:  false,
		},
		{
			name:           "relative path handling",
			rootPath:       ".",
			fileCandidates: []string{"test_file"},
			setupFunc: func() func() {
				// 在当前目录创建临时文件
				filePath := "test_file"
				f, err := os.Create(filePath)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				f.Close()

				return func() {
					os.Remove(filePath)
				}
			},
			expectedPath: "test_file",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行setup函数（如果有）并获取清理函数
			var cleanup func()
			if tt.setupFunc != nil {
				cleanup = tt.setupFunc()
				if cleanup != nil {
					defer cleanup()
				}
			}

			path, err := getFinalPath(tt.rootPath, tt.fileCandidates)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Equal(t, tt.expectedPath, path)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPath, path)
			}
		})
	}
}

// TestGetCgroupSearchPath 测试getCgroupSearchPath函数的各种场景
func TestGetCgroupSearchPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name           string
		setupFunc      func(t *testing.T) (cgroupMountPath, expectedPath string, cleanup func())
		mockStatOutput []byte
		mockStatError  error
		expectError    bool
		errorContains  string
	}{
		{
			name: "cgroup v2 with kubepods directory",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tmpDir := t.TempDir()
				dirPath := filepath.Join(tmpDir, "kubepods")
				err := os.MkdirAll(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, dirPath, func() {}
			},
			mockStatOutput: []byte("cgroup2\n"),
			expectError:    false,
		},
		{
			name: "cgroup v2 with kubepods.slice directory",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tmpDir := t.TempDir()
				dirPath := filepath.Join(tmpDir, "kubepods.slice")
				err := os.MkdirAll(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, dirPath, func() {}
			},
			mockStatOutput: []byte("cgroup2\n"),
			expectError:    false,
		},
		{
			name: "cgroup v1 with cpu,cpuacct and kubepods directory",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tmpDir := t.TempDir()
				basePath := filepath.Join(tmpDir, "cpu,cpuacct")
				err := os.MkdirAll(basePath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}

				dirPath := filepath.Join(basePath, "kubepods")
				err = os.MkdirAll(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, dirPath, func() {}
			},
			mockStatOutput: []byte("tmpfs\n"),
			expectError:    false,
		},
		{
			name: "cgroup v1 with cpu,cpuacct and kubepods.slice directory",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tmpDir := t.TempDir()
				basePath := filepath.Join(tmpDir, "cpu,cpuacct")
				err := os.MkdirAll(basePath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}

				dirPath := filepath.Join(basePath, "kubepods.slice")
				err = os.MkdirAll(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, dirPath, func() {}
			},
			mockStatOutput: []byte("tmpfs\n"),
			expectError:    false,
		},
		{
			name: "empty cgroupMountPath",
			setupFunc: func(t *testing.T) (string, string, func()) {
				return "", "", func() {}
			},
			mockStatOutput: nil,
			expectError:    true,
			errorContains:  "cgroupMountPath uninitialized",
		},
		{
			name: "nil cgroupMountPath",
			setupFunc: func(t *testing.T) (string, string, func()) {
				// 保存原始值并设置为nil
				originalCgroupMountPath := cgroupMountPath
				cgroupMountPath = nil
				return "", "", func() {
					cgroupMountPath = originalCgroupMountPath
				}
			},
			mockStatOutput: nil,
			expectError:    true,
			errorContains:  "cgroupMountPath uninitialized",
		},
		{
			name: "stat command fails",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tmpDir := t.TempDir()
				return tmpDir, "", func() {}
			},
			mockStatOutput: nil,
			mockStatError:  assert.AnError,
			expectError:    true,
			errorContains:  "failed to check cgroup version",
		},
		{
			name: "unknown cgroup filesystem",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tmpDir := t.TempDir()
				return tmpDir, "", func() {}
			},
			mockStatOutput: []byte("ext4\n"),
			expectError:    true,
			errorContains:  "failed to check cgroup version",
		},
		{
			name: "no target directories found",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tmpDir := t.TempDir()
				// 确保目标目录不存在，临时目录是空的
				return tmpDir, "", func() {}
			},
			mockStatOutput: []byte("cgroup2\n"),
			expectError:    true,
			errorContains:  "failed to get cgroupSearchPath path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始函数和变量
			originalExecCommand := execCommand
			originalCgroupMountPath := cgroupMountPath

			// 执行setup函数获取路径和清理函数
			cgroupMountPathStr, expectedPath, cleanup := tt.setupFunc(t)
			defer cleanup()

			// 设置mock的execCommand
			execCommand = func(command string, args ...string) ([]byte, error) {
				require.Equal(t, "stat", command)
				require.Equal(t, []string{"-fc", "%T", cgroupMountPathStr}, args)
				return tt.mockStatOutput, tt.mockStatError
			}

			// 设置cgroupMountPath
			if cgroupMountPathStr != "" {
				cgroupMountPath = &cgroupMountPathStr
			}

			// 测试结束后恢复原始函数和变量
			defer func() {
				execCommand = originalExecCommand
				cgroupMountPath = originalCgroupMountPath
			}()

			path, err := getCgroupSearchPath(logger)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Equal(t, expectedPath, path)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedPath, path)
			}
		})
	}
}

// TestNewPSICollector 测试NewPSICollector函数的各种场景
func TestNewPSICollector(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name            string
		setupFunc       func(t *testing.T) (cgroupMountPath string, mockStatOutput []byte, mockStatError error, cleanup func())
		expectError     bool
		errorContains   string
		expectCollector bool
	}{
		{
			name: "successful creation with cgroup v2 and kubepods",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				tmpDir := t.TempDir()
				dirPath := filepath.Join(tmpDir, "kubepods")
				err := os.MkdirAll(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, []byte("cgroup2\n"), nil, func() {}
			},
			expectError:     false,
			expectCollector: true,
		},
		{
			name: "successful creation with cgroup v1 and kubepods.slice",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				tmpDir := t.TempDir()
				basePath := filepath.Join(tmpDir, "cpu,cpuacct")
				err := os.MkdirAll(basePath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}

				dirPath := filepath.Join(basePath, "kubepods.slice")
				err = os.MkdirAll(dirPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, []byte("tmpfs\n"), nil, func() {}
			},
			expectError:     false,
			expectCollector: true,
		},
		{
			name: "cgroup mount path stat fails",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				tmpDir := t.TempDir()
				// 删除临时目录，使其不存在
				os.RemoveAll(tmpDir)
				return tmpDir, []byte("cgroup2\n"), nil, func() {}
			},
			expectError:     true,
			errorContains:   "failed to stat cgroup mount path",
			expectCollector: false,
		},
		{
			name: "getCgroupSearchPath fails with stat error",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				tmpDir := t.TempDir()
				// 创建目录确保stat成功
				err := os.MkdirAll(tmpDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, nil, assert.AnError, func() {}
			},
			expectError:     true,
			errorContains:   "failed to check cgroup version",
			expectCollector: false,
		},
		{
			name: "getCgroupSearchPath fails with unknown filesystem",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				tmpDir := t.TempDir()
				// 创建目录确保stat成功
				err := os.MkdirAll(tmpDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, []byte("ext4\n"), nil, func() {}
			},
			expectError:     true,
			errorContains:   "failed to check cgroup version",
			expectCollector: false,
		},
		{
			name: "getCgroupSearchPath fails with no target directories",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				tmpDir := t.TempDir()
				// 创建目录确保stat成功，但不创建目标目录
				err := os.MkdirAll(tmpDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
				return tmpDir, []byte("cgroup2\n"), nil, func() {}
			},
			expectError:     true,
			errorContains:   "failed to get cgroupSearchPath path",
			expectCollector: false,
		},
		{
			name: "nil cgroupMountPath",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				// 保存原始值并设置为nil
				originalCgroupMountPath := cgroupMountPath
				cgroupMountPath = nil
				return "", nil, nil, func() {
					cgroupMountPath = originalCgroupMountPath
				}
			},
			expectError:     true,
			errorContains:   "cgroupMountPath uninitialized",
			expectCollector: false,
		},
		{
			name: "empty cgroupMountPath",
			setupFunc: func(t *testing.T) (string, []byte, error, func()) {
				// 保存原始值并设置为空字符串
				originalCgroupMountPath := cgroupMountPath
				emptyPath := ""
				cgroupMountPath = &emptyPath
				return "", nil, nil, func() {
					cgroupMountPath = originalCgroupMountPath
				}
			},
			expectError:     true,
			errorContains:   "cgroupMountPath uninitialized",
			expectCollector: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始函数和变量
			originalExecCommand := execCommand
			originalCgroupMountPath := cgroupMountPath

			// 执行setup函数获取路径、mock数据和清理函数
			cgroupMountPathStr, mockStatOutput, mockStatError, cleanup := tt.setupFunc(t)
			defer cleanup()

			// 设置mock的execCommand
			execCommand = func(command string, args ...string) ([]byte, error) {
				require.Equal(t, "stat", command)
				if cgroupMountPathStr != "" {
					require.Equal(t, []string{"-fc", "%T", cgroupMountPathStr}, args)
				}
				return mockStatOutput, mockStatError
			}

			// 设置cgroupMountPath
			if cgroupMountPathStr != "" {
				cgroupMountPath = &cgroupMountPathStr
			}

			// 测试结束后恢复原始函数和变量
			defer func() {
				execCommand = originalExecCommand
				cgroupMountPath = originalCgroupMountPath
			}()

			collector, err := NewPSICollector(logger)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, collector)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, collector)

				// 验证collector的类型和字段
				psiCollector, ok := collector.(*psiCollector)
				assert.True(t, ok)
				assert.NotNil(t, psiCollector)
				assert.NotEmpty(t, psiCollector.cgroupSearchPath)
				assert.NotNil(t, psiCollector.pressureMetric)
				assert.Equal(t, logger, psiCollector.logger)
			}
		})
	}
}

// TestGetPSICgroups 测试getPSICgroups函数的各种场景
func TestGetPSICgroups(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name           string
		setupFunc      func(t *testing.T) string
		expectedGroups []string
		expectError    bool
		errorContains  string
	}{
		{
			name: "multiple_psi_cgroups",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				groupDirs := []string{"groupA", "groupB", "groupC"}
				for _, dir := range groupDirs {
					path := filepath.Join(testDir, dir)
					require.NoError(t, os.MkdirAll(path, 0755))
					require.NoError(t, os.WriteFile(filepath.Join(path, psiCgroupFeatureFile),
						[]byte("some avg10=0.00 avg60=0.00 avg300=0.00 total=0"), 0644))
				}
				return testDir
			},
			expectedGroups: []string{"groupA", "groupB", "groupC"},
			expectError:    false,
		},
		{
			name: "single_psi_cgroup",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				path := filepath.Join(testDir, "singleGroup")
				require.NoError(t, os.MkdirAll(path, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(path, psiCgroupFeatureFile),
					[]byte("some avg10=0.00 avg60=0.00 avg300=0.00 total=0"), 0644))
				return testDir
			},
			expectedGroups: []string{"singleGroup"},
			expectError:    false,
		},
		{
			name: "empty_directory",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			expectedGroups: []string{},
			expectError:    true,
			errorContains:  "PSI cgroup is empty",
		},
		{
			name: "non_existent_path",
			setupFunc: func(t *testing.T) string {
				return "/path/that/does/not/exist"
			},
			expectedGroups: []string{},
			expectError:    true,
			errorContains:  "failed to get PSI cgroups",
		},
		{
			name: "no_cpu_pressure_files",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				groupDirs := []string{"groupA", "groupB"}
				for _, dir := range groupDirs {
					path := filepath.Join(testDir, dir)
					require.NoError(t, os.MkdirAll(path, 0755))
					require.NoError(t, os.WriteFile(filepath.Join(path, "other.file"),
						[]byte("content"), 0644))
				}
				return testDir
			},
			expectedGroups: []string{},
			expectError:    true,
			errorContains:  "PSI cgroup is empty",
		},
		{
			name: "mixed_cpu_pressure_files",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()

				// 有cpu.pressure文件的目录
				validPath := filepath.Join(testDir, "validGroup")
				require.NoError(t, os.MkdirAll(validPath, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(validPath, psiCgroupFeatureFile),
					[]byte("some avg10=0.00 avg60=0.00 avg300=0.00 total=0"), 0644))

				// 没有cpu.pressure文件的目录
				invalidPath := filepath.Join(testDir, "invalidGroup")
				require.NoError(t, os.MkdirAll(invalidPath, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(invalidPath, "other.file"),
					[]byte("content"), 0644))

				return testDir
			},
			expectedGroups: []string{"validGroup"},
			expectError:    false,
		},
		{
			name: "nested_directory_structure",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				nestedGroups := []string{"level1/groupA", "level1/level2/groupB"}
				for _, dir := range nestedGroups {
					path := filepath.Join(testDir, dir)
					require.NoError(t, os.MkdirAll(path, 0755))
					require.NoError(t, os.WriteFile(filepath.Join(path, psiCgroupFeatureFile),
						[]byte("some avg10=0.00 avg60=0.00 avg300=0.00 total=0"), 0644))
				}
				return testDir
			},
			expectedGroups: []string{"groupA", "groupB"},
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行setup函数获取测试目录
			testDir := tt.setupFunc(t)

			// 创建psiCollector实例
			c := &psiCollector{
				cgroupSearchPath: testDir,
				logger:           logger,
			}

			// 执行测试
			result, err := c.getPSICgroups()

			// 验证结果
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, len(tt.expectedGroups), len(result))

				// 验证每个预期的group都存在
				for _, expectedGroup := range tt.expectedGroups {
					assert.Contains(t, result, expectedGroup)
				}

				// 验证没有额外的group
				assert.Equal(t, len(tt.expectedGroups), len(result))
			}
		})
	}
}

// TestParsePSILine 测试parsePSILine函数的各种场景
func TestParsePSILine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		delimiter     string
		expectedData  PSICgroupResourceData
		expectError   bool
		errorContains string
	}{
		{
			name:      "valid_full_line_with_space_delimiter",
			line:      "full avg10=0.00 avg60=0.00 avg300=0.00 total=123456",
			delimiter: " ",
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.00,
					"avg60":  0.00,
					"avg300": 0.00,
					"total":  123456.00,
				},
			},
			expectError: false,
		},
		{
			name:      "valid_some_line_with_tab_delimiter",
			line:      "some\tavg10=1.50\tavg60=2.75\tavg300=3.25\ttotal=987654",
			delimiter: "\t",
			expectedData: PSICgroupResourceData{
				"some": PSICgroupResourceItemData{
					"avg10":  1.50,
					"avg60":  2.75,
					"avg300": 3.25,
					"total":  987654.00,
				},
			},
			expectError: false,
		},
		{
			name:      "valid_line_with_mixed_values",
			line:      "full avg10=0.05 avg60=0.10 avg300=0.15 total=123456789",
			delimiter: " ",
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.05,
					"avg60":  0.10,
					"avg300": 0.15,
					"total":  123456789.00,
				},
			},
			expectError: false,
		},
		{
			name:          "invalid_line_format_missing_delimiter",
			line:          "fullavg10=0.00avg60=0.00",
			delimiter:     " ",
			expectError:   true,
			errorContains: "invalid line format",
		},
		{
			name:          "invalid_line_format_only_item",
			line:          "full",
			delimiter:     " ",
			expectError:   true,
			errorContains: "invalid line format",
		},
		{
			name:          "invalid_key_value_pair_missing_equal",
			line:          "full avg10 0.00 avg60=0.00",
			delimiter:     " ",
			expectError:   true,
			errorContains: "invalid key-value pair",
		},
		{
			name:          "invalid_key_value_pair_empty_pair",
			line:          "full  avg10=0.00 =0.00",
			delimiter:     " ",
			expectError:   true,
			errorContains: "invalid key-value pair",
		},
		{
			name:          "invalid_numeric_value",
			line:          "full avg10=abc avg60=0.00",
			delimiter:     " ",
			expectError:   true,
			errorContains: "failed to convert the string to float64",
		},
		{
			name:      "valid_line_with_extra_spaces",
			line:      "full   avg10=0.00   avg60=0.00   avg300=0.00   total=123456  ",
			delimiter: " ",
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.00,
					"avg60":  0.00,
					"avg300": 0.00,
					"total":  123456.00,
				},
			},
			expectError: false,
		},
		{
			name:      "valid_line_with_empty_pairs_ignored",
			line:      "full avg10=0.00  avg60=0.00   avg300=0.00 total=123456",
			delimiter: " ",
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.00,
					"avg60":  0.00,
					"avg300": 0.00,
					"total":  123456.00,
				},
			},
			expectError: false,
		},
		{
			name:      "valid_line_with_custom_delimiter",
			line:      "full|avg10=0.00|avg60=0.00|avg300=0.00|total=123456",
			delimiter: "|",
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.00,
					"avg60":  0.00,
					"avg300": 0.00,
					"total":  123456.00,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行测试
			result, err := parsePSILine(tt.line, tt.delimiter)

			// 验证结果
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				// 错误情况下，result应该为nil
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, len(tt.expectedData), len(result))

				// 验证每个预期的item都存在
				for expectedItem, expectedValues := range tt.expectedData {
					assert.Contains(t, result, expectedItem)
					assert.Equal(t, len(expectedValues), len(result[expectedItem]))

					// 验证每个key-value对
					for expectedKey, expectedValue := range expectedValues {
						assert.Contains(t, result[expectedItem], expectedKey)
						assert.Equal(t, expectedValue, result[expectedItem][expectedKey])
					}
				}
			}
		})
	}
}

// TestGetPSIData 测试getPSIData函数的各种场景
func TestGetPSIData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) string
		expectedData  PSICgroupResourceData
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_psi_file_with_multiple_lines",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				psiFile := filepath.Join(testDir, "cpu.pressure")
				content := `full avg10=0.00 avg60=0.00 avg300=0.00 total=123456
some avg10=1.50 avg60=2.75 avg300=3.25 total=987654`
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				return psiFile
			},
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.00,
					"avg60":  0.00,
					"avg300": 0.00,
					"total":  123456.00,
				},
				"some": PSICgroupResourceItemData{
					"avg10":  1.50,
					"avg60":  2.75,
					"avg300": 3.25,
					"total":  987654.00,
				},
			},
			expectError: false,
		},
		{
			name: "valid_psi_file_single_line",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				psiFile := filepath.Join(testDir, "cpu.pressure")
				content := "full avg10=0.05 avg60=0.10 avg300=0.15 total=123456789"
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				return psiFile
			},
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.05,
					"avg60":  0.10,
					"avg300": 0.15,
					"total":  123456789.00,
				},
			},
			expectError: false,
		},
		{
			name: "valid_psi_file_with_extra_spaces",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				psiFile := filepath.Join(testDir, "cpu.pressure")
				content := `full   avg10=0.00   avg60=0.00   avg300=0.00   total=123456  
some   avg10=1.00   avg60=2.00   avg300=3.00   total=456789  `
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				return psiFile
			},
			expectedData: PSICgroupResourceData{
				"full": PSICgroupResourceItemData{
					"avg10":  0.00,
					"avg60":  0.00,
					"avg300": 0.00,
					"total":  123456.00,
				},
				"some": PSICgroupResourceItemData{
					"avg10":  1.00,
					"avg60":  2.00,
					"avg300": 3.00,
					"total":  456789.00,
				},
			},
			expectError: false,
		},
		{
			name: "non_existent_file",
			setupFunc: func(t *testing.T) string {
				return "/path/that/does/not/exist/cpu.pressure"
			},
			expectError:   true,
			errorContains: "failed to open psi file",
		},
		{
			name: "psi_file_with_invalid_line",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				psiFile := filepath.Join(testDir, "cpu.pressure")
				content := `full avg10=0.00 avg60=0.00 avg300=0.00 total=123456
invalid_line_format
some avg10=1.00 avg60=2.00 avg300=3.00 total=456789`
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				return psiFile
			},
			expectError:   true,
			errorContains: "error parsing line 2",
		},
		{
			name: "psi_file_with_invalid_numeric_value",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				psiFile := filepath.Join(testDir, "cpu.pressure")
				content := `full avg10=abc avg60=0.00 avg300=0.00 total=123456`
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				return psiFile
			},
			expectError:   true,
			errorContains: "error parsing line 1",
		},
		{
			name: "empty_psi_file",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				psiFile := filepath.Join(testDir, "cpu.pressure")
				require.NoError(t, os.WriteFile(psiFile, []byte(""), 0644))
				return psiFile
			},
			expectedData: PSICgroupResourceData{},
			expectError:  false,
		},
		{
			name: "psi_file_with_only_whitespace",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				psiFile := filepath.Join(testDir, "cpu.pressure")
				require.NoError(t, os.WriteFile(psiFile, []byte("   \n   \n"), 0644))
				return psiFile
			},
			expectedData: nil,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行setup函数获取测试文件路径
			psiFilePath := tt.setupFunc(t)

			// 创建psiCollector实例
			c := &psiCollector{
				logger: logger,
			}

			// 执行测试
			result, err := c.getPSIData(psiFilePath)

			// 验证结果
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, len(tt.expectedData), len(result))

				// 验证每个预期的item都存在
				for expectedItem, expectedValues := range tt.expectedData {
					assert.Contains(t, result, expectedItem)
					assert.Equal(t, len(expectedValues), len(result[expectedItem]))

					// 验证每个key-value对
					for expectedKey, expectedValue := range expectedValues {
						assert.Contains(t, result[expectedItem], expectedKey)
						assert.Equal(t, expectedValue, result[expectedItem][expectedKey])
					}
				}
			}
		})
	}
}

// TestGetPSIMetric 测试getPSIMetric函数的各种场景
func TestGetPSIMetric(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) (string, string, string)
		expectedCalls []struct {
			psiCgroupName string
			resource      string
			item          string
			key           string
			value         float64
		}
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_psi_data_single_item",
			setupFunc: func(t *testing.T) (string, string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建PSI文件
				psiFile := filepath.Join(cgroupPath, "cpu.pressure")
				content := "full avg10=0.00 avg60=0.00 avg300=0.00 total=123456"
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))

				return testDir, cgroupRelPath, "cpu.pressure"
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{
				{"test-cgroup", "cpu.pressure", "full", "avg10", 0.00},
				{"test-cgroup", "cpu.pressure", "full", "avg60", 0.00},
				{"test-cgroup", "cpu.pressure", "full", "avg300", 0.00},
				{"test-cgroup", "cpu.pressure", "full", "total", 123456.00},
			},
			expectError: false,
		},
		{
			name: "valid_psi_data_multiple_items",
			setupFunc: func(t *testing.T) (string, string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-multi"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建PSI文件
				psiFile := filepath.Join(cgroupPath, "memory.pressure")
				content := `full avg10=0.05 avg60=0.10 avg300=0.15 total=123456789
some avg10=1.50 avg60=2.75 avg300=3.25 total=987654`
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))

				return testDir, cgroupRelPath, "memory.pressure"
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{
				{"test-cgroup-multi", "memory.pressure", "full", "avg10", 0.05},
				{"test-cgroup-multi", "memory.pressure", "full", "avg60", 0.10},
				{"test-cgroup-multi", "memory.pressure", "full", "avg300", 0.15},
				{"test-cgroup-multi", "memory.pressure", "full", "total", 123456789.00},
				{"test-cgroup-multi", "memory.pressure", "some", "avg10", 1.50},
				{"test-cgroup-multi", "memory.pressure", "some", "avg60", 2.75},
				{"test-cgroup-multi", "memory.pressure", "some", "avg300", 3.25},
				{"test-cgroup-multi", "memory.pressure", "some", "total", 987654.00},
			},
			expectError: false,
		},
		{
			name: "non_existent_psi_file",
			setupFunc: func(t *testing.T) (string, string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构，但不创建PSI文件
				cgroupRelPath := "test-cgroup-no-file"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				return testDir, cgroupRelPath, "io.pressure"
			},
			expectError:   true,
			errorContains: "failed to get psi data",
		},
		{
			name: "invalid_psi_file_content",
			setupFunc: func(t *testing.T) (string, string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-invalid"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建包含无效内容的PSI文件
				psiFile := filepath.Join(cgroupPath, "cpu.pressure")
				content := "full avg10=abc avg60=0.00 avg300=0.00 total=123456"
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))

				return testDir, cgroupRelPath, "cpu.pressure"
			},
			expectError:   true,
			errorContains: "failed to get psi data",
		},
		{
			name: "empty_psi_file",
			setupFunc: func(t *testing.T) (string, string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-empty"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建空PSI文件
				psiFile := filepath.Join(cgroupPath, "memory.pressure")
				require.NoError(t, os.WriteFile(psiFile, []byte(""), 0644))

				return testDir, cgroupRelPath, "memory.pressure"
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{},
			expectError:   true,
			errorContains: "PSI cgroup resource data is empty for file",
		},
		{
			name: "psi_file_with_extra_spaces",
			setupFunc: func(t *testing.T) (string, string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-spaces"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建包含额外空格的PSI文件
				psiFile := filepath.Join(cgroupPath, "io.pressure")
				content := `full   avg10=0.00   avg60=0.00   avg300=0.00   total=123456  
some   avg10=1.00   avg60=2.00   avg300=3.00   total=456789  `
				require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))

				return testDir, cgroupRelPath, "io.pressure"
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{
				{"test-cgroup-spaces", "io.pressure", "full", "avg10", 0.00},
				{"test-cgroup-spaces", "io.pressure", "full", "avg60", 0.00},
				{"test-cgroup-spaces", "io.pressure", "full", "avg300", 0.00},
				{"test-cgroup-spaces", "io.pressure", "full", "total", 123456.00},
				{"test-cgroup-spaces", "io.pressure", "some", "avg10", 1.00},
				{"test-cgroup-spaces", "io.pressure", "some", "avg60", 2.00},
				{"test-cgroup-spaces", "io.pressure", "some", "avg300", 3.00},
				{"test-cgroup-spaces", "io.pressure", "some", "total", 456789.00},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行setup函数获取测试参数
			cgroupSearchPath, psiCgroupRelPath, resource := tt.setupFunc(t)
			psiCgroupName := filepath.Base(psiCgroupRelPath)

			// 创建psiCollector实例
			c := &psiCollector{
				logger:           logger,
				cgroupSearchPath: cgroupSearchPath,
				pressureMetric: prometheus.NewDesc(
					"psi_pressure",
					"Pressure Stall Information metric",
					[]string{"cgroup", "resource", "item", "key"},
					nil,
				),
			}

			// 创建通道来收集发送的指标
			ch := make(chan prometheus.Metric, 100)
			defer close(ch)

			// 执行测试
			err := c.getPSIMetric(ch, psiCgroupName, psiCgroupRelPath, resource)

			// 验证结果
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)

				// 验证发送的指标数量
				assert.Equal(t, len(tt.expectedCalls), len(ch))

				// 验证每个预期的指标调用
				for _, expectedCall := range tt.expectedCalls {
					select {
					case metric := <-ch:
						// 验证指标描述符
						desc := metric.Desc()
						assert.Equal(t, c.pressureMetric, desc)
					default:
						t.Errorf("Expected metric not found: %+v", expectedCall)
					}
				}
			}
		})
	}
}

// TestUpdatePSIMetrics 测试updatePSIMetrics函数的各种场景
func TestUpdatePSIMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) (string, string)
		expectedCalls []struct {
			psiCgroupName string
			resource      string
			item          string
			key           string
			value         float64
		}
		expectError   bool
		errorContains string
	}{
		{
			name: "all_common_resources_exist",
			setupFunc: func(t *testing.T) (string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-all"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建所有常见PSI资源文件
				resources := []string{"cpu.pressure", "io.pressure", "memory.pressure"}
				for _, resource := range resources {
					psiFile := filepath.Join(cgroupPath, resource)
					content := "full avg10=0.01 avg60=0.02 avg300=0.03 total=1000"
					require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				}

				return testDir, cgroupRelPath
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{
				{"test-cgroup-all", "cpu.pressure", "full", "avg10", 0.01},
				{"test-cgroup-all", "cpu.pressure", "full", "avg60", 0.02},
				{"test-cgroup-all", "cpu.pressure", "full", "avg300", 0.03},
				{"test-cgroup-all", "cpu.pressure", "full", "total", 1000.00},
				{"test-cgroup-all", "io.pressure", "full", "avg10", 0.01},
				{"test-cgroup-all", "io.pressure", "full", "avg60", 0.02},
				{"test-cgroup-all", "io.pressure", "full", "avg300", 0.03},
				{"test-cgroup-all", "io.pressure", "full", "total", 1000.00},
				{"test-cgroup-all", "memory.pressure", "full", "avg10", 0.01},
				{"test-cgroup-all", "memory.pressure", "full", "avg60", 0.02},
				{"test-cgroup-all", "memory.pressure", "full", "avg300", 0.03},
				{"test-cgroup-all", "memory.pressure", "full", "total", 1000.00},
			},
			expectError: false,
		},
		{
			name: "missing_common_resource_file",
			setupFunc: func(t *testing.T) (string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-missing"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 只创建部分PSI资源文件，故意缺失一个
				resources := []string{"cpu.pressure", "memory.pressure"} // 缺失io.pressure
				for _, resource := range resources {
					psiFile := filepath.Join(cgroupPath, resource)
					content := "full avg10=0.01 avg60=0.02 avg300=0.03 total=1000"
					require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				}

				return testDir, cgroupRelPath
			},
			expectError:   true,
			errorContains: "failed to update psi metric from io.pressure",
		},
		{
			name: "additional_resource_exists",
			setupFunc: func(t *testing.T) (string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-additional"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建所有PSI资源文件，包括附加资源
				resources := []string{"cpu.pressure", "io.pressure", "memory.pressure", "irq.pressure"}
				for _, resource := range resources {
					psiFile := filepath.Join(cgroupPath, resource)
					content := "full avg10=0.01 avg60=0.02 avg300=0.03 total=1000"
					require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				}

				return testDir, cgroupRelPath
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{
				{"test-cgroup-additional", "cpu.pressure", "full", "avg10", 0.01},
				{"test-cgroup-additional", "cpu.pressure", "full", "avg60", 0.02},
				{"test-cgroup-additional", "cpu.pressure", "full", "avg300", 0.03},
				{"test-cgroup-additional", "cpu.pressure", "full", "total", 1000.00},
				{"test-cgroup-additional", "io.pressure", "full", "avg10", 0.01},
				{"test-cgroup-additional", "io.pressure", "full", "avg60", 0.02},
				{"test-cgroup-additional", "io.pressure", "full", "avg300", 0.03},
				{"test-cgroup-additional", "io.pressure", "full", "total", 1000.00},
				{"test-cgroup-additional", "memory.pressure", "full", "avg10", 0.01},
				{"test-cgroup-additional", "memory.pressure", "full", "avg60", 0.02},
				{"test-cgroup-additional", "memory.pressure", "full", "avg300", 0.03},
				{"test-cgroup-additional", "memory.pressure", "full", "total", 1000.00},
				{"test-cgroup-additional", "irq.pressure", "full", "avg10", 0.01},
				{"test-cgroup-additional", "irq.pressure", "full", "avg60", 0.02},
				{"test-cgroup-additional", "irq.pressure", "full", "avg300", 0.03},
				{"test-cgroup-additional", "irq.pressure", "full", "total", 1000.00},
			},
			expectError: false,
		},
		{
			name: "missing_additional_resource_file",
			setupFunc: func(t *testing.T) (string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-missing-additional"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 只创建常见PSI资源文件，缺失附加资源
				resources := []string{"cpu.pressure", "io.pressure", "memory.pressure"}
				for _, resource := range resources {
					psiFile := filepath.Join(cgroupPath, resource)
					content := "full avg10=0.01 avg60=0.02 avg300=0.03 total=1000"
					require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				}

				return testDir, cgroupRelPath
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{
				{"test-cgroup-missing-additional", "cpu.pressure", "full", "avg10", 0.01},
				{"test-cgroup-missing-additional", "cpu.pressure", "full", "avg60", 0.02},
				{"test-cgroup-missing-additional", "cpu.pressure", "full", "avg300", 0.03},
				{"test-cgroup-missing-additional", "cpu.pressure", "full", "total", 1000.00},
				{"test-cgroup-missing-additional", "io.pressure", "full", "avg10", 0.01},
				{"test-cgroup-missing-additional", "io.pressure", "full", "avg60", 0.02},
				{"test-cgroup-missing-additional", "io.pressure", "full", "avg300", 0.03},
				{"test-cgroup-missing-additional", "io.pressure", "full", "total", 1000.00},
				{"test-cgroup-missing-additional", "memory.pressure", "full", "avg10", 0.01},
				{"test-cgroup-missing-additional", "memory.pressure", "full", "avg60", 0.02},
				{"test-cgroup-missing-additional", "memory.pressure", "full", "avg300", 0.03},
				{"test-cgroup-missing-additional", "memory.pressure", "full", "total", 1000.00},
			},
			expectError: false, // 附加资源缺失应该只记录错误，不中断流程
		},
		{
			name: "invalid_content_in_common_resource",
			setupFunc: func(t *testing.T) (string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-invalid"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建PSI资源文件，其中一个包含无效内容
				resources := []string{"cpu.pressure", "io.pressure", "memory.pressure"}
				for i, resource := range resources {
					psiFile := filepath.Join(cgroupPath, resource)
					var content string
					if i == 1 { // io.pressure包含无效内容
						content = "full avg10=abc avg60=0.02 avg300=0.03 total=1000"
					} else {
						content = "full avg10=0.01 avg60=0.02 avg300=0.03 total=1000"
					}
					require.NoError(t, os.WriteFile(psiFile, []byte(content), 0644))
				}

				return testDir, cgroupRelPath
			},
			expectError:   true,
			errorContains: "failed to update psi metric from io.pressure",
		},
		{
			name: "empty_common_resource_files",
			setupFunc: func(t *testing.T) (string, string) {
				testDir := t.TempDir()
				// 创建cgroup目录结构
				cgroupRelPath := "test-cgroup-empty"
				cgroupPath := filepath.Join(testDir, cgroupRelPath)
				require.NoError(t, os.MkdirAll(cgroupPath, 0755))

				// 创建空的PSI资源文件
				resources := []string{"cpu.pressure", "io.pressure", "memory.pressure"}
				for _, resource := range resources {
					psiFile := filepath.Join(cgroupPath, resource)
					require.NoError(t, os.WriteFile(psiFile, []byte(""), 0644))
				}

				return testDir, cgroupRelPath
			},
			expectedCalls: []struct {
				psiCgroupName string
				resource      string
				item          string
				key           string
				value         float64
			}{}, // 空文件应该不产生任何指标
			expectError:   true,
			errorContains: "PSI cgroup resource data is empty for file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行setup函数获取测试参数
			cgroupSearchPath, psiCgroupRelPath := tt.setupFunc(t)
			psiCgroupName := filepath.Base(psiCgroupRelPath)

			// 创建psiCollector实例
			c := &psiCollector{
				logger:           logger,
				cgroupSearchPath: cgroupSearchPath,
				pressureMetric: prometheus.NewDesc(
					"psi_pressure",
					"Pressure Stall Information metric",
					[]string{"cgroup", "resource", "item", "key"},
					nil,
				),
			}

			// 创建通道来收集发送的指标
			ch := make(chan prometheus.Metric, 100)
			defer close(ch)

			// 执行测试
			err := c.updatePSIMetrics(ch, psiCgroupName, psiCgroupRelPath)

			// 验证结果
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)

				// 验证发送的指标数量
				assert.Equal(t, len(tt.expectedCalls), len(ch))

				// 验证每个预期的指标调用
				for _, expectedCall := range tt.expectedCalls {
					select {
					case metric := <-ch:
						// 验证指标描述符
						desc := metric.Desc()
						assert.Equal(t, c.pressureMetric, desc)
					default:
						t.Errorf("Expected metric not found: %+v", expectedCall)
					}
				}
			}
		})
	}
}
