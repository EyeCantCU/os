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
	"strings"

	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/converter"
	"chainguard.dev/apkoaas/pkg/utils"
	"github.com/chainguard-dev/clog"
)

var ErrDiskConversion = errors.New("disk conversion failed")

// New creates a new converter.Interface for translating a tarball-based image
// into an EFI disk image.
func New(ctx context.Context, kernel string, kernelCmdlineAppend string, buildArch string, ic types.ImageConfiguration) (converter.Interface, error) {
	// Create the builder initrd on startup.
	f, err := os.CreateTemp("", "builder-*.cpio")
	if err != nil {
		return nil, fmt.Errorf("os.CreateTemp() failed with %w", err)
	} else if err := f.Close(); err != nil {
		return nil, fmt.Errorf("f.Close() failed with %w", err)
	}
	if err := utils.CreateCpio(ctx, f.Name(),
		build.WithImageConfiguration(ic),
		build.WithArch(types.ParseArchitecture(buildArch)),
	); err != nil {
		return nil, fmt.Errorf("utils.CreateCpio() failed with %w", err)
	}

	return NewFromCpio(ctx, f.Name(), kernel, kernelCmdlineAppend, buildArch)
}

// NewFromCpio creates a new converter.Interface for translating a tarball-based image
// into an EFI disk image.
// This one does not build a cpio, but needs one to be provided
func NewFromCpio(ctx context.Context, cpio, kernel string, kcmdlineAppend, buildArch string) (converter.Interface, error) {
	// Force Cloud Run to pull in these files at startup to avoid them creating
	// a big hit at request time.
	if _, err := os.ReadFile(kernel); err != nil {
		return nil, fmt.Errorf("can't read kernel %w", err)
	}

	return &t2e{
		kernel:              kernel,
		kernelCmdlineAppend: kcmdlineAppend,
		builder:             cpio,
		buildArch:           buildArch,
	}, nil
}

type t2e struct {
	kernel              string
	kernelCmdlineAppend string
	builder             string
	buildArch           string
}

// Check that we implement the interface
var _ converter.Interface = (*t2e)(nil)

// Cleanup implements converter.Interface
func (c *t2e) Cleanup() error {
	return os.RemoveAll(c.builder)
}

// Convert implements converter.Interface
func (c *t2e) Convert(ctx context.Context, input io.Reader, output io.Writer, fwvars io.Writer, arch types.Architecture) error {
	// Create a scratch space for ourselves.
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("os.MkdirTemp() failed with %w", err)
	}
	defer os.RemoveAll(tmp)

	diskPath := filepath.Join(tmp, "disk.raw")
	err = c.ConvertToFile(ctx, input, diskPath, arch)
	if err != nil {
		return fmt.Errorf("ConvertToFile() failed with %w", err)
	}

	f, err := os.Open(diskPath)
	if err != nil {
		return fmt.Errorf("os.Open() failed with %w", err)
	}
	if _, err := io.Copy(output, f); err != nil {
		return fmt.Errorf("io.Copy() failed with %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("f.Close() failed with %w", err)
	}

	fwPath := filepath.Join(tmp, "uefi-data.fd")
	f, err = os.Open(fwPath)
	if err != nil {
		return fmt.Errorf("os.Open() on %s failed with %w", fwPath, err)
	}
	if _, err := io.Copy(fwvars, f); err != nil {
		return fmt.Errorf("io.Copy() failed with %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("f.Close() failed with %w", err)
	}

	return nil
}

// ConvertToFile implements converter.Interface
func (c *t2e) ConvertToFile(ctx context.Context, input io.Reader, output string, arch types.Architecture) error {
	// Write the uncompressed filesystem into a tarball for us to mount.
	var size int64
	outd := filepath.Dir(output)

	tmpd, err := os.MkdirTemp(outd, "")
	if err != nil {
		return fmt.Errorf("os.MkdirTemp() failed with %w", err)
	}
	defer os.RemoveAll(tmpd)

	imageTarball, err := os.Create(filepath.Join(tmpd, "image.tar"))
	if err != nil {
		return fmt.Errorf("os.Create() failed with %w", err)
	}
	if size, err = io.Copy(imageTarball, input); err != nil {
		return fmt.Errorf("io.Copy() of input tar failed with %w", err)
	}
	if err := imageTarball.Close(); err != nil {
		return fmt.Errorf("f.Close() failed with %w", err)
	}

	// Most CSPs require disks to be in 1GB increments.  Compute the size of the
	// disk we need by doubling the size of the tarball we received and then
	// rounding that up to the nearest GB.
	const oneGB int64 = 1024 * 1024 * 1024
	numGB := (2*size + oneGB - 1) / oneGB
	// we have a 1Gb EFI partition, let's ensure we're counting it.
	// refers to iac/builder.yaml line 77
	numGB++

	// Create a raw disk with the appropriate size, which we will mount as a
	// block device and write the converted image into.
	// TODO(mattmoor): We should experiment with writing the GPT table in Go
	// vs. with sfdisk.  smoser has used this in the past:
	//     https://pkg.go.dev/github.com/rekby/gpt
	diskFileName := filepath.Join(tmpd, "disk.raw")
	diskFile, err := os.Create(diskFileName)
	if err != nil {
		return fmt.Errorf("os.Create() failed with %w", err)
	} else if err := diskFile.Truncate(numGB * oneGB); err != nil {
		return fmt.Errorf("f.Truncate() failed with %w", err)
	} else if err := diskFile.Close(); err != nil {
		return fmt.Errorf("f.Close() failed with %w", err)
	}

	diskFile.Close()

	scratchDiskFileName := filepath.Join(tmpd, "scratch.raw")
	scratchDiskFile, err := os.Create(scratchDiskFileName)
	if err != nil {
		return fmt.Errorf("os.Create() failed with %w", err)
	} else if err := scratchDiskFile.Truncate(numGB * oneGB); err != nil {
		return fmt.Errorf("f.Truncate() failed with %w", err)
	} else if err := scratchDiskFile.Close(); err != nil {
		return fmt.Errorf("f.Close() failed with %w", err)
	}
	scratchDiskFile.Close()
	defer os.RemoveAll(scratchDiskFileName)

	workDir := filepath.Join(tmpd, "workDir")
	if err := os.Mkdir(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create results tmpdir")
	}

	if err := os.WriteFile(filepath.Join(workDir, "result"), []byte("1"), 0o600); err != nil {
		return fmt.Errorf("os.WriteFile() failed with %w", err)
	}

	// write to /output/buildArch what arch we're building for, x86 or arm
	// this allows to cross build between architectures
	// in the builder there are some stuff that needs to be different between
	// arm and amd64:
	//	- root partition UUID type as specified by: https://uapi-group.org/specifications/specs/discoverable_partitions_specification
	if err := os.WriteFile(filepath.Join(workDir, "buildArch"), []byte(arch.ToAPK()), 0o600); err != nil {
		return fmt.Errorf("os.WriteFile() failed with %w", err)
	}

	q := QemuInfo[types.ParseArchitecture(c.buildArch)]
	// Convert the image to a raw disk image.
	buf := bytes.NewBuffer(nil)
	kcmdline := "panic=-1 quiet console=" + q.Console
	if c.kernelCmdlineAppend != "" {
		kcmdline += " " + c.kernelCmdlineAppend
	}
	{
		// nolint:gosec // We trust the kernel argument here.
		cmd := exec.CommandContext(ctx, q.Command, append(slices.Clone(q.MachineArgs),
			"-m", "4G",
			"-nographic",

			// Disable networking.
			"-nic", "none",

			"-device", "virtio-9p-pci,id=fs101,fsdev=fsdev101,mount_tag=output",
			"-fsdev", "local,multidevs=remap,security_model=mapped,id=fsdev101,path="+workDir,

			"-device", "virtio-blk-pci,drive=image.tar,serial=input-tar,discard=true",
			"-blockdev", "driver=raw,node-name=image.tar,file.driver=file,file.filename="+imageTarball.Name(),

			"-device", "virtio-blk-pci,drive=disk.raw.tmp,serial=install-target-disk,discard=true",
			"-blockdev", "driver=raw,node-name=disk.raw.tmp,file.driver=file,file.filename="+diskFileName,

			"-device", "virtio-blk-pci,drive=scratch.raw.tmp,serial=scratch-disk,discard=true",
			"-blockdev", "driver=raw,node-name=scratch.raw.tmp,file.driver=file,file.filename="+scratchDiskFileName,

			// Don't reboot on a kernel panic
			"-no-reboot",

			// The console=ttyS0 gets us useful debug output on x86_64
			// (in the Cloud Run service), but hides test output on aarch64.
			"-kernel", c.kernel,
			"-append", kcmdline,
			"-initrd", c.builder,
		)...)

		clog.Info("launching disk conversion")
		clog.Debugf("executing %s\n", strings.Join(cmd.Args, " "))
		cmd.Stdout = buf
		cmd.Stderr = buf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("qemu failed with %w: %s", err, buf.String())
		}

		clog.Debug(buf.String())
	}

	outDir := filepath.Dir(output)
	if err := os.Rename(filepath.Join(workDir, "install.log"), filepath.Join(outDir, "install.log")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to save install.log to %s: %w", outDir, err)
	}

	if b, err := os.ReadFile(filepath.Join(workDir, "result")); err != nil {
		return fmt.Errorf("os.ReadFile() failed with %w", err)
	} else {
		rc := strings.TrimSpace(string(b))
		if rc != "0" {
			return fmt.Errorf("%w: result was '%s'\n%s\n", ErrDiskConversion, rc, buf.String())
		}
	}

	// Secureboot variables side-effect
	if sbdir, err := os.Stat(filepath.Join(workDir, "secureboot")); err == nil && sbdir.IsDir() {
		efifiles := []string{
			// For new UEFI vars tools
			"uefi-data.json",
			// For AWS registration
			"uefi-data.aws",
			// For OVMF/AAMVF vars template
			"uefi-data.fd",
			"uefi-data.empty.fd",
			// For enrollment in GCP / firmware / redfish / bmc
			"PK.auth.bin", "KEK.auth.bin", "dbx.auth.bin", "db.auth.bin",
			// List of DB hashes for Azure
			"db-b64-hashes.txt",
		}
		for _, efi := range efifiles {
			efiFilePath := filepath.Join(workDir, "secureboot", efi)
			efiFileOutput := filepath.Join(filepath.Dir(output), efi)
			if err := os.Rename(efiFilePath, efiFileOutput); err != nil {
				return fmt.Errorf("failed to rename EFI vars into %s: %w", output, err)
			}
			clog.Infof("writting UEFI variables %s", efiFileOutput)
		}
	}

	if err := os.Rename(diskFileName, output); err != nil {
		return fmt.Errorf("failed to rename disk into %s: %w", output, err)
	}

	return nil
}
