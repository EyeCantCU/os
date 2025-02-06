// Copyright 2022 Chainguard, Inc.
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
	"os"
	"os/exec"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/utils"
	"github.com/spf13/cobra"
)

func debugCmd() *cobra.Command {
	var arch string
	var ovmf string

	cmd := &cobra.Command{
		Use:     "debug",
		Short:   "Debug a vm disk",
		Example: `  wolfi-vm debug [disk.rawl]`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var efiDisk string
			if len(args) > 0 {
				efiDisk = args[0] // e.g. "disk.raw"
			}

			if efiDisk == "" {
				return fmt.Errorf("empty config, specify valid raw disk path")
			}

			if arch == "" {
				return fmt.Errorf("please specify target VM architecture")
			}

			// standardize everywhere
			arch := types.ParseArchitecture(arch).ToAPK()

			return DebugCmd(ctx, efiDisk, arch, ovmf)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "arch to build the disk")
	cmd.Flags().StringVar(&ovmf, "ovmf", "", "uefi file to boot")

	return cmd
}

func DebugCmd(ctx context.Context, efiDisk, arch, ovmf string) error {
	if ovmf == "" {
		ovmfDir, err := os.MkdirTemp("", "")
		if err != nil {
			return fmt.Errorf("os.MkdirTemp() failed with %w", err)
		}

		ovmf, err = utils.FetchBios(ovmfDir, arch)
		if err != nil {
			return err
		}

		defer os.RemoveAll(ovmfDir)
	}

	command := utils.GenerateQEMUCommand(arch, efiDisk, ovmf)

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
