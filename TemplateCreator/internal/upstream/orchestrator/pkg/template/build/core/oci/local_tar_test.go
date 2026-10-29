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
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	containerregistry "github.com/google/go-containerregistry/pkg/v1"
)

func TestGetLocalImageSelectsRequestedArchitecture(t *testing.T) {
	path := writeDockerSaveTar(t, []dockerSaveImage{
		{Tag: "example/image:amd64", Architecture: "amd64"},
		{Tag: "example/image:arm64", Architecture: "arm64"},
	})

	img, err := GetLocalImage(path, containerregistry.Platform{OS: "linux", Architecture: "arm64"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if config.Architecture != "arm64" {
		t.Fatalf("selected architecture = %q, want arm64", config.Architecture)
	}
}

func TestGetLocalImageRejectsUnsupportedArchitecture(t *testing.T) {
	path := writeDockerSaveTar(t, []dockerSaveImage{{Tag: "example/image:amd64", Architecture: "amd64"}})

	_, err := GetLocalImage(path, containerregistry.Platform{OS: "linux", Architecture: "s390x"})
	if err == nil || !strings.Contains(err.Error(), "no image for platform linux/s390x") {
		t.Fatalf("GetLocalImage() error = %v, want platform error", err)
	}
}

type dockerSaveImage struct {
	Tag          string
	Architecture string
}

func writeDockerSaveTar(t *testing.T, images []dockerSaveImage) string {
	t.Helper()
	type descriptor struct {
		Config   string   `json:"Config"`
		RepoTags []string `json:"RepoTags"`
		Layers   []string `json:"Layers"`
	}
	manifest := make([]descriptor, 0, len(images))
	files := map[string][]byte{}
	for index, image := range images {
		layer := []byte(fmt.Sprintf("layer-%d", index))
		layerHash := fmt.Sprintf("%x", sha256.Sum256(layer))
		configName := fmt.Sprintf("config-%d.json", index)
		layerName := fmt.Sprintf("layer-%d.tar", index)
		config, err := json.Marshal(map[string]any{
			"architecture": image.Architecture,
			"os":           "linux",
			"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{"sha256:" + layerHash}},
		})
		if err != nil {
			t.Fatal(err)
		}
		manifest = append(manifest, descriptor{Config: configName, RepoTags: []string{image.Tag}, Layers: []string{layerName}})
		files[configName] = config
		files[layerName] = layer
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "image.tar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(file)
	write := func(name string, data []byte) {
		t.Helper()
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(tw, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", manifestBytes)
	for name, data := range files {
		write(name, data)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
