package tron

import (
	"bytes"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/sha3"
)

// isOnSecp256k1 检查 (x, y) 是否在 secp256k1 曲线上。
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

	// decoded = 前缀(1) + 地址(20) + 校验和(4) = 25 字节
	// Tron API 需要前 21 字节：前缀 + 地址 (41xxxxxxxx...)
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

// pkixAlgorithmIdentifier 表示 PKIX AlgorithmIdentifier。
type pkixAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.ObjectIdentifier `asn1:"optional"`
}

// pkixSubjectPublicKeyInfo 表示 PKIX SubjectPublicKeyInfo 结构。
// 我们手动解析它，因为 x509.ParsePKIXPublicKey 不支持 secp256k1。
type pkixSubjectPublicKeyInfo struct {
	Algorithm pkixAlgorithmIdentifier
	PublicKey asn1.BitString
}

// secp256k1 OID：1.3.132.0.10
var oidSecp256k1 = asn1.ObjectIdentifier{1, 3, 132, 0, 10}

// PublicKeyToAddress 将 PKIX-PEM 编码的公钥转换为 TRON 地址。
// 与 x509.ParsePKIXPublicKey 不同，此函数支持 secp256k1 公钥。
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

	// 手动解析 PKIX SubjectPublicKeyInfo 以支持 secp256k1。
	var pkix pkixSubjectPublicKeyInfo
	_, err := asn1.Unmarshal(block.Bytes, &pkix)
	if err != nil {
		return "", fmt.Errorf("failed to parse PKIX public key: %w", err)
	}

	// 验证算法 OID 是 ecPublicKey (id-ecPublicKey = 1.2.840.10045.2.1)。
	var oidECPublicKey = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
	if !pkix.Algorithm.Algorithm.Equal(oidECPublicKey) {
		return "", fmt.Errorf("unsupported algorithm OID: %v", pkix.Algorithm.Algorithm)
	}

	// 验证曲线参数是 secp256k1。
	if !pkix.Algorithm.Parameters.Equal(oidSecp256k1) {
		return "", fmt.Errorf("unsupported curve: %v (expected secp256k1)", pkix.Algorithm.Parameters)
	}

	// 从 BIT STRING 中提取未压缩的 EC 点。
	pubKeyBytes := pkix.PublicKey.Bytes
	if len(pubKeyBytes) == 0 {
		return "", fmt.Errorf("empty public key in BIT STRING")
	}

	// BIT STRING 内容格式：<未使用位字节 (0)> || <负载>
	// 对于未压缩点，第一个字节必须是 0x04。
	firstByte := pubKeyBytes[0]
	if firstByte != 0x04 {
		return "", fmt.Errorf("unsupported public key format: expected 0x04 (uncompressed), got 0x%02x", firstByte)
	}

	// 未压缩点：0x04 || x (32 字节) || y (32 字节) = 65 字节。
	ecPoint := pubKeyBytes
	if len(ecPoint) != 65 {
		return "", fmt.Errorf("invalid uncompressed point length: %d (expected 65)", len(ecPoint))
	}

	x := new(big.Int).SetBytes(ecPoint[1:33])
	y := new(big.Int).SetBytes(ecPoint[33:65])

	// 验证该点在 secp256k1 上。
	if !isOnSecp256k1(x, y) {
		return "", fmt.Errorf("public key is not on secp256k1 curve")
	}

	// Keccak-256（传统版本，不是 NIST SHA3-256）。
	hash := sha3.NewLegacyKeccak256()
	hash.Write(ecPoint)
	keccakDigest := hash.Sum(nil)
	if len(keccakDigest) != 32 {
		return "", fmt.Errorf("keccak-256 produced unexpected length: %d", len(keccakDigest))
	}

	// 取最后 20 字节并加上 0x41 前缀。
	addressBytes := keccakDigest[12:]
	tronPayload := append([]byte{0x41}, addressBytes...)

	// Base58Check 编码。
	return EncodeBase58Check(tronPayload), nil
}

// EncodeBase58 将字节编码为 base58 字符串。
// 这是 DecodeBase58 的逆操作。
func EncodeBase58(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// 统计前导零字节
	leadingZeros := 0
	for _, b := range data {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	// 转换为大整数
	n := new(big.Int).SetBytes(data)

	// 编码为 base58
	const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	base := big.NewInt(58)
	result := ""
	for n.Sign() > 0 {
		mod := new(big.Int)
		n.DivMod(n, base, mod)
		result = string(base58Alphabet[mod.Int64()]) + result
	}

	// 为前导零字节添加对应的 '1' 字符
	for i := 0; i < leadingZeros; i++ {
		result = "1" + result
	}

	return result
}

// EncodeBase58Check 使用 base58check（双重 SHA256 校验和）编码数据。
func EncodeBase58Check(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	checksum := Sha256(Sha256(data))[:4]
	return EncodeBase58(append(data, checksum...))
}

// RecoverPublicKey 使用 ecrecover 从 secp256k1 签名中恢复公钥。
// txIDHex 是十六进制编码的交易 ID（raw_data 的 SHA256 哈希）。
// rBytes、sBytes 是 32 字节的 R 和 S 值。vByte 是 0 或 1（y 坐标奇偶性）。
func RecoverPublicKey(txIDHex string, rBytes, sBytes []byte, vByte byte) ([]byte, error) {
	if len(rBytes) != 32 || len(sBytes) != 32 {
		return nil, fmt.Errorf("r and s must be 32 bytes each")
	}

	txID, err := hex.DecodeString(txIDHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode txID hex: %w", err)
	}
	if len(txID) != 32 {
		return nil, fmt.Errorf("txID must be 32 bytes, got %d", len(txID))
	}

	r := new(big.Int).SetBytes(rBytes)
	s := new(big.Int).SetBytes(sBytes)

	//nolint:staticcheck // SA1019: btcec.S256 是标准的 secp256k1 曲线
	curve := btcec.S256()

	// 验证 r 和 s 在范围内
	if r.Sign() == 0 || r.Cmp(curve.Params().N) >= 0 {
		return nil, fmt.Errorf("r is out of range")
	}
	if s.Sign() == 0 || s.Cmp(curve.Params().N) >= 0 {
		return nil, fmt.Errorf("s is out of range")
	}

	// 对于 secp256k1，恢复 ID 可以是 0-3（每个 r 有 4 个可能的 y 值）
	// vByte 0 或 1 映射到 recID，其中 y 的奇偶性对应
	// 在 Ethereum/TRON 约定中：vByte 表示 y 的奇偶性
	// 我们尝试 i=0 和 i=1（x = r + i*N）并选择与 vByte 奇偶性匹配的那个
	recID := int(vByte & 1)

	for i := recID; i < 4; i += 2 {
		x := new(big.Int).Add(r, new(big.Int).Mul(big.NewInt(int64(i/2)), curve.Params().N))

		// y^2 = x^3 + 7 (mod p)
		x3 := new(big.Int).Exp(x, big.NewInt(3), nil)
		ySquared := new(big.Int).Add(x3, curve.Params().B)
		ySquared.Mod(ySquared, curve.Params().P)

		y := new(big.Int).ModSqrt(ySquared, curve.Params().P)
		if y == nil {
			continue
		}

		// 检查奇偶性与 recID 匹配
		if y.Bit(0) != uint(i&1) {
			y.Sub(curve.Params().P, y)
		}

		// 验证点在曲线上
		if !isOnSecp256k1(x, y) {
			continue
		}

		// 构建未压缩公钥：0x04 || x || y
		pubKeyBytes := make([]byte, 65)
		pubKeyBytes[0] = 0x04
		xBytes := x.Bytes()
		yBytes := y.Bytes()
		copy(pubKeyBytes[33-len(xBytes):33], xBytes)
		copy(pubKeyBytes[65-len(yBytes):65], yBytes)
		return pubKeyBytes, nil
	}

	return nil, fmt.Errorf("failed to recover public key")
}

// RecoverTronAddressFromSignature 从已签名交易中恢复 TRON 地址。
// txIDHex 是原始交易数据的 SHA256 哈希（十六进制编码）。
// signatureBytes 是 R||S||V（总共 65 字节）。
// 返回从签名中恢复的 Base58Check TRON 地址。
func RecoverTronAddressFromSignature(txIDHex string, signatureBytes []byte) (string, error) {
	if len(signatureBytes) != 65 {
		return "", fmt.Errorf("signature must be 65 bytes, got %d", len(signatureBytes))
	}

	txID, err := hex.DecodeString(txIDHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode txID hex: %w", err)
	}
	if len(txID) != 32 {
		return "", fmt.Errorf("txID must be 32 bytes, got %d", len(txID))
	}

	rBytes := signatureBytes[:32]
	sBytes := signatureBytes[32:64]
	vByte := signatureBytes[64]

	// 使用 ecrecover 恢复公钥
	pubKeyBytes, err := RecoverPublicKey(hex.EncodeToString(txID), rBytes, sBytes, vByte)
	if err != nil {
		return "", fmt.Errorf("failed to recover public key: %w", err)
	}

	// 使用 Keccak-256 转换为 Tron 地址（传统版本，不是 NIST SHA3）
	//nolint:staticcheck // SA1019: TRON 使用传统 Keccak-256
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKeyBytes)
	keccakDigest := hash.Sum(nil)

	// 取最后 20 字节并加上 0x41 前缀
	addressBytes := keccakDigest[12:]
	tronPayload := append([]byte{0x41}, addressBytes...)
	return EncodeBase58Check(tronPayload), nil
}

// PublicKeyToTronAddress 将原始公钥字节转换为 Tron 地址。
// 公钥可以是 65 字节的未压缩形式（0x04 || x || y）或 33 字节的压缩形式。
func PublicKeyToTronAddress(pubKeyBytes []byte) (string, error) {
	if len(pubKeyBytes) == 0 {
		return "", fmt.Errorf("empty public key")
	}

	var x, y *big.Int

	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		// 未压缩形式：0x04 || x(32) || y(32)
		x = new(big.Int).SetBytes(pubKeyBytes[1:33])
		y = new(big.Int).SetBytes(pubKeyBytes[33:65])
	} else if len(pubKeyBytes) == 33 && (pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03) {
		// 压缩形式 - 使用曲线方程从 x 推导 y
		x = new(big.Int).SetBytes(pubKeyBytes[1:33])
		//nolint:staticcheck // SA1019: btcec.S256 是标准的 secp256k1 曲线
		curve := btcec.S256()
		// y^2 = x^3 + 7 (mod p)
		y3 := new(big.Int).Exp(x, big.NewInt(3), nil)
		y3.Add(y3, big.NewInt(7))
		y = new(big.Int).ModSqrt(y3, curve.Params().P)
		if y == nil {
			return "", fmt.Errorf("failed to derive y coordinate")
		}
		// 根据前缀字节调整奇偶性（0x02 = y 为偶数，0x03 = y 为奇数）
		if (pubKeyBytes[0] == 0x03) != (y.Bit(0) == 1) {
			y.Sub(curve.Params().P, y)
		}
	} else if len(pubKeyBytes) == 64 {
		// 原始 x||y 没有前缀
		x = new(big.Int).SetBytes(pubKeyBytes[:32])
		y = new(big.Int).SetBytes(pubKeyBytes[32:64])
	} else {
		return "", fmt.Errorf("unsupported public key length: %d", len(pubKeyBytes))
	}

	// 验证点在曲线上
	if !isOnSecp256k1(x, y) {
		return "", fmt.Errorf("public key is not on secp256k1 curve")
	}

	// 构建 65 字节未压缩点用于 Keccak-256
	ecPoint := make([]byte, 65)
	ecPoint[0] = 0x04
	copy(ecPoint[1:33], x.Bytes())
	copy(ecPoint[33:65], y.Bytes())

	// Keccak-256（传统版本，不是 NIST SHA3-256）
	//nolint:staticcheck // SA1019: gotron-sdk/gotron 使用 golang.org/x/crypto/sha3
	hash := sha3.NewLegacyKeccak256()
	hash.Write(ecPoint)
	keccakDigest := hash.Sum(nil)

	// 取最后 20 字节并加上 0x41 前缀
	addressBytes := keccakDigest[12:]
	tronPayload := append([]byte{0x41}, addressBytes...)
	return EncodeBase58Check(tronPayload), nil
}
