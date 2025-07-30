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

package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/utils"
	"github.com/spf13/cobra"
)

func fetchCmd() *cobra.Command {
	var arch string
	cmd := &cobra.Command{
		Use:     "fetch",
		Short:   "fetch a thing (kernel or ovmf)",
		Example: `  apkoaas fetch --arch=x86_64 kernel ./kernel-x86_64`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var artifactName, output string

			artifactName = args[0] // e.g. "disk.raw"
			output = args[1]

			if arch == "" {
				arch = runtime.GOARCH
			}
			// standardize everywhere
			arch := types.ParseArchitecture(arch)

			return FetchCmd(ctx, artifactName, output, arch)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "arch to download (apk arch)")

	return cmd
}

func FetchCmd(ctx context.Context, artifactName string, output string, arch types.Architecture) error {
	apkArch := arch.ToAPK()

	var fetcher func(string, string) (string, error)
	switch artifactName {
	case "ovmf":
		fetcher = utils.FetchBios
	case "kernel":
		fetcher = utils.FetchKernel
	default:
		return fmt.Errorf("Unexpected artifact '%s'", artifactName)
	}

	err := os.MkdirAll(filepath.Dir(output), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create dir for output %s: %v", output, err)
	}

	tmpDir, err := os.MkdirTemp(filepath.Dir(output), "")
	if err != nil {
		return fmt.Errorf("failed to create tmpdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log.Printf("Fetching %s -> %s", artifactName, output)
	tmpFile, err := fetcher(tmpDir, apkArch)
	if err != nil {
		return fmt.Errorf("failed to pull %s: %v", artifactName, err)
	}

	err = os.Rename(tmpFile, output)
	if err != nil {
		return fmt.Errorf("failed to rename downloaded %s: %v", artifactName, err)
	}
	return nil
}
