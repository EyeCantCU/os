//go:build unittest

package qemu

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"chainguard.dev/apko/pkg/build/types"
	"github.com/google/go-cmp/cmp"
)

func TestCreateTestDisk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "qemu-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	filename := "test-disk.raw"

	diskPath, err := CreateTestDisk(ctx, tmpDir, filename)
	if err != nil {
		t.Fatalf("CreateTestDisk failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, filename)
	if diskPath != expectedPath {
		t.Errorf("CreateTestDisk returned path %q, want %q", diskPath, expectedPath)
	}

	// Check that file exists and has expected size (1MB)
	stat, err := os.Stat(diskPath)
	if err != nil {
		t.Fatalf("failed to stat created disk: %v", err)
	}

	expectedSize := int64(1024 * 1024) // 1MB
	if stat.Size() != expectedSize {
		t.Errorf("disk size = %d, want %d", stat.Size(), expectedSize)
	}

	// Check that it's all zeros (blank disk)
	data, err := os.ReadFile(diskPath)
	if err != nil {
		t.Fatalf("failed to read disk: %v", err)
	}

	for i, b := range data {
		if b != 0 {
			t.Errorf("disk not blank: byte at offset %d is %d, want 0", i, b)
			break
		}
	}
}

func TestHasVMActivity(t *testing.T) {
	tests := []struct {
		name   string
		logStr string
		want   bool
	}{
		{
			name:   "EFI activity",
			logStr: "EFI v2.70 (EDK II, 0x00010000)",
			want:   true,
		},
		{
			name:   "UEFI activity",
			logStr: "UEFI Firmware v2.70",
			want:   true,
		},
		{
			name:   "EDK activity",
			logStr: "EDK II Firmware v2.70",
			want:   true,
		},
		{
			name:   "TianoCore activity",
			logStr: "TianoCore EDK II",
			want:   true,
		},
		{
			name:   "substantial output",
			logStr: "This is a long log output that contains more than 100 characters to simulate substantial VM activity even without specific keywords",
			want:   true,
		},
		{
			name:   "short output without keywords",
			logStr: "short log",
			want:   false,
		},
		{
			name:   "empty log",
			logStr: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasVMActivity(tt.logStr); got != tt.want {
				t.Errorf("hasVMActivity(%q) = %v, want %v", tt.logStr, got, tt.want)
			}
		})
	}
}

func TestGenerateQEMUCommand(t *testing.T) {
	tests := []struct {
		name   string
		config QEMUConfig
		want   []string
	}{
		{
			name: "x86_64 with KVM",
			config: QEMUConfig{
				Architecture: types.ParseArchitecture("x86_64"),
				DiskPath:     "/tmp/disk.raw",
				FirmwarePath: "/tmp/fw.fd",
				UseKVM:       true,
				Memory:       "512M",
				TempDir:      "/tmp/test",
				VhostCID:     3,
			},
			want: []string{
				"qemu-system-x86_64", "-machine", "q35",
				"-cpu", "host", "-accel", "kvm",
				"-m", "512M", "-nographic",
				"-serial", "file:/tmp/test/console.log",
				"-monitor", "unix:/tmp/test/monitor.sock,server,nowait",
				"-device", "virtio-rng-pci",
				"-drive", "if=pflash,format=raw,file=/tmp/fw.fd,readonly=on",
				"-drive", "if=virtio,format=raw,file=/tmp/disk.raw",
				"-netdev", "user,id=net0",
				"-device", "virtio-net,netdev=net0",
				"-device", "vhost-vsock-pci,guest-cid=3",
			},
		},
		{
			name: "aarch64 with emulation",
			config: QEMUConfig{
				Architecture: types.ParseArchitecture("aarch64"),
				DiskPath:     "/tmp/disk.raw",
				FirmwarePath: "/tmp/fw.fd",
				UseKVM:       false,
				Memory:       "256M",
				TempDir:      "/tmp/test",
				VhostCID:     -1,
			},
			want: []string{
				"qemu-system-aarch64", "-machine", "virt",
				"-cpu", "cortex-a57", "-accel", "tcg",
				"-m", "256M", "-nographic",
				"-serial", "file:/tmp/test/console.log",
				"-monitor", "unix:/tmp/test/monitor.sock,server,nowait",
				"-device", "virtio-rng-pci",
				"-drive", "if=pflash,format=raw,file=/tmp/fw.fd,readonly=on",
				"-drive", "if=virtio,format=raw,file=/tmp/disk.raw",
				"-netdev", "user,id=net0",
				"-device", "virtio-net,netdev=net0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateQEMUCommand(tt.config)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GenerateQEMUCommand() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCleanConsoleOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "simple text without control characters",
			input: "Hello World\nSecond line",
			want:  "Hello World\nSecond line",
		},
		{
			name:  "ANSI clear screen sequences",
			input: "\x1b[2J\x1b[01;01H\x1b[=3hHello World\x1b[2J\x1b[01;01H",
			want:  "Hello World",
		},
		{
			name:  "carriage returns",
			input: "Line one\r\nLine two\r\nLine three",
			want:  "Line one\nLine two\nLine three",
		},
		{
			name:  "mixed control characters and real content",
			input: "\x1b[2J\x1b[01;01H\x1b[=3hBdsDxe: failed to load Boot0001\r\n\x1b[2J\x1b[01;01H>>Start PXE over IPv4.\r\n",
			want:  "BdsDxe: failed to load Boot0001\n>>Start PXE over IPv4.",
		},
		{
			name:  "multiple empty lines",
			input: "Line 1\n\n\n\nLine 2\n\n",
			want:  "Line 1\nLine 2",
		},
		{
			name:  "whitespace-only lines",
			input: "Line 1\n   \n\t\nLine 2\n  \t  \n",
			want:  "Line 1\nLine 2",
		},
		{
			name:  "real QEMU aarch64 output",
			input: "UEFI firmware (version edk2-stable202408-prebuilt.qemu.org built at 16:28:50 on Sep 12 2024)\r\nArmTrngLib could not be correctly initialized.\r\n\x1b[2J\x1b[01;01H\x1b[=3h\x1b[2J\x1b[01;01HCheckCrc32: Crc check failed\r\n",
			want:  "UEFI firmware (version edk2-stable202408-prebuilt.qemu.org built at 16:28:50 on Sep 12 2024)\nArmTrngLib could not be correctly initialized.\nCheckCrc32: Crc check failed",
		},
		{
			name:  "only control characters",
			input: "\x1b[2J\x1b[01;01H\x1b[=3h\r\n\r\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanConsoleOutput([]byte(tt.input))
			if string(got) != tt.want {
				t.Errorf("CleanConsoleOutput() = %q, want %q", string(got), tt.want)
			}
		})
	}
}
