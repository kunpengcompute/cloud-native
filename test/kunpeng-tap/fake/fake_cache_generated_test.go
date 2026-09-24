/*
 * Copyright (c) 2026 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package fake

import "testing"

func TestFakeCacheSliceCopiesPreserveInputState(t *testing.T) {
	tests := []struct {
		name string
		call func(*FakeCache, []string)
		args func(*FakeCache, int) []string
	}{
		{
			name: "CleanupStaleContainers",
			call: func(fake *FakeCache, input []string) { fake.CleanupStaleContainers(input) },
			args: func(fake *FakeCache, index int) []string {
				return fake.CleanupStaleContainersArgsForCall(index)
			},
		},
		{
			name: "ValidateCachedContainers",
			call: func(fake *FakeCache, input []string) { fake.ValidateCachedContainers(input) },
			args: func(fake *FakeCache, index int) []string {
				return fake.ValidateCachedContainersArgsForCall(index)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &FakeCache{}
			test.call(fake, nil)
			if got := test.args(fake, 0); got != nil {
				t.Fatalf("nil input copied as non-nil slice: %#v", got)
			}

			empty := make([]string, 0)
			test.call(fake, empty)
			if got := test.args(fake, 1); got == nil || len(got) != 0 {
				t.Fatalf("empty input state was not preserved: %#v", got)
			}

			input := []string{"before"}
			test.call(fake, input)
			input[0] = "after"
			if got := test.args(fake, 2); len(got) != 1 || got[0] != "before" {
				t.Fatalf("non-empty input was not copied: %#v", got)
			}
		})
	}
}
