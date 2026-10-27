package publisher

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	upstreamstorage "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
	upstreamheader "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

// Closure 是恢复一个 final build 所需的完整 E2B-native 对象集合。
type Closure struct {
	Objects          []ObjectSource
	AncestorBuildIDs []string
}

// ResolveAll 枚举 Builder staging 中实际存在的全部对象，保留原始 E2B object key。
// 该模式用于复刻 infra local storage 或保留完整构建历史，不会伪造缺失文件。
func ResolveAll(stagingRoot, finalBuildID string) (Closure, error) {
	finalUUID, err := uuid.Parse(finalBuildID)
	if err != nil || finalUUID.String() != finalBuildID {
		return Closure{}, fmt.Errorf("invalid final BuildID %q", finalBuildID)
	}
	if !filepath.IsAbs(stagingRoot) {
		return Closure{}, fmt.Errorf("staging root must be absolute")
	}

	objects := make([]ObjectSource, 0)
	ancestors := make(map[string]struct{})
	err = filepath.WalkDir(stagingRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("staging object %q is not a regular file", path)
		}
		relative, err := filepath.Rel(stagingRoot, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relative)
		if err := validateObjectKey(key); err != nil {
			return err
		}
		parts := strings.Split(key, "/")
		if len(parts) > 1 && parts[0] != finalBuildID {
			if _, err := uuid.Parse(parts[0]); err == nil {
				ancestors[parts[0]] = struct{}{}
			}
		}
		objects = append(objects, ObjectSource{Key: key, Path: path})
		return nil
	})
	if err != nil {
		return Closure{}, fmt.Errorf("enumerate staging objects: %w", err)
	}
	if len(objects) == 0 {
		return Closure{}, fmt.Errorf("staging root contains no objects")
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
	ancestorIDs := make([]string, 0, len(ancestors))
	for ancestor := range ancestors {
		ancestorIDs = append(ancestorIDs, ancestor)
	}
	sort.Strings(ancestorIDs)
	return Closure{Objects: objects, AncestorBuildIDs: ancestorIDs}, nil
}

// ResolveClosure 读取 infra 2026.09 V3 header，解析全部引用 layer，且不改写原生 object key。
func ResolveClosure(stagingRoot, finalBuildID string) (Closure, error) {
	finalUUID, err := uuid.Parse(finalBuildID)
	if err != nil || finalUUID.String() != finalBuildID {
		return Closure{}, fmt.Errorf("invalid final BuildID %q", finalBuildID)
	}
	if !filepath.IsAbs(stagingRoot) {
		return Closure{}, fmt.Errorf("staging root must be absolute")
	}

	objects := make(map[string]ObjectSource)
	ancestors := make(map[string]struct{})
	addObject := func(key string) error {
		if err := validateObjectKey(key); err != nil {
			return err
		}
		objectPath := filepath.Join(stagingRoot, filepath.FromSlash(key))
		info, statErr := os.Stat(objectPath)
		if os.IsNotExist(statErr) {
			return fmt.Errorf("missing referenced object %q", key)
		}
		if statErr != nil {
			return fmt.Errorf("inspect referenced object %q: %w", key, statErr)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("referenced object %q is not a regular file", key)
		}
		objects[key] = ObjectSource{Key: key, Path: objectPath}
		return nil
	}

	finalFiles := upstreamstorage.TemplateFiles{BuildID: finalBuildID}
	artifacts := []struct {
		name      string
		headerKey string
		dataKey   func(upstreamstorage.TemplateFiles) string
	}{
		{name: upstreamstorage.MemfileName, headerKey: finalFiles.StorageMemfileHeaderPath(), dataKey: func(files upstreamstorage.TemplateFiles) string { return files.StorageMemfilePath() }},
		{name: upstreamstorage.RootfsName, headerKey: finalFiles.StorageRootfsHeaderPath(), dataKey: func(files upstreamstorage.TemplateFiles) string { return files.StorageRootfsPath() }},
	}
	for _, artifact := range artifacts {
		if err := addObject(artifact.headerKey); err != nil {
			return Closure{}, err
		}
		headerBytes, err := os.ReadFile(filepath.Join(stagingRoot, filepath.FromSlash(artifact.headerKey)))
		if err != nil {
			return Closure{}, fmt.Errorf("read header %q: %w", artifact.headerKey, err)
		}
		header, err := upstreamheader.DeserializeBytes(headerBytes)
		if err != nil {
			return Closure{}, fmt.Errorf("deserialize header %q: %w", artifact.headerKey, err)
		}
		if header.Metadata.BuildId != finalUUID {
			return Closure{}, fmt.Errorf("header %q build ID %s does not match final BuildID %s", artifact.headerKey, header.Metadata.BuildId, finalBuildID)
		}
		if err := upstreamheader.ValidateMappings(header.Mapping, header.Metadata.Size, header.Metadata.BlockSize); err != nil {
			return Closure{}, fmt.Errorf("validate header %q: %w", artifact.headerKey, err)
		}

		seenBuilds := make(map[uuid.UUID]struct{})
		for _, mapping := range header.Mapping {
			buildID := mapping.BuildId
			// uuid.Nil 表示空块，不对应 storage 中的物理 data object。
			if buildID == uuid.Nil {
				continue
			}
			if _, exists := seenBuilds[buildID]; exists {
				continue
			}
			seenBuilds[buildID] = struct{}{}
			if buildID != finalUUID {
				ancestors[buildID.String()] = struct{}{}
			}
			if err := addObject(artifact.dataKey(upstreamstorage.TemplateFiles{BuildID: buildID.String()})); err != nil {
				return Closure{}, err
			}
		}
	}
	for _, key := range []string{finalFiles.StorageSnapfilePath(), finalFiles.StorageMetadataPath()} {
		if err := addObject(key); err != nil {
			return Closure{}, err
		}
	}

	closure := Closure{Objects: make([]ObjectSource, 0, len(objects)), AncestorBuildIDs: make([]string, 0, len(ancestors))}
	for _, object := range objects {
		closure.Objects = append(closure.Objects, object)
	}
	for ancestor := range ancestors {
		closure.AncestorBuildIDs = append(closure.AncestorBuildIDs, ancestor)
	}
	sort.Slice(closure.Objects, func(i, j int) bool { return closure.Objects[i].Key < closure.Objects[j].Key })
	sort.Strings(closure.AncestorBuildIDs)

	return closure, nil
}
