package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/metrics"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// mockT is a mock implementation of testingT for unit testing
type mockT struct {
	name      string
	cleanups  []func()
	calledAll *bool
}

func (m *mockT) Logf(format string, args ...any) {
	fmt.Sprintf(format, args...)
}

func (m *mockT) Name() string {
	return m.name
}

func (m *mockT) Cleanup(f func()) {
	m.cleanups = append(m.cleanups, f)
}

func (m *mockT) callAllCleanups() {
	for _, f := range m.cleanups {
		f()
	}
	if m.calledAll != nil {
		*m.calledAll = true
	}
}

func TestLog(t *testing.T) {
	tests := []struct {
		name     string
		mID      metrics.ID
		val      string
		flagTags map[string]string
		extra    map[string]any
		expect   TestMetric
	}{
		{
			name:     "standard_metric_with_extra",
			mID:      metrics.SSHDStartTime,
			val:      "12345",
			extra:    map[string]any{"foo": "bar"},
			flagTags: map[string]string{"baz": "qux"},
			expect: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "12345",
				Data:  map[string]any{"foo": "bar", "baz": "qux"},
			},
		},
		{
			name:  "metric_with_no_extra",
			mID:   metrics.SSHDStartTime,
			val:   "abcde",
			extra: nil,
			expect: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "abcde",
				Data:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tr := TestRun{
				Name:      "logtest-binary",
				Functions: make(map[string]TestFunction),
				LogPrefix: tmpDir,
			}
			called := false
			mt := &mockT{name: "TestFunction1", calledAll: &called}
			oldFT := flagTags
			flagTags = tt.flagTags
			t.Cleanup(func() { flagTags = oldFT })

			tr.Log(mt, tt.mID, tt.val, tt.extra)

			mt.callAllCleanups()

			dest := filepath.Join(tmpDir, tr.Name)
			data, err := os.ReadFile(dest)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v, want nil", dest, err)
			}
			var parsed TestRun
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want nil", string(data), err)
			}

			if diff := cmp.Diff(tr, parsed, cmpopts.IgnoreUnexported(TestRun{})); diff != "" {
				t.Errorf("json.Unmarshal(%s) mismatch (-want +got):\n%s", data, diff)
			}
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		metric    TestMetric
		expectMap map[string]any
	}{
		{
			name: "with_extra_fields",
			metric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val1",
				Data:  map[string]any{"x": 42, "extra": "yes"},
			},
			expectMap: map[string]any{
				"ID":    "SSHDStartTime",
				"Value": "val1",
				"Data": map[string]any{
					"x":     float64(42),
					"extra": "yes",
				},
			},
		},
		{
			name: "without_extra_fields",
			metric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val2",
			},
			expectMap: map[string]any{
				"ID":    "SSHDStartTime",
				"Value": "val2",
				"Data":  nil,
			},
		},
		{
			name: "empty_extra_map",
			metric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val3",
				Data:  map[string]any{},
			},
			expectMap: map[string]any{
				"ID":    "SSHDStartTime",
				"Value": "val3",
				"Data":  map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.metric)
			if err != nil {
				t.Fatalf("json.Marshal(%+v) = error %v, want nil", tt.metric, err)
			}

			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want nil", string(data), err)
			}

			if diff := cmp.Diff(tt.expectMap, got); diff != "" {
				t.Errorf("MarshalJSON(%+v) mismatch (-want +got):\n%s", tt.metric, diff)
			}
		})
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		metric       TestMetric
		expectMetric TestMetric
	}{
		{
			name: "with_extra_fields",
			metric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val1",
				Data:  map[string]any{"x": 42, "extra": "yes"},
			},
			expectMetric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val1",
				Data:  map[string]any{"x": float64(42), "extra": "yes"},
			},
		},
		{
			name: "without_extra_fields",
			metric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val2",
			},
			expectMetric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val2",
			},
		},
		{
			name: "empty_extra_map",
			metric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val3",
				Data:  map[string]any{},
			},
			expectMetric: TestMetric{
				ID:    metrics.SSHDStartTime,
				Value: "val3",
				Data:  map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.metric)
			if err != nil {
				t.Fatalf("json.Marshal(%+v) = error %v, want nil", tt.metric, err)
			}

			var got TestMetric
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want nil", string(data), err)
			}

			if diff := cmp.Diff(tt.expectMetric, got); diff != "" {
				t.Errorf("json.Unmarshal(%s) mismatch (-want +got):\n%s", data, diff)
			}
		})
	}
}

func TestFlush(t *testing.T) {
	tests := []struct {
		name     string
		testRun  TestRun
		setup    func(tmpDir string) string
		validate func(t *testing.T, filePath string, parsed TestRun)
	}{
		{
			name: "flush_single_function_with_one_metric",
			testRun: TestRun{
				Name: "test-binary-1",
				Functions: map[string]TestFunction{
					"FuncA": {
						Metrics: []TestMetric{
							{ID: metrics.SSHDStartTime, Value: "9999"},
						},
					},
				},
			},
			setup: func(tmpDir string) string {
				return tmpDir
			},
			validate: func(t *testing.T, _ string, parsed TestRun) {
				if parsed.Name != "test-binary-1" {
					t.Errorf("Flush().Name = %q, want %q", parsed.Name, "test-binary-1")
				}
			},
		},
		{
			name: "flush_with_no_functions",
			testRun: TestRun{
				Name:      "empty-binary",
				Functions: map[string]TestFunction{},
			},
			setup: func(tmpDir string) string {
				return tmpDir
			},
			validate: func(t *testing.T, _ string, parsed TestRun) {
				if len(parsed.Functions) != 0 {
					t.Errorf("Flush().Functions length = %d, want 0", len(parsed.Functions))
				}
			},
		},
		{
			name: "flush_creates_subdirectories",
			testRun: TestRun{
				Name: "nested-binary",
				Functions: map[string]TestFunction{
					"FuncX": {},
				},
			},
			setup: func(tmpDir string) string {
				nested := filepath.Join(tmpDir, "sub", "dir")
				os.MkdirAll(nested, 0755)
				return nested
			},
			validate: func(t *testing.T, filePath string, _ TestRun) {
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Fatalf("os.Stat(%q) = file not found, want file to exist", filePath)
				}
			},
		},
		{
			name: "flush_with_extra_values",
			testRun: TestRun{
				Name: "extra-values-test",
				Functions: map[string]TestFunction{
					"FuncZ": {
						Metrics: []TestMetric{
							{
								ID:    metrics.SSHDStartTime,
								Value: "value",
								Data:  map[string]any{"env": "prod", "code": 200},
							},
						},
					},
				},
			},
			setup: func(tmpDir string) string {
				return tmpDir
			},
			validate: func(t *testing.T, _ string, parsed TestRun) {
				metric := parsed.Functions["FuncZ"].Metrics[0]
				if metric.Data["env"] != "prod" || metric.Data["code"] != 200.0 {
					t.Errorf("Flush().Data = %+v, want env=prod and code=200", metric.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logDir := tt.setup(tmpDir)
			tt.testRun.LogPrefix = logDir

			err := tt.testRun.Flush()
			if err != nil {
				t.Fatalf("Flush() = error %v, want nil", err)
			}

			filePath := filepath.Join(logDir, tt.testRun.Name)
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) = error %v, want nil", filePath, err)
			}

			var parsed TestRun
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want nil", string(data), err)
			}

			tt.validate(t, filePath, parsed)
		})
	}
}

func TestTagValueSet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected tagValue
	}{
		{
			name:     "single_key_value",
			input:    "foo:bar",
			expected: tagValue{"foo": "bar"},
		},
		{
			name:     "multiple_key_values",
			input:    "foo:bar,baz:qux",
			expected: tagValue{"foo": "bar", "baz": "qux"},
		},
		{
			name:     "missing_value",
			input:    "key:",
			expected: tagValue{"key": ""},
		},
		{
			name:     "missing_key",
			input:    ":value",
			expected: tagValue{},
		},
		{
			name:     "mixed_valid_and_invalid",
			input:    "valid:entry,:missingkey,another:one",
			expected: tagValue{"valid": "entry", "another": "one"},
		},
		{
			name:     "empty_input",
			input:    "",
			expected: tagValue{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := tagValue{}
			if err := tv.Set(tt.input); err != nil {
				t.Fatalf("Set() returned unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.expected, tv); diff != "" {
				t.Errorf("Set() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTagValueString(t *testing.T) {
	tests := []struct {
		name string
		tv   tagValue
	}{
		{
			name: "single_entry",
			tv:   tagValue{"key": "value"},
		},
		{
			name: "multiple_entries",
			tv:   tagValue{"a": "1", "b": "2"},
		},
		{
			name: "empty_map",
			tv:   tagValue{},
		},
		{
			name: "empty_value_entry",
			tv:   tagValue{"key": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str := tt.tv.String()
			// Map iteration order is non-deterministic, so rather than playing around
			// with string comparisons it's easier to just to check that the output is
			// something parseable back into the same set. Then we know it's valid, as
			// long as TestTagValueSet also passed.
			var parsed tagValue = make(map[string]string)
			if err := parsed.Set(str); err != nil {
				t.Fatalf("Set() returned unexpected error: %v", err)
			}

			if diff := cmp.Diff(tt.tv, parsed); diff != "" {
				t.Errorf("round-trip mismatch (-original +parsed):\n%s", diff)
			}
		})
	}
}
