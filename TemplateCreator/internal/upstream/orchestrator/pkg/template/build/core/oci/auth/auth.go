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

	"github.com/google/go-containerregistry/pkg/v1/remote"

	templatemanager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
)

// RegistryAuthProvider is an interface for different registry authentication providers
type RegistryAuthProvider interface {
	// GetAuthOption returns the remote.Option for authentication
	GetAuthOption(ctx context.Context) (remote.Option, error)
}

// NewAuthProvider is a factory function that creates the appropriate auth provider
func NewAuthProvider(registry *templatemanager.FromImageRegistry) RegistryAuthProvider {
	if registry == nil {
		return nil
	}

	switch auth := registry.GetType().(type) {
	case *templatemanager.FromImageRegistry_Aws:
		return NewAWSAuthProvider(auth.Aws)
	case *templatemanager.FromImageRegistry_Gcp:
		return NewGCPAuthProvider(auth.Gcp)
	case *templatemanager.FromImageRegistry_General:
		return NewGeneralAuthProvider(auth.General)
	default:
		return nil
	}
}
