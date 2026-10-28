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

package cfg

import (
	"strings"
)

type ServiceType string

const (
	UnknownService  ServiceType = "orch-unknown"
	Orchestrator    ServiceType = "orchestrator"
	TemplateManager ServiceType = "template-manager"
)

// ParseServiceType converts a string to a ServiceType.
// It is case-insensitive and defaults to UnknownService.
func ParseServiceType(s string) ServiceType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(Orchestrator):
		return Orchestrator
	case string(TemplateManager):
		return TemplateManager
	default:
		return UnknownService
	}
}

// GetServices parses the ORCHESTRATOR_SERVICES environment variable
// and returns a slice of known ServiceTypes.
func GetServices(config Config) []ServiceType {
	rawServiceNames := config.Services

	var services []ServiceType
	for _, name := range rawServiceNames {
		service := ParseServiceType(name)
		if service != UnknownService {
			services = append(services, service)
		}
	}

	return services
}

// GetServiceName returns a single string identifier for the given services.
// If multiple services are present, they are joined with underscores.
func GetServiceName(services []ServiceType) string {
	if len(services) == 0 {
		return string(UnknownService)
	}

	var builder strings.Builder
	for i, s := range services {
		if i > 0 {
			builder.WriteString("_")
		}
		builder.WriteString(string(s))
	}

	return builder.String()
}
