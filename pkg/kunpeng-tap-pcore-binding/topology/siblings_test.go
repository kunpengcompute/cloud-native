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

package topology

import (
	"reflect"
	"testing"
)

func TestSiblingPairString(t *testing.T) {
	tests := []struct {
		name string
		pair SiblingPair
		want string
	}{
		{name: "ordered", pair: SiblingPair{CPU0: 2, CPU1: 3}, want: "2,3"},
		{name: "reversed", pair: SiblingPair{CPU0: 7, CPU1: 4}, want: "4,7"},
		{name: "equal", pair: SiblingPair{CPU0: 5, CPU1: 5}, want: "5,5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pair.String(); got != tt.want {
				t.Fatalf("SiblingPair.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePair(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want SiblingPair
		ok   bool
	}{
		{name: "list", raw: "3,2", want: SiblingPair{CPU0: 2, CPU1: 3}, ok: true},
		{name: "range", raw: "6-7", want: SiblingPair{CPU0: 6, CPU1: 7}, ok: true},
		{name: "single cpu", raw: "2"},
		{name: "three cpus", raw: "1-3"},
		{name: "invalid", raw: "one,two"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePair(tt.raw)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parsePair(%q) = (%v, %v), want (%v, %v)", tt.raw, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParseCPUList(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "empty", raw: "", want: nil},
		{name: "list and ranges", raw: " 5, 0-2, 4 ", want: []int{0, 1, 2, 4, 5}},
		{name: "skip empty items", raw: "1,, 3", want: []int{1, 3}},
		{name: "invalid range shape", raw: "1-2-3", wantErr: true},
		{name: "invalid range start", raw: "x-2", wantErr: true},
		{name: "invalid range end", raw: "1-x", wantErr: true},
		{name: "invalid cpu", raw: "x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCPUList(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCPUList(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCPUList(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
