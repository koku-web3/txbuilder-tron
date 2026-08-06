package tron

import (
	"testing"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "valid Tron address",
			address: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
			want:    true,
		},
		{
			name:    "invalid address - starts with 3",
			address: "3NsVCWoo9REhQVKbwuhSkxRYRAxTDagMqn",
			want:    false,
		},
		{
			name:    "invalid address - wrong length",
			address: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy4",
			want:    false,
		},
		{
			name:    "invalid address - empty",
			address: "",
			want:    false,
		},
		{
			name:    "invalid address - starts with A",
			address: "AQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateAddress(tt.address)
			if got != tt.want {
				t.Errorf("ValidateAddress(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}

func TestValidateContractAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "valid contract address",
			address: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
			want:    true,
		},
		{
			name:    "invalid contract address - starts with 3",
			address: "3NsVCWoo9REhQVKbwuhSkxRYRAxTDagMqn",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateContractAddress(tt.address)
			if got != tt.want {
				t.Errorf("ValidateContractAddress(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}

func TestIsPureNumber(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		want   bool
	}{
		{
			name:   "pure number",
			amount: "123456",
			want:   true,
		},
		{
			name:   "zero",
			amount: "0",
			want:   true,
		},
		{
			name:   "large number",
			amount: "99999999999999999999999999999999999999",
			want:   true,
		},
		{
			name:   "empty string",
			amount: "",
			want:   false,
		},
		{
			name:   "with decimal point",
			amount: "123.456",
			want:   false,
		},
		{
			name:   "with letters",
			amount: "123abc",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPureNumber(tt.amount)
			if got != tt.want {
				t.Errorf("IsPureNumber(%q) = %v, want %v", tt.amount, got, tt.want)
			}
		})
	}
}

func TestValidateAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		want   bool
	}{
		{
			name:   "integer amount",
			amount: "100",
			want:   true,
		},
		{
			name:   "decimal amount",
			amount: "100.50",
			want:   true,
		},
		{
			name:   "zero",
			amount: "0",
			want:   true,
		},
		{
			name:   "empty string",
			amount: "",
			want:   false,
		},
		{
			name:   "starts with decimal",
			amount: ".50",
			want:   false,
		},
		{
			name:   "ends with decimal",
			amount: "50.",
			want:   false,
		},
		{
			name:   "double decimal",
			amount: "1.2.3",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateAmount(tt.amount)
			if got != tt.want {
				t.Errorf("ValidateAmount(%q) = %v, want %v", tt.amount, got, tt.want)
			}
		})
	}
}

func TestSha256(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "hello world",
			data: []byte("hello world"),
		},
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "binary data",
			data: []byte{0x00, 0xFF, 0x01, 0xFE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := Sha256(tt.data)
			if len(hash) != 32 {
				t.Errorf("Sha256() should return 32 bytes, got %d", len(hash))
			}
		})
	}
}

func TestDecodeBase58(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantLen int
		wantErr bool
	}{
		{
			name:    "valid Tron address",
			address: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
			wantLen: 25,
			wantErr: false,
		},
		{
			name:    "empty string",
			address: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeBase58(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeBase58() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("DecodeBase58() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestBase58ToHex(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantLen int
	}{
		{
			name:    "valid Tron address",
			address: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
			wantLen: 42, // 21 bytes = 42 hex chars (41 prefix + 20 bytes payload)
		},
		{
			name:    "hex string not 64 chars -> DecodeBase58 succeeds (32 bytes -> 64 hex)",
			address: "4168F775B6A2D460F2F9D6C4B0E7B8D5F1C3A9E7D2B4F6A8C0E2D4B6F8A0C2E4",
			wantLen: 64, // DecodeBase58 succeeds -> 32 bytes -> 64 hex chars (42 not reached)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hex := Base58ToHex(tt.address)
			if len(hex) != tt.wantLen {
				t.Errorf("Base58ToHex() len = %d, want %d", len(hex), tt.wantLen)
			}
		})
	}
}
