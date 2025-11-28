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
	"context"
	"fmt"

	"github.com/chainguard-dev/clog"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ExpandTags expands each base tag into timestamped and latest variants.
// For each tag "foo", creates "foo-{timestamp}" and "foo-latest".
// Example: ["azure-python-slim"] with timestamp "20241103-1234" becomes:
//   - "azure-python-slim-20241103-1234"
//   - "azure-python-slim-latest"
func ExpandTags(tags []string, timestamp string) []string {
	result := make([]string, 0, len(tags)*2)
	for _, tag := range tags {
		result = append(result, fmt.Sprintf("%s-%s", tag, timestamp))
		result = append(result, fmt.Sprintf("%s-latest", tag))
	}
	return result
}

// applyTagsToTarget applies a list of tags to a target (image or index) and returns the applied tags
func applyTagsToTarget(ctx context.Context, ref name.Repository, target interface{}, tags []string, remoteOpts []remote.Option) ([]string, error) {
	log := clog.FromContext(ctx)
	appliedTags := []string{}

	for _, tag := range tags {
		tagRef := ref.Tag(tag)
		var err error

		switch t := target.(type) {
		case v1.Image:
			err = remote.Tag(tagRef, t, remoteOpts...)
		case v1.ImageIndex:
			err = remote.Tag(tagRef, t, remoteOpts...)
		default:
			return appliedTags, fmt.Errorf("unsupported target type: %T", target)
		}

		if err != nil {
			return appliedTags, fmt.Errorf("applying tag %s: %w", tag, err)
		}
		log.Infof("Tagged: %s", tagRef.String())
		appliedTags = append(appliedTags, tag)
	}

	return appliedTags, nil
}

