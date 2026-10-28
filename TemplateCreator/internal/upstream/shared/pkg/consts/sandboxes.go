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

package consts

import (
	"strconv"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/env"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/utils"
)

const NodeIDLength = 8

// ClientID Sandbox ID client part used during migration when we are still returning client but its no longer serving its purpose,
// and we are returning it only for backward compatibility with SDK clients.
// We don't want to use some obviously dummy value such as empty zeros, because for users it will look like something is wrong with the sandbox id
const ClientID = "6532622b"

var OrchestratorAPIPort = uint16(utils.Must(strconv.ParseUint(env.GetEnv("ORCHESTRATOR_PORT", "5008"), 10, 16)))
