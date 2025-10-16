package qemu

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util"
)

// QEMUConfig holds configuration for QEMU VM execution
type QEMUConfig struct {
	Architecture types.Architecture
	DiskPath     string
	FirmwarePath string
	UseKVM       bool
	Memory       string
	TempDir      string
}

// CreateTestDisk creates a minimal blank disk image for QEMU testing
func CreateTestDisk(ctx context.Context, workDir string, filename string) (string, error) {
	diskPath := filepath.Join(workDir, filename)

	// Create a small blank disk image (1MB is sufficient for testing)
	if err := exec.CommandContext(ctx, "dd", "if=/dev/zero", "of="+diskPath,
		"bs=1M", "count=1").Run(); err != nil {
		return "", fmt.Errorf("failed to create test disk: %v", err)
	}

	return diskPath, nil
}

// GetOVMFFirmwarePath returns the standard OVMF firmware path for the given architecture
func GetOVMFFirmwarePath(arch types.Architecture) string {
	return fmt.Sprintf("/usr/share/qemu/edk2-%s-code.fd", arch.ToQEmu())
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

// GenerateQEMUCommand generates a QEMU command line for the given configuration
func GenerateQEMUCommand(config QEMUConfig) []string {
	consolePath := filepath.Join(config.TempDir, "console.log")
	monitorPath := filepath.Join(config.TempDir, "monitor.sock")
	var cmd []string

	// Base command with architecture-specific settings
	if config.Architecture.ToQEmu() == "aarch64" {
		cmd = []string{
			"qemu-system-aarch64",
			"-machine", "virt",
		}
		if config.UseKVM {
			cmd = append(cmd, "-cpu", "host", "-accel", "kvm")
		} else {
			cmd = append(cmd, "-cpu", "cortex-a57", "-accel", "tcg")
		}
	} else {
		cmd = []string{
			"qemu-system-x86_64",
			"-machine", "q35",
		}
		if config.UseKVM {
			cmd = append(cmd, "-cpu", "host", "-accel", "kvm")
		} else {
			cmd = append(cmd, "-cpu", "qemu64", "-accel", "tcg")
		}
	}

	// Common settings
	cmd = append(cmd,
		"-m", config.Memory,
		"-nographic",
		"-serial", "file:"+consolePath,
		"-monitor", "unix:"+monitorPath+",server,nowait",
		"-device", "virtio-rng-pci",
		"-device", fmt.Sprintf("vhost-vsock-pci,guest-cid=%d", randomCID()),
		"-drive", "if=pflash,format=raw,file="+config.FirmwarePath+",readonly=on",
		"-drive", "if=virtio,format=raw,file="+config.DiskPath,
		"-netdev", "user,id=net0",
		"-device", "virtio-net,netdev=net0",
	)

	return cmd
}

// WaitForVMActivity waits for signs of VM activity in the console log
func WaitForVMActivity(ctx context.Context, workDir string) error {
	consolePath := filepath.Join(workDir, "console.log")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Check if we got any useful output before timing out
			if logData, err := os.ReadFile(consolePath); err == nil {
				if hasVMActivity(string(logData)) {
					return nil
				}
			}
			return ctx.Err()
		case <-ticker.C:
			if logData, err := os.ReadFile(consolePath); err == nil {
				if hasVMActivity(string(logData)) {
					return nil
				}
			}
		}
	}
}

// hasVMActivity checks for any VM activity (firmware loading, etc.)
func hasVMActivity(logStr string) bool {
	activityPatterns := []string{
		"EFI", "UEFI", "EDK", "TianoCore",
	}

	for _, pattern := range activityPatterns {
		if strings.Contains(logStr, pattern) {
			return true
		}
	}

	// Any substantial output suggests the VM is working
	return len(logStr) > 100
}

// TerminateVM gracefully terminates a running QEMU VM
func TerminateVM(ctx context.Context, workDir string) error {
	monitorPath := filepath.Join(workDir, "monitor.sock")

	// Try to connect to monitor socket with timeout
	timeout := 5 * time.Second
	conn, err := util.PollUnixSocket(ctx, monitorPath, timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to monitor socket: %v", err)
	}
	defer conn.Close()

	// Send quit command
	_, err = conn.Write([]byte("quit\n"))
	return err
}

// CanUseKVM checks if KVM acceleration is available
func CanUseKVM() bool {
	return util.CanUseKVM()
}

// CheckQEMUBinary verifies that the required QEMU binary is available
func CheckQEMUBinary(arch types.Architecture) error {
	qemuBinary := fmt.Sprintf("qemu-system-%s", arch.ToQEmu())
	if _, err := exec.LookPath(qemuBinary); err != nil {
		return fmt.Errorf("%s not found in PATH", qemuBinary)
	}
	return nil
}

// CleanConsoleOutput removes ANSI escape sequences and cleans up console output
func CleanConsoleOutput(raw []byte) []byte {
	if raw == nil {
		return nil
	}

	// Convert to string for processing
	rawStr := string(raw)

	// Remove ANSI escape sequences (including colors, cursor movements, etc.)
	cleaned := strings.ReplaceAll(rawStr, "\x1b[2J", "")     // Clear screen
	cleaned = strings.ReplaceAll(cleaned, "\x1b[01;01H", "") // Move cursor to 1,1
	cleaned = strings.ReplaceAll(cleaned, "\x1b[=3h", "")    // Set mode

	// Remove other common control characters
	cleaned = strings.ReplaceAll(cleaned, "\r", "") // Carriage returns

	// Split into lines and filter out empty ones
	lines := strings.Split(cleaned, "\n")
	var filteredLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			filteredLines = append(filteredLines, trimmed)
		}
	}

	return []byte(strings.Join(filteredLines, "\n"))
}
