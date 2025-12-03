/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"chainguard.dev/apko/pkg/build/types"
)

func GenerateQEMUCommand(arch, efiDisk, ovmf string) []string {
	result := []string{}

	socketPath := efiDisk + ".socket"
	varsPath := filepath.Join(filepath.Dir(efiDisk), "uefi-data.fd")

	switch types.ParseArchitecture(arch).ToAPK() {
	case "aarch64":
		result = generateArmCommand(arch, efiDisk, ovmf, socketPath, varsPath)
	case "x86_64":
		result = generateAmdCommand(arch, efiDisk, ovmf, socketPath, varsPath)
	}

	return result
}

func getDisplayArgs() []string {
	display := os.Getenv("WVM_DISPLAY")
	if display == "" {
		display = "none"
	}

	vnc := os.Getenv("WVM_VNC")
	if vnc == "" {
		vnc = "none"
	}

	return []string{"-display", display, "-vnc", vnc}
}

func getHostFwd() string {
	port := os.Getenv("WVM_SSH_PORT")
	if port == "" {
		port = "6379"
	}
	return "hostfwd=tcp:127.0.0.1:" + port + "-:22"
}

// ensureTPMRunning checks if swtpm is already running for the socket path,
// and launches it with --daemon if not.
func ensureTPMRunning(tpmSocketPath string) error {
	stateDir := filepath.Join(filepath.Dir(tpmSocketPath), "swtpm-state")
	// Check if swtpm is already running by checking for the socket
	if _, err := os.Stat(tpmSocketPath); err == nil {
		// Socket exists, assume swtpm is running
		return nil
	}

	// Create TPM state directory
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create TPM directory: %w", err)
	}

	// Launch swtpm with --daemon
	cmd := exec.Command("swtpm", "socket",
		"--tpmstate", "dir="+stateDir,
		"--ctrl", "type=unixio,path="+tpmSocketPath,
		"--tpm2",
		"--log", "level=20",
		"--daemon")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start swtpm daemon: %w", err)
	}

	// Wait a bit for the socket to be created
	for range 50 {
		if _, err := os.Stat(tpmSocketPath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("swtpm daemon started but socket not created after 5 seconds")
}

func getTPMArgs(socketPath string) []string {
	tpmSocketPath := filepath.Join(
		filepath.Dir(socketPath),
		"swtpm-sock")
	tpm := os.Getenv("WVM_TPM")
	if tpm == "" || tpm == "0" || tpm == "false" {
		return []string{}
	}

	// Ensure swtpm is running before returning args
	if err := ensureTPMRunning(tpmSocketPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to start swtpm: %v\n", err)
		return []string{}
	}

	// Use emulator mode (requires swtpm running on socket)
	args := []string{
		"-chardev", "socket,id=chrtpm,path=" + tpmSocketPath,
		"-tpmdev", "emulator,id=tpm0,chardev=chrtpm",
		"-device", "tpm-tis,tpmdev=tpm0",
	}

	return args
}

func generateArmCommand(arch, efiDisk, ovmf, socketPath, varsPath string) []string {
	cmd := []string{
		"qemu-system-aarch64",
		"-machine", "virt",
		"-m", "4G"}
	cmd = append(cmd, getDisplayArgs()...)
	cmd = append(cmd, getTPMArgs(socketPath)...)
	cmd = append(cmd, []string{
		"-serial", "mon:stdio",
		"-echr", "0x05",
		"-device", "virtio-rng-pci",
		"-drive", "if=pflash,format=raw,unit=0,file=" + ovmf + ",readonly=on",
		"-drive", "if=pflash,format=raw,unit=1,file=" + varsPath,
		"-blockdev", "driver=raw,node-name=disk-debug.raw,file.driver=file,file.filename=" + efiDisk,
		"-device", "virtio-blk-pci,drive=disk-debug.raw,serial=boot-disk,discard=true",
		"-device", "virtio-net-pci,netdev=id1",
		"-netdev", "user,id=id1," + getHostFwd(),
		"-chardev", "socket,path=" + socketPath + ",server=on,wait=off,id=debugshell",
		"-device", "pci-serial,id=serial0,chardev=debugshell",
	}...)

	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "kern.hv_support").Output()
		if err == nil && strings.Contains(string(out), "1") {
			return append(cmd, []string{
				"-cpu", "host", "-accel", "hvf",
			}...)
		}
	case "linux":
		cmd = append(cmd, "-device", fmt.Sprintf("vhost-vsock-pci,guest-cid=%d", randomCID()))
		if CanUseKVM() &&
			types.ParseArchitecture(arch).ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
			return append(cmd, []string{
				"-machine", "virt", "-cpu", "host", "-accel", "kvm",
			}...)
		}
	}

	return append(cmd, []string{"-cpu", "cortex-a53", "-accel", "tcg"}...)
}

func randomCID() uint32 {
	rand.Seed(time.Now().UnixNano())
	var cid uint32
	for {
		cid = rand.Uint32()
		if cid > 2 {
			break
		}
	}
	return cid
}

func generateAmdCommand(arch, efiDisk, ovmf, socketPath, varsPath string) []string {
	cmd := []string{
		"qemu-system-x86_64",
		"-machine", "q35",
		"-m", "4G",
	}
	cmd = append(cmd, getDisplayArgs()...)
	cmd = append(cmd, getTPMArgs(socketPath)...)
	cmd = append(cmd, []string{
		"-serial", "mon:stdio",
		"-echr", "0x05",
		"-device", "virtio-rng-pci",
		"-drive", "if=pflash,format=raw,unit=0,file=" + ovmf + ",readonly=on",
		"-drive", "if=pflash,format=raw,unit=1,file=" + varsPath,
		"-blockdev", "driver=raw,node-name=disk-debug.raw,file.driver=file,file.filename=" + efiDisk,
		"-device", "virtio-blk-pci,drive=disk-debug.raw,serial=boot-disk,discard=true",
		"-device", "virtio-net-pci,netdev=id1",
		"-netdev", "user,id=id1," + getHostFwd(),
		"-serial", "unix:" + socketPath + ",wait=off,server=on",
	}...)

	if runtime.GOOS == "linux" {
		cmd = append(cmd, "-device", fmt.Sprintf("vhost-vsock-pci,guest-cid=%d", randomCID()))
	}

	// on linux, with kvm and if arches match, let's use acceleration
	if runtime.GOOS == "linux" &&
		CanUseKVM() &&
		types.ParseArchitecture(arch).ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
		cmd = append(cmd, []string{"-cpu", "host", "-accel", "kvm"}...)
	} else {
		cmd = append(cmd, []string{"-cpu", "Haswell-v4", "-accel", "tcg"}...)
	}

	return cmd
}

// driveOptions represents a parsed drive/blockdev specification maintaining original order
type driveOptions struct {
	opts  map[string]string
	order []string
}

// parseDriveOptions parses a comma-separated drive/blockdev specification
func parseDriveOptions(spec string) *driveOptions {
	d := &driveOptions{
		opts:  make(map[string]string),
		order: []string{},
	}

	parts := strings.Split(spec, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			d.opts[kv[0]] = kv[1]
			d.order = append(d.order, kv[0])
		} else if len(kv) == 1 && kv[0] != "" {
			d.opts[kv[0]] = ""
			d.order = append(d.order, kv[0])
		}
	}

	return d
}

// Get retrieves a value by key
func (d *driveOptions) Get(key string) (string, bool) {
	val, ok := d.opts[key]
	return val, ok
}

// Set updates or adds a key-value pair, preserving order for existing keys
func (d *driveOptions) Set(key, value string) {
	if _, exists := d.opts[key]; !exists {
		d.order = append(d.order, key)
	}
	d.opts[key] = value
}

func (d *driveOptions) Drop(key string) {
	if _, exists := d.opts[key]; !exists {
		return
	}
	delete(d.opts, key)
	newOrder := []string{}
	for _, k := range d.order {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	d.order = newOrder
}

// String returns the drive specification as a comma-separated string
func (d *driveOptions) String() string {
	var parts []string
	for _, key := range d.order {
		if val, ok := d.opts[key]; ok {
			if val != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", key, val))
			} else {
				parts = append(parts, key)
			}
		}
	}
	return strings.Join(parts, ",")
}

func (d *driveOptions) isReadOnly() bool {
	if v, ok := d.Get("readonly"); ok {
		return v == "on" || v == ""
	}
	v, _ := d.Get("read-only")
	return v == "on"
}

// qcow2Creator is a function type for creating qcow2 overlay files
type qcow2Creator func(backingFile, backingFormat string) (overlayPath string, err error)

// defaultQcow2Creator creates a qcow2 file backed by the given file
func defaultQcow2Creator(backingFile, backingFormat string) (string, error) {
	// Resolve backing file to absolute path
	// qcow2 backing files are resolved relative to the qcow2 file location, not cwd
	absBackingFile, err := filepath.Abs(backingFile)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for %s: %w", backingFile, err)
	}

	// Create temporary qcow2 file
	tempFile, err := os.CreateTemp("", "qemu-snapshot-*.qcow2")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// Create qcow2 backed by original file (using absolute path)
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-F", backingFormat, "-b", absBackingFile, tempPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("qemu-img create failed: %w, output: %s", err, output)
	}

	return tempPath, nil
}

// Snapshotify modifies QEMU arguments to use temporary qcow2 snapshots for writable drives.
// It returns the modified arguments, a cleanup function to remove temporary files, and any error.
func Snapshotify(qemuArgs []string) ([]string, func() error, error) {
	return snapshotifyWithCreator(qemuArgs, defaultQcow2Creator)
}

// snapshotifyWithCreator is the internal implementation that accepts a custom qcow2Creator
func snapshotifyWithCreator(qemuArgs []string, creator qcow2Creator) ([]string, func() error, error) {
	var tempFiles []string
	result := make([]string, 0, len(qemuArgs))

	cleanup := func() error {
		var errs []error
		for _, f := range tempFiles {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("failed to remove %s: %w", f, err))
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("cleanup errors: %v", errs)
		}
		return nil
	}

	for i := 0; i < len(qemuArgs); i++ {
		arg := qemuArgs[i]

		// Check for -drive arguments
		if arg == "-drive" && i+1 < len(qemuArgs) {
			driveSpec := qemuArgs[i+1]
			result = append(result, arg)

			opts := parseDriveOptions(driveSpec)
			// Check if readonly

			if opts.isReadOnly() {
				result = append(result, driveSpec)
			} else {
				// Parse drive options

				// Add snapshot=on for writable drives
				// QEMU will handle the snapshotting internally without changing format
				opts.Set("snapshot", "on")

				// Rebuild drive specification
				result = append(result, opts.String())
			}
			i++ // Skip the next argument as we've processed it
			continue
		}

		// Check for -blockdev arguments
		if arg == "-blockdev" && i+1 < len(qemuArgs) {
			blockdevSpec := qemuArgs[i+1]
			result = append(result, arg)

			opts := parseDriveOptions(blockdevSpec)
			if opts.isReadOnly() {
				result = append(result, blockdevSpec)
			} else {
				// Parse blockdev options
				driver, hasDriver := opts.Get("driver")

				needsSnapshot := false
				var originalFile string
				var originalFormat string

				// Check various patterns for file-backed blockdevs
				if hasDriver {
					// Pattern 1: driver=file with filename=
					if driver == "file" {
						if filename, ok := opts.Get("filename"); ok && filename != "" {
							needsSnapshot = true
							originalFile = filename
							originalFormat = "raw"
						} else if ok {
							cleanup()
							return nil, nil, fmt.Errorf("Unsupported -blockdev arg %s: no 'filename'",
								blockdevSpec)
						}
					}

					// Pattern 2: driver=raw/qcow2/etc with file.filename=
					if filename, ok := opts.Get("file.filename"); ok && filename != "" {
						needsSnapshot = true
						originalFile = filename
						// Determine the original format from the driver
						originalFormat = driver
					} else if ok {
						cleanup()
						return nil, nil, fmt.Errorf("Unsupported -blockdev arg %s: no 'file.filename'",
							blockdevSpec)
					}
				}

				if needsSnapshot {
					// Create qcow2 overlay using the creator function
					tempPath, err := creator(originalFile, originalFormat)
					if err != nil {
						cleanup()
						return nil, nil, err
					}

					tempFiles = append(tempFiles, tempPath)

					// Update options based on the original pattern
					if driver == "file" {
						// For driver=file, we need to change to qcow2 format
						opts.Set("driver", "qcow2")
						opts.Set("file.driver", "file")
						opts.Set("file.filename", tempPath)
						// Remove the old filename key if it exists
						opts.Drop("filename")
					} else {
						// For driver=raw/qcow2/etc, update to qcow2
						opts.Set("driver", "qcow2")
						opts.Set("file.filename", tempPath)
					}

					result = append(result, opts.String())
				} else {
					result = append(result, blockdevSpec)
				}
			}
			i++ // Skip the next argument as we've processed it
			continue
		}

		// For all other arguments, copy as-is
		result = append(result, arg)
	}

	return result, cleanup, nil
}
