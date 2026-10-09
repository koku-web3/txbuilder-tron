package service

import (
	"strings"
	"testing"
)

func TestValidateTxBroadcast_FromAddress(t *testing.T) {
	const validAddr = "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ"

	tests := []struct {
		name        string
		fromAddress string
		wantErr     bool
	}{
		{
			name:        "valid address",
			fromAddress: validAddr,
			wantErr:     false,
		},
		{
			name:        "missing from_address",
			fromAddress: "",
			wantErr:     true,
		},
		{
			name:        "malformed address",
			fromAddress: "invalid",
			wantErr:     true,
		},
		{
			name:        "from_address exceeds 256 chars",
			fromAddress: "T" + strings.Repeat("a", 300),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTxBroadcast("trace-001", "0xdeadbeef", "abcd1234", tt.fromAddress)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTxBroadcast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTxBroadcast_RawData(t *testing.T) {
	tests := []struct {
		name    string
		rawData string
		wantErr bool
	}{
		{
			name:    "missing raw_data",
			rawData: "",
			wantErr: true,
		},
		{
			name:    "valid raw_data",
			rawData: "0xdeadbeef",
			wantErr: false,
		},
		{
			name:    "raw_data too long",
			rawData: strings.Repeat("a", 4097),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTxBroadcast("trace-001", tt.rawData, "abcd1234", "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTxBroadcast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTxBroadcast_Signature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		wantErr   bool
	}{
		{
			name:      "missing signature",
			signature: "",
			wantErr:   true,
		},
		{
			name:      "valid signature length",
			signature: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
			wantErr:   false,
		},
		{
			name:      "signature too long",
			signature: strings.Repeat("a", 513),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTxBroadcast("trace-001", "0xdeadbeef", tt.signature, "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTxBroadcast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeSignature(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name:    "valid hex signature",
			input:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantLen: 65,
			wantErr: false,
		},
		{
			name:    "invalid hex",
			input:   "not-hex",
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeSignature(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeSignature() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) != tt.wantLen {
				t.Errorf("decodeSignature() len = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}
