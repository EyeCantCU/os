// Copyright 2025 Chainguard, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package publisher

import (
	"testing"
)

func TestExpandTags(t *testing.T) {
	tests := []struct {
		name      string
		tags      []string
		timestamp string
		want      []string
	}{
		{
			name:      "single tag expansion",
			tags:      []string{"azure-python-slim"},
			timestamp: "20241103-1234",
			want: []string{
				"azure-python-slim-20241103-1234",
				"azure-python-slim-latest",
			},
		},
		{
			name: "multiple tags expansion",
			tags: []string{
				"azure-python-3.13-slim",
				"azure-python-slim",
			},
			timestamp: "20241103-1234",
			want: []string{
				"azure-python-3.13-slim-20241103-1234",
				"azure-python-3.13-slim-latest",
				"azure-python-slim-20241103-1234",
				"azure-python-slim-latest",
			},
		},
		{
			name:      "tag with hyphens",
			tags:      []string{"qemu-base-slim"},
			timestamp: "20241103-1234",
			want: []string{
				"qemu-base-slim-20241103-1234",
				"qemu-base-slim-latest",
			},
		},
		{
			name:      "tag with version numbers",
			tags:      []string{"aws-eks-1.33"},
			timestamp: "20241103-1234",
			want: []string{
				"aws-eks-1.33-20241103-1234",
				"aws-eks-1.33-latest",
			},
		},
		{
			name:      "empty tags slice",
			tags:      []string{},
			timestamp: "20241103-1234",
			want:      []string{},
		},
		{
			name:      "three tags",
			tags:      []string{"tag1", "tag2", "tag3"},
			timestamp: "20241103-1234",
			want: []string{
				"tag1-20241103-1234",
				"tag1-latest",
				"tag2-20241103-1234",
				"tag2-latest",
				"tag3-20241103-1234",
				"tag3-latest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandTags(tt.tags, tt.timestamp)

			if len(got) != len(tt.want) {
				t.Errorf("ExpandTags() returned %d tags, want %d", len(got), len(tt.want))
				t.Errorf("got:  %v", got)
				t.Errorf("want: %v", tt.want)
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("ExpandTags()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
