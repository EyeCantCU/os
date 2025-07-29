//go:build vmtest

package waagent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/files"
	"github.com/moby/sys/mountinfo"
)

const (
	datalossWarningFilename = "DATALOSS_WARNING_README.txt"
)

// Run this on a machine type that has local temp disk storage
func TestResourceDisk(t *testing.T) {
	resourceDiskPath, err := filepath.EvalSymlinks("/dev/disk/azure/resource")
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(/dev/disk/azure/resource) = err %v, want nil", err)
	}
	resourceDiskMountPoint := getDiskMountPoint(t, resourceDiskPath)
	datalossWarningPath := filepath.Join(resourceDiskMountPoint, datalossWarningFilename)
	if _, err := os.Stat(datalossWarningPath); err != nil {
		t.Fatalf("os.Stat(%s) = err %v, want nil\nwalinuxagent is supposed to write this file", datalossWarningPath, err)
	}
}

func getDiskMountPoint(t *testing.T, disk string) string {
	t.Helper()
	mountInfo, err := os.ReadFile("/proc/1/mountinfo")
	artifacts.File(t, files.Proc1Mountinfo, mountInfo, err, nil)
	if err != nil {
		t.Fatalf("os.ReadFile(/proc/1/mountinfo) = err %v, want nil", err)
	}
	mountFilter := func(i *mountinfo.Info) (bool, bool) {
		if strings.HasPrefix(i.Source, disk) {
			// Don't skip this entry, do stop processing any further entries
			return false, true
		}
		// Skip this entry, keep looking for more
		return true, false
	}
	mountPoints, err := mountinfo.GetMountsFromReader(bytes.NewBuffer(mountInfo), mountFilter)
	if err != nil {
		t.Fatalf("mountinfo.GetMountsFromReader(/proc/1/mountinfo) = err %v, want nil", err)
	}
	if len(mountPoints) > 0 {
		return mountPoints[0].Mountpoint
	}
	t.Fatalf("%s is not mounted", disk)
	return ""
}
