package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-tron/api"
	"github.com/koku-web3/txbuilder-tron/internal/config"
	"github.com/koku-web3/txbuilder-tron/internal/tron"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TxBuilderService struct {
	api.UnimplementedTxBuilderServer
	log logko.Logger
	rpc *tron.RPCClient
}

func NewTxBuilderService() *TxBuilderService {
	return &TxBuilderService{
		log: config.GetLogger(),
		rpc: tron.NewRPCClient(),
	}
}

func (s *TxBuilderService) verifyAddress(ctx context.Context, methodName, traceId, address string, validateFn func(string) bool) (bool, error) {
	s.log.Info("["+methodName+"] received request",
		"trace_id", traceId,
		"address", address,
	)

	if traceId == "" {
		s.log.Warn("["+methodName+"] missing trace_id",
			"trace_id", traceId,
			"address", address,
		)
		return false, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if address == "" {
		s.log.Warn("["+methodName+"] missing address",
			"trace_id", traceId,
			"address", address,
		)
		return false, status.Error(codes.InvalidArgument, "address is required")
	}

	isValid := validateFn(address)

	s.log.Info("["+methodName+"] returning response",
		"trace_id", traceId,
		"address", address,
		"is_valid", isValid,
	)

	return isValid, nil
}

func (s *TxBuilderService) VerifyAddress(ctx context.Context, req *api.VerifyAddressRequest) (*api.VerifyAddressResponse, error) {
	isValid, err := s.verifyAddress(ctx, "VerifyAddress", req.TraceId, req.Address, tron.ValidateAddress)
	if err != nil {
		return nil, err
	}
	return &api.VerifyAddressResponse{IsValid: isValid}, nil
}

func (s *TxBuilderService) VerifyContractAddress(ctx context.Context, req *api.VerifyContractAddressRequest) (*api.VerifyContractAddressResponse, error) {
	isValid, err := s.verifyAddress(ctx, "VerifyContractAddress", req.TraceId, req.Address, tron.ValidateContractAddress)
	if err != nil {
		return nil, err
	}
	return &api.VerifyContractAddressResponse{IsValid: isValid}, nil
}

func (s *TxBuilderService) CheckSufficientBalance(ctx context.Context, req *api.CheckSufficientBalanceRequest) (*api.CheckSufficientBalanceResponse, error) {
	s.log.Info("[CheckSufficientBalance] received request",
		"trace_id", req.TraceId,
		"chain_id", req.ChainId,
		"coin_id", req.CoinId,
		"is_basic_coin", req.IsBasicCoin,
		"from_address", req.FromAddress,
		"amount", req.Amount,
		"contract", req.Contract,
	)

	if req.TraceId == "" {
		s.log.Warn("[CheckSufficientBalance] missing trace_id",
			"trace_id", req.TraceId,
		)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.ChainId == "" || len(req.ChainId) > 36 {
		s.log.Warn("[CheckSufficientBalance] invalid chain_id",
			"trace_id", req.TraceId,
			"chain_id", req.ChainId,
			"chain_id_length", len(req.ChainId),
		)
		return nil, status.Error(codes.InvalidArgument, "chain_id is required and must be 1-36 characters")
	}
	if req.CoinId == "" || len(req.CoinId) > 36 {
		s.log.Warn("[CheckSufficientBalance] invalid coin_id",
			"trace_id", req.TraceId,
			"coin_id", req.CoinId,
			"coin_id_length", len(req.CoinId),
		)
		return nil, status.Error(codes.InvalidArgument, "coin_id is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		s.log.Warn("[CheckSufficientBalance] invalid from_address",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"from_address_length", len(req.FromAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "from_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		s.log.Warn("[CheckSufficientBalance] invalid amount",
			"trace_id", req.TraceId,
			"amount", req.Amount,
			"amount_length", len(req.Amount),
		)
		return nil, status.Error(codes.InvalidArgument, "amount is required and must be 1-64 characters")
	}
	if !req.IsBasicCoin && (req.Contract == "" || len(req.Contract) > 256) {
		s.log.Warn("[CheckSufficientBalance] missing contract for token",
			"trace_id", req.TraceId,
			"is_basic_coin", req.IsBasicCoin,
			"contract", req.Contract,
		)
		return nil, status.Error(codes.InvalidArgument, "contract is required when is_basic_coin is false")
	}

	s.log.Info("[CheckSufficientBalance] validation passed, checking balance",
		"trace_id", req.TraceId,
		"is_basic_coin", req.IsBasicCoin,
	)

	if req.IsBasicCoin {
		s.log.Info("[CheckSufficientBalance] checking basic coin (TRX) balance",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
		)

		balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
		if err != nil {
			s.log.Error("[CheckSufficientBalance] failed to get basic coin balance",
				"trace_id", req.TraceId,
				"from_address", req.FromAddress,
				"error", err,
			)
			return nil, status.Errorf(codes.Internal, "failed to get balance: %v", err)
		}

		s.log.Info("[CheckSufficientBalance] got basic coin balance",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"balance", balance.String(),
		)

		amountInt, err := parseAmount(req.Amount)
		if err != nil {
			s.log.Warn("[CheckSufficientBalance] invalid amount format",
				"trace_id", req.TraceId,
				"amount", req.Amount,
				"error", err,
			)
			return nil, status.Error(codes.InvalidArgument, "invalid amount format")
		}

		isSufficient := balance.Cmp(amountInt) >= 0

		s.log.Info("[CheckSufficientBalance] basic coin balance check result",
			"trace_id", req.TraceId,
			"balance", balance.String(),
			"required_amount", amountInt.String(),
			"is_sufficient", isSufficient,
		)

		return &api.CheckSufficientBalanceResponse{IsSufficient: isSufficient}, nil
	}

	s.log.Info("[CheckSufficientBalance] checking token balance",
		"trace_id", req.TraceId,
		"from_address", req.FromAddress,
		"contract", req.Contract,
	)

	baseFee, err := s.getBaseFee()
	if err != nil {
		s.log.Error("[CheckSufficientBalance] failed to get base fee",
			"trace_id", req.TraceId,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to get base fee: %v", err)
	}

	s.log.Info("[CheckSufficientBalance] got base fee",
		"trace_id", req.TraceId,
		"base_fee", baseFee.String(),
	)

	balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
	if err != nil {
		s.log.Error("[CheckSufficientBalance] failed to get TRX balance for fee",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to get balance: %v", err)
	}

	s.log.Info("[CheckSufficientBalance] got TRX balance for fee check",
		"trace_id", req.TraceId,
		"balance", balance.String(),
		"base_fee", baseFee.String(),
	)

	if balance.Cmp(baseFee) < 0 {
		s.log.Warn("[CheckSufficientBalance] insufficient TRX for fee",
			"trace_id", req.TraceId,
			"balance", balance.String(),
			"base_fee", baseFee.String(),
		)
		return &api.CheckSufficientBalanceResponse{IsSufficient: false}, nil
	}

	tokenBalance, err := s.getTokenBalance(ctx, req.FromAddress, req.Contract)
	if err != nil {
		s.log.Error("[CheckSufficientBalance] failed to get token balance",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"contract", req.Contract,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to get token balance: %v", err)
	}

	s.log.Info("[CheckSufficientBalance] got token balance",
		"trace_id", req.TraceId,
		"from_address", req.FromAddress,
		"contract", req.Contract,
		"token_balance", tokenBalance.String(),
	)

	amountInt, err := parseAmount(req.Amount)
	if err != nil {
		s.log.Warn("[CheckSufficientBalance] invalid amount format",
			"trace_id", req.TraceId,
			"amount", req.Amount,
			"error", err,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid amount format")
	}

	isSufficient := tokenBalance.Cmp(amountInt) >= 0

	s.log.Info("[CheckSufficientBalance] token balance check result",
		"trace_id", req.TraceId,
		"token_balance", tokenBalance.String(),
		"required_amount", amountInt.String(),
		"is_sufficient", isSufficient,
	)

	return &api.CheckSufficientBalanceResponse{IsSufficient: isSufficient}, nil
}

func (s *TxBuilderService) BuildSignRawData(ctx context.Context, req *api.BuildSignRawDataRequest) (*api.BuildSignRawDataResponse, error) {
	s.log.Info("[BuildSignRawData] received request",
		"biz_id", req.BizId,
		"chain_id", req.ChainId,
		"coin_id", req.CoinId,
		"coin_symbol", req.CoinSymbol,
		"is_basic_coin", req.IsBasicCoin,
		"from_address", req.FromAddress,
		"to_address", req.ToAddress,
		"amount", req.Amount,
		"contract", req.Contract,
	)

	if req.BizId == "" || len(req.BizId) > 36 {
		s.log.Warn("[BuildSignRawData] invalid biz_id",
			"biz_id", req.BizId,
			"biz_id_length", len(req.BizId),
		)
		return nil, status.Error(codes.InvalidArgument, "biz_id is required and must be 1-36 characters")
	}
	if req.ChainId == "" || len(req.ChainId) > 36 {
		s.log.Warn("[BuildSignRawData] invalid chain_id",
			"biz_id", req.BizId,
			"chain_id", req.ChainId,
			"chain_id_length", len(req.ChainId),
		)
		return nil, status.Error(codes.InvalidArgument, "chain_id is required and must be 1-36 characters")
	}
	if req.CoinId == "" || len(req.CoinId) > 36 {
		s.log.Warn("[BuildSignRawData] invalid coin_id",
			"biz_id", req.BizId,
			"coin_id", req.CoinId,
			"coin_id_length", len(req.CoinId),
		)
		return nil, status.Error(codes.InvalidArgument, "coin_id is required and must be 1-36 characters")
	}
	if req.CoinSymbol == "" || len(req.CoinSymbol) > 36 {
		s.log.Warn("[BuildSignRawData] invalid coin_symbol",
			"biz_id", req.BizId,
			"coin_symbol", req.CoinSymbol,
			"coin_symbol_length", len(req.CoinSymbol),
		)
		return nil, status.Error(codes.InvalidArgument, "coin_symbol is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		s.log.Warn("[BuildSignRawData] invalid from_address",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
			"from_address_length", len(req.FromAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "from_address is required and must be 1-256 characters")
	}
	if req.ToAddress == "" || len(req.ToAddress) > 256 {
		s.log.Warn("[BuildSignRawData] invalid to_address",
			"biz_id", req.BizId,
			"to_address", req.ToAddress,
			"to_address_length", len(req.ToAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "to_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		s.log.Warn("[BuildSignRawData] invalid amount",
			"biz_id", req.BizId,
			"amount", req.Amount,
			"amount_length", len(req.Amount),
		)
		return nil, status.Error(codes.InvalidArgument, "amount is required and must be 1-64 characters")
	}
	if !req.IsBasicCoin && (req.Contract == "" || len(req.Contract) > 256) {
		s.log.Warn("[BuildSignRawData] missing contract for token",
			"biz_id", req.BizId,
			"is_basic_coin", req.IsBasicCoin,
			"contract", req.Contract,
		)
		return nil, status.Error(codes.InvalidArgument, "contract is required when is_basic_coin is false")
	}

	if !tron.IsPureNumber(req.Amount) {
		s.log.Warn("[BuildSignRawData] amount is not pure number",
			"biz_id", req.BizId,
			"amount", req.Amount,
		)
		return nil, status.Error(codes.InvalidArgument, "amount must be a pure number")
	}

	if !tron.ValidateAddress(req.FromAddress) {
		s.log.Warn("[BuildSignRawData] invalid from_address",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid from_address")
	}
	if !tron.ValidateAddress(req.ToAddress) {
		s.log.Warn("[BuildSignRawData] invalid to_address",
			"biz_id", req.BizId,
			"to_address", req.ToAddress,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid to_address")
	}

	s.log.Info("[BuildSignRawData] validation passed, building transaction",
		"biz_id", req.BizId,
		"is_basic_coin", req.IsBasicCoin,
	)

	var rawData []byte
	var err error

	if req.IsBasicCoin {
		s.log.Info("[BuildSignRawData] building basic coin (TRX) transaction",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
			"to_address", req.ToAddress,
			"amount", req.Amount,
		)

		rawData, err = s.buildBasicCoinTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount)
	} else {
		s.log.Info("[BuildSignRawData] building token transaction",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
			"to_address", req.ToAddress,
			"amount", req.Amount,
			"contract", req.Contract,
		)

		rawData, err = s.buildTokenTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount, req.Contract)
	}

	if err != nil {
		s.log.Error("[BuildSignRawData] failed to build transaction",
			"biz_id", req.BizId,
			"is_basic_coin", req.IsBasicCoin,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to build transaction: %v", err)
	}

	s.log.Info("[BuildSignRawData] transaction built successfully",
		"biz_id", req.BizId,
		"raw_data_length", len(rawData),
	)

	hash := tron.Sha256(rawData)
	msg := hex.EncodeToString(hash)

	s.log.Info("[BuildSignRawData] computed sign message",
		"biz_id", req.BizId,
		"message_hash", msg,
	)

	resp := &api.BuildSignRawDataResponse{Msg: msg}

	s.log.Info("[BuildSignRawData] returning response",
		"biz_id", req.BizId,
		"msg", msg,
	)

	return resp, nil
}

func (s *TxBuilderService) getBasicCoinBalance(ctx context.Context, address string) (*big.Int, error) {
	s.log.Debug("[getBasicCoinBalance] getting TRX balance",
		"address", address,
	)

	balance, err := s.rpc.GetBalance(ctx, address)
	if err != nil {
		s.log.Error("[getBasicCoinBalance] failed to get balance from node",
			"address", address,
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to get balance from node")
	}

	s.log.Debug("[getBasicCoinBalance] got balance from node",
		"address", address,
		"balance", balance,
	)

	return big.NewInt(balance), nil
}

func (s *TxBuilderService) getTokenBalance(ctx context.Context, address, contractAddress string) (*big.Int, error) {
	s.log.Debug("[getTokenBalance] getting token balance",
		"address", address,
		"contract_address", contractAddress,
	)

	functionSelector := "balanceOf(address)"
	addrHex := tron.Base58ToHex(address)
	parameter := fmt.Sprintf("0000000000000000000000%s", addrHex)

	s.log.Debug("[getTokenBalance] calling TriggerConstantContract",
		"address", address,
		"contract_address", contractAddress,
		"function_selector", functionSelector,
		"parameter", parameter,
	)

	result, err := s.rpc.TriggerConstantContract(ctx, contractAddress, functionSelector, parameter, address)
	if err != nil {
		s.log.Error("[getTokenBalance] failed to trigger constant contract",
			"address", address,
			"contract_address", contractAddress,
			"function_selector", functionSelector,
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to trigger constant contract")
	}

	type resultStruct struct {
		ConstantResult []string `json:"constant_result"`
	}

	var res resultStruct
	if err := json.Unmarshal(result, &res); err != nil {
		s.log.Error("[getTokenBalance] failed to unmarshal token balance result",
			"address", address,
			"contract_address", contractAddress,
			"raw_result", string(result),
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to unmarshal token balance result")
	}

	if len(res.ConstantResult) == 0 {
		s.log.Debug("[getTokenBalance] no constant result, returning zero balance",
			"address", address,
			"contract_address", contractAddress,
		)
		return big.NewInt(0), nil
	}

	balanceHex := strings.TrimPrefix(res.ConstantResult[0], "0x")
	balance, ok := new(big.Int).SetString(balanceHex, 16)
	if !ok {
		s.log.Warn("[getTokenBalance] failed to parse balance hex",
			"address", address,
			"contract_address", contractAddress,
			"balance_hex", balanceHex,
		)
		return big.NewInt(0), nil
	}

	s.log.Debug("[getTokenBalance] got token balance",
		"address", address,
		"contract_address", contractAddress,
		"balance", balance.String(),
	)

	return balance, nil
}

func (s *TxBuilderService) getBaseFee() (*big.Int, error) {
	s.log.Debug("[getBaseFee] getting base fee")
	return big.NewInt(1000000), nil
}

func (s *TxBuilderService) buildBasicCoinTransaction(ctx context.Context, from, to, amount string) ([]byte, error) {
	s.log.Debug("[buildBasicCoinTransaction] building TRX transaction",
		"from", from,
		"to", to,
		"amount", amount,
	)

	amountInt, err := parseAmount(amount)
	if err != nil {
		s.log.Error("[buildBasicCoinTransaction] invalid amount",
			"amount", amount,
			"error", err,
		)
		return nil, errors.Wrap(err, "invalid amount")
	}

	owner := tron.Base58ToHex(from)
	receiver := tron.Base58ToHex(to)

	param := map[string]interface{}{
		"owner_address": owner,
		"to_address":    receiver,
		"amount":        amountInt.Int64(),
		"visible":       false,
	}

	s.log.Debug("[buildBasicCoinTransaction] calling wallet/createtransaction",
		"owner_address", owner,
		"to_address", receiver,
		"amount", amountInt.Int64(),
	)

	paramJSON, err := json.Marshal(param)
	if err != nil {
		s.log.Error("[buildBasicCoinTransaction] failed to marshal param",
			"param", param,
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to marshal param")
	}

	result, err := s.rpc.Call(ctx, "wallet/createtransaction", json.RawMessage(paramJSON))
	if err != nil {
		s.log.Error("[buildBasicCoinTransaction] failed to create transaction",
			"owner_address", owner,
			"to_address", receiver,
			"amount", amountInt.Int64(),
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to create transaction")
	}

	type txResult struct {
		RawData struct {
			Contract []json.RawMessage `json:"contract"`
		} `json:"raw_data"`
	}

	var tx txResult
	if err := json.Unmarshal(result, &tx); err != nil {
		s.log.Error("[buildBasicCoinTransaction] failed to unmarshal transaction",
			"raw_result", string(result),
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to unmarshal transaction")
	}

	if len(tx.RawData.Contract) == 0 {
		s.log.Error("[buildBasicCoinTransaction] no contract in transaction",
			"raw_result", string(result),
		)
		return nil, errors.New("no contract in transaction")
	}

	s.log.Debug("[buildBasicCoinTransaction] transaction created successfully",
		"from", from,
		"to", to,
		"amount", amount,
		"contract_length", len(tx.RawData.Contract[0]),
	)

	return tx.RawData.Contract[0], nil
}

func (s *TxBuilderService) buildTokenTransaction(ctx context.Context, from, to, amount, contract string) ([]byte, error) {
	s.log.Debug("[buildTokenTransaction] building token transaction",
		"from", from,
		"to", to,
		"amount", amount,
		"contract", contract,
	)

	amountInt, err := parseAmount(amount)
	if err != nil {
		s.log.Error("[buildTokenTransaction] invalid amount",
			"amount", amount,
			"error", err,
		)
		return nil, errors.Wrap(err, "invalid amount")
	}

	owner := tron.Base58ToHex(from)
	receiver := tron.Base58ToHex(to)
	contractAddr := tron.Base58ToHex(contract)

	parameterValue := fmt.Sprintf("0000000000000000000000%s000000000000000000000000000000000000000000000000000000000000%s",
		receiver, amountInt.Text(16))

	param := map[string]interface{}{
		"type": "TriggerSmartContract",
		"parameter": map[string]interface{}{
			"value": map[string]interface{}{
				"owner_address":     owner,
				"contract_address":  contractAddr,
				"function_selector": "transfer(address,uint256)",
				"parameter":         parameterValue,
			},
		},
		"raw_data": map[string]interface{}{
			"ref_block_bytes": "0000",
			"ref_block_hash":  "0000000000000000000000000000000000000000000000000000000000000000",
			"expiration":      0,
			"timestamp":       0,
		},
	}

	s.log.Debug("[buildTokenTransaction] calling wallet/triggersmartcontract",
		"owner_address", owner,
		"contract_address", contractAddr,
		"function_selector", "transfer(address,uint256)",
		"amount", amountInt.Text(16),
	)

	paramJSON, err := json.Marshal(param)
	if err != nil {
		s.log.Error("[buildTokenTransaction] failed to marshal param",
			"param", param,
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to marshal param")
	}

	result, err := s.rpc.Call(ctx, "wallet/triggersmartcontract", json.RawMessage(paramJSON))
	if err != nil {
		s.log.Error("[buildTokenTransaction] failed to trigger smart contract",
			"owner_address", owner,
			"contract_address", contractAddr,
			"amount", amount,
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to trigger smart contract")
	}

	type triggerResult struct {
		Transaction struct {
			RawData struct {
				Data []byte `json:"data"`
			} `json:"raw_data"`
		} `json:"transaction"`
	}

	var tx triggerResult
	if err := json.Unmarshal(result, &tx); err != nil {
		s.log.Error("[buildTokenTransaction] failed to unmarshal transaction",
			"raw_result", string(result),
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to unmarshal transaction")
	}

	s.log.Debug("[buildTokenTransaction] token transaction built successfully",
		"from", from,
		"to", to,
		"amount", amount,
		"contract", contract,
		"data_length", len(tx.Transaction.RawData.Data),
	)

	return tx.Transaction.RawData.Data, nil
}

func parseAmount(amount string) (*big.Int, error) {
	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, errors.New("invalid amount format")
	}
	return amountInt, nil
}
