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
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/converter"
	"chainguard.dev/apkoaas/pkg/converter/tar2efi"
	"chainguard.dev/apkoaas/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	prefix    string = "chainguard"
	separator string = "-"
)

func buildCmd() *cobra.Command {
	var builderConfFilePath string
	var builderCpioPath string
	var kernelPath string
	var arch string
	var buildArch string
	var err error
	var output string

	cmd := &cobra.Command{
		Use:     "build",
		Short:   "Build a disk from a YAML configuration file",
		Long:    `Build a disk from a YAML configuration file.`,
		Example: `  wolfi-vm build [config.yaml]`,
		Args:    cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var buildFilePath string
			if len(args) > 0 {
				buildFilePath = args[0] // e.g. "generic.yaml"
			}

			if arch == "" {
				arch = runtime.GOARCH
			}
			// standardize everywhere
			arch := types.ParseArchitecture(arch).ToAPK()

			if buildArch == "" {
				buildArch = runtime.GOARCH
			}
			// standardize everywhere
			buildArch := types.ParseArchitecture(buildArch).ToAPK()

			if auth, ok := os.LookupEnv("HTTP_AUTH"); !ok {
				// Fine, no auth.
			} else if parts := strings.SplitN(auth, ":", 4); len(parts) != 4 {
				return fmt.Errorf("HTTP_AUTH must be in the form 'basic:REALM:USERNAME:PASSWORD' (got %d parts)", len(parts))
			} else if parts[0] != "basic" {
				return fmt.Errorf("HTTP_AUTH must be in the form 'basic:REALM:USERNAME:PASSWORD' (got %q for first part)", parts[0])
			}

			if buildFilePath == "" {
				return fmt.Errorf("empty config, specify valid yaml path")
			}

			if builderConfFilePath == "" && builderCpioPath == "" {
				return fmt.Errorf("empty builder config, specify valid yaml with --builder, or a valid cpio with --builder-cpio")
			}

			if builderConfFilePath != "" && builderCpioPath != "" {
				return fmt.Errorf("specify valid yaml with --builder, or a valid cpio with --builder-cpio, not both")
			}

			if kernelPath == "" {
				destDir := os.TempDir()

				kernelPath, err = utils.FetchKernel(destDir, buildArch)
				if err != nil {
					return fmt.Errorf("missing kernel path, specify a valid kernel with --kernel")
				}
			}

			return BuildCmd(ctx, buildFilePath, builderConfFilePath, builderCpioPath, kernelPath, buildArch, arch, output)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "arch present in disk")
	cmd.Flags().StringVar(&output, "output", "disk.raw", "write the created disk image here")
	cmd.Flags().StringVar(&buildArch, "build-arch", "", "arch used to build the disk (qemu-system-<arch>)")
	cmd.Flags().StringVar(&builderConfFilePath, "builder", "", "path to builder yaml definition")
	cmd.Flags().StringVar(&builderCpioPath, "builder-cpio", "", "path to premade builder cpio")
	cmd.Flags().StringVar(&kernelPath, "kernel", "", "path to kernel to use")

	return cmd
}

func createDisk(ctx context.Context, converter converter.Interface, buildTargetPath, arch string, inputTar *gzip.Reader, output string) error {
	var err error

	err = os.MkdirAll(filepath.Dir(output), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create output dir: %w", err)
	}

	return converter.ConvertToFile(ctx, inputTar, output, types.ParseArchitecture(arch))
}

func createBuilder(ctx context.Context, builderConfigPath, builderCpio, kernelPath, arch string) (converter.Interface, error) {
	if builderCpio == "" {
		builderConfig, err := os.Open(builderConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open apko yaml: %w", err)
		}
		defer builderConfig.Close()

		var ic types.ImageConfiguration
		dec := yaml.NewDecoder(builderConfig)
		dec.KnownFields(true)
		if err := dec.Decode(&ic); err != nil {
			return nil, fmt.Errorf("failed to parse image configuration: %v", err)
		}

		return tar2efi.New(ctx, kernelPath, arch, ic)
	}

	// just convert using the provided cpio
	return tar2efi.NewFromCpio(ctx, builderCpio, kernelPath, arch)
}

func BuildCmd(ctx context.Context, buildFilePath, builderConf, builderCpio, kernelPath, buildArch, arch, output string) error {
	apkoTar, err := utils.CreateTar(ctx, buildFilePath, arch)
	if err != nil {
		return fmt.Errorf("error creating image.tar: %w", err)
	}
	defer os.RemoveAll(apkoTar)

	converter, err := createBuilder(ctx, builderConf, builderCpio, kernelPath, buildArch)
	if err != nil {
		return fmt.Errorf("error creating tar converter: %w", err)
	}

	// in case it's not a persistent cpio, we want to cleanup
	if builderCpio == "" {
		defer converter.Cleanup()
	}

	tar, err := os.Open(apkoTar)
	if err != nil {
		return fmt.Errorf("error reading image.tar: %w", err)
	}
	defer tar.Close()

	gz, err := gzip.NewReader(tar)
	if err != nil {
		return fmt.Errorf("error opening image.tar: %w", err)
	}
	defer gz.Close()

	err = createDisk(ctx, converter, buildFilePath, arch, gz, output)
	if err != nil {
		return fmt.Errorf("error converting to disk image: %w", err)
	}

	fmt.Println(output)
	return nil
}
