/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"chainguard.dev/apko/pkg/build/types"
)

func GetCommand(arch types.Architecture) []string {
	f := generateAmdCommand

	switch arch.ToQEmu() {
	case "aarch64":
		f = generateArmCommand
	case "x86_64":
		f = generateAmdCommand
	default:
		return []string{}
	}

	return f(arch, "disk.raw", "fw-code.fd")
}

func generateArmCommand(arch types.Architecture, efiDisk, fwcode string) []string {
	qemuArmCommand := []string{
		"qemu-system-aarch64",
		"-nodefaults",
		"-machine", "virt",
		"-m", "4G",
		"-display", "none",
		"-chardev", "file,id=serial0,path=ttyS0.log",
		"-chardev", "socket,id=serial1,path=ttyS1.sock,server=on,wait=off",
		"-chardev", "socket,id=monitor0,path=hmp.sock,server=on,wait=off",
		"-chardev", "socket,id=qmp0,path=qmp.sock,server=on,wait=off",
		"-serial", "chardev:serial0",
		"-serial", "chardev:serial1",
		"-monitor", "chardev:monitor0",
		"-qmp", "chardev:qmp0",
		"-device", "virtio-rng-pci",
		"-drive", "if=pflash,format=raw,file=" + fwcode + ",readonly=on",
		"-blockdev", "driver=raw,node-name=disk-debug.raw,file.driver=file,file.filename=" + efiDisk,
		"-device", "virtio-blk-pci,drive=disk-debug.raw,serial=boot-disk,discard=true",
		"-device", "virtio-net-pci,netdev=id1",
		"-netdev", "user,id=id1,hostfwd=tcp:127.0.0.1:0-:22",
		"-snapshot",
	}

	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "kern.hv_support").Output()
		if err == nil && strings.Contains(string(out), "1") {
			return append(qemuArmCommand, []string{
				"-cpu", "host", "-accel", "hvf",
			}...)
		}
	case "linux":
		if CanUseKVM() &&
			arch.ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
			return append(qemuArmCommand, []string{
				"-machine", "virt", "-cpu", "host", "-accel", "kvm",
			}...)
		}
	}

	return append(qemuArmCommand, []string{"-cpu", "cortex-a53", "-accel", "tcg"}...)
}

func generateAmdCommand(arch types.Architecture, efiDisk, fwcode string) []string {
	qemuAmdCommand := []string{
		"qemu-system-x86_64",
		"-nodefaults",
		"-machine", "q35",
		"-m", "4G",
		"-display", "none",
		"-chardev", "file,id=serial0,path=ttyS0.log",
		"-chardev", "socket,id=serial1,path=ttyS1.sock,server=on,wait=off",
		"-chardev", "socket,id=monitor0,path=hmp.sock,server=on,wait=off",
		"-chardev", "socket,id=qmp0,path=qmp.sock,server=on,wait=off",
		"-serial", "chardev:serial0",
		"-serial", "chardev:serial1",
		"-monitor", "chardev:monitor0",
		"-qmp", "chardev:qmp0",
		"-device", "virtio-rng-pci",
		"-drive", "if=pflash,format=raw,file=" + fwcode + ",readonly=on",
		"-blockdev", "driver=raw,node-name=disk-debug.raw,file.driver=file,file.filename=" + efiDisk,
		"-device", "virtio-blk-pci,drive=disk-debug.raw,serial=boot-disk,discard=true",
		"-device", "virtio-net-pci,netdev=id1",
		"-netdev", "user,id=id1,hostfwd=tcp:127.0.0.1:0-:22",
		"-snapshot",
	}
	// on linux, with kvm and if arches match, let's use acceleration
	if runtime.GOOS == "linux" &&
		CanUseKVM() &&
		arch.ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
		qemuAmdCommand = append(qemuAmdCommand, []string{"-cpu", "host", "-accel", "kvm"}...)
	} else {
		qemuAmdCommand = append(qemuAmdCommand, []string{"-cpu", "Haswell-v4", "-accel", "tcg"}...)
	}
	return qemuAmdCommand
}

func CanUseKVM() bool {
	file, err := os.OpenFile("/dev/kvm", os.O_WRONLY, 0o644)
	if err != nil {
		// it did not exist, no permission to open, or something else
		return false
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		// probably should not happen. we had an open file handle but couldn't stat
		return false
	}

	if fileInfo.Mode()&os.ModeCharDevice == 0 {
		// /dev/kvm existed and we could write to it, but it is not a char device
		return false
	}

	return true
}
