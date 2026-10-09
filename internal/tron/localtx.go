package tron

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"

	//nolint:staticcheck // gotron-sdk uses github.com/golang/protobuf; must use same package to unmarshal its types.
	proto "github.com/golang/protobuf/proto"
	gogotypes "github.com/golang/protobuf/ptypes/any"
	log "github.com/koku-web3/logko"
)

// RefBlockFetcher 是获取最新区块头的接口。
// RPCClient 通过 GetNowBlock 实现此接口。
type RefBlockFetcher interface {
	GetNowBlock(ctx context.Context) (json.RawMessage, error)
}

// RefBlockInfo 保存从最新区块派生出的 ref_block_bytes 和 ref_block_hash，
// 用于本地构造交易。
type RefBlockInfo struct {
	RefBlockBytes []byte // 2 bytes: big-endian block_num [6:8]
	RefBlockHash  []byte // 8 bytes: blockID [8:16]
	BlockNum      int64
}

// FetchRefBlock 调用 GetNowBlock 并派生 ref_block_bytes / ref_block_hash。
func FetchRefBlock(ctx context.Context, fetcher RefBlockFetcher) (*RefBlockInfo, error) {
	result, err := fetcher.GetNowBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get now block: %w", err)
	}

	type blockHeader struct {
		RawData struct {
			Number int64 `json:"number"`
		} `json:"raw_data"`
	}
	type blockResp struct {
		BlockID     string      `json:"blockID"`
		BlockHeader blockHeader `json:"block_header"`
	}

	var block blockResp
	if err := json.Unmarshal(result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal now block response: %w", err)
	}

	if block.BlockID == "" {
		return nil, fmt.Errorf("blockID is empty in now block response")
	}

	blockNum := block.BlockHeader.RawData.Number

	// ref_block_bytes: 大端序的区块号，取字节 [6:8]
	numBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numBytes, uint64(blockNum))
	refBlockBytes := numBytes[6:8]

	// ref_block_hash: blockID 的十六进制 [8:16]
	blockIDHex := strings.TrimPrefix(block.BlockID, "0x")
	blockIDBytes, err := hex.DecodeString(blockIDHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode blockID hex: %w", err)
	}

	var refBlockHash []byte
	if len(blockIDBytes) >= 16 {
		refBlockHash = blockIDBytes[8:16]
	} else if len(blockIDBytes) == 8 {
		refBlockHash = blockIDBytes
	} else if len(blockIDBytes) > 0 {
		// 备用方案：取最后 8 个字节
		refBlockHash = blockIDBytes[len(blockIDBytes)-8:]
	} else {
		return nil, fmt.Errorf("blockID bytes length too short: %d", len(blockIDBytes))
	}

	if len(refBlockHash) != 8 {
		return nil, fmt.Errorf("ref_block_hash length is not 8: got %d", len(refBlockHash))
	}

	log.Debug("Fetch ref block completed", "block_num", blockNum, "ref_block_bytes", hex.EncodeToString(refBlockBytes), "ref_block_hash", hex.EncodeToString(refBlockHash))

	return &RefBlockInfo{
		RefBlockBytes: refBlockBytes,
		RefBlockHash:  refBlockHash,
		BlockNum:      blockNum,
	}, nil
}

// parseAmountInt64 解析十进制字符串金额（TRX 的 SUN）并返回 int64。
// 如果金额不是有效的非负整数则返回错误。
func parseAmountInt64(amount string) (int64, error) {
	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return 0, fmt.Errorf("invalid amount format: %s", amount)
	}
	if amountInt.Sign() < 0 {
		return 0, fmt.Errorf("amount must be non-negative: %s", amount)
	}
	if !amountInt.IsInt64() {
		return 0, fmt.Errorf("amount overflows int64: %s", amount)
	}
	return amountInt.Int64(), nil
}

// trc20TransferMethodSignature 是 ERC20/TRC20 转账的函数选择器（不带 0x 前缀）。
const trc20TransferMethodSignature = "a9059cbb"

// ExpirationDuration 是交易过期时间（5 分钟，以毫秒为单位）。
const ExpirationDuration int64 = 5 * 60 * 1000

// DefaultTRC20FeeLimit 是 TRC20 转账的默认费用上限（30 TRX = 30,000,000 SUN）。
const DefaultTRC20FeeLimit int64 = 30_000_000

// BuildBasicCoinRawData 本地构造 TRX 转账交易。
func BuildBasicCoinRawData(ctx context.Context, fetcher RefBlockFetcher, from, to, amount string) ([]byte, error) {
	amountInt64, err := parseAmountInt64(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	fromDesc, err := address.Base58ToAddress(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from address %s: %w", from, err)
	}
	toDesc, err := address.Base58ToAddress(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to address %s: %w", to, err)
	}

	// 构建 TransferContract
	tc := &core.TransferContract{
		OwnerAddress: fromDesc.Bytes(),
		ToAddress:    toDesc.Bytes(),
		Amount:       amountInt64,
	}

	tcBytes, err := proto.Marshal(tc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TransferContract: %w", err)
	}

	parameter := &gogotypes.Any{
		TypeUrl: "/protocol.TransferContract",
		Value:   tcBytes,
	}

	// 获取引用区块
	ref, err := FetchRefBlock(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ref block: %w", err)
	}

	now := time.Now().UnixMilli()

	rawData := &core.TransactionRaw{
		RefBlockBytes: ref.RefBlockBytes,
		RefBlockHash:  ref.RefBlockHash,
		Contract: []*core.Transaction_Contract{
			{
				Type:      core.Transaction_Contract_TransferContract,
				Parameter: parameter,
			},
		},
		Timestamp:  now,
		Expiration: now + ExpirationDuration,
		FeeLimit:   0,
	}

	rawBytes, err := proto.Marshal(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TransactionRaw: %w", err)
	}

	return rawBytes, nil
}

// BuildTokenRawData 本地构造 TRC20 代币转账交易。
func BuildTokenRawData(ctx context.Context, fetcher RefBlockFetcher, from, to, amount, contract string, feeLimit int64) ([]byte, error) {
	amountInt64, err := parseAmountInt64(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	fromDesc, err := address.Base58ToAddress(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from address %s: %w", from, err)
	}
	toDesc, err := address.Base58ToAddress(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to address %s: %w", to, err)
	}
	contractDesc, err := address.Base58ToAddress(contract)
	if err != nil {
		return nil, fmt.Errorf("invalid contract address %s: %w", contract, err)
	}

	// 构建 TRC20 转账调用数据
	// transfer(address,uint256) 选择器 + 地址填充 + 金额填充
	// toDesc.Hex() = "0x41..." (44 字符 = 0x + 21 字节 = 42 十六进制)。取最后 20 字节 = 最后 40 个十六进制字符。
	addrHex := toDesc.Hex()                // "0x41..." (44 字符)
	addr20Hex := addrHex[len(addrHex)-40:] // 最后 40 个十六进制字符 = 20 字节地址，不带 0x 前缀
	addrPadded := "000000000000000000000000" + addr20Hex
	amountHex := common.Bytes2Hex(common.LeftPadBytes(new(big.Int).SetInt64(amountInt64).Bytes(), 32))
	dataHex := trc20TransferMethodSignature + addrPadded + amountHex

	data, err := common.FromHex(dataHex)
	if err != nil {
		return nil, fmt.Errorf("failed to build TRC20 calldata: %w", err)
	}

	// 构建 TriggerSmartContract
	tsc := &core.TriggerSmartContract{
		OwnerAddress:    fromDesc.Bytes(),
		ContractAddress: contractDesc.Bytes(),
		Data:            data,
		CallValue:       0,
	}

	tscBytes, err := proto.Marshal(tsc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TriggerSmartContract: %w", err)
	}

	parameter := &gogotypes.Any{
		TypeUrl: "/protocol.TriggerSmartContract",
		Value:   tscBytes,
	}

	// 获取引用区块
	ref, err := FetchRefBlock(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ref block: %w", err)
	}

	now := time.Now().UnixMilli()

	rawData := &core.TransactionRaw{
		RefBlockBytes: ref.RefBlockBytes,
		RefBlockHash:  ref.RefBlockHash,
		Contract: []*core.Transaction_Contract{
			{
				Type:      core.Transaction_Contract_TriggerSmartContract,
				Parameter: parameter,
			},
		},
		Timestamp:  now,
		Expiration: now + ExpirationDuration,
		FeeLimit:   feeLimit,
	}

	rawBytes, err := proto.Marshal(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TransactionRaw: %w", err)
	}

	return rawBytes, nil
}
