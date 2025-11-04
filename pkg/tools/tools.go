//go:build tools

// tools tracks and versions dependencies for code generation tools
package tools

import (
	_ "github.com/chainguard-dev/yam"
	_ "github.com/chainguard-images/images-private/x/cue/cmd/art"
	_ "github.com/mikefarah/yq/v4"
)
