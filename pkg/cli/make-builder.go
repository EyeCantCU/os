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
	"log"
	"os"
	"path/filepath"
	"strings"

	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/converter/tar2efi"
	"chainguard.dev/apkoaas/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func makeBuilderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "make-builder",
		Short:   "Make builder cpio",
		Example: `  wolfi-vm make-builder [manifest.yaml]`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var builderConfig string
			if len(args) > 0 {
				builderConfig = args[0] // e.g. "disk.raw"
			}

			if builderConfig == "" {
				return fmt.Errorf("empty config, specify valid yaml path")
			}

			return MakeBuilderCmd(ctx, builderConfig)
		},
	}

	return cmd
}

func MakeBuilderCmd(ctx context.Context, builderConfigPath string) error {
	builderDir := filepath.Join("output", tar2efi.TargetArch.ToAPK(), strings.Split(filepath.Base(builderConfigPath), ".")[0])
	builderName := filepath.Join(builderDir, "initramfs.cpio")
	kernelName := "kernel-" + tar2efi.TargetArch.ToAPK()
	biosName := "ovmf-" + tar2efi.TargetArch.ToAPK() + ".fd"

	err := os.MkdirAll(filepath.Dir(builderName), os.ModePerm)
	if err != nil {
		return fmt.Errorf("error building cpio: %w", err)
	}

	builderConfig, err := os.Open(builderConfigPath)
	if err != nil {
		return fmt.Errorf("failed to open apko yaml: %w", err)
	}
	defer builderConfig.Close()

	var ic types.ImageConfiguration
	dec := yaml.NewDecoder(builderConfig)
	dec.KnownFields(true)
	if err := dec.Decode(&ic); err != nil {
		return fmt.Errorf("failed to parse image configuration: %v", err)
	}

	if err := utils.CreateCpio(ctx, builderName,
		build.WithImageConfiguration(ic),
		build.WithArch(tar2efi.TargetArch),
	); err != nil {
		return fmt.Errorf("createBuilder() failed with %w", err)
	}

	// fetch needed deps
	tmpDir, err := os.MkdirTemp(builderDir, "")
	if err != nil {
		return fmt.Errorf("failed to pull dependencies: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log.Println("Fetching dependencies: bios")

	bios, err := utils.FetchBios(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to pull bios: %v", err)
	}

	log.Println("Fetching dependencies: kernel")

	kernel, err := utils.FetchKernel(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to pull kernel: %v", err)
	}

	err = os.Rename(bios,
		filepath.Join(builderDir, biosName))
	if err != nil {
		return fmt.Errorf("failed to copy back bios: %v", err)
	}

	err = os.Rename(kernel,
		filepath.Join(builderDir, kernelName))
	if err != nil {
		return fmt.Errorf("failed to copy back kernel: %v", err)
	}

	return err
}
