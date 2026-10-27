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

package env

import (
	"os"
	"strconv"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/utils"
)

var environment = GetEnv("ENVIRONMENT", "prod")

func IsLocal() bool {
	return environment == "local"
}

func IsDevelopment() bool {
	return environment == "dev" || environment == "local"
}

func IsDebug() bool {
	return GetEnv("E2B_DEBUG", "false") == "true"
}

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}

	return value
}

func GetEnvAsInt(key string, defaultValue int) (int, error) {
	if v := os.Getenv(key); v != "" {
		value, err := strconv.Atoi(v)
		if err != nil {
			return defaultValue, err
		}

		return value, nil
	}

	return defaultValue, nil
}

func GetNodeID() string {
	return utils.RequiredEnv("NODE_ID", "Node ID of the instance node is required")
}

func GetNodeIP() string {
	return utils.RequiredEnv("NODE_IP", "Node IP of the instance node is required")
}

func LogsCollectorAddress() string {
	return os.Getenv("LOGS_COLLECTOR_ADDRESS")
}
