/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tar2efi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/converter"
	"chainguard.dev/apkoaas/pkg/utils"
)

var ErrDiskConversion = errors.New("disk conversion failed")

// New creates a new converter.Interface for translating a tarball-based image
// into an EFI disk image.
func New(ctx context.Context, kernel string, buildArch string, ic types.ImageConfiguration) (converter.Interface, error) {
	// Create the builder initrd on startup.
	f, err := os.CreateTemp("", "builder-*.cpio")
	if err != nil {
		return nil, fmt.Errorf("os.CreateTemp() failed with %w", err)
	} else if err := f.Close(); err != nil {
		return nil, fmt.Errorf("f.Close() failed with %w", err)
	}
	if err := utils.CreateCpio(ctx, f.Name(),
		build.WithImageConfiguration(ic),
		build.WithArch(TargetArch),
	); err != nil {
		return nil, fmt.Errorf("utils.CreateCpio() failed with %w", err)
	}

	return NewFromCpio(ctx, f.Name(), kernel, buildArch)
}

// NewFromCpio creates a new converter.Interface for translating a tarball-based image
// into an EFI disk image.
// This one does not build a cpio, but needs one to be provided
func NewFromCpio(ctx context.Context, cpio, kernel string, buildArch string) (converter.Interface, error) {
	// Force Cloud Run to pull in these files at startup to avoid them creating
	// a big hit at request time.
	if _, err := os.ReadFile(kernel); err != nil {
		return nil, fmt.Errorf("can't read kernel %w", err)
	}

	return &t2e{
		kernel:    kernel,
		builder:   cpio,
		buildArch: buildArch,
	}, nil
}

type t2e struct {
	kernel    string
	builder   string
	buildArch string
}

// Check that we implement the interface
var _ converter.Interface = (*t2e)(nil)

// Cleanup implements converter.Interface
func (c *t2e) Cleanup() error {
	return os.RemoveAll(c.builder)
}

// Convert implements converter.Interface
func (c *t2e) Convert(ctx context.Context, input io.Reader, output io.Writer) error {
	// Create a scratch space for ourselves.
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("os.MkdirTemp() failed with %w", err)
	}
	defer os.RemoveAll(tmp)

	name, err := c.ConvertToFile(ctx, input, tmp)
	if err != nil {
		return fmt.Errorf("ConvertToFile() failed with %w", err)
	}

	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("os.Open() failed with %w", err)
	}
	if _, err := io.Copy(output, f); err != nil {
		return fmt.Errorf("io.Copy() failed with %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("f.Close() failed with %w", err)
	}
	return nil
}

// ConvertToFile implements converter.Interface
func (c *t2e) ConvertToFile(ctx context.Context, input io.Reader, tmp string) (string, error) {
	// Write the uncompressed filesystem into a tarball for us to mount.
	var size int64
	imageTarball, err := os.Create(filepath.Join(tmp, "image.tar"))
	if err != nil {
		return "", fmt.Errorf("os.Create() failed with %w", err)
	} else if size, err = io.Copy(imageTarball, input); err != nil {
		return "", fmt.Errorf("io.Copy() failed with %w", err)
	} else if err := imageTarball.Close(); err != nil {
		return "", fmt.Errorf("f.Close() failed with %w", err)
	}

	// Most CSPs require disks to be in 1GB increments.  Compute the size of the
	// disk we need by doubling the size of the tarball we received and then
	// rounding that up to the nearest GB.
	const oneGB int64 = 1024 * 1024 * 1024
	numGB := (2*size + oneGB - 1) / oneGB
	// we have a 1Gb EFI partition, let's ensure we're using at least 2Gb
	if numGB < 2 {
		numGB = 2
	}

	// Create a raw disk with the appropriate size, which we will mount as a
	// block device and write the converted image into.
	// TODO(mattmoor): We should experiment with writing the GPT table in Go
	// vs. with sfdisk.  smoser has used this in the past:
	//     https://pkg.go.dev/github.com/rekby/gpt
	diskFilename, err := os.Create(filepath.Join(tmp, "disk.raw"))
	if err != nil {
		return "", fmt.Errorf("os.Create() failed with %w", err)
	} else if err := diskFilename.Truncate(numGB * oneGB); err != nil {
		return "", fmt.Errorf("f.Truncate() failed with %w", err)
	} else if err := diskFilename.Close(); err != nil {
		return "", fmt.Errorf("f.Close() failed with %w", err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "result"), []byte("1"), 0o600); err != nil {
		return "", fmt.Errorf("os.WriteFile() failed with %w", err)
	}

	// write to /output/buildArch what arch we're building for, x86 or arm
	// this allows to cross build between architectures
	// in the builder there are some stuff that needs to be different between
	// arm and amd64:
	//	- root partition UUID type as specified by: https://uapi-group.org/specifications/specs/discoverable_partitions_specification
	if err := os.WriteFile(filepath.Join(tmp, "buildArch"), []byte(c.buildArch), 0o600); err != nil {
		return "", fmt.Errorf("os.WriteFile() failed with %w", err)
	}

	// Convert the image to a raw disk image.
	buf := bytes.NewBuffer(nil)
	{
		// nolint:gosec // We trust the kernel argument here.
		cmd := exec.CommandContext(ctx, qemuCommand, append(slices.Clone(baseQEMUArgs),
			"-m", "4G",
			"-nographic",

			// Disable networking.
			"-nic", "none",

			"-device", "virtio-9p-pci,id=fs101,fsdev=fsdev101,mount_tag=output",
			"-fsdev", "local,multidevs=remap,security_model=mapped,id=fsdev101,path="+tmp,

			"-device", "virtio-blk-pci,drive=image.tar,serial=input-tar,discard=true",
			"-blockdev", "driver=raw,node-name=image.tar,file.driver=file,file.filename="+imageTarball.Name(),

			"-device", "virtio-blk-pci,drive=disk.raw.tmp,serial=install-target-disk,discard=true",
			"-blockdev", "driver=raw,node-name=disk.raw.tmp,file.driver=file,file.filename="+diskFilename.Name(),

			// Don't reboot on a kernel panic
			"-no-reboot",

			// The console=ttyS0 gets us useful debug output on x86_64
			// (in the Cloud Run service), but hides test output on aarch64.
			"-kernel", c.kernel, "-append", "panic=-1 quiet console=ttyS0",
			"-initrd", c.builder,
		)...)
		cmd.Stdout = buf
		cmd.Stderr = buf
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("qemu failed with %w: %s", err, buf.String())
		}
	}

	if b, err := os.ReadFile(filepath.Join(tmp, "result")); err != nil {
		return "", fmt.Errorf("os.ReadFile() failed with %w", err)
	} else if string(b) != "0" {
		return "", fmt.Errorf("%w: %s", ErrDiskConversion, buf.String())
	}

	return diskFilename.Name(), nil
}
