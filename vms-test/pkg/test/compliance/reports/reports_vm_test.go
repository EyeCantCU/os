//go:build vmtest

package reports

import (
	"bytes"
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"

	"chainguard.dev/wolfi-vm/vms-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vms-test/pkg/artifacts/files"
	"chainguard.dev/wolfi-vm/vms-test/pkg/vmtest"
)

const (
	ssgFile         = "ssg-chainguard-ds.xml"
	embeddedSSGFile = "ssg/" + ssgFile + ".gz"
)

// Embed the SCAP Security Guide to use during build so the test is self-contained.
// Go requires an embedded file to exist or the build will fail. We may want to
// not run compliance reporting sometimes, though, and the absence of an SSG file
// signals so. Embed a directory instead (with files being optional).
//
//go:embed ssg/*
var embeddedSSG embed.FS

// Extracts the SCAP Security Guide embedded into the test binary.
// Returns (true, nil) if the SSG was extraced successfully, (false, nil) if
// no SSG was embedded, or (false, error) on failure.
func extractSCAPSecurityGuideFromSelf(outputPath string) (bool, error) {
	compressedSSG, err := embeddedSSG.ReadFile(embeddedSSGFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		} else {
			return false, fmt.Errorf("cannot extract embedded SSG file: %v", err)
		}
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(compressedSSG))
	if err != nil {
		return false, fmt.Errorf("cannot gunzip data: %v", err)
	}
	defer gzipReader.Close()

	decompressedSSG, err := io.ReadAll(gzipReader)
	if err != nil {
		return false, fmt.Errorf("cannot gunzip data: %v", err)
	}

	err = os.WriteFile(outputPath, decompressedSSG, 0600)
	if err != nil {
		return false, fmt.Errorf("cannot write to '%v': %v", outputPath, err)
	}

	return true, nil
}

func TestGenerateComplianceReports(t *testing.T) {
	ctx := vmtest.Context(t)

	// Extract the SCAP Security Guide that is piggybacking on the test
	// binary. If none is present this is intentional; compliance scanning
	// is not being asked for.
	present, err := extractSCAPSecurityGuideFromSelf(ssgFile)
	if err != nil {
		t.Fatalf("cannot extract SCAP security guide: %v", err)
	} else if !present {
		t.Skip("no SCAP Security Guide present")
	}

	// Ensure OpenSCAP is installed
	if _, err := exec.LookPath("oscap"); err != nil {
		if _, err := exec.LookPath("apk"); err != nil {
			t.Skip("OpenSCAP not installed and apk not available to install it")
		}
		t.Log("OpenSCAP not found, installing...")
		installCmd := exec.CommandContext(ctx, "apk", "add", "openscap")
		if output, err := installCmd.CombinedOutput(); err != nil {
			t.Skipf("failed to install OpenSCAP: %v\nOutput: %s", err, output)
		}
		t.Log("OpenSCAP installed successfully")
	}

	profiles := []struct {
		prettyName  string
		profileName string
	}{
		{"CIS Server L1", "xccdf_org.ssgproject.content_profile_cis_server_l1"},
		{"STIG", "xccdf_org.ssgproject.content_profile_stig"},
		{"STIG GPOS", "xccdf_org.ssgproject.content_profile_stig_gpos"},
	}

	for _, profile := range profiles {
		// Golang-friendly name, e.g. "CISServerL1"
		methodName := strings.ReplaceAll(profile.prettyName, " ", "")

		t.Run(methodName, func(t *testing.T) {
			// Filesystem-friendly name, e.g. "cis-server-l1"
			fsName := strings.ToLower(strings.ReplaceAll(profile.prettyName, " ", "-"))
			resultsFile := fsName + "-results.xml"
			viewerFile := fsName + "-stigviewer.xml"
			reportFile := fsName + "-report.html"

			// Run the scan. This needs elevated privileges, because with hardening
			// applied some of the rules will read files inaccessible to regular users.
			cmd := exec.CommandContext(ctx, "oscap", "xccdf", "eval",
				"--profile", profile.profileName,
				"--results", resultsFile,
				"--report", reportFile,
				ssgFile)
			// Run stig-viewer output only on GPOS for now
			if profile.profileName == "xccdf_org.ssgproject.content_profile_stig_gpos" {
				cmd = exec.CommandContext(ctx, "oscap", "xccdf", "eval",
					"--profile", profile.profileName,
					"--results", resultsFile,
					"--stig-viewer", viewerFile,
					"--report", reportFile,
					ssgFile)
			}
			output, err := cmd.CombinedOutput()
			t.Logf("OpenSCAP output:\n%s", output)

			if err != nil {
				// OpenSCAP returns non-zero exit codes for various reasons:
				// - Exit code 1: Error during evaluation
				// - Exit code 2: Evaluation successful but some rules failed (non-compliance)
				// We only want to fail the test for actual errors (exit code 1)
				if exitErr, ok := err.(*exec.ExitError); ok {
					switch ec := exitErr.ExitCode(); ec {
					case 1:
						t.Errorf("OpenSCAP evaluation error, check stderr")
					case 2:
						t.Logf("OpenSCAP found the system non-compliant")
					default:
						t.Errorf("OpenSCAP exited with an unknown exit code: %v", ec)
					}
				}
			}

			if _, err := os.Stat(resultsFile); os.IsNotExist(err) {
				t.Error("expected results XML file to be created")
			}
			if _, err := os.Stat(reportFile); os.IsNotExist(err) {
				t.Error("expected HTML report file to be created")
			}
			t.Logf("compliance report generated: %s", reportFile)

			// Store test artifacts.
			meta := map[string]any{"scap_profile": profile.profileName}
			var content []byte

			content, err = os.ReadFile(resultsFile)
			artifacts.File(t, files.ComplianceXMLResults, content, err, meta)

			content, err = os.ReadFile(reportFile)
			artifacts.File(t, files.ComplianceHTMLReport, content, err, meta)

			content, err = os.ReadFile(viewerFile)
			artifacts.File(t, files.ComplianceStigViewerXML, content, err, meta)
		})
	}
}
