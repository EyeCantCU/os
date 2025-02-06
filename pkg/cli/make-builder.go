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
	"runtime"

	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func makeBuilderCmd() *cobra.Command {
	var arch string
	var kernel string
	cmd := &cobra.Command{
		Use:     "make-builder",
		Short:   "Make builder cpio",
		Example: `  wolfi-vm make-builder manifest.yaml`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var builderConfig, output string

			builderConfig = args[0] // e.g. "disk.raw"
			output = args[1]

			if arch == "" {
				arch = runtime.GOARCH
			}
			// standardize everywhere
			arch := types.ParseArchitecture(arch)

			return MakeBuilderCmd(ctx, builderConfig, arch, output, kernel)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "runtime arch for builder (qemu-system-<arch>)")
	cmd.Flags().StringVar(&kernel, "kernel", "", "download a kernel to <path>")

	return cmd
}

func MakeBuilderCmd(ctx context.Context, builderConfigPath string, archType types.Architecture, initrdPath, kernelPath string) error {
	apkArch := archType.ToAPK()

	err := os.MkdirAll(filepath.Dir(initrdPath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating dir for %s: %w", initrdPath, err)
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

	if err := utils.CreateCpio(ctx, initrdPath,
		build.WithImageConfiguration(ic),
		build.WithArch(archType),
	); err != nil {
		return fmt.Errorf("createBuilder() failed with %w", err)
	}

	if kernelPath != "" {
		// fetch needed deps
		tmpDir, err := os.MkdirTemp(filepath.Dir(kernelPath), "")
		if err != nil {
			return fmt.Errorf("failed to create tmpdir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		log.Println("Fetching dependencies: kernel")

		kpath, err := utils.FetchKernel(tmpDir, apkArch)
		if err != nil {
			return fmt.Errorf("failed to pull kernel: %v", err)
		}

		err = os.Rename(kpath, kernelPath)
		if err != nil {
			return fmt.Errorf("failed to copy back kernel: %v", err)
		}
	}
	return err
}
