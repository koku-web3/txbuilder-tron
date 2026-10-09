package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-tron/grpc"
	"github.com/koku-web3/txbuilder-tron/internal/pkg/errors"
	"github.com/koku-web3/txbuilder-tron/internal/pkg/params"
	"github.com/koku-web3/txbuilder-tron/internal/tron"
)

// CheckSufficientBalance 检查地址余额是否足够发起转账
// - contract 为空: 检查 TRX 余额是否 >= 转账金额
// - contract 非空: 先检查 TRX 余额是否足够支付 FeeLimit，再检查 Token 余额是否 >= 转账金额
func (s *TxBuilderService) CheckSufficientBalance(ctx context.Context, req *txbuilder.CheckSufficientBalanceRequest) (*txbuilder.CheckSufficientBalanceResponse, error) {
	log.Debug("CheckSufficientBalance received", "trace_id", req.TraceId,
		"chain_code", req.ChainCode, "coin", req.Coin,
		"from_address", req.FromAddress, "amount", req.Amount, "contract", req.Contract)

	if err := s.validateCheckSufficientBalance(req); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
	if err != nil {
		log.Error("Failed to get balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "error", err)
		return nil, errors.Internal("failed to get balance.")
	}

	amountInt, err := parseAmount(req.Amount)
	if err != nil {
		log.Warn("Invalid amount format", "trace_id", req.TraceId, "amount", req.Amount, "error", err)
		return nil, errors.InvalidArgument("amount")
	}

	isContract := params.IsContractOpt(req.Contract)

	if !isContract {
		// 原生币转账: 余额需要 >= 金额
		isCoinSufficient := balance.Cmp(amountInt) >= 0
		log.Info("CheckSufficientBalance success", "trace_id", req.TraceId,
			"balance", balance.String(), "required", amountInt.String(), "is_coin_sufficient", isCoinSufficient)
		return &txbuilder.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient}, nil
	}

	// Token 转账: 需要 TRX 支付 FeeLimit，同时需要 Token 余额足够
	if !tron.ValidateContractAddress(req.Contract) {
		log.Warn("Invalid contract address", "trace_id", req.TraceId, "contract", req.Contract)
		return nil, errors.InvalidArgument("contract")
	}

	baseFee, err := s.getBaseFee()
	if err != nil {
		log.Error("Failed to get base fee", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal("failed to get base fee.")
	}

	// 检查 TRX 余额是否足够支付 FeeLimit
	isCoinSufficient := balance.Cmp(baseFee) >= 0
	log.Debug("CheckSufficientBalance TRX fee check", "trace_id", req.TraceId,
		"balance", balance.String(), "base_fee", baseFee.String(), "is_coin_sufficient", isCoinSufficient)

	tokenBalance, err := s.getTokenBalance(ctx, req.FromAddress, req.Contract)
	if err != nil {
		log.Error("Failed to get token balance", "trace_id", req.TraceId,
			"from_address", req.FromAddress, "contract", req.Contract, "error", err)
		return nil, errors.Internal("failed to get token balance.")
	}

	// amount 已是链上最小单位，与 balanceOf 返回值直接比较
	isTokenSufficient := tokenBalance.Cmp(amountInt) >= 0

	log.Info("CheckSufficientBalance success", "trace_id", req.TraceId,
		"token_balance", tokenBalance.String(), "required_amount", amountInt.String(),
		"is_coin_sufficient", isCoinSufficient, "is_token_sufficient", isTokenSufficient)
	return &txbuilder.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient, IsTokenSufficient: isTokenSufficient}, nil
}

// GetBalance 获取指定地址的主链币余额或合约代币余额
func (s *TxBuilderService) GetBalance(ctx context.Context, req *txbuilder.GetBalanceRequest) (*txbuilder.GetBalanceResponse, error) {
	log.Debug("GetBalance received", "trace_id", req.TraceId, "address", req.Address, "contract", req.Contract)

	if !tron.ValidateAddress(req.Address) {
		log.Warn("Invalid address", "trace_id", req.TraceId, "address", req.Address)
		return nil, errors.InvalidArgument("address")
	}

	var balance *big.Int
	var err error
	if params.IsContractOpt(req.Contract) {
		if !tron.ValidateContractAddress(req.Contract) {
			log.Warn("Invalid contract", "trace_id", req.TraceId, "contract", req.Contract)
			return nil, errors.InvalidArgument("contract")
		}
		balance, err = s.getTokenBalanceInt(ctx, req.Address, req.Contract)
	} else {
		bal, err := s.rpc.GetBalance(ctx, req.Address)
		if err != nil {
			log.Error("Get balance failed", "trace_id", req.TraceId, "error", err)
			return nil, errors.InternalSilent()
		}
		balance = big.NewInt(bal)
	}
	if err != nil {
		log.Error("Get balance failed", "trace_id", req.TraceId, "error", err)
		return nil, errors.InternalSilent()
	}
	log.Info("GetBalance success!", "trace_id", req.TraceId,
		"address", req.Address, "contract", req.Contract, "balance_amount", balance.String())
	return &txbuilder.GetBalanceResponse{Amount: balance.String()}, nil
}

func (*TxBuilderService) validateCheckSufficientBalance(req *txbuilder.CheckSufficientBalanceRequest) error {
	if req.TraceId == "" {
		return errors.New("trace_id is required")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		return errors.New("chain_code is required and must be 1-36 characters")
	}
	if req.Coin == "" || len(req.Coin) > 36 {
		return errors.New("coin_id is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		return errors.New("from_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		return errors.New("amount is required and must be 1-64 characters")
	}
	return nil
}

func parseAmount(amount string) (*big.Int, error) {
	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, errors.New("invalid amount format")
	}
	return amountInt, nil
}

func (s *TxBuilderService) getBasicCoinBalance(ctx context.Context, address string) (*big.Int, error) {
	log.Debug("Get basic coin balance", "address", address)
	balance, err := s.rpc.GetBalance(ctx, address)
	if err != nil {
		log.Error("Failed to get balance from node", "address", address, "error", err)
		return nil, errors.Wrap(err, "failed to get balance from node")
	}
	log.Debug("Get basic coin balance from node", "address", address, "balance", balance)
	return big.NewInt(balance), nil
}

func (s *TxBuilderService) getTokenBalance(ctx context.Context, address, contractAddress string) (*big.Int, error) {
	log.Debug("Get token balance", "address", address, "contract_address", contractAddress)
	functionSelector := "balanceOf(address)"
	addrHex := tron.Base58ToHex(address)
	parameter := fmt.Sprintf("0000000000000000000000%s", addrHex)
	log.Debug("Calling trigger constant contract", "address", address,
		"contract_address", contractAddress, "function_selector", functionSelector, "parameter", parameter)
	result, err := s.rpc.TriggerConstantContract(ctx, contractAddress, functionSelector, parameter, address)
	if err != nil {
		log.Error("Failed to trigger constant contract", "address", address,
			"contract_address", contractAddress, "error", err)
		return nil, errors.Wrap(err, "failed to trigger constant contract")
	}
	type resultStruct struct {
		ConstantResult []string `json:"constant_result"`
	}
	var res resultStruct
	if err := json.Unmarshal(result, &res); err != nil {
		log.Error("Failed to unmarshal token balance result", "address", address,
			"contract_address", contractAddress, "raw_result", string(result), "error", err)
		return nil, errors.Wrap(err, "failed to unmarshal token balance result")
	}
	if len(res.ConstantResult) == 0 {
		log.Debug("No constant result, returning zero balance", "address", address, "contract_address", contractAddress)
		return big.NewInt(0), nil
	}
	balanceHex := strings.TrimPrefix(res.ConstantResult[0], "0x")
	balance, ok := new(big.Int).SetString(balanceHex, 16)
	if !ok {
		log.Warn("Failed to parse balance hex", "address", address,
			"contract_address", contractAddress, "balance_hex", balanceHex)
		return big.NewInt(0), nil
	}
	log.Debug("Got token balance", "address", address,
		"contract_address", contractAddress, "balance", balance.String())
	return balance, nil
}

func (s *TxBuilderService) getTokenBalanceInt(ctx context.Context, address, contractAddress string) (*big.Int, error) {
	return s.getTokenBalance(ctx, address, contractAddress)
}

func (s *TxBuilderService) getBaseFee() (*big.Int, error) {
	log.Debug("Get base fee")
	// TRON TRC20 最小 FeeLimit 为 1,000,000 SUN = 1 TRX
	return big.NewInt(1_000_000), nil
}
