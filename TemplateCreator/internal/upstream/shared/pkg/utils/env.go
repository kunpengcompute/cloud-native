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

package utils

import (
	"fmt"
	"os"
	"strings"
)

// RequiredEnv returns the value of the environment variable for key if it is set, non-empty and not only whitespace.
// It panics otherwise.
//
// Pass the envUsageMsg to describe what the environment variable is used for. This will be used in the error message.
func RequiredEnv(key string, envUsageMsg string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("Required environment variable \"%s\" (%s) is not set. Please set it to a non-empty value.", key, envUsageMsg))
	}

	if value == "" {
		panic(fmt.Sprintf("Required environment variable \"%s\" (%s) is set but it is empty. Please set it to a non-empty value.", key, envUsageMsg))
	}

	if strings.TrimSpace(value) == "" {
		panic(fmt.Sprintf("Required environment variable \"%s\" (%s) is set but it contains only whitespace. Please set it to a non-empty value.", key, envUsageMsg))
	}

	return value
}

// OptionalEnv returns the value of the environment variable for key if it is set, non-empty and not only whitespace.
//
// Pass the envUsageMsg to describe what the environment variable is used for.
// This will be used in the message that is printed if the environment variable is not returned.
func OptionalEnv(key string, envUsageMsg string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		fmt.Fprintf(os.Stderr, "Optional environment variable \"%s\" (%s) is not set.\n", key, envUsageMsg)

		return "", false
	}

	if strings.TrimSpace(value) == "" {
		fmt.Fprintf(os.Stderr, "Optional environment variable \"%s\" (%s) is set but it contains only whitespace.\n", key, envUsageMsg)

		return "", false
	}

	return value, true
}
