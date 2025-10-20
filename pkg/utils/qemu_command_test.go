/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseDriveOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
		order    []string
	}{
		{
			name:  "simple drive options",
			input: "if=pflash,format=raw,unit=0,file=test.img",
			expected: map[string]string{
				"if":     "pflash",
				"format": "raw",
				"unit":   "0",
				"file":   "test.img",
			},
			order: []string{"if", "format", "unit", "file"},
		},
		{
			name:  "blockdev options",
			input: "driver=raw,node-name=disk,file.driver=file,file.filename=disk.raw",
			expected: map[string]string{
				"driver":        "raw",
				"node-name":     "disk",
				"file.driver":   "file",
				"file.filename": "disk.raw",
			},
			order: []string{"driver", "node-name", "file.driver", "file.filename"},
		},
		{
			name:  "option with empty value",
			input: "driver=raw,readonly",
			expected: map[string]string{
				"driver":   "raw",
				"readonly": "",
			},
			order: []string{"driver", "readonly"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseDriveOptions(tt.input)

			// Check all expected keys and values
			for key, expectedVal := range tt.expected {
				val, ok := opts.Get(key)
				if !ok {
					t.Errorf("expected key %s not found", key)
					continue
				}
				if val != expectedVal {
					t.Errorf("key %s: expected %s, got %s", key, expectedVal, val)
				}
			}

			// Check order
			if len(opts.order) != len(tt.order) {
				t.Errorf("order length mismatch: expected %d, got %d", len(tt.order), len(opts.order))
			}
			for i, expectedKey := range tt.order {
				if i >= len(opts.order) {
					break
				}
				if opts.order[i] != expectedKey {
					t.Errorf("order[%d]: expected %s, got %s", i, expectedKey, opts.order[i])
				}
			}

			// Check String() returns correct format
			result := opts.String()
			if result != tt.input {
				t.Errorf("String() should return original: expected %s, got %s", tt.input, result)
			}
		})
	}
}

func TestDriveOptionsSet(t *testing.T) {
	opts := parseDriveOptions("driver=raw,node-name=disk,file.filename=test.raw")

	// Set existing key - should preserve order
	opts.Set("driver", "qcow2")
	result := opts.String()
	expectedOrder := []string{"driver=qcow2", "node-name=disk", "file.filename=test.raw"}
	for i, exp := range expectedOrder {
		if i > 0 {
			// Check relative order
			idx1 := strings.Index(result, expectedOrder[i-1])
			idx2 := strings.Index(result, exp)
			if idx2 <= idx1 {
				t.Errorf("order not preserved after Set")
			}
		}
	}

	// Set new key - should append to end
	opts.Set("cache.direct", "on")
	result = opts.String()
	if !strings.HasSuffix(result, ",cache.direct=on") {
		t.Errorf("new key should be appended to end, got: %s", result)
	}
}

func TestSnapshotify(t *testing.T) {
	// Mock qcow2 creator that tracks calls
	type creatorCall struct {
		backingFile   string
		backingFormat string
	}
	var calls []creatorCall

	mockCreator := func(backingFile, backingFormat string) (string, error) {
		calls = append(calls, creatorCall{backingFile, backingFormat})
		return fmt.Sprintf("/tmp/mock-%d.qcow2", len(calls)), nil
	}

	tests := []struct {
		name           string
		input          []string
		expectedOutput []string
		expectedCalls  []creatorCall
	}{
		{
			name: "readonly drive unchanged",
			input: []string{
				"-drive", "if=pflash,format=raw,unit=0,file=fwcode.img,readonly=on",
			},
			expectedOutput: []string{
				"-drive", "if=pflash,format=raw,unit=0,file=fwcode.img,readonly=on",
			},
			expectedCalls: []creatorCall{},
		},
		{
			name: "writable drive gets snapshot=on",
			input: []string{
				"-drive", "if=pflash,format=raw,unit=1,file=fwvars.img",
			},
			expectedOutput: []string{
				"-drive", "if=pflash,format=raw,unit=1,file=fwvars.img,snapshot=on",
			},
			expectedCalls: []creatorCall{},
		},
		{
			name: "blockdev driver=raw creates qcow2",
			input: []string{
				"-blockdev", "driver=raw,node-name=disk-debug,file.driver=file,file.filename=disk.raw",
			},
			expectedOutput: []string{
				"-blockdev", "driver=qcow2,node-name=disk-debug,file.driver=file,file.filename=/tmp/mock-1.qcow2",
			},
			expectedCalls: []creatorCall{
				{backingFile: "disk.raw", backingFormat: "raw"},
			},
		},
		{
			name: "blockdev driver=file creates qcow2",
			input: []string{
				"-blockdev", "driver=file,node-name=disk,filename=disk.img",
			},
			expectedOutput: []string{
				"-blockdev", "driver=qcow2,node-name=disk,file.driver=file,file.filename=/tmp/mock-1.qcow2",
			},
			expectedCalls: []creatorCall{
				{backingFile: "disk.img", backingFormat: "raw"},
			},
		},
		{
			name: "blockdev driver=qcow2 creates qcow2",
			input: []string{
				"-blockdev", "driver=qcow2,node-name=disk,file.filename=my-disk.qcow2",
			},
			expectedOutput: []string{
				"-blockdev", "driver=qcow2,node-name=disk,file.filename=/tmp/mock-1.qcow2",
			},
			expectedCalls: []creatorCall{
				{backingFile: "my-disk.qcow2", backingFormat: "qcow2"},
			},
		},
		{
			name: "blockdev readonly unchanged",
			input: []string{
				"-blockdev", "driver=raw,node-name=disk,file.filename=disk.raw,read-only=on",
			},
			expectedOutput: []string{
				"-blockdev", "driver=raw,node-name=disk,file.filename=disk.raw,read-only=on",
			},
			expectedCalls: []creatorCall{},
		},
		{
			name: "mixed arguments",
			input: []string{
				"-device", "virtio-rng-pci",
				"-drive", "if=pflash,format=raw,unit=0,file=fwcode.img,readonly=on",
				"-drive", "if=pflash,format=raw,unit=1,file=fwvars.img",
				"-blockdev", "driver=raw,node-name=disk-debug,file.driver=file,file.filename=disk.raw",
				"-device", "virtio-blk-pci,drive=disk-debug,serial=boot-disk",
			},
			expectedOutput: []string{
				"-device", "virtio-rng-pci",
				"-drive", "if=pflash,format=raw,unit=0,file=fwcode.img,readonly=on",
				"-drive", "if=pflash,format=raw,unit=1,file=fwvars.img,snapshot=on",
				"-blockdev", "driver=qcow2,node-name=disk-debug,file.driver=file,file.filename=/tmp/mock-1.qcow2",
				"-device", "virtio-blk-pci,drive=disk-debug,serial=boot-disk",
			},
			expectedCalls: []creatorCall{
				{backingFile: "disk.raw", backingFormat: "raw"},
			},
		},
		{
			name: "order preserved in drive",
			input: []string{
				"-drive", "if=pflash,format=raw,unit=1,file=fwvars.img,cache=writeback",
			},
			expectedOutput: []string{
				"-drive", "if=pflash,format=raw,unit=1,file=fwvars.img,cache=writeback,snapshot=on",
			},
			expectedCalls: []creatorCall{},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state
			calls = []creatorCall{}

			result, cleanup, err := snapshotifyWithCreator(tt.input, mockCreator)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cleanup == nil {
				t.Errorf("cleanup function should not be nil")
			}

			// Check output matches expected
			if diff := cmp.Diff(result, tt.expectedOutput); diff != "" {
				t.Errorf("snapshotifyWithCreator(%d) mismatch (-want +got):\n%s", i, diff)
			}

			// Check creator was called correctly
			if len(calls) != len(tt.expectedCalls) {
				t.Errorf("creator call count mismatch: expected %d, got %d", len(tt.expectedCalls), len(calls))
				t.Logf("expected calls: %+v", tt.expectedCalls)
				t.Logf("actual calls: %+v", calls)
				return
			}

			for i, expectedCall := range tt.expectedCalls {
				if calls[i].backingFile != expectedCall.backingFile {
					t.Errorf("call[%d].backingFile: expected %s, got %s", i, expectedCall.backingFile, calls[i].backingFile)
				}
				if calls[i].backingFormat != expectedCall.backingFormat {
					t.Errorf("call[%d].backingFormat: expected %s, got %s", i, expectedCall.backingFormat, calls[i].backingFormat)
				}
			}
		})
	}
}
