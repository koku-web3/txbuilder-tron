package service

import (
	"context"
	"encoding/hex"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-tron/grpc"
	"github.com/koku-web3/txbuilder-tron/internal/pkg/errors"
	"github.com/koku-web3/txbuilder-tron/internal/pkg/params"
	"github.com/koku-web3/txbuilder-tron/internal/tron"
)

// BuildSignRawData 构建交易签名原始数据
// 业务流程：
//  1. 验证请求参数（trace_id, 地址, 金额等）
//  2. 根据 contract 字段判断是原生币（TRX）还是 Token 转账
//  3. 构建交易并计算签名哈希（SHA256 of raw_data）
//  4. 返回 msg（待签名哈希）和 raw_data（TRON protobuf 编码的原始交易数据）
//
// 返回值：
//   - msg: SHA256 哈希（Hex 编码），用于外部签名
//   - raw_data: TRON protobuf 编码的交易数据（Hex 字符串），由 Coordinator 签名后用于广播
func (s *TxBuilderService) BuildSignRawData(ctx context.Context, req *txbuilder.BuildSignRawDataRequest) (*txbuilder.BuildSignRawDataResponse, error) {
	log.Debug("BuildSignRawData received", "trace_id", req.TraceId,
		"chain_code", req.ChainCode, "coin", req.Coin, "coin_symbol", req.CoinSymbol,
		"from_address", req.FromAddress, "to_address", req.ToAddress,
		"amount", req.Amount, "contract", req.Contract)

	if err := s.validateBuildSignRawData(req); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	var (
		err     error
		rawData []byte
	)

	if params.IsContractOpt(req.Contract) {
		rawData, err = s.buildTokenTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount, req.Contract)
	} else {
		rawData, err = s.buildBasicCoinTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount)
	}

	if err != nil {
		log.Error("Failed to build transaction", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal("failed to build transaction.")
	}

	hash := tron.Sha256(rawData)
	msg := hex.EncodeToString(hash)
	rawDataHex := hex.EncodeToString(rawData)

	log.Info("BuildSignRawData success", "trace_id", req.TraceId,
		"msg_length", len(msg), "raw_data_length", len(rawDataHex))
	return &txbuilder.BuildSignRawDataResponse{Msg: msg, RawData: rawDataHex}, nil
}

// buildBasicCoinTransaction 构建原生币（TRX）转账交易
// 返回值：
//   - rawData: TRON protobuf TransactionRaw（hex 编码）
func (s *TxBuilderService) buildBasicCoinTransaction(ctx context.Context, from, to, amount string) ([]byte, error) {
	log.Debug("Building TRX transfer", "from", from, "to", to, "amount", amount)
	rawData, err := tron.BuildBasicCoinRawData(ctx, s.rpc, from, to, amount)
	if err != nil {
		log.Error("Failed to build TRX transfer", "from", from, "to", to, "amount", amount, "error", err)
		return nil, errors.Wrap(err, "failed to build basic coin transaction")
	}
	log.Debug("Transaction built", "from", from, "raw_data_length", len(rawData))
	return rawData, nil
}

// buildTokenTransaction 构建 TRC20 Token 转账交易
// 返回值：
//   - rawData: TRON protobuf TransactionRaw（hex 编码）
func (s *TxBuilderService) buildTokenTransaction(ctx context.Context, from, to, amount, contract string) ([]byte, error) {
	log.Debug("Building TRC20 transfer", "from", from, "to", to, "amount", amount, "contract", contract)
	// FeeLimit = 30 TRX = 30,000,000 SUN
	rawData, err := tron.BuildTokenRawData(ctx, s.rpc, from, to, amount, contract, tron.DefaultTRC20FeeLimit)
	if err != nil {
		log.Error("Failed to build TRC20 transfer", "from", from, "to", to, "amount", amount, "contract", contract, "error", err)
		return nil, errors.Wrap(err, "failed to build token transaction")
	}
	log.Debug("Transaction built", "from", from, "raw_data_length", len(rawData))
	return rawData, nil
}

func (s *TxBuilderService) validateBuildSignRawData(req *txbuilder.BuildSignRawDataRequest) error {
	if req.TraceId == "" || len(req.TraceId) > 36 {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		return errors.New("chain_code is required and must be 1-36 characters")
	}
	if req.Coin == "" || len(req.Coin) > 36 {
		return errors.New("coin_id is required and must be 1-36 characters")
	}
	if req.CoinSymbol == "" || len(req.CoinSymbol) > 36 {
		return errors.New("coin_symbol is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		return errors.New("from_address is required and must be 1-256 characters")
	}
	if req.ToAddress == "" || len(req.ToAddress) > 256 {
		return errors.New("to_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		return errors.New("amount is required and must be 1-64 characters")
	}
	if !tron.IsPureNumber(req.Amount) {
		return errors.New("amount must be a pure number")
	}
	if !tron.ValidateAddress(req.FromAddress) {
		return errors.New("invalid from_address")
	}
	if !tron.ValidateAddress(req.ToAddress) {
		return errors.New("invalid to_address")
	}
	return nil
}
