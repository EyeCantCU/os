/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import "os"

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
