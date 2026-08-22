package tron

import (
	"bytes"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/sha3"
)

// isOnSecp256k1 checks if (x, y) is on the secp256k1 curve.
func isOnSecp256k1(x, y *big.Int) bool {
	//nolint:staticcheck // SA1019: btcec.S256().IsOnCurve uses the specialized, faster method.
	return btcec.S256().IsOnCurve(x, y)
}

const (
	alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

var (
	base58DecodeMap [256]byte
)

func init() {
	for i := range base58DecodeMap {
		base58DecodeMap[i] = 0xFF
	}
	for i, c := range alphabet {
		base58DecodeMap[c] = byte(i)
	}
}

func Sha256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func DecodeBase58(s string) ([]byte, error) {
	if s == "" {
		return nil, fmt.Errorf("empty string")
	}
	s = strings.TrimPrefix(s, "0x")

	result := big.NewInt(0)
	base := big.NewInt(58)

	for _, c := range s {
		idx := base58DecodeMap[c]
		if idx == 0xFF {
			return nil, fmt.Errorf("invalid base58 character: %c", c)
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(idx)))
	}

	decoded := result.Bytes()

	if strings.HasPrefix(s, "T") || strings.HasPrefix(s, "2") || strings.HasPrefix(s, "1") {
		leadingZeros := 0
		for _, c := range s {
			if c == rune(alphabet[0]) {
				leadingZeros++
			} else {
				break
			}
		}
		if leadingZeros > 0 {
			decoded = append(make([]byte, leadingZeros), decoded...)
		}
	}

	return decoded, nil
}

func ValidateAddress(address string) bool {
	if len(address) != 34 {
		return false
	}

	if !strings.HasPrefix(address, "T") {
		return false
	}

	decoded, err := DecodeBase58(address)
	if err != nil || len(decoded) != 25 {
		return false
	}

	checksum := decoded[len(decoded)-4:]
	payload := decoded[:len(decoded)-4]

	hash1 := Sha256(payload)
	hash2 := Sha256(hash1)

	return bytes.Equal(checksum, hash2[:4])
}

func ValidateContractAddress(address string) bool {
	return ValidateAddress(address)
}

func ValidateAmount(amount string) bool {
	if len(amount) == 0 {
		return false
	}

	hasDecimal := false
	for i, c := range amount {
		if c == '.' {
			if hasDecimal || i == 0 || i == len(amount)-1 {
				return false
			}
			hasDecimal = true
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func IsPureNumber(amount string) bool {
	for _, c := range amount {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(amount) > 0
}

func Base58ToHex(address string) string {
	if address == "" {
		return ""
	}

	if len(address) > 2 && address[:2] == "0x" {
		return address[2:]
	}

	if len(address) == 64 && isHexString(address) {
		return address
	}

	decoded, err := DecodeBase58(address)
	if err != nil {
		return ""
	}

	// decoded = prefix(1) + address(20) + checksum(4) = 25 bytes
	// Tron API needs the first 21 bytes: prefix + address (41xxxxxxxx...)
	if len(decoded) == 25 {
		decoded = decoded[:21]
	}

	var hexStr strings.Builder
	hexStr.Grow(len(decoded) * 2)
	for _, b := range decoded {
		hexStr.WriteString(fmt.Sprintf("%02x", b))
	}
	return hexStr.String()
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// pkixAlgorithmIdentifier reflects a PKIX AlgorithmIdentifier.
type pkixAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.ObjectIdentifier `asn1:"optional"`
}

// pkixSubjectPublicKeyInfo reflects a PKIX SubjectPublicKeyInfo structure.
// We parse it manually because x509.ParsePKIXPublicKey does not support secp256k1.
type pkixSubjectPublicKeyInfo struct {
	Algorithm pkixAlgorithmIdentifier
	PublicKey asn1.BitString
}

// secp256k1 OID: 1.3.132.0.10
var oidSecp256k1 = asn1.ObjectIdentifier{1, 3, 132, 0, 10}

// PublicKeyToAddress converts a PKIX-PEM-encoded public key to a TRON address.
// Unlike x509.ParsePKIXPublicKey, this function supports secp256k1 keys.
func PublicKeyToAddress(pkixPubkeyPem string) (string, error) {
	if pkixPubkeyPem == "" {
		return "", fmt.Errorf("empty PEM string")
	}

	block, _ := pem.Decode([]byte(pkixPubkeyPem))
	if block == nil {
		return "", fmt.Errorf("invalid PEM format: no PEM block found")
	}
	if block.Type != "PUBLIC KEY" {
		return "", fmt.Errorf("invalid PEM type: expected PUBLIC KEY, got %q", block.Type)
	}

	// Manually parse PKIX SubjectPublicKeyInfo to support secp256k1.
	var pkix pkixSubjectPublicKeyInfo
	_, err := asn1.Unmarshal(block.Bytes, &pkix)
	if err != nil {
		return "", fmt.Errorf("failed to parse PKIX public key: %w", err)
	}

	// Verify algorithm OID is ecPublicKey (id-ecPublicKey = 1.2.840.10045.2.1).
	var oidECPublicKey = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
	if !pkix.Algorithm.Algorithm.Equal(oidECPublicKey) {
		return "", fmt.Errorf("unsupported algorithm OID: %v", pkix.Algorithm.Algorithm)
	}

	// Verify curve parameter is secp256k1.
	if !pkix.Algorithm.Parameters.Equal(oidSecp256k1) {
		return "", fmt.Errorf("unsupported curve: %v (expected secp256k1)", pkix.Algorithm.Parameters)
	}

	// Extract uncompressed EC point from BIT STRING.
	pubKeyBytes := pkix.PublicKey.Bytes
	if len(pubKeyBytes) == 0 {
		return "", fmt.Errorf("empty public key in BIT STRING")
	}

	// BIT STRING content is: <unused bits byte (0)> || <payload>
	// For uncompressed point, first byte must be 0x04.
	firstByte := pubKeyBytes[0]
	if firstByte != 0x04 {
		return "", fmt.Errorf("unsupported public key format: expected 0x04 (uncompressed), got 0x%02x", firstByte)
	}

	// Uncompressed point: 0x04 || x (32 bytes) || y (32 bytes) = 65 bytes.
	ecPoint := pubKeyBytes
	if len(ecPoint) != 65 {
		return "", fmt.Errorf("invalid uncompressed point length: %d (expected 65)", len(ecPoint))
	}

	x := new(big.Int).SetBytes(ecPoint[1:33])
	y := new(big.Int).SetBytes(ecPoint[33:65])

	// Verify the point is on secp256k1.
	if !isOnSecp256k1(x, y) {
		return "", fmt.Errorf("public key is not on secp256k1 curve")
	}

	// Keccak-256 (Legacy, NOT NIST SHA3-256).
	hash := sha3.NewLegacyKeccak256()
	hash.Write(ecPoint)
	keccakDigest := hash.Sum(nil)
	if len(keccakDigest) != 32 {
		return "", fmt.Errorf("keccak-256 produced unexpected length: %d", len(keccakDigest))
	}

	// Take last 20 bytes + 0x41 prefix.
	addressBytes := keccakDigest[12:]
	tronPayload := append([]byte{0x41}, addressBytes...)

	// Base58Check.
	return EncodeBase58Check(tronPayload), nil
}

// EncodeBase58 encodes bytes to a base58 string.
// This is the inverse of DecodeBase58.
func EncodeBase58(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Count leading zeros
	leadingZeros := 0
	for _, b := range data {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	// Convert to big int
	n := new(big.Int).SetBytes(data)

	// Encode to base58
	const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	base := big.NewInt(58)
	result := ""
	for n.Sign() > 0 {
		mod := new(big.Int)
		n.DivMod(n, base, mod)
		result = string(base58Alphabet[mod.Int64()]) + result
	}

	// Add leading '1's for leading zero bytes
	for i := 0; i < leadingZeros; i++ {
		result = "1" + result
	}

	return result
}

// EncodeBase58Check encodes data with base58check (double SHA256 checksum).
func EncodeBase58Check(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	checksum := Sha256(Sha256(data))[:4]
	return EncodeBase58(append(data, checksum...))
}
