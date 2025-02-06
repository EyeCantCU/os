/*
Copyright 2024 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package converter

import (
	"context"
	"io"

	"chainguard.dev/apko/pkg/build/types"
)

type Interface interface {
	// Convert reads the image from the reader and writers the converted image
	// to the writer.
	Convert(context.Context, io.Reader, io.Writer, types.Architecture) error

	// ConvertToFile acts like Convert, but takes an output path
	// and returns error.
	ConvertToFile(context.Context, io.Reader, string, types.Architecture) error

	// Cleanup deletes any temporary files created for the functioning of
	// the converter.  It should not be used after calling this.
	Cleanup() error
}
