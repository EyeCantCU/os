//go:build unittest

package selinux

import (
	"strings"
	"testing"
)

func TestParseSELinuxStatusDisabled(t *testing.T) {
	input := `SELinux status:                 disabled`

	status, err := ParseSELinuxStatus(input)
	if err != nil {
		t.Fatalf("ParseSELinuxStatus failed: %v", err)
	}

	if status.Status != StatusDisabled {
		t.Errorf("Status = %q, want %q", status.Status, StatusDisabled)
	}

	// When disabled, other fields should be empty/zero
	if status.CurrentMode != Unknown {
		t.Errorf("CurrentMode = %q, want %q (disabled systems have no mode)", status.CurrentMode, Unknown)
	}
	if status.MaxKernelPolicyVersion != 0 {
		t.Errorf("MaxKernelPolicyVersion = %d, want 0 (disabled systems have no version)", status.MaxKernelPolicyVersion)
	}
}

func TestParseSELinuxStatusEnabled(t *testing.T) {
	input := `SELinux status:                 enabled
SELinuxfs mount:                /sys/fs/selinux
SELinux root directory:         /etc/selinux
Loaded policy name:             targeted
Current mode:                   enforcing
Mode from config file:          enforcing
Policy MLS status:              enabled
Policy deny_unknown status:     allowed
Memory protection checking:     actual (secure)
Max kernel policy version:      35`

	status, err := ParseSELinuxStatus(input)
	if err != nil {
		t.Fatalf("ParseSELinuxStatus failed: %v", err)
	}

	if status.Status != StatusEnabled {
		t.Errorf("Status = %q, want %q", status.Status, StatusEnabled)
	}
	if status.SELinuxFSMount != "/sys/fs/selinux" {
		t.Errorf("SELinuxFSMount = %q, want %q", status.SELinuxFSMount, "/sys/fs/selinux")
	}
	if status.RootDirectory != "/etc/selinux" {
		t.Errorf("RootDirectory = %q, want %q", status.RootDirectory, "/etc/selinux")
	}
	if status.LoadedPolicyName != "targeted" {
		t.Errorf("LoadedPolicyName = %q, want %q", status.LoadedPolicyName, "targeted")
	}
	if status.CurrentMode != ModeEnforcing {
		t.Errorf("CurrentMode = %q, want %q", status.CurrentMode, ModeEnforcing)
	}
	if status.ModeFromConfigFile != ModeEnforcing {
		t.Errorf("ModeFromConfigFile = %q, want %q", status.ModeFromConfigFile, ModeEnforcing)
	}
	if status.PolicyMLSStatus != StatusEnabled {
		t.Errorf("PolicyMLSStatus = %q, want %q", status.PolicyMLSStatus, StatusEnabled)
	}
	if status.PolicyDenyUnknownStatus != DenyUnknownAllowed {
		t.Errorf("PolicyDenyUnknownStatus = %q, want %q", status.PolicyDenyUnknownStatus, DenyUnknownAllowed)
	}
	if status.MemoryProtectionChecking != MemoryProtectionActual {
		t.Errorf("MemoryProtectionChecking = %q, want %q", status.MemoryProtectionChecking, MemoryProtectionActual)
	}
	if status.MaxKernelPolicyVersion != 35 {
		t.Errorf("MaxKernelPolicyVersion = %d, want %d", status.MaxKernelPolicyVersion, 35)
	}
}

func TestParseSELinuxStatusEmpty(t *testing.T) {
	input := ``

	_, err := ParseSELinuxStatus(input)
	if err == nil {
		t.Error("ParseSELinuxStatus should have failed with empty input")
	}
}

func TestParseSELinuxStatusInvalidVersion(t *testing.T) {
	input := `SELinux status:                 enabled
Max kernel policy version:      not-a-number`

	_, err := ParseSELinuxStatus(input)
	if err == nil {
		t.Error("ParseSELinuxStatus should have failed with invalid version")
	}
}

func TestExtractAVCDenialsWithHappyJournal(t *testing.T) {
	journal := `Nov 11 12:16:21 chainguard kernel: Linux version 6.17.7-r1-gcp-generic (noreply@chainguard.dev) (gcc (Wolfi 15.2.0-r3) 15.2.0, GNU ld (GNU Binutils) 2.45) #Chainguard SMP PREEMPT_DYNAMIC Mon Nov  3 17:31:05 UTC 2025
Nov 11 12:16:21 chainguard kernel: Command line: root=PARTLABEL=Chainguard console=ttyS0 rw
Nov 11 12:16:21 chainguard kernel: BIOS-provided physical RAM map:
Nov 11 12:16:21 chainguard kernel: BIOS-e820: [mem 0x0000000000000000-0x0000000000000fff] reserved
Nov 11 12:16:21 chainguard kernel: BIOS-e820: [mem 0x0000000000001000-0x0000000000054fff] usable
Nov 11 15:20:33 linky-work-v2 sshd-session[35551]: Failed password for root from 59.56.73.141 port 45216 ssh2
Nov 11 15:20:35 linky-work-v2 sshd-session[35551]: error: maximum authentication attempts exceeded for root from 59.56.73.141 port 45216 ssh2 [preauth]
Nov 11 15:20:35 linky-work-v2 sshd-session[35551]: Disconnecting authenticating user root 59.56.73.141 port 45216: Too many authentication failures [preauth]
Nov 11 15:20:35 linky-work-v2 sshd-session[35551]: PAM 2 more authentication failures; logname= uid=0 euid=0 tty=ssh ruser= rhost=59.56.73.141  user=root
Nov 11 15:20:36 linky-work-v2 sudo[35584]: linky_chainguard_dev : TTY=pts/0 ; PWD=/home/linky_chainguard_dev ; USER=root ; COMMAND=/usr/sbin/journalctl -b0
`
	entries := strings.Split(journal, "\n")

	avcDenials := ExtractAVCDenials(entries)
	if numDenials := len(avcDenials); numDenials > 0 {
		t.Errorf("ExtractAVCDenials found %d denials but expected 0", numDenials)
	}
}

func TestExtractAVCDenialsWithDenials(t *testing.T) {
	journal := `Nov 11 15:28:27 chainguard systemd[1]: Finished Create System Users.
Nov 11 15:28:27 chainguard audit[208]: AVC avc:  denied  { rename } for  pid=208 comm="passwd" name="shadow+" dev="vda2" ino=1179804 scontext=system_u:system_r:init_t:s0 tcontext=system_u:object_r:shadow_t:s0 tclass=file permissive=1
Nov 11 15:28:27 chainguard audit[208]: AVC avc:  denied  { unlink } for  pid=208 comm="passwd" name="shadow" dev="vda2" ino=1518025 scontext=system_u:system_r:init_t:s0 tcontext=system_u:object_r:shadow_t:s0 tclass=file permissive=1
Nov 11 15:28:27 chainguard audit[208]: SYSCALL arch=c000003e syscall=82 success=yes exit=0 a0=7ffed0432260 a1=5606b2d2f1a0 a2=7ffed04321d0 a3=100 items=2 ppid=1 pid=208 auid=4294967295 uid=0 gid=0 euid=0 suid=0 fsuid=0 egid=0 sgid=0 fsgid=0 tty=(none) ses=4294967295 comm="passwd" exe="/usr/bin/passwd" subj=system_u:system_r:init_t:s0 key=(null)
Nov 11 15:28:27 chainguard audit: CWD cwd="/"
`
	entries := strings.Split(journal, "\n")

	avcDenials := ExtractAVCDenials(entries)
	if numDenials := len(avcDenials); numDenials != 2 {
		t.Errorf("ExtractAVCDenials found %d denials but expected 2", numDenials)
	}
}

func TestExtractAuditSELinuxErrors(t *testing.T) {
	journal := `Nov 21 04:38:56 i-0b7654899732b4b0b kernel: audit: type=1131 audit(1763699936.037:926): pid=1 uid=0 auid=4294967295 ses=4294967295 subj=system_u:system_r:init_t:s0 msg='unit=refresh-policy-routes@ens5 comm="systemd" exe="/usr/lib/systemd/systemd" hostname=? addr=? terminal=? res=success'
Nov 21 04:39:41 i-0b7654899732b4b0b audit: SELINUX_ERR op=security_compute_sid invalid_context="system_u:system_r:cloud_ssh_keys_fetcher_t:s0-s0:c0.c1023" scontext=system_u:system_r:sshd_t:s0-s0:c0.c1023 tcontext=system_u:object_r:cloud_ssh_keys_fetcher_exec_t:s0 tclass=process
Nov 21 04:39:41 i-0b7654899732b4b0b sshd-session[19147]: error: AuthorizedKeysCommand execve "/usr/bin/cloud-ssh-keys-fetcher ec2-user SHA256:leHZrfnE5RsCx+v0AdIzWULtwYe0FsI/AIQ+0dpBGZ0": Permission denied
`
	entries := strings.Split(journal, "\n")

	selinuxErrors := ExtractAuditSELinuxErrors(entries)
	if numErrors := len(selinuxErrors); numErrors != 1 {
		t.Errorf("ExtractAuditSELinuxErrors found %d errors but expected 1", numErrors)
	}
}
