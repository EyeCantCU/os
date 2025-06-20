//go:build vmtest

package startupscripts

import (
	"os"
	"testing"
)

// Start the VM that runs this test with with ssmsendcommand_launch.sh script
// to start the VM and then run aws ssm [ .. ] to do 'printf hello wolfi > /tmp/hello-wolfi.txt'.
//
// Put this key in the vm test configuration to enable such a thing:
//
// launch: ../path/to/vms-test/pkg/test/aws/ssmsendcommand/ssmsendcommand_launch.sh
func TestStartupScripts(t *testing.T) {
	targetFile := "/tmp/hello-wolfi.txt"
	expectContent := "hello wolfi"
	contents, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("os.Readfile(%s) = err %v, want nil", targetFile, err)
	}
	if string(contents) != expectContent {
		t.Fatalf("%s contents are %q, want %q", targetFile, contents, expectContent)
	}
}
