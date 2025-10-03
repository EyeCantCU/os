//go:build vmtest

package secureboot

import (
	"bytes"
	efi "github.com/canonical/go-efilib"
	"testing"
)

func TestSecureBootVariables(t *testing.T) {
	ShimLockGUID := efi.MakeGUID(0x605dab50, 0xe046, 0x4300, 0xabb6, [...]uint8{0x3d, 0xd8, 0x10, 0xdd, 0x8b, 0x23})
	CustomGUID := efi.MakeGUID(0xc076ec0c, 0x7028, 0x4399, 0xa072, [...]uint8{0x71, 0xee, 0x5c, 0x44, 0x8b, 0x9f})

	// Positive tests
	tests := []struct {
		variable string
		guid     efi.GUID
		required bool
		expected []byte
	}{
		{"SecureBoot", efi.GlobalVariable, true, []byte{1}},
		{"AuditMode", efi.GlobalVariable, false, []byte{0}},
		{"SetupMode", efi.GlobalVariable, true, []byte{0}},
		{"CustomMode", CustomGUID, false, []byte{0}},
		{"MokSBStateRT", ShimLockGUID, false, []byte{0}},
	}
	for _, tt := range tests {
		t.Run(tt.variable, func(t *testing.T) {
			data, _, err := efi.ReadVariable(efi.DefaultVarContext, tt.variable, tt.guid)
			if !tt.required && err == efi.ErrVarNotExist {
				t.Logf("%v does not exist, ignoring", tt.variable)
				return
			}
			if err != nil {
				t.Fatalf("ReadVariable %v failed = %v, want nil", tt.variable, err)
			}
			if bytes.Equal(data, tt.expected) {
				t.Logf("%v is %v", tt.variable, data)
			} else {
				t.Errorf("%v is %v, want %v", tt.variable, data, tt.expected)
			}

		})
	}
}
