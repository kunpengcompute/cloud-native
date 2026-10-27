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

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package dockerhub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/consts"
)

type GCPRemoteRepository struct {
	repositoryURL string
	registry      *artifactregistry.Client
	authToken     authn.Authenticator
}

var gcpAuthConfig = authn.Basic{
	Username: "_json_key_base64",
	Password: consts.GoogleServiceAccountSecret,
}

func NewGCPRemoteRepository(ctx context.Context, repositoryURL string) (*GCPRemoteRepository, error) {
	registry, err := artifactregistry.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating artifact registry client: %w", err)
	}

	authToken, err := getAuthToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting auth token: %w", err)
	}

	return &GCPRemoteRepository{repositoryURL: repositoryURL, registry: registry, authToken: authToken}, nil
}

func (g *GCPRemoteRepository) GetImage(_ context.Context, tag string, platform containerregistry.Platform) (containerregistry.Image, error) {
	tagWithoutRegistry, err := removeRegistryFromTag(tag)
	if err != nil {
		return nil, fmt.Errorf("error removing registry from tag: %w", err)
	}

	ref, err := name.ParseReference(g.repositoryURL + "/" + tagWithoutRegistry)
	if err != nil {
		return nil, fmt.Errorf("invalid image reference: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuth(g.authToken), remote.WithPlatform(platform))
	if err != nil {
		return nil, fmt.Errorf("error pulling image: %w", err)
	}

	return img, nil
}

func getAuthToken(_ context.Context) (authn.Authenticator, error) {
	authCfg := consts.DockerAuthConfig
	if authCfg == "" {
		return &gcpAuthConfig, nil
	}

	decoded, err := base64.URLEncoding.DecodeString(authCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode auth config: %w", err)
	}

	var cfg struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.Unmarshal(decoded, &cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON auth config: %w", err)
	}

	return &authn.Basic{
		Username: cfg.Username,
		Password: cfg.Password,
	}, nil
}

func (g *GCPRemoteRepository) Close() error {
	return g.registry.Close()
}
