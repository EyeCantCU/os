/*
Copyright 2024 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package converter

import (
	"context"
	"io"
)

type Interface interface {
	// Convert reads the image from the reader and writers the converted image
	// to the writer.
	Convert(context.Context, io.Reader, io.Writer) error

	// ConvertToFile acts like Convert, but takes a temporary directory and
	// returns the name of a temporary file containing the result.
	ConvertToFile(context.Context, io.Reader, string) (string, error)

	// Cleanup deletes any temporary files created for the functioning of
	// the converter.  It should not be used after calling this.
	Cleanup() error
}
