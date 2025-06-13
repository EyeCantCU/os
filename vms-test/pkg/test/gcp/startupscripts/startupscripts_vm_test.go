//go:build vmtest

package startupscripts

import (
	"os"
	"testing"
)

// Start the VM that runs this test with a startup script that does
// printf 'hello wolfi' > /tmp/hello-wolfi.txt
// Put this key in the vm test configuration to enable such a thing:
// metadata: 'startup-script=printf "hello wolfi" > /tmp/hello-wolfi.txt;'
//
// Depending on system utilities in VM tests is not nice, but this is kind of
// acceptable because bash is a runtime dependency of google guest agent for
// running startup scripts, so we only depending on the pieces under test.
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
