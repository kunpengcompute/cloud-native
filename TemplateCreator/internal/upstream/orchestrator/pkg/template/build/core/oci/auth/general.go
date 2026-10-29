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

package auth

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	templatemanager "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
)

// GeneralAuthProvider implements authentication for registries with username/password
type GeneralAuthProvider struct {
	registry *templatemanager.GeneralRegistry
}

// NewGeneralAuthProvider creates a new general auth provider
func NewGeneralAuthProvider(registry *templatemanager.GeneralRegistry) *GeneralAuthProvider {
	return &GeneralAuthProvider{
		registry: registry,
	}
}

// GetAuthOption returns the authentication option for general registries
func (p *GeneralAuthProvider) GetAuthOption(context.Context) (remote.Option, error) {
	return remote.WithAuth(&authn.Basic{
		Username: p.registry.GetUsername(),
		Password: p.registry.GetPassword(),
	}), nil
}
