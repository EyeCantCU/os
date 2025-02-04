/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"os/exec"
	"runtime"
	"strings"

	"chainguard.dev/apko/pkg/build/types"
)

func GenerateQEMUCommand(arch, efiDisk, ovmf string) []string {
	result := []string{}

	socketPath := efiDisk + ".socket"

	switch types.ParseArchitecture(arch).ToAPK() {
	case "aarch64":
		result = generateArmCommand(arch, efiDisk, ovmf, socketPath)
	case "x86_64":
		result = generateAmdCommand(arch, efiDisk, ovmf, socketPath)
	}

	return result
}

func generateArmCommand(arch, efiDisk, ovmf, socketPath string) []string {
	qemuArmCommand := []string{
		"qemu-system-aarch64",
		"-machine", "virt",
		"-m", "4G",
		"-display", "none",
		"-serial", "mon:stdio",
		"-echr", "0x05",
		"-device", "virtio-rng-pci",
		"-drive", "if=pflash,format=raw,file=" + ovmf + ",readonly=on",
		"-blockdev", "driver=raw,node-name=disk-debug.raw,file.driver=file,file.filename=" + efiDisk,
		"-device", "virtio-blk-pci,drive=disk-debug.raw,serial=boot-disk,discard=true",
		"-device", "virtio-net-pci,netdev=id1",
		"-netdev", "user,id=id1,hostfwd=tcp:127.0.0.1:6379-:6379",
		"-chardev", "socket,path=" + socketPath + ",server=on,wait=off,id=debugshell",
		"-device", "pci-serial,id=serial0,chardev=debugshell",
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
			types.ParseArchitecture(arch).ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
			return append(qemuArmCommand, []string{
				"-machine", "virt", "-cpu", "host", "-accel", "kvm",
			}...)
		}
	}

	return append(qemuArmCommand, []string{"-cpu", "cortex-a53", "-accel", "tcg"}...)
}

func generateAmdCommand(arch, efiDisk, ovmf, socketPath string) []string {
	qemuAmdCommand := []string{
		"qemu-system-x86_64",
		"-machine", "q35",
		"-m", "4G",
		"-display", "none",
		"-serial", "mon:stdio",
		"-echr", "0x05",
		"-device", "virtio-rng-pci",
		"-drive", "if=pflash,format=raw,file=" + ovmf + ",readonly=on",
		"-blockdev", "driver=raw,node-name=disk-debug.raw,file.driver=file,file.filename=" + efiDisk,
		"-device", "virtio-blk-pci,drive=disk-debug.raw,serial=boot-disk,discard=true",
		"-device", "virtio-net-pci,netdev=id1",
		"-netdev", "user,id=id1,hostfwd=tcp:127.0.0.1:6379-:6379",
		"-serial", "unix:" + socketPath + ",wait=off,server=on",
		"-snapshot",
	}
	// on linux, with kvm and if arches match, let's use acceleration
	if runtime.GOOS == "linux" &&
		CanUseKVM() &&
		types.ParseArchitecture(arch).ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
		qemuAmdCommand = append(qemuAmdCommand, []string{"-cpu", "host", "-accel", "kvm"}...)
	} else {
		qemuAmdCommand = append(qemuAmdCommand, []string{"-cpu", "Haswell-v4", "-accel", "tcg"}...)
	}
	return qemuAmdCommand
}
