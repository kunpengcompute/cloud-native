package collector

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
			name:           "cgroup2 without newline",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("cgroup2"),
			mockError:      nil,
			expectedResult: cgroupV2,
			expectError:    false,
		},
		{
			name:           "tmpfs with newline",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("tmpfs\n"),
			mockError:      nil,
			expectedResult: cgroupV1,
			expectError:    false,
		},
		{
			name:           "tmpfs without newline",
			cgroupPath:     "/sys/fs/cgroup",
			mockOutput:     []byte("tmpfs"),
			mockError:      nil,
			expectedResult: cgroupV1,
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

// TestGetPSICgroupsEdgeCases 测试getPSICgroups函数的边界情况
func TestGetPSICgroupsEdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Run("symlink_handling", func(t *testing.T) {
		testDir := t.TempDir()

		// 创建真实目录
		realDir := filepath.Join(testDir, "realGroup")
		require.NoError(t, os.MkdirAll(realDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(realDir, psiCgroupFeatureFile),
			[]byte("some avg10=0.00 avg60=0.00 avg300=0.00 total=0"), 0644))

		// 创建符号链接
		symlinkDir := filepath.Join(testDir, "symlinkGroup")
		require.NoError(t, os.Symlink(realDir, symlinkDir))

		c := &psiCollector{
			cgroupSearchPath: testDir,
			logger:           logger,
		}

		result, err := c.getPSICgroups()
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// 符号链接应该被正确处理，只返回真实目录
		assert.Equal(t, 1, len(result))
		assert.Contains(t, result, "realGroup")
	})

	t.Run("empty_cpu_pressure_file", func(t *testing.T) {
		testDir := t.TempDir()
		path := filepath.Join(testDir, "emptyFileGroup")
		require.NoError(t, os.MkdirAll(path, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(path, psiCgroupFeatureFile),
			[]byte(""), 0644)) // 空文件

		c := &psiCollector{
			cgroupSearchPath: testDir,
			logger:           logger,
		}

		result, err := c.getPSICgroups()
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// 空文件应该不影响目录识别
		assert.Equal(t, 1, len(result))
		assert.Contains(t, result, "emptyFileGroup")
	})

	t.Run("invalid_cpu_pressure_content", func(t *testing.T) {
		testDir := t.TempDir()
		path := filepath.Join(testDir, "invalidContentGroup")
		require.NoError(t, os.MkdirAll(path, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(path, psiCgroupFeatureFile),
			[]byte("invalid content"), 0644)) // 无效内容

		c := &psiCollector{
			cgroupSearchPath: testDir,
			logger:           logger,
		}

		result, err := c.getPSICgroups()
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// 文件内容不影响目录识别
		assert.Equal(t, 1, len(result))
		assert.Contains(t, result, "invalidContentGroup")
	})
}
