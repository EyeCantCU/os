//go:build withauth

/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tar2efi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"chainguard.dev/apko/pkg/apk/auth"
	apkfs "chainguard.dev/apko/pkg/apk/fs"
	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/converter"
	"chainguard.dev/apkoaas/pkg/utils"
	"gopkg.in/yaml.v3"
)

func buildImage(t *testing.T, c converter.Interface, ic types.ImageConfiguration) string {
	// We should comfortably be able to convert all of these images
	// in under a minute.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	fs := apkfs.DirFS(t.TempDir(), apkfs.WithCreateDir())
	arch := types.ParseArchitecture(runtime.GOARCH)
	bc, err := build.New(ctx, fs,
		build.WithAuthenticator(auth.CGRAuth{}),
		build.WithArch(arch),
		build.WithImageConfiguration(ic),
	)
	if err != nil {
		t.Fatalf("build.New() failed with %v", err)
	}

	_, layer, err := bc.BuildLayer(ctx)
	if err != nil {
		t.Fatalf("bc.BuildLayer() failed with %v", err)
	}

	ucl, err := layer.Uncompressed()
	if err != nil {
		t.Fatalf("layer.Uncompressed() failed with %v", err)
	}

	disk, err := os.Create(filepath.Join(t.TempDir(), "disk.raw"))
	if err != nil {
		t.Fatalf("os.CreateTemp() failed with %v", err)
	}

	if err := c.Convert(ctx, ucl, disk, arch); err != nil {
		t.Fatalf("c.Convert() failed with %v", err)
	}
	return disk.Name()
}

func boot(ctx context.Context, t *testing.T, bios, disk string, arch types.Architecture) string {
	buf := bytes.NewBuffer(nil)
	qemuCmd := utils.GenerateQEMUCommand(arch.ToAPK(), disk, bios)
	cmd := exec.CommandContext(ctx, qemuCmd[0], qemuCmd[1:]...)
	cmd.Stdout = buf
	cmd.Stderr = buf
	if err := cmd.Run(); err != nil {
		t.Log(buf.String())
		t.Fatalf("qemu failed with %v", err)
	}

	return buf.String()
}

func TestConverter(t *testing.T) {
	destDir := t.TempDir()
	defer os.RemoveAll(destDir)
	arch := types.ParseArchitecture(runtime.GOARCH)

	kernel, err := utils.FetchKernel(destDir, arch.ToAPK())
	if err != nil {
		t.Fatalf("FetchKernel failed with %v", err)
	}

	bios, err := utils.FetchBios(destDir, arch.ToAPK())
	if err != nil {
		t.Fatalf("FetchBios failed with %v", err)
	}

	cfg := readBuildConfig(t)
	c, err := New(context.Background(), kernel, arch.ToAPK(), cfg)
	if err != nil {
		t.Fatalf("New() failed with %v", err)
	}

	const GiB = 1024 * 1024 * 1024

	tests := []struct {
		filename     string
		expectedSize int64
		testDisk     func(t *testing.T, disk string)
	}{{
		filename:     "testdata/curl.yaml",
		expectedSize: 2 * GiB,
		testDisk: func(t *testing.T, disk string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)
			output := boot(ctx, t, bios, disk, arch)
			if !strings.Contains(output, `"cmd":"--fail file:///etc/apko.json"`) {
				t.Fatalf("unexpected output %q", output)
			}
		},
	}, {
		filename:     "testdata/generic.yaml",
		expectedSize: 2 * GiB,
		// TODO(mattmoor): How do we want to test the generic image?
	}, {
		filename:     "testdata/docker-runner.yaml",
		expectedSize: 2 * GiB,
		// TODO(mattmoor): How do we want to test the docker-runner image?
	}, {
		filename:     "testdata/google.yaml",
		expectedSize: 2 * GiB,
		testDisk: func(t *testing.T, disk string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)
			// TODO(mattmoor): if we are given a bucket/project, then upload the
			// converted disk to it, and then create an image/VM from it.
			if err := utils.WriteDiskTarGzip(ctx, disk, io.Discard); err != nil {
				t.Fatalf("WriteDiskTarGzip() failed with %v", err)
			}
		},
	}, {
		filename:     "testdata/aws-ec2.yaml",
		expectedSize: 2 * GiB,
		testDisk: func(t *testing.T, disk string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)
			// TODO(mattmoor): if we are given a bucket/account, then upload the
			// converted disk to it, and then create an image/VM from it.
			if err := utils.WriteDiskVPCGzip(ctx, disk, io.Discard); err != nil {
				t.Fatalf("WriteDiskVPCGzip() failed with %v", err)
			}
		},
	}, {
		filename:     "testdata/workstation.yaml",
		expectedSize: 6 * GiB,
		// TODO(mattmoor): How do we want to test the workstation image?
	}}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			b, err := os.ReadFile(test.filename)
			if err != nil {
				t.Fatalf("os.ReadFile() failed with %v", err)
			}
			var ic types.ImageConfiguration
			dec := yaml.NewDecoder(bytes.NewBuffer(b))
			dec.KnownFields(true)
			if err := dec.Decode(&ic); err != nil {
				t.Fatalf("failed to parse image configuration: %v", err)
			}
			disk := buildImage(t, c, ic)

			// Based on the spec above, the disk should have a size of 1 GiB.
			if stat, err := os.Stat(disk); err != nil {
				t.Fatalf("os.Stat() failed with %v", err)
			} else if got, want := stat.Size(), test.expectedSize; got != want {
				t.Fatalf("disk size = %d, want %d", got, want)
			}

			if test.testDisk != nil {
				test.testDisk(t, disk)
			}
		})
	}

	t.Run("malformed", func(t *testing.T) {
		malformedTarball := bytes.NewBufferString("asdf")

		disk, err := os.CreateTemp(t.TempDir(), "disk.raw")
		if err != nil {
			t.Fatalf("os.CreateTemp() failed with %v", err)
		}

		if err := c.Convert(context.Background(), malformedTarball, disk, arch); err == nil {
			t.Fatalf("c.Convert() failed with %v", err)
		} else if !errors.Is(err, ErrDiskConversion) {
			t.Fatalf("c.Convert() failed with %v", err)
		}
	})
}

// Read the file at testdata/builder.yaml and store it in the BUILDER_CONFIG
// environment variable.
func readBuildConfig(t *testing.T) types.ImageConfiguration {
	t.Helper()
	b, err := os.ReadFile("testdata/builder.yaml")
	if err != nil {
		t.Fatalf("failed to read builder.yaml: %v", err)
	}
	dec := yaml.NewDecoder(bytes.NewBuffer(b))
	dec.KnownFields(true)

	var ic types.ImageConfiguration
	if err := dec.Decode(&ic); err != nil {
		t.Fatalf("failed to parse image configuration: %v", err)
	}
	return ic
}
