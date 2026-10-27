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

	"golang.org/x/mod/semver"
)

func sanitizeVersion(version string) string {
	if len(version) > 0 && version[0] != 'v' {
		version = "v" + version
	}

	return version
}

func IsGTEVersion(curVersion, minVersion string) (bool, error) {
	curVersion = sanitizeVersion(curVersion)
	minVersion = sanitizeVersion(minVersion)

	if !semver.IsValid(curVersion) {
		return false, fmt.Errorf("invalid current version format: %s", curVersion)
	}

	if !semver.IsValid(minVersion) {
		return false, fmt.Errorf("invalid minimum version format: %s", minVersion)
	}

	return semver.Compare(curVersion, minVersion) >= 0, nil
}

func IsSmallerVersion(curVersion, maxVersionExcluded string) (bool, error) {
	curVersion = sanitizeVersion(curVersion)
	maxVersionExcluded = sanitizeVersion(maxVersionExcluded)

	if !semver.IsValid(curVersion) {
		return false, fmt.Errorf("invalid current version format: %s", curVersion)
	}

	if !semver.IsValid(maxVersionExcluded) {
		return false, fmt.Errorf("invalid maximum version format: %s", maxVersionExcluded)
	}

	return semver.Compare(curVersion, maxVersionExcluded) < 0, nil
}

func IsVersion(curVersion, eqVersion string) bool {
	curVersion = sanitizeVersion(curVersion)
	eqVersion = sanitizeVersion(eqVersion)

	return semver.Compare(curVersion, eqVersion) == 0
}
