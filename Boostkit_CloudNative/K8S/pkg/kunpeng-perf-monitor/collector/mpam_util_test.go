package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAllTargetDirs(t *testing.T) {
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

		result, err := getAllTargetDirs(tmpRoot, "l3_cache")
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Contains(t, result, "group1")
		assert.Contains(t, result, "group2")
	})

	t.Run("no_target_subdir", func(t *testing.T) {
		tmpRoot := t.TempDir()
		os.MkdirAll(filepath.Join(tmpRoot, "group1"), 0755)

		result, err := getAllTargetDirs(tmpRoot, "l3_cache")
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("invalid_root_path", func(t *testing.T) {
		_, err := getAllTargetDirs("/non/existent/path", "l3_cache")
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

		result, err := getAllTargetDirs(tmpRoot, "l3_cache")
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
		assert.ErrorContains(t, err, "permission denied")
	})
}
