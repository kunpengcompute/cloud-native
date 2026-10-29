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

package sbxlogger

import (
	"go.uber.org/zap"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
)

type SandboxMetadata struct {
	SandboxID  string
	TemplateID string
	TeamID     string
}

type LoggerMetadata interface {
	LoggerMetadata() SandboxMetadata
}

func (sm SandboxMetadata) LoggerMetadata() SandboxMetadata {
	return sm
}

func (sm SandboxMetadata) Fields() []zap.Field {
	return []zap.Field{
		logger.WithSandboxID(sm.SandboxID),
		logger.WithTemplateID(sm.TemplateID),
		logger.WithTeamID(sm.TeamID),

		// Fields for Vector
		zap.String("instanceID", sm.SandboxID),
		zap.String("envID", sm.TemplateID),
	}
}
