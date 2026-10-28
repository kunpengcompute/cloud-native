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

package dockerhub

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/env"
)

type RemoteRepositoryProvider string

const (
	GCPStorageProvider   RemoteRepositoryProvider = "GCP_REMOTE_REPOSITORY"
	AWSStorageProvider   RemoteRepositoryProvider = "AWS_ECR"
	LocalStorageProvider RemoteRepositoryProvider = "Local"

	DefaultRegistryProvider RemoteRepositoryProvider = GCPStorageProvider

	storageProviderEnv         = "DOCKERHUB_REMOTE_REPOSITORY_PROVIDER"
	storageRemoteRepositoryURL = "DOCKERHUB_REMOTE_REPOSITORY_URL"

	setupTimeout = 10 * time.Second
)

type RemoteRepository interface {
	GetImage(ctx context.Context, tag string, platform containerregistry.Platform) (containerregistry.Image, error)
	Close() error
}

func GetRemoteRepository(ctx context.Context) (RemoteRepository, error) {
	provider := RemoteRepositoryProvider(env.GetEnv(storageProviderEnv, string(DefaultRegistryProvider)))

	dockerRemoteRepositoryURL := env.GetEnv(storageRemoteRepositoryURL, "")
	if dockerRemoteRepositoryURL == "" {
		return NewNoopRemoteRepository(), nil
	}

	setupCtx, setupCtxCancel := context.WithTimeout(ctx, setupTimeout)
	defer setupCtxCancel()

	switch provider {
	case AWSStorageProvider:
		return NewAWSRemoteRepository(setupCtx, dockerRemoteRepositoryURL)
	case GCPStorageProvider:
		return NewGCPRemoteRepository(setupCtx, dockerRemoteRepositoryURL)
	case LocalStorageProvider:
		return NewNoopRemoteRepository(), nil
	}

	return nil, fmt.Errorf("unknown dockerhub remote repository provider: %s", provider)
}

func removeRegistryFromTag(tag string) (string, error) {
	ref, err := name.ParseReference(tag)
	if err != nil {
		return "", fmt.Errorf("invalid image reference: %w", err)
	}

	registry := ref.Context().RegistryStr()
	withoutRegistry := strings.TrimPrefix(ref.Name(), registry)
	withoutRegistry = strings.TrimPrefix(withoutRegistry, "/")

	return withoutRegistry, nil
}
