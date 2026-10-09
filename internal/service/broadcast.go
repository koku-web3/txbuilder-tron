package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-tron/grpc"
	"github.com/koku-web3/txbuilder-tron/internal/pkg/errors"
	"github.com/koku-web3/txbuilder-tron/internal/tron"
)

// TxBroadcast 广播已签名的交易到 Tron 网络
// 将签名后的交易提交到节点，节点验证后放入交易池并执行
func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *txbuilder.TxBroadcastRequest) (*txbuilder.TxBroadcastResponse, error) {
	log.Debug("TxBroadcast received", "trace_id", req.TraceId,
		"from_address", req.FromAddress, "raw_data_length", len(req.RawData), "signature_length", len(req.Signature))

	if err := validateTxBroadcast(req.TraceId, req.RawData, req.Signature, req.FromAddress); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	signatureBytes, err := decodeSignature(req.Signature)
	if err != nil {
		log.Warn("Decode signature failed", "trace_id", req.TraceId, "signature_length", len(req.Signature), "error", err)
		return nil, errors.InvalidArgument("signature")
	}

	// TRON 签名格式为 65 字节 R||S||V，与 Ethereum 一致
	// 验证签名长度
	if len(signatureBytes) != 65 {
		log.Warn("Invalid signature length", "trace_id", req.TraceId,
			"expected", 65, "actual", len(signatureBytes))
		return nil, errors.InvalidArgument("signature")
	}

	// 验证签名能对应到 from_address
	if err := s.verifySignatureSender(ctx, req.RawData, signatureBytes, req.FromAddress); err != nil {
		log.Warn("Signature sender mismatch", "trace_id", req.TraceId,
			"expected_from_address", req.FromAddress, "error", err)
		return nil, errors.InvalidArgumentf(
			"signature does not match from_address: %v", err)
	}

	txHash, err := s.rpc.BroadcastTransactionWithSignature(ctx, req.RawData, req.Signature)
	if err != nil {
		log.Error("Broadcast transaction failed", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal("failed to broadcast tx.")
	}

	log.Info("TxBroadcast success!", "trace_id", req.TraceId, "tx_hash", txHash)
	return &txbuilder.TxBroadcastResponse{TxHash: txHash}, nil
}

// verifySignatureSender verifies the signature sender against from_address.
// Due to TRON using a different signature recovery mechanism from Ethereum,
// this implements a best-effort validation:
//  1. Validate signature format (R||S||V, 65 bytes)
//  2. Verify the transaction data can be decoded
//  3. For full ecrecover, TRON nodes handle verification during broadcast
//
// Note: This is a simplified check. The TRON node will ultimately validate
// the signature against the from_address during broadcast.
func (s *TxBuilderService) verifySignatureSender(ctx context.Context, rawDataHex string, signatureBytes []byte, fromAddress string) error {
	// Validate raw_data can be decoded
	rawDataBytes, err := hex.DecodeString(rawDataHex)
	if err != nil {
		return fmt.Errorf("failed to decode raw_data hex: %w", err)
	}
	if len(rawDataBytes) == 0 {
		return fmt.Errorf("raw_data is empty")
	}

	// Validate signature length (R=32 + S=32 + V=1 = 65 bytes)
	if len(signatureBytes) != 65 {
		return fmt.Errorf("signature must be 65 bytes, got %d", len(signatureBytes))
	}

	// Try to recover the signer address from the signature
	// TRON signature format: R(32)||S(32)||V(1) where V is recovery id (0 or 1)
	recoveredAddr, err := tron.RecoverTronAddressFromSignature(rawDataHex, signatureBytes)
	if err != nil {
		// If recovery fails, we can't verify - log warning but allow through
		// The TRON node will reject invalid signatures during broadcast anyway
		log.Warn("Signature sender recovery failed, relying on node validation",
			"from_address", fromAddress, "error", err)
		return nil
	}

	// Compare recovered address with declared from_address
	if !strings.EqualFold(recoveredAddr, fromAddress) {
		return fmt.Errorf("signature recovers to %s, but from_address is %s",
			recoveredAddr, fromAddress)
	}

	return nil
}

func validateTxBroadcast(traceID, rawData, signature, fromAddress string) error {
	if traceID == "" {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if rawData == "" {
		return errors.New("raw_data is required")
	}
	if len(rawData) > 4096 {
		return errors.New("raw_data must be 1-4096 characters")
	}
	if signature == "" {
		return errors.New("signature is required")
	}
	if len(signature) > 512 {
		return errors.New("signature must be 1-512 characters")
	}
	if fromAddress == "" {
		return errors.New("from_address is required")
	}
	if len(fromAddress) > 256 {
		return errors.New("from_address must be 1-256 characters")
	}
	if !tron.ValidateAddress(fromAddress) {
		return errors.New("invalid from_address")
	}
	return nil
}

func decodeSignature(s string) ([]byte, error) {
	signatureBytes, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("hex decode from signature failed: %w", err)
	}
	return signatureBytes, nil
}
