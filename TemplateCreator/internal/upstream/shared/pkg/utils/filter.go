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

// Filter takes a slice of any type T and a predicate function f.
// It returns a new slice containing only the elements from the input slice
// for which the predicate function returns true.
func Filter[T any](input []T, f func(T) bool) []T {
	var output []T
	for _, v := range input {
		if f(v) {
			output = append(output, v)
		}
	}

	return output
}
