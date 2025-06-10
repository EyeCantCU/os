package artifacts

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/files"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/metrics"
)

var (
	// CurrentTestRun represents the currently running test binary.
	CurrentTestRun = TestRun{Functions: make(map[string]TestFunction), LogPrefix: DefaultLogPrefix}
	// DefaultLogPrefix is the folder where logs will be flushed
	DefaultLogPrefix = "/tmp/chainguard-vmtest-metrics/"
	// user-provided extra tags
	// `go test` binaries already call flag.Parse, main packages using this library must call it as well
	flagTags = make(tagValue)
)

// A map[string]string flag value used as extra tags
type tagValue map[string]string

// Display set tag values
func (tv tagValue) String() string {
	var s string
	for k, v := range tv {
		if len(s) > 0 {
			s = fmt.Sprintf("%s,%s:%s", s, k, v)
		} else {
			s = fmt.Sprintf("%s:%s", k, v)
		}
	}
	return s
}

// Parse incoming flag values
func (tv tagValue) Set(s string) error {
	kvPairs := strings.Split(s, ",")
	for _, kvp := range kvPairs {
		k, v, _ := strings.Cut(kvp, ":")
		if k == "" {
			// Lack of key is invalid, lack of value is not
			continue
		}
		tv[k] = v
	}
	return nil
}

func init() {
	flag.Var(flagTags, "metric-tags", "Comma separated key-value string pairs to tag metric logs (ex. key1:val,key2:val)")
	binPath, err := os.Executable()
	if err == nil {
		CurrentTestRun.Name = filepath.Base(binPath)
	}
	host, err := os.Hostname()
	if err == nil {
		CurrentTestRun.Hostname = host
	}
}

// TestRun is an execution of a test binary.
type TestRun struct {
	// Name is the name of the test binary
	Name string
	// Hostname is the name of the machine the binary is executed on.
	Hostname string
	// Functions is a map of function names to the corresponding TestFunction
	Functions map[string]TestFunction
	// LogPrefix is the folder TestRun logs will be written to
	LogPrefix string
	// mu is the read/write lock for the TestRun
	mu sync.RWMutex
}

// TestFunction is an execution of a function within a test binary.
type TestFunction struct {
	// Metrics is the list of metrics logged by that function. Logging the same metric ID multiple times is allowed.
	Metrics []TestMetric `json:"metrics,omitempty"`
	Files   []TestFile   `json:"files,omitempty"`
}

// TestMetric is a single metric logged by a function. Logging the same metric ID multiple times is allowed.
type TestMetric struct {
	// ID is the metric identifier
	ID metrics.ID
	// Value is the value of the metric
	Value string
	// Data contains arbitary extra fields.
	Data map[string]any
}

// TestFile is a file (content) collected by a test
type TestFile struct {
	// ID is the file identifier
	ID      files.ID
	Content []byte
	Data    map[string]any
	Error   error `json:"error,omitempty"`
}

// testingT is an interface containing what metrics needs from testing.T
// It exists to be mockable for unit tests.
type testingT interface {
	Cleanup(func())
	Name() string
	Logf(string, ...any)
}

// Log calls Log for the current running test
func Log(t testingT, mID metrics.ID, val string, extra map[string]any) {
	CurrentTestRun.Log(t, mID, val, extra)
}

// File calls File for the current running test
func File(t testingT, fID files.ID, content []byte, fErr error, extra map[string]any) {
	CurrentTestRun.File(t, fID, content, fErr, extra)
}

// Log logs a single TestMetric.
func (tr *TestRun) Log(t testingT, mID metrics.ID, val string, extra map[string]any) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	testfunc, _ := tr.Functions[t.Name()]
	newmetric := TestMetric{
		ID:    mID,
		Value: val,
		Data:  extra,
	}
	if newmetric.Data == nil {
		newmetric.Data = make(map[string]any)
	}
	for k, v := range flagTags {
		newmetric.Data[k] = v
	}
	testfunc.Metrics = append(testfunc.Metrics, newmetric)
	tr.Functions[t.Name()] = testfunc
	t.Cleanup(func() {
		if err := tr.Flush(); err != nil {
			t.Logf("failed to flush logs for %s: %v", t.Name(), err)
		}
	})
}

func (tr *TestRun) File(t testingT, fID files.ID, content []byte, fErr error, extra map[string]any) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	testfunc, _ := tr.Functions[t.Name()]
	newfile := TestFile{
		ID:      fID,
		Content: content,
		Data:    extra,
		Error:   fErr,
	}
	if newfile.Data == nil {
		newfile.Data = make(map[string]any)
	}
	for k, v := range flagTags {
		newfile.Data[k] = v
	}
	testfunc.Files = append(testfunc.Files, newfile)

	tr.Functions[t.Name()] = testfunc
	t.Cleanup(func() {
		if err := tr.Flush(); err != nil {
			t.Logf("failed to flush logs for %s: %v", t.Name(), err)
		}
	})
}

// Flush flushes logs to the given folder
func (tr *TestRun) Flush() error {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	destFile := filepath.Join(tr.LogPrefix, tr.Name)
	b, err := json.Marshal(tr)
	if err != nil {
		return fmt.Errorf("unable to marshal TestRun: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return fmt.Errorf("failed to make parent directory for log file: %v", err)
	}
	if err := os.WriteFile(destFile, b, 0644); err != nil {
		return fmt.Errorf("failed to write log file: %v", err)
	}
	return nil
}

// Turn any map[string]... into a map[string]any for Log()
func MapOfAny[T any](m map[string]T) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		out[k] = v
	}
	return out
}
