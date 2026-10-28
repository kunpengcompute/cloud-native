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

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package artifacts_registry

import (
	"context"
	"errors"
	"fmt"
	"time"

	containerregistry "github.com/google/go-containerregistry/pkg/v1"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/env"
)

type RegistryProvider string

const (
	GCPStorageProvider   RegistryProvider = "GCP_ARTIFACTS"
	AWSStorageProvider   RegistryProvider = "AWS_ECR"
	LocalStorageProvider RegistryProvider = "Local"

	DefaultRegistryProvider RegistryProvider = GCPStorageProvider

	storageProviderEnv = "ARTIFACTS_REGISTRY_PROVIDER"
)

var ErrImageNotExists = errors.New("image does not exist")

type ArtifactsRegistry interface {
	GetTag(ctx context.Context, templateId string, buildId string) (string, error)
	GetImage(ctx context.Context, templateId string, buildId string, platform containerregistry.Platform) (containerregistry.Image, error)
	Delete(ctx context.Context, templateId string, buildId string) error
}

func GetArtifactsRegistryProvider(ctx context.Context) (ArtifactsRegistry, error) {
	provider := RegistryProvider(env.GetEnv(storageProviderEnv, string(DefaultRegistryProvider)))

	setupCtx, setupCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	defer setupCtxCancel()

	switch provider {
	case AWSStorageProvider:
		return NewAWSArtifactsRegistry(setupCtx)
	case GCPStorageProvider:
		return NewGCPArtifactsRegistry(setupCtx)
	case LocalStorageProvider:
		return NewLocalArtifactsRegistry()
	}

	return nil, fmt.Errorf("unknown artifacts registry provider: %s", provider)
}
