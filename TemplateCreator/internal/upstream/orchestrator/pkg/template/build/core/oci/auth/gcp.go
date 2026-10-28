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

	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	templatemanager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
)

// GCPAuthProvider implements authentication for Google Container Registry
type GCPAuthProvider struct {
	registry *templatemanager.GCPRegistry
}

// NewGCPAuthProvider creates a new GCP auth provider
func NewGCPAuthProvider(registry *templatemanager.GCPRegistry) *GCPAuthProvider {
	return &GCPAuthProvider{
		registry: registry,
	}
}

// GetAuthOption returns the authentication option for GCP
func (p *GCPAuthProvider) GetAuthOption(context.Context) (remote.Option, error) {
	// Create authenticator using the service account JSON
	authenticator := google.NewJSONKeyAuthenticator(p.registry.GetServiceAccountJson())

	return remote.WithAuth(authenticator), nil
}
