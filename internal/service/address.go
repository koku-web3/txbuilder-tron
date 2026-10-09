package service

import (
	"context"
	"strings"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-tron/grpc"
	"github.com/koku-web3/txbuilder-tron/internal/pkg/errors"
	"github.com/koku-web3/txbuilder-tron/internal/tron"
)

// VerifyAddress 验证 TRON 地址格式（Base58Check 编码，以 T 开头）
func (s *TxBuilderService) VerifyAddress(ctx context.Context, req *txbuilder.VerifyAddressRequest) (*txbuilder.VerifyAddressResponse, error) {
	log.Debug("VerifyAddress received", "trace_id", req.TraceId, "address", req.Address)

	if err := verifyAddressBasic(req.TraceId, req.Address); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	isValid := tron.ValidateAddress(req.Address)
	log.Info("VerifyAddress success!", "trace_id", req.TraceId, "address", req.Address, "is_valid", isValid)
	return &txbuilder.VerifyAddressResponse{IsValid: isValid}, nil
}

// VerifyContractAddress 验证 TRON 合约地址格式
// 合约地址与普通地址格式相同（Base58Check），使用相同的校验逻辑
func (s *TxBuilderService) VerifyContractAddress(ctx context.Context, req *txbuilder.VerifyContractAddressRequest) (*txbuilder.VerifyContractAddressResponse, error) {
	log.Debug("VerifyContractAddress received", "trace_id", req.TraceId, "address", req.Address)

	if err := verifyAddressBasic(req.TraceId, req.Address); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	isValid := tron.ValidateContractAddress(req.Address)
	log.Info("VerifyContractAddress success!", "trace_id", req.TraceId, "contract", req.Address, "is_valid", isValid)
	return &txbuilder.VerifyContractAddressResponse{IsValid: isValid}, nil
}

// ConvertAddress 将 PEM-encoded PKIX 格式公钥转换为 Tron 地址
func (s *TxBuilderService) ConvertAddress(ctx context.Context, req *txbuilder.ConvertAddressRequest) (*txbuilder.ConvertAddressResponse, error) {
	log.Debug("ConvertAddress received", "trace_id", req.TraceId, "keys_count", len(req.Keys))

	if err := validateConvertAddress(req); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	results := make([]*txbuilder.PublicKeysResponse, 0, len(req.Keys))
	for _, key := range req.Keys {
		addr, err := tron.PublicKeyToAddress(key.PkixPubkeyPem)
		if err != nil {
			log.Error("Failed to convert public key", "trace_id", req.TraceId, "account_index", key.AccountIndex, "error", err)
			return nil, errors.InvalidArgument("pkix_pubkey_pem")
		}
		results = append(results, &txbuilder.PublicKeysResponse{
			AccountIndex: key.AccountIndex,
			Address:      addr,
		})
	}

	log.Info("ConvertAddress success!", "trace_id", req.TraceId, "success_count", len(results))
	return &txbuilder.ConvertAddressResponse{Keys: results}, nil
}

// verifyAddressBasic 验证地址的基本校验逻辑
func verifyAddressBasic(traceID, address string) error {
	if traceID == "" {
		return errors.New("trace_id is required")
	}
	if address == "" {
		return errors.New("address is required")
	}
	return nil
}

func validateConvertAddress(req *txbuilder.ConvertAddressRequest) error {
	if req.TraceId == "" || len(req.TraceId) > 36 {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if len(req.Keys) == 0 {
		return errors.New("keys is required")
	}
	if len(req.Keys) > 100 {
		return errors.New("keys length exceeds maximum of 100")
	}
	for _, key := range req.Keys {
		if strings.TrimSpace(key.PkixPubkeyPem) == "" {
			return errors.New("pkix_pubkey_pem is required and must not be empty")
		}
		if len(key.PkixPubkeyPem) > 4096 {
			return errors.New("pkix_pubkey_pem exceeds maximum length of 4096 characters")
		}
	}
	return nil
}
