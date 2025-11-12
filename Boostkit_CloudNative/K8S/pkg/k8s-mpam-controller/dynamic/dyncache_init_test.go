/*
 * Copyright (c) 2025 Huawei Technology corp.
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

package dynamic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetL3IdList(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expectIds   []int
		expectErr   bool
	}{
		{
			name:        "valid L3 line",
			fileContent: "L3:0=ff;1=aa;2=bb\n",
			expectIds:   []int{0, 1, 2},
			expectErr:   false,
		},
		{
			name:        "no L3 line",
			fileContent: "L2:0=ff;1=aa\n",
			expectIds:   nil,
			expectErr:   true,
		},
		{
			name:        "with invalid entries",
			fileContent: "L3:0=ff;bad;2=aa\n",
			expectIds:   []int{0, 2},
			expectErr:   false,
		},
		{
			name:        "empty file",
			fileContent: "",
			expectIds:   nil,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "schemata")

			// 写入测试内容
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			ids, err := getL3IdList(tmpFile)

			if tt.expectErr && err == nil {
				t.Fatalf("expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(ids) != len(tt.expectIds) {
				t.Fatalf("expected %v IDs, got %v", tt.expectIds, ids)
			}

			for i := range ids {
				if ids[i] != tt.expectIds[i] {
					t.Fatalf("expected ID %v at index %d, got %v", tt.expectIds[i], i, ids[i])
				}
			}
		})
	}
}
