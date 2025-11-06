package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrToFloat64ForIntStr(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    float64
		expectError bool
	}{
		{"decimal_normal", "1234", 1234, false},
		{"hex_no_prefix", "ffff", 65535, false},
		{"invalid_char", "12g4", 0, true},
		{"empty_string", "", 0, true},
		{"overflow", "9999999999999999999", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := strToFloat64(tc.input, true)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to parse int")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestStrToFloat64ForFloatStr(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    float64
		expectError bool
	}{
		{"float_normal", "1234", 1234, false},
		{"float_invalid_char", "12g4", 0, true},
		{"empty_string", "", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := strToFloat64(tc.input, false)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to parse")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestAddToConfigData(t *testing.T) {
	t.Run("nil_config_data", func(t *testing.T) {
		err := addToConfigData(nil, "cpu", "cores", 8)
		assert.ErrorContains(t, err, "configData is nil")
	})

	t.Run("empty_map_initialization", func(t *testing.T) {
		// 创建非nil空map
		data := new(ConfigData) // data is not nil, but *data is nil
		err := addToConfigData(data, "network", "throughput", 1000)

		// 验证无错误且值正确
		assert.NoError(t, err)
		assert.Equal(t, float64(1000), (*data)["network"]["throughput"])

		// 验证map长度
		assert.Len(t, *data, 1)
		assert.Len(t, (*data)["network"], 1)
	})

	t.Run("valid_addition", func(t *testing.T) {
		data := &ConfigData{}
		err := addToConfigData(data, "memory", "bandwidth", 256)

		assert.NoError(t, err)
		assert.Equal(t, float64(256), (*data)["memory"]["bandwidth"])
	})

	t.Run("empty_item_name", func(t *testing.T) {
		data := &ConfigData{}
		err := addToConfigData(data, "", "invalid", 0)

		assert.NoError(t, err)
		assert.Contains(t, *data, "")
	})

	t.Run("update_existing", func(t *testing.T) {
		data := &ConfigData{"cache": {"L3": 2048}}
		err := addToConfigData(data, "cache", "L3", 4096)

		assert.NoError(t, err)
		assert.Equal(t, float64(4096), (*data)["cache"]["L3"])
	})
}

func TestParseLine(t *testing.T) {
	config1 := &ConfigData{}
	config2 := &ConfigData{}

	tests := []struct {
		name    string
		line    string
		wantErr bool
	}{
		{
			name:    "empty_line",
			line:    "   ",
			wantErr: false,
		},
		{
			name:    "invalid_format",
			line:    "L3",
			wantErr: true,
		},
		{
			name:    "valid_config_base10",
			line:    "L3cache: 1=100;2=200",
			wantErr: false,
		},
		{
			name:    "valid_config_base16",
			line:    "L3cache: 3=ffffff;4=ffff",
			wantErr: false,
		},
		{
			name:    "invalid_key_value",
			line:    "L3cache: 1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseLine(tt.line, config1, "cache", config2, "mem")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// 验证有效配置的解析结果
	assert.Equal(t, float64(100), (*config1)["L3cache"]["1"]) // "100"转换为十进制float64(100)
	assert.Equal(t, float64(200), (*config1)["L3cache"]["2"])
	assert.Equal(t, float64(16777215), (*config1)["L3cache"]["3"])
	assert.Equal(t, float64(65535), (*config1)["L3cache"]["4"])
}

// 辅助函数创建临时测试文件
func createTestFile(t *testing.T, content string) string {
	tmpfile, err := os.CreateTemp("", "testconfig")
	assert.NoError(t, err)
	_, err = tmpfile.WriteString(content)
	assert.NoError(t, err)
	tmpfile.Close()
	return tmpfile.Name()
}

func TestGetMPAMConfigData(t *testing.T) {
	// 创建临时测试文件
	t.Run("valid_file", func(t *testing.T) {
		content := "L3cache: 1=100\nmem: 2=200"
		tmpfile := createTestFile(t, content)
		defer os.Remove(tmpfile)

		config1, config2, err := getMPAMConfigData(tmpfile, "cache", "mem")
		assert.NoError(t, err)
		assert.Equal(t, float64(100), (*config1)["L3cache"]["1"])
		assert.Equal(t, float64(200), (*config2)["mem"]["2"])
	})

	t.Run("file_not_exist", func(t *testing.T) {
		_, _, err := getMPAMConfigData("nonexistent.txt", "", "")
		assert.ErrorContains(t, err, "no such file")
	})

	t.Run("invalid_line_format", func(t *testing.T) {
		content := `invalid_line`
		tmpfile := createTestFile(t, content)
		defer os.Remove(tmpfile)

		_, _, err := getMPAMConfigData(tmpfile, "", "")
		assert.ErrorContains(t, err, "error parsing line 1")
	})

	t.Run("empty_file", func(t *testing.T) {
		tmpfile := createTestFile(t, "")
		defer os.Remove(tmpfile)

		config1, config2, err := getMPAMConfigData(tmpfile, "", "")
		assert.NoError(t, err)
		assert.Empty(t, config1)
		assert.Empty(t, config2)
	})
}

func TestGetAllTargetDirWhenIsDirIsTrue(t *testing.T) {
	t.Run("normal_case", func(t *testing.T) {
		tmpRoot := t.TempDir()

		// 创建测试目录结构
		// tmpRoot/
		// ├── group1/
		// │   └── l3_cache/
		// └── group2/
		//     └── l3_cache/
		os.MkdirAll(filepath.Join(tmpRoot, "group1", "l3_cache"), 0755)
		os.MkdirAll(filepath.Join(tmpRoot, "group2", "l3_cache"), 0755)

		result, err := getAllTargetDirs(tmpRoot, "l3_cache", true)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Contains(t, result, "group1")
		assert.Contains(t, result, "group2")
	})

	t.Run("no_target_subdir", func(t *testing.T) {
		tmpRoot := t.TempDir()
		os.MkdirAll(filepath.Join(tmpRoot, "group1"), 0755)

		result, err := getAllTargetDirs(tmpRoot, "l3_cache", true)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("invalid_root_path", func(t *testing.T) {
		_, err := getAllTargetDirs("/non/existent/path", "l3_cache", true)
		assert.ErrorContains(t, err, "no such file")
	})

	t.Run("nested_structure", func(t *testing.T) {
		tmpRoot := t.TempDir()
		// 创建嵌套目录结构
		// tmpRoot/
		// └── parent/
		//     ├── child1/
		//     │   └── l3_cache/
		//     └── child2/
		//         └── l3_cache/
		os.MkdirAll(filepath.Join(tmpRoot, "parent", "child1", "l3_cache"), 0755)
		os.MkdirAll(filepath.Join(tmpRoot, "parent", "child2", "l3_cache"), 0755)

		result, err := getAllTargetDirs(tmpRoot, "l3_cache", true)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Contains(t, result, "child1")
		assert.Contains(t, result, "child2")
	})
}

func TestGetAllTargetDirWhenIsDirIsFalse(t *testing.T) {
	t.Run("normal_case", func(t *testing.T) {
		tmpRoot := t.TempDir()

		// 创建测试目录结构
		// tmpRoot/
		// ├── group1/
		// │   └── l3_cache/
		// └── group2/
		//     └── l3_cache/
		os.MkdirAll(filepath.Join(tmpRoot, "group1"), 0755)
		os.MkdirAll(filepath.Join(tmpRoot, "group2"), 0755)
		os.Create(filepath.Join(tmpRoot, "group1", "l3_cache"))
		os.Create(filepath.Join(tmpRoot, "group2", "l3_cache"))

		result, err := getAllTargetDirs(tmpRoot, "l3_cache", false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Contains(t, result, "group1")
		assert.Contains(t, result, "group2")
	})

	t.Run("no_target_subfile", func(t *testing.T) {
		tmpRoot := t.TempDir()
		os.MkdirAll(filepath.Join(tmpRoot, "group1"), 0755)

		result, err := getAllTargetDirs(tmpRoot, "l3_cache", false)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("invalid_root_path", func(t *testing.T) {
		_, err := getAllTargetDirs("/non/existent/path", "l3_cache", false)
		assert.ErrorContains(t, err, "no such file")
	})

	t.Run("nested_structure", func(t *testing.T) {
		tmpRoot := t.TempDir()
		// 创建嵌套目录结构
		// tmpRoot/
		// └── parent/
		//     ├── child1/
		//     │   └── l3_cache/
		//     └── child2/
		//         └── l3_cache/
		os.MkdirAll(filepath.Join(tmpRoot, "parent", "child1"), 0755)
		os.MkdirAll(filepath.Join(tmpRoot, "parent", "child2"), 0755)
		os.Create(filepath.Join(tmpRoot, "parent", "child1", "l3_cache"))
		os.Create(filepath.Join(tmpRoot, "parent", "child2", "l3_cache"))

		result, err := getAllTargetDirs(tmpRoot, "l3_cache", false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Contains(t, result, "child1")
		assert.Contains(t, result, "child2")
	})
}

func TestListResInfoSubDirs(t *testing.T) {
	t.Run("normal_case", func(t *testing.T) {
		tmpDir := t.TempDir()

		// 创建测试目录结构
		os.MkdirAll(filepath.Join(tmpDir, "l3cache_group1"), 0755)
		os.MkdirAll(filepath.Join(tmpDir, "mem_group1"), 0755)
		os.MkdirAll(filepath.Join(tmpDir, "l3cache_group2"), 0755)

		l3Dirs, memDirs, err := listResInfoSubDirs(tmpDir, "l3cache", "mem")
		assert.NoError(t, err)
		assert.Equal(t, 2, len(l3Dirs))
		assert.Equal(t, 1, len(memDirs))
		assert.Contains(t, l3Dirs, "l3cache_group1")
		assert.Contains(t, l3Dirs, "l3cache_group2")
		assert.Contains(t, memDirs, "mem_group1")
	})

	t.Run("empty_directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		l3Dirs, memDirs, err := listResInfoSubDirs(tmpDir, "l3cache", "mem")
		assert.NoError(t, err)
		assert.Empty(t, l3Dirs)
		assert.Empty(t, memDirs)
	})

	t.Run("invalid_path", func(t *testing.T) {
		_, _, err := listResInfoSubDirs("/non/existent/path", "l3cache", "mem")
		assert.ErrorContains(t, err, "no such file")
	})

	t.Run("mixed_files_and_dirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		// 创建文件和目录混合场景
		os.WriteFile(filepath.Join(tmpDir, "l3cache_file"), []byte("data"), 0644)
		os.Mkdir(filepath.Join(tmpDir, "mem_group1"), 0755)

		l3Dirs, memDirs, err := listResInfoSubDirs(tmpDir, "l3cache", "mem")
		assert.NoError(t, err)
		assert.Empty(t, l3Dirs)
		assert.Equal(t, 1, len(memDirs))
		assert.Contains(t, memDirs, "mem_group1")
	})

	t.Run("insufficient_permission", func(t *testing.T) {
		tmpDir := t.TempDir()

		// 创建一个子目录并设置执行权限
		restrictedDir := filepath.Join(tmpDir, "restricted")
		err := os.MkdirAll(restrictedDir, 0644) // 0644: rw-r--r-- (没有执行权限)
		assert.NoError(t, err)

		// 然后修改权限为0300（有写和执行权限，无读权限，必定返回错误）
		err = os.Chmod(restrictedDir, 0300)
		assert.NoError(t, err)

		// 在测试结束后恢复权限，确保清理能正常进行
		defer os.Chmod(restrictedDir, 0755)

		// 测试：尝试读取没有执行权限的目录
		l3Dirs, memDirs, err := listResInfoSubDirs(restrictedDir, "l3cache", "mem")
		if err == nil {
			t.Skip("A permission error is expected but got nil. May be you are running the test as root.")
			return
		}

		// 验证：应该返回权限错误
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
		assert.Nil(t, l3Dirs)
		assert.Nil(t, memDirs)
	})

	t.Run("partial_permission_issue", func(t *testing.T) {
		tmpDir := t.TempDir()

		// 创建多个目录，其中一个没有执行权限
		normalDir := filepath.Join(tmpDir, "normal")
		restrictedDir := filepath.Join(tmpDir, "restricted")

		// 正常目录
		os.MkdirAll(normalDir, 0755)
		os.MkdirAll(filepath.Join(normalDir, "l3cache_normal"), 0755)

		// 受限目录
		os.MkdirAll(restrictedDir, 0644)
		os.MkdirAll(filepath.Join(restrictedDir, "l3cache_restricted"), 0755)

		// 然后修改权限为0300（有写和执行权限，无读权限，必定返回错误）
		err := os.Chmod(restrictedDir, 0300)
		assert.NoError(t, err)

		// 在测试结束后恢复权限，确保清理能正常进行
		defer os.Chmod(restrictedDir, 0755)

		// 测试正常目录应该能正常工作
		l3Dirs, _, err := listResInfoSubDirs(normalDir, "l3cache", "mem")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(l3Dirs))
		assert.Contains(t, l3Dirs, "l3cache_normal")

		// 测试受限目录应该失败
		_, _, err = listResInfoSubDirs(restrictedDir, "l3cache", "mem")
		if err == nil {
			t.Skip("A permission error is expected but got nil. May be you are running the test as root.")
			return
		}
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})
}

func TestGetFileContent(t *testing.T) {
	t.Run("normal_file", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "test")
		os.WriteFile(tmpFile, []byte("  test content\n"), 0644)

		content, err := getFileContent(tmpFile)
		assert.NoError(t, err)
		assert.Equal(t, "test content", content)
	})

	t.Run("file_not_exist", func(t *testing.T) {
		_, err := getFileContent("/non/existent/path")
		assert.ErrorContains(t, err, "no such file")
	})

	t.Run("permission_denied", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "restricted.txt")
		os.WriteFile(tmpFile, []byte("data"), 0200) // 只写权限

		_, err := getFileContent(tmpFile)
		if err == nil {
			t.Skip("A permission error is expected but got nil. May be you are running the test as root.")
			return
		}
		assert.ErrorContains(t, err, "permission denied")
	})
}
