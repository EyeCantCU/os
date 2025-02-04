//go:build amd64

/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tar2efi

import (
	"runtime"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/utils"
)

const qemuCommand = "qemu-system-x86_64"

var TargetArch = types.ParseArchitecture("x86_64")

var baseQEMUArgs []string

func init() {
	args := []string{
		"-machine", "q35,accel=tcg",
		"-cpu", "Haswell-v4",
	}

	if runtime.GOOS == "linux" {
		if utils.CanUseKVM() {
			args = []string{
				"-machine", "q35", "-cpu", "max", "-accel", "kvm",
			}
		}
	}

	baseQEMUArgs = args
}
