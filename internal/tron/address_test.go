package tron

import (
	"bytes"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/sha3"
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

func TestPublicKeyToAddress(t *testing.T) {
	// Error cases
	errorTests := []struct {
		name        string
		pem         string
		errContains string
	}{
		{
			name:        "empty PEM",
			pem:         "",
			errContains: "empty",
		},
		{
			name:        "invalid PEM - no block",
			pem:         "not a pem string at all",
			errContains: "invalid PEM",
		},
		{
			name: "invalid PEM type",
			pem: `-----BEGIN CERTIFICATE-----
MIIBqDCCARCgAwIBAgIBATANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDDA1UZXN0
-----END CERTIFICATE-----
`,
			errContains: "PUBLIC KEY",
		},
		{
			name: "truncated PEM",
			pem: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQQ==
-----END PUBLIC KEY-----
`,
			errContains: "truncated",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := PublicKeyToAddress(tt.pem)
			if err == nil {
				t.Errorf("PublicKeyToAddress() = %s, expected error containing %q", addr, tt.errContains)
				return
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("PublicKeyToAddress() error = %v, want error containing %q", err, tt.errContains)
			}
		})
	}

	// Valid secp256k1 key test
	t.Run("valid secp256k1 key", func(t *testing.T) {
		// Generate a deterministic secp256k1 key using btcec.
		seed := []byte("txbuilder-tron-convert-address-test")
		_, pubKey := generateTestBtcecKey(seed)

		// Build PKIX SubjectPublicKeyInfo using btcec's SerializeUncompressed.
		// This avoids manual ellipticMarshal which could have subtle bugs.
		oidECPublicKey := asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
		oidSecp256k1 := asn1.ObjectIdentifier{1, 3, 132, 0, 10}

		algoIDBytes, err := asn1.Marshal(struct {
			Algorithm  asn1.ObjectIdentifier
			Parameters asn1.ObjectIdentifier
		}{oidECPublicKey, oidSecp256k1})
		if err != nil {
			t.Fatalf("asn1.Marshal(algoID): %v", err)
		}

		// Use btcec's SerializeUncompressed for the EC point.
		pubBytes := pubKey.SerializeUncompressed()
		if len(pubBytes) != 65 || pubBytes[0] != 0x04 {
			t.Fatalf("SerializeUncompressed: unexpected result len=%d prefix=0x%02x", len(pubBytes), pubBytes[0])
		}

		// BIT STRING: tag 0x03, length, unused bits (0), content.
		bitStringContent := append([]byte{0x00}, pubBytes...)
		bitString := append([]byte{0x03, byte(len(bitStringContent))}, bitStringContent...)

		// Full PKIX: SEQUENCE { algoID, bitString }.
		pkixInner := append(algoIDBytes, bitString...)
		pkixBytes := append([]byte{0x30, byte(len(pkixInner))}, pkixInner...)

		// Encode as PEM.
		pemBlock := &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pkixBytes,
		}
		validPEM := string(pem.EncodeToMemory(pemBlock))

		// Compute expected address directly from the known-good pubBytes.
		hash := sha3.NewLegacyKeccak256()
		hash.Write(pubBytes)
		keccakDigest := hash.Sum(nil)
		addressBytes := keccakDigest[12:]
		tronPayload := append([]byte{0x41}, addressBytes...)
		expected := EncodeBase58Check(tronPayload)

		addr, err := PublicKeyToAddress(validPEM)
		if err != nil {
			t.Fatalf("PublicKeyToAddress() error = %v", err)
		}

		if addr != expected {
			t.Errorf("PublicKeyToAddress() = %s, expected %s", addr, expected)
		}

		if !ValidateAddress(addr) {
			t.Errorf("PublicKeyToAddress() produced invalid TRON address: %s", addr)
		}
	})
}

func TestEncodeBase58Check(t *testing.T) {
	// Verify the known TRON address round-trips correctly:
	// TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ → DecodeBase58 → take first 21 bytes → EncodeBase58Check
	addr := "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ"
	decoded, err := DecodeBase58(addr)
	if err != nil {
		t.Fatalf("DecodeBase58(%s): %v", addr, err)
	}
	if len(decoded) != 25 {
		t.Errorf("DecodeBase58(%s) len=%d, want 25", addr, len(decoded))
	}
	encoded := EncodeBase58Check(decoded[:21])
	if encoded != addr {
		t.Errorf("EncodeBase58Check(21-byte payload) = %s, want %s", encoded, addr)
	}
}

func TestEncodeBase58_RoundTrip(t *testing.T) {
	testData := [][]byte{
		{0x01},
		{0xff},
		{0x41, 0xa7, 0xd8, 0xa3, 0x5b, 0x26, 0x03, 0x95, 0xc1, 0x4a, 0xa9, 0x45, 0x62, 0x97, 0x22, 0x09, 0xba, 0x3b, 0x76, 0xfc, 0x0d},
		{0x41, 0xa7, 0xd8, 0xa3, 0x5b, 0x26, 0x03, 0x95, 0xc1, 0x4a, 0xa9, 0x45, 0x62, 0x97, 0x22, 0x09, 0xba, 0x3b, 0x76, 0xfc, 0x0d, 0x12, 0x34, 0x56, 0x78},
	}

	for _, data := range testData {
		encoded := EncodeBase58(data)
		decoded, err := DecodeBase58(encoded)
		if err != nil {
			t.Errorf("DecodeBase58(%q) error = %v", encoded, err)
			continue
		}
		if !bytes.Equal(decoded, data) {
			t.Errorf("DecodeBase58(EncodeBase58(%x)) = %x, want %x", data, decoded, data)
		}
	}
}

func generateTestBtcecKey(seed []byte) (*btcec.PrivateKey, *btcec.PublicKey) {
	h := sha256.Sum256(seed)
	return btcec.PrivKeyFromBytes(h[:])
}
