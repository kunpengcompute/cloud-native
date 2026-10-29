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

package utils

// Map goes through each item in the input slice and applies the function f to it.
// It returns a new slice with the results.
func Map[T any, U any](input []T, f func(T) U) []U {
	output := make([]U, len(input))
	for i, v := range input {
		output[i] = f(v)
	}

	return output
}
