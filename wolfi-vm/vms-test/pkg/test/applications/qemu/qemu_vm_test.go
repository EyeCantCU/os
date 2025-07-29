//go:build vmtest

package qemu

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/files"
	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

// TestQemuX86 runs a basic qemu-system-x86_64 VM and verifies it works
func TestQemuX86(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("TestQemuX86 requires amd64 architecture")
	}

	arch := types.ParseArchitecture("x86_64")
	if err := CheckQEMUBinary(arch); err != nil {
		t.Fatalf("QEMU not available for %s: %v", arch.ToQEmu(), err)
	}

	testQemuNative(t, arch)
}

// TestQemuAarch64 runs a basic qemu aarch64 VM and verifies it works
func TestQemuAarch64(t *testing.T) {
	arch := types.ParseArchitecture("aarch64")

	if err := CheckQEMUBinary(arch); err != nil {
		t.Fatalf("QEMU not available for %s: %v", arch.ToQEmu(), err)
	}

	// Allow emulation on x86_64 hosts - this tests QEMU's TCG emulation
	if runtime.GOARCH == "arm64" {
		// Native execution with KVM acceleration
		testQemuNative(t, arch)
	} else if runtime.GOARCH == "amd64" {
		// Emulated execution using TCG
		testQemuEmulated(t, arch)
	} else {
		t.Skipf("TestQemuAarch64 not supported on %s architecture", runtime.GOARCH)
	}
}

func testQemuNative(t *testing.T, arch types.Architecture) {
	ctx := vmtest.Context(t)

	// Check prerequisites
	if !CanUseKVM() {
		t.Fatal("KVM is not available - this is required for native testing")
	}

	// Setup test environment
	tmpDir := t.TempDir()

	// Create test disk and firmware
	diskPath, fwPath := setupVMResources(t, ctx, tmpDir, arch)

	// Configure and run VM
	config := QEMUConfig{
		Architecture: arch,
		DiskPath:     diskPath,
		FirmwarePath: fwPath,
		UseKVM:       true,
		Memory:       "256M",
		TempDir:      tmpDir,
	}

	runQEMUTest(t, ctx, config, "native KVM")
}

func testQemuEmulated(t *testing.T, arch types.Architecture) {
	ctx := vmtest.Context(t)

	// Setup test environment
	tmpDir := t.TempDir()

	// Create test disk and firmware
	diskPath, fwPath := setupVMResources(t, ctx, tmpDir, arch)

	// Configure and run VM
	config := QEMUConfig{
		Architecture: arch,
		DiskPath:     diskPath,
		FirmwarePath: fwPath,
		UseKVM:       false,
		Memory:       "256M",
		TempDir:      tmpDir,
	}

	runQEMUTest(t, ctx, config, "emulation")
}

// setupVMResources creates the disk image and finds/creates firmware
func setupVMResources(t *testing.T, ctx context.Context, tmpDir string, arch types.Architecture) (string, string) {
	// Create disk image
	filename := "test-disk-" + arch.ToQEmu() + ".raw"
	diskPath, err := CreateTestDisk(ctx, tmpDir, filename)
	if err != nil {
		t.Fatalf("failed to create test disk: %v", err)
	}

	// Get firmware path
	fwPath := GetOVMFFirmwarePath(arch)
	if _, err := os.Stat(fwPath); err != nil {
		t.Fatalf("OVMF firmware not found at %s: %v", fwPath, err)
	}

	return diskPath, fwPath
}

// runQEMUTest executes the QEMU VM test with the given configuration
func runQEMUTest(t *testing.T, ctx context.Context, config QEMUConfig, testType string) {
	// Generate QEMU command
	qemuCmd := GenerateQEMUCommand(config)
	t.Logf("Running QEMU %s test: %s", testType, qemuCmd[0])

	// Start QEMU
	cmd := exec.CommandContext(ctx, qemuCmd[0], qemuCmd[1:]...)
	cmd.Dir = config.TempDir

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start QEMU: %v", err)
	}

	// Ensure cleanup
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	// Wait for VM activity
	if err := WaitForVMActivity(ctx, config.TempDir); err != nil {
		t.Fatalf("VM failed to show activity: %v", err)
	}

	t.Logf("Successfully started %s QEMU VM with %s", config.Architecture.ToQEmu(), testType)

	// Save console log as artifact file for easier debugging
	consolePath := filepath.Join(config.TempDir, "console.log")
	logData, err := os.ReadFile(consolePath)
	cleanedLog := CleanConsoleOutput(logData)

	// Save console log as artifact file
	artifacts.File(t, files.NestedQemuConsole, cleanedLog, err, map[string]any{
		"test_type":    testType,
		"architecture": config.Architecture.ToQEmu(),
	})

	// Terminate VM gracefully
	if err := TerminateVM(ctx, config.TempDir); err != nil {
		t.Logf("Warning: failed to terminate VM gracefully: %v", err)
	}
}
