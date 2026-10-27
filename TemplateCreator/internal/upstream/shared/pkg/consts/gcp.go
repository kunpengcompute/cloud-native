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

package consts

import (
	"encoding/base64"
	"fmt"
	"os"
)

var (
	GCPProject                 = os.Getenv("GCP_PROJECT_ID")
	Domain                     = os.Getenv("DOMAIN_NAME")
	DockerRegistry             = os.Getenv("GCP_DOCKER_REPOSITORY_NAME")
	GoogleServiceAccountSecret = os.Getenv("GOOGLE_SERVICE_ACCOUNT_BASE64")
	DockerAuthConfig           = os.Getenv("DOCKER_AUTH_BASE64")
	GCPRegion                  = os.Getenv("GCP_REGION")
)

var EncodedDockerCredentials = base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "_json_key_base64:%s", GoogleServiceAccountSecret))
