package selinux

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Status represents whether a feature is enabled or disabled
type Status string

const (
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
)

// Mode represents the SELinux operating mode
type Mode string

const (
	ModeEnforcing  Mode = "enforcing"
	ModePermissive Mode = "permissive"
	ModeDisabled   Mode = "disabled"
)

// DenyUnknown represents the policy deny_unknown status
type DenyUnknown string

const (
	DenyUnknownAllowed DenyUnknown = "allowed"
	DenyUnknownDenied  DenyUnknown = "denied"
)

// MemoryProtection represents the memory protection checking status
type MemoryProtection string

const (
	MemoryProtectionActual    MemoryProtection = "actual (secure)"
	MemoryProtectionRequested MemoryProtection = "requested (insecure)"
)

// Unknown is a sentinel value used across all typed string fields to indicate an unknown or unset value
const Unknown = ""

// SELinuxStatus represents the parsed output of the sestatus command
type SELinuxStatus struct {
	Status                   Status           // enabled/disabled
	SELinuxFSMount           string           // mount point for kernel virtual filesystem
	RootDirectory            string           // SELinux configuration directory
	LoadedPolicyName         string           // name of loaded policy
	CurrentMode              Mode             // enforcing/permissive/disabled
	ModeFromConfigFile       Mode             // intended mode from config
	PolicyMLSStatus          Status           // enabled/disabled
	PolicyDenyUnknownStatus  DenyUnknown      // allowed/denied
	MemoryProtectionChecking MemoryProtection // actual (secure)/requested (insecure)
	MaxKernelPolicyVersion   int              // maximum policy version understood by the kernel
}

var (
	// Regular expression to identify AVC denials in the system journal
	avcDenialPattern = regexp.MustCompile(`avc:\s+denied\s+.*`)

	// Regular expression to identify generic SELinux errors in the system journal
	selinuxErrorsPattern = regexp.MustCompile(`audit:\s+SELINUX_ERR`)
)

// Determine the status and configuration of SELinux on the running system
func GetSELinuxStatus(ctx context.Context) (*SELinuxStatus, error) {
	// libselinux does the busy work of collecting and interpreting status
	// information from the various sources, and sestatus conveniently exposes
	// it, so just shell out to it.
	cmd := exec.CommandContext(ctx, "sestatus")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run sestatus: %w", err)
	}

	return ParseSELinuxStatus(string(output))
}

// Parse the sestatus command output into a SELinuxStatus struct
func ParseSELinuxStatus(output string) (*SELinuxStatus, error) {
	status := &SELinuxStatus{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "SELinux status":
			status.Status = Status(value)
		case "SELinuxfs mount":
			status.SELinuxFSMount = value
		case "SELinux root directory":
			status.RootDirectory = value
		case "Loaded policy name":
			status.LoadedPolicyName = value
		case "Current mode":
			status.CurrentMode = Mode(value)
		case "Mode from config file":
			status.ModeFromConfigFile = Mode(value)
		case "Policy MLS status":
			status.PolicyMLSStatus = Status(value)
		case "Policy deny_unknown status":
			status.PolicyDenyUnknownStatus = DenyUnknown(value)
		case "Memory protection checking":
			status.MemoryProtectionChecking = MemoryProtection(value)
		case "Max kernel policy version":
			version, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("failed to parse max kernel policy version: %w", err)
			}
			status.MaxKernelPolicyVersion = version
		}
	}

	// Status should always be set in valid sestatus output. Other fields may
	// be missing when SELinux is disabled, though.
	if status.Status == Unknown {
		return nil, fmt.Errorf("SELinux status field not found in output")
	}

	return status, nil
}

// Extract all AVC denials from the given system journal entries
func ExtractAVCDenials(journalEntries []string) (denials []string) {
	for _, entry := range journalEntries {
		if denial := avcDenialPattern.FindString(entry); denial != "" {
			denials = append(denials, denial)
		}
	}
	return denials
}

func ExtractAuditSELinuxErrors(journalEntries []string) (selinuxErrors []string) {
	for _, entry := range journalEntries {
		if selinuxError := selinuxErrorsPattern.FindString(entry); selinuxError != "" {
			selinuxErrors = append(selinuxErrors, selinuxError)
		}
	}
	return selinuxErrors
}
