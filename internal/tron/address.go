package tron

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"
)

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
