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
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	templatemanager "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
)

// AWSAuthProvider implements authentication for AWS ECR
type AWSAuthProvider struct {
	registry *templatemanager.AWSRegistry
}

// NewAWSAuthProvider creates a new AWS auth provider
func NewAWSAuthProvider(registry *templatemanager.AWSRegistry) *AWSAuthProvider {
	return &AWSAuthProvider{
		registry: registry,
	}
}

// GetAuthOption returns the authentication option for AWS ECR
func (p *AWSAuthProvider) GetAuthOption(ctx context.Context) (remote.Option, error) {
	// Load AWS configuration with the provided credentials
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(p.registry.GetAwsRegion()),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			p.registry.GetAwsAccessKeyId(),
			p.registry.GetAwsSecretAccessKey(),
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create ECR client and get authorization token
	ecrClient := ecr.NewFromConfig(cfg)
	token, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(token.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no ECR authorization data returned")
	}

	// Decode the authorization token
	authData := token.AuthorizationData[0]
	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ECR token: %w", err)
	}

	// Parse the token (format is username:password)
	parts := strings.SplitN(string(decodedToken), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ECR token format")
	}

	return remote.WithAuth(&authn.Basic{
		Username: parts[0],
		Password: parts[1],
	}), nil
}
