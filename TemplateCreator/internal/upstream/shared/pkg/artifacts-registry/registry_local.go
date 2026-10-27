// Copyright (c) 2023-2026 e2b Devs Inc.
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

package artifacts_registry

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
)

type LocalArtifactsRegistry struct{}

func NewLocalArtifactsRegistry() (*LocalArtifactsRegistry, error) {
	return &LocalArtifactsRegistry{}, nil
}

func (g *LocalArtifactsRegistry) Delete(context.Context, string, string) error {
	// for now, just assume local image can be deleted manually
	return nil
}

func (g *LocalArtifactsRegistry) GetTag(_ context.Context, templateId string, buildId string) (string, error) {
	return fmt.Sprintf("%s:%s", templateId, buildId), nil
}

func (g *LocalArtifactsRegistry) GetImage(ctx context.Context, templateId string, buildId string, _ containerregistry.Platform) (containerregistry.Image, error) {
	imageUrl, err := g.GetTag(ctx, templateId, buildId)
	if err != nil {
		return nil, fmt.Errorf("failed to get image URL: %w", err)
	}

	ref, err := name.ParseReference(imageUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid image reference: %w", err)
	}

	img, err := daemon.Image(ref, daemon.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get image from local registry: %w", err)
	}

	return img, nil
}
