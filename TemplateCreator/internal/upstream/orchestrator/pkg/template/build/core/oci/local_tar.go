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

package oci

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// GetLocalImage 从 docker save 归档中按目标平台选择一个镜像。
// 归档内可能包含多个架构，不能直接使用 tarball.Image(nil) 让库按位置猜测。
func GetLocalImage(imageTarPath string, platform containerregistry.Platform) (containerregistry.Image, error) {
	if imageTarPath == "" {
		return nil, fmt.Errorf("image tar path is required")
	}
	info, err := os.Stat(imageTarPath)
	if err != nil {
		return nil, fmt.Errorf("stat image tar %q: %w", imageTarPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("image tar %q must be a regular file", imageTarPath)
	}
	if platform.OS == "" {
		platform.OS = "linux"
	}
	if platform.Architecture == "" {
		platform.Architecture = DefaultPlatform.Architecture
	}

	manifest, err := tarball.LoadManifest(func() (io.ReadCloser, error) { return os.Open(imageTarPath) })
	if err != nil {
		return nil, fmt.Errorf("load docker save manifest: %w", err)
	}
	if len(manifest) == 0 {
		return nil, fmt.Errorf("docker save archive contains no images")
	}

	selected := -1
	for index, descriptor := range manifest {
		configBytes, readErr := readTarEntry(imageTarPath, descriptor.Config)
		if readErr != nil {
			return nil, fmt.Errorf("read image config %q: %w", descriptor.Config, readErr)
		}
		var config containerregistry.ConfigFile
		if unmarshalErr := json.Unmarshal(configBytes, &config); unmarshalErr != nil {
			return nil, fmt.Errorf("decode image config %q: %w", descriptor.Config, unmarshalErr)
		}
		if config.OS == platform.OS && config.Architecture == platform.Architecture {
			if len(descriptor.Layers) == 0 {
				return nil, fmt.Errorf("image for platform %s/%s contains no layers", platform.OS, platform.Architecture)
			}
			selected = index
			break
		}
	}
	if selected < 0 {
		return nil, fmt.Errorf("no image for platform %s/%s", platform.OS, platform.Architecture)
	}

	if len(manifest) == 1 {
		img, imageErr := tarball.ImageFromPath(imageTarPath, nil)
		if imageErr != nil {
			return nil, fmt.Errorf("load local image: %w", imageErr)
		}
		return img, nil
	}
	descriptor := manifest[selected]
	if len(descriptor.RepoTags) == 0 {
		return nil, fmt.Errorf("image for platform %s/%s has no repository tag in docker save archive", platform.OS, platform.Architecture)
	}
	tag, err := name.NewTag(descriptor.RepoTags[0])
	if err != nil {
		return nil, fmt.Errorf("parse image tag %q: %w", descriptor.RepoTags[0], err)
	}
	img, err := tarball.ImageFromPath(imageTarPath, &tag)
	if err != nil {
		return nil, fmt.Errorf("load local image %q: %w", imageTarPath, err)
	}
	return img, nil
}

func readTarEntry(tarPath, wanted string) ([]byte, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, nextErr
		}
		if path.Clean(header.Name) != path.Clean(wanted) {
			continue
		}
		if !header.FileInfo().Mode().IsRegular() {
			return nil, fmt.Errorf("entry is not a regular file")
		}
		return io.ReadAll(reader)
	}
	return nil, fmt.Errorf("entry %q not found", wanted)
}
