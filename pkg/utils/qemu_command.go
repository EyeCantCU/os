/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

func generateArmCommand(arch, efiDisk, ovmf, socketPath, varsPath string) []string {
	cmd := []string{
		"qemu-system-aarch64",
		"-machine", "virt",
		"-m", "4G"}
	cmd = append(cmd, getDisplayArgs()...)
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
		"-snapshot",
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
		if CanUseKVM() &&
			types.ParseArchitecture(arch).ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
			return append(cmd, []string{
				"-machine", "virt", "-cpu", "host", "-accel", "kvm",
			}...)
		}
	}

	return append(cmd, []string{"-cpu", "cortex-a53", "-accel", "tcg"}...)
}

func generateAmdCommand(arch, efiDisk, ovmf, socketPath, varsPath string) []string {
	cmd := []string{
		"qemu-system-x86_64",
		"-machine", "q35",
		"-m", "4G",
	}
	cmd = append(cmd, getDisplayArgs()...)
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
		"-snapshot",
	}...)
	// on linux, with kvm and if arches match, let's use acceleration
	if runtime.GOOS == "linux" &&
		CanUseKVM() &&
		types.ParseArchitecture(arch).ToAPK() == types.ParseArchitecture(runtime.GOARCH).ToAPK() {
		cmd = append(cmd, []string{"-cpu", "host", "-accel", "kvm"}...)
	} else {
		cmd = append(cmd, []string{"-cpu", "Haswell-v4", "-accel", "tcg"}...)
	}
	log.Printf("cmd: %v", cmd)
	return cmd
}
