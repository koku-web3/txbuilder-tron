package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strings"

	log "github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-tron/grpc"
	"github.com/koku-web3/txbuilder-tron/internal/tron"
	"github.com/pkg/errors"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type TxBuilderService struct {
	grpc.UnimplementedTxBuilderServer
	rpc *tron.RPCClient
}

type GRPCServer struct {
	grpcSrv *ggrpc.Server
	host    string
	port    int
}

func NewTxBuilderService() *TxBuilderService {
	return &TxBuilderService{
		rpc: tron.NewRPCClient(),
	}
}

func NewGRPCServer(host string, port int) *GRPCServer {
	return &GRPCServer{
		host: host,
		port: port,
	}
}

func (s *GRPCServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.grpcSrv = ggrpc.NewServer()
	grpc.RegisterTxBuilderServer(s.grpcSrv, NewTxBuilderService())
	reflection.Register(s.grpcSrv)

	log.Info("Starting gRPC server", "address", addr)

	go func() {
		<-ctx.Done()
		log.Info("Shutting down gRPC server")
		s.grpcSrv.GracefulStop()
	}()

	return s.grpcSrv.Serve(lis)
}

func (s *TxBuilderService) VerifyAddress(ctx context.Context, req *grpc.VerifyAddressRequest) (*grpc.VerifyAddressResponse, error) {
	log.Info("VerifyAddress] received request", "trace_id", req.TraceId, "address", req.Address)

	if req.TraceId == "" {
		log.Warn("VerifyAddress] missing trace_id", "trace_id", req.TraceId, "address", req.Address)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.Address == "" {
		log.Warn("[VerifyAddress] missing address", "trace_id", req.TraceId, "address", req.Address)
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}

	isValid := tron.ValidateAddress(req.Address)
	log.Info("[VerifyAddress] returning response", "trace_id", req.TraceId, "address", req.Address, "is_valid", isValid)
	return &grpc.VerifyAddressResponse{IsValid: isValid}, nil
}

func (s *TxBuilderService) VerifyContractAddress(ctx context.Context, req *grpc.VerifyContractAddressRequest) (*grpc.VerifyContractAddressResponse, error) {
	log.Info("VerifyContractAddress] received request", "trace_id", req.TraceId, "address", req.Address)

	if req.TraceId == "" {
		log.Warn("VerifyContractAddress] missing trace_id", "trace_id", req.TraceId, "address", req.Address)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.Address == "" {
		log.Warn("[VerifyContractAddress] missing address", "trace_id", req.TraceId, "address", req.Address)
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}

	isValid := tron.ValidateAddress(req.Address)
	log.Info("[VerifyContractAddress] returning response", "trace_id", req.TraceId, "address", req.Address, "is_valid", isValid)
	return &grpc.VerifyContractAddressResponse{IsValid: isValid}, nil
}

func (s *TxBuilderService) ConvertAddress(ctx context.Context, req *grpc.ConvertAddressRequest) (*grpc.ConvertAddressResponse, error) {
	log.Info("[ConvertAddress] received request", "trace_id", req.TraceId, "keys_count", len(req.Keys))

	if req.TraceId == "" {
		log.Warn("[ConvertAddress] missing trace_id", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if len(req.Keys) == 0 {
		log.Warn("[ConvertAddress] missing keys", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "keys is required and cannot be empty")
	}

	addressList := make([]*grpc.PublicKeysResponse, 0, len(req.Keys))
	for _, key := range req.Keys {
		if key.PkixPubkeyPem == "" {
			log.Warn("[ConvertAddress] missing pkix_pubkey_pem", "trace_id", req.TraceId, "address_index", key.AccountIndex)
			return nil, status.Errorf(codes.InvalidArgument, "pkix_pubkey_pem is required for address_index %d", key.AccountIndex)
		}
		if len(key.PkixPubkeyPem) > 8192 {
			log.Warn("[ConvertAddress] pkix_pubkey_pem too long", "trace_id", req.TraceId, "address_index", key.AccountIndex, "pem_length", len(key.PkixPubkeyPem))
			return nil, status.Errorf(codes.InvalidArgument, "pkix_pubkey_pem exceeds maximum length of 8192 characters for address_index %d", key.AccountIndex)
		}

		address, err := tron.PublicKeyToAddress(key.PkixPubkeyPem)
		if err != nil {
			log.Warn("[ConvertAddress] failed to convert public key to address", "trace_id", req.TraceId, "address_index", key.AccountIndex, "error", err)
			return nil, status.Errorf(codes.InvalidArgument, "failed to convert public key to address for address_index %d: %v", key.AccountIndex, err)
		}

		addressList = append(addressList, &grpc.PublicKeysResponse{
			AccountIndex: key.AccountIndex,
			Address:      address,
		})
	}

	log.Info("[ConvertAddress] returning response", "trace_id", req.TraceId, "address_count", len(addressList))

	return &grpc.ConvertAddressResponse{AddressList: addressList}, nil
}

func (s *TxBuilderService) CheckSufficientBalance(ctx context.Context, req *grpc.CheckSufficientBalanceRequest) (*grpc.CheckSufficientBalanceResponse, error) {
	if err := s.validateCheckSufficientBalance(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
	if err != nil {
		log.Error("[CheckSufficientBalance] failed to get basic coin balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get balance: %v", err)
	}

	amountInt, err := parseAmount(req.Amount)
	if err != nil {
		log.Warn("[CheckSufficientBalance] invalid amount format", "trace_id", req.TraceId, "amount", req.Amount, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid amount format")
	}

	isCoinSufficient, isTokenSufficient := false, false

	isCoinSufficient = balance.Cmp(amountInt) >= 0

	// 如果只是想知道主链币余额是否足够，直接返回
	if req.IsBasicCoin {
		log.Info("[CheckSufficientBalance] basic coin balance check result", "trace_id", req.TraceId, "balance", balance.String(), "required_amount", amountInt.String(), "is_sufficient", isCoinSufficient)
		return &grpc.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient}, nil
	}

	// 继续执行查询合约余额是否足够的逻辑
	baseFee, err := s.getBaseFee()
	if err != nil {
		log.Error("[CheckSufficientBalance] failed to get base fee", "trace_id", req.TraceId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get base fee: %v", err)
	}

	// 手续费是否足够
	isCoinSufficient = balance.Cmp(baseFee) >= 0
	log.Warn("[CheckSufficientBalance] TRX fee", "trace_id", req.TraceId, "balance", balance.String(), "base_fee", baseFee.String())

	tokenBalance, err := s.getTokenBalance(ctx, req.FromAddress, req.Contract)
	if err != nil {
		log.Error("[CheckSufficientBalance] failed to get token balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "contract", req.Contract, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get token balance: %v", err)
	}

	log.Info("[CheckSufficientBalance] got token balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "contract", req.Contract, "token_balance", tokenBalance.String())

	isTokenSufficient = tokenBalance.Cmp(amountInt) >= 0

	log.Info("[CheckSufficientBalance] token balance check result", "trace_id", req.TraceId, "token_balance", tokenBalance.String(), "required_amount", amountInt.String(), "is_coin_sufficient", isCoinSufficient, "is_token_sufficient", isTokenSufficient)

	return &grpc.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient, IsTokenSufficient: isTokenSufficient}, nil
}

func (s *TxBuilderService) BuildSignRawData(ctx context.Context, req *grpc.BuildSignRawDataRequest) (*grpc.BuildSignRawDataResponse, error) {
	log.Info("[BuildSignRawData] received request", "biz_id", req.BizId, "chain_code", req.ChainCode, "coin_id", req.CoinId, "coin_symbol", req.CoinSymbol, "is_basic_coin", req.IsBasicCoin, "from_address", req.FromAddress, "to_address", req.ToAddress, "amount", req.Amount, "contract", req.Contract)

	if err := s.validateBuildSignRawData(req); err != nil {
		return nil, err
	}

	log.Info("[BuildSignRawData] validation passed, building transaction", "biz_id", req.BizId, "is_basic_coin", req.IsBasicCoin)

	var rawData []byte
	var err error

	if req.IsBasicCoin {
		rawData, err = s.buildBasicCoinTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount)
	} else {
		rawData, err = s.buildTokenTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount, req.Contract)
	}

	if err != nil {
		log.Error("[BuildSignRawData] failed to build transaction", "biz_id", req.BizId, "is_basic_coin", req.IsBasicCoin, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to build transaction: %v", err)
	}

	log.Info("[BuildSignRawData] transaction built successfully", "biz_id", req.BizId, "raw_data_length", len(rawData))

	hash := tron.Sha256(rawData)
	msg := hex.EncodeToString(hash)
	rawDataHex := hex.EncodeToString(rawData)

	resp := &grpc.BuildSignRawDataResponse{
		Msg:     msg,
		RawData: rawDataHex,
	}

	log.Info("[BuildSignRawData] returning response", "biz_id", req.BizId, "msg", msg, "raw_data_length", len(rawDataHex))

	return resp, nil
}

func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *grpc.TxBroadcastRequest) (*grpc.TxBroadcastResponse, error) {
	log.Info("[TxBroadcast] received request", "trace_id", req.TraceId, "raw_data_length", len(req.RawData), "signature_length", len(req.Signature))

	if req.TraceId == "" {
		log.Warn("[TxBroadcast] missing trace_id", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.RawData == "" {
		log.Warn("[TxBroadcast] missing raw_data", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "raw_data is required")
	}
	if len(req.RawData) < 1 || len(req.RawData) > 4096 {
		log.Warn("[TxBroadcast] invalid raw_data length", "trace_id", req.TraceId, "raw_data_length", len(req.RawData))
		return nil, status.Error(codes.InvalidArgument, "raw_data must be 1-4096 characters")
	}
	if req.Signature == "" {
		log.Warn("[TxBroadcast] missing signature", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "signature is required")
	}
	if len(req.Signature) < 1 || len(req.Signature) > 512 {
		log.Warn("[TxBroadcast] invalid signature length", "trace_id", req.TraceId, "signature_length", len(req.Signature))
		return nil, status.Error(codes.InvalidArgument, "signature must be 1-512 characters")
	}

	log.Info("[TxBroadcast] validation passed, broadcasting transaction", "trace_id", req.TraceId)

	success, err := s.rpc.BroadcastTransactionWithSignature(ctx, req.RawData, req.Signature)
	if err != nil {
		log.Error("[TxBroadcast] failed to broadcast transaction", "trace_id", req.TraceId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to broadcast transaction: %v", err)
	}

	log.Info("[TxBroadcast] returning response", "trace_id", req.TraceId, "success", success)

	return &grpc.TxBroadcastResponse{Success: success}, nil
}

func (s *TxBuilderService) getBasicCoinBalance(ctx context.Context, address string) (*big.Int, error) {
	log.Debug("[getBasicCoinBalance] getting TRX balance", "address", address)

	balance, err := s.rpc.GetBalance(ctx, address)
	if err != nil {
		log.Error("[getBasicCoinBalance] failed to get balance from node", "address", address, "error", err)
		return nil, errors.Wrap(err, "failed to get balance from node")
	}

	log.Debug("[getBasicCoinBalance] got balance from node", "address", address, "balance", balance)
	return big.NewInt(balance), nil
}

func (s *TxBuilderService) getTokenBalance(ctx context.Context, address, contractAddress string) (*big.Int, error) {
	log.Debug("[getTokenBalance] getting token balance", "address", address, "contract_address", contractAddress)

	functionSelector := "balanceOf(address)"
	addrHex := tron.Base58ToHex(address)
	parameter := fmt.Sprintf("0000000000000000000000%s", addrHex)

	log.Debug("[getTokenBalance] calling TriggerConstantContract", "address", address, "contract_address", contractAddress, "function_selector", functionSelector, "parameter", parameter)

	result, err := s.rpc.TriggerConstantContract(ctx, contractAddress, functionSelector, parameter, address)
	if err != nil {
		log.Error("[getTokenBalance] failed to trigger constant contract", "address", address, "contract_address", contractAddress, "function_selector", functionSelector, "error", err)
		return nil, errors.Wrap(err, "failed to trigger constant contract")
	}

	type resultStruct struct {
		ConstantResult []string `json:"constant_result"`
	}

	var res resultStruct
	if err := json.Unmarshal(result, &res); err != nil {
		log.Error("[getTokenBalance] failed to unmarshal token balance result", "address", address, "contract_address", contractAddress, "raw_result", string(result), "error", err)
		return nil, errors.Wrap(err, "failed to unmarshal token balance result")
	}

	if len(res.ConstantResult) == 0 {
		log.Debug("[getTokenBalance] no constant result, returning zero balance", "address", address, "contract_address", contractAddress)
		return big.NewInt(0), nil
	}

	balanceHex := strings.TrimPrefix(res.ConstantResult[0], "0x")
	balance, ok := new(big.Int).SetString(balanceHex, 16)
	if !ok {
		log.Warn("[getTokenBalance] failed to parse balance hex", "address", address, "contract_address", contractAddress, "balance_hex", balanceHex)
		return big.NewInt(0), nil
	}

	log.Debug("[getTokenBalance] got token balance", "address", address, "contract_address", contractAddress, "balance", balance.String())

	return balance, nil
}

func (s *TxBuilderService) getBaseFee() (*big.Int, error) {
	log.Debug("[getBaseFee] getting base fee")
	return big.NewInt(1000000), nil
}

func (s *TxBuilderService) buildBasicCoinTransaction(ctx context.Context, from, to, amount string) ([]byte, error) {
	log.Debug("[buildBasicCoinTransaction] building TRX transaction", "from", from, "to", to, "amount", amount)

	amountInt, err := parseAmount(amount)
	if err != nil {
		log.Error("[buildBasicCoinTransaction] invalid amount", "amount", amount, "error", err)
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

	log.Debug("[buildBasicCoinTransaction] calling wallet/createtransaction", "owner_address", owner, "to_address", receiver, "amount", amountInt.Int64())

	paramJSON, err := json.Marshal(param)
	if err != nil {
		log.Error("[buildBasicCoinTransaction] failed to marshal param", "param", param, "error", err)
		return nil, errors.Wrap(err, "failed to marshal param")
	}

	result, err := s.rpc.Call(ctx, "wallet/createtransaction", json.RawMessage(paramJSON))
	if err != nil {
		log.Error("[buildBasicCoinTransaction] failed to create transaction", "owner_address", owner, "to_address", receiver, "amount", amountInt.Int64(), "error", err)
		return nil, errors.Wrap(err, "failed to create transaction")
	}

	type txResult struct {
		RawData struct {
			Contract []json.RawMessage `json:"contract"`
		} `json:"raw_data"`
	}

	var tx txResult
	if err := json.Unmarshal(result, &tx); err != nil {
		log.Error("[buildBasicCoinTransaction] failed to unmarshal transaction", "raw_result", string(result), "error", err)
		return nil, errors.Wrap(err, "failed to unmarshal transaction")
	}

	if len(tx.RawData.Contract) == 0 {
		log.Error("[buildBasicCoinTransaction] no contract in transaction", "raw_result", string(result))
		return nil, errors.New("no contract in transaction")
	}

	log.Debug("[buildBasicCoinTransaction] transaction created successfully", "from", from, "to", to, "amount", amount, "raw_data_length", len(result))

	// Return the full raw_data which contains the contract
	return result, nil
}

func (s *TxBuilderService) buildTokenTransaction(ctx context.Context, from, to, amount, contract string) ([]byte, error) {
	log.Debug("[buildTokenTransaction] building token transaction", "from", from, "to", to, "amount", amount, "contract", contract)

	amountInt, err := parseAmount(amount)
	if err != nil {
		log.Error("[buildTokenTransaction] invalid amount", "amount", amount, "error", err)
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

	log.Debug("[buildTokenTransaction] calling wallet/triggersmartcontract", "owner_address", owner, "contract_address", contractAddr, "function_selector", "transfer(address,uint256)", "amount", amountInt.Text(16))

	paramJSON, err := json.Marshal(param)
	if err != nil {
		log.Error("[buildTokenTransaction] failed to marshal param", "param", param, "error", err)
		return nil, errors.Wrap(err, "failed to marshal param")
	}

	result, err := s.rpc.Call(ctx, "wallet/triggersmartcontract", json.RawMessage(paramJSON))
	if err != nil {
		log.Error("[buildTokenTransaction] failed to trigger smart contract", "owner_address", owner, "contract_address", contractAddr, "amount", amount, "error", err)
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
		log.Error("[buildTokenTransaction] failed to unmarshal transaction", "raw_result", string(result), "error", err)
		return nil, errors.Wrap(err, "failed to unmarshal transaction")
	}

	log.Debug("[buildTokenTransaction] token transaction built successfully", "from", from, "to", to, "amount", amount, "contract", contract, "result_length", len(result))

	// Return the full transaction result which contains raw_data with contract
	return result, nil
}

func (*TxBuilderService) validateCheckSufficientBalance(req *grpc.CheckSufficientBalanceRequest) error {
	log.Info("[CheckSufficientBalance] received request", "trace_id", req.TraceId, "chain_code", req.ChainCode, "coin_id", req.CoinId, "is_basic_coin", req.IsBasicCoin, "from_address", req.FromAddress, "amount", req.Amount, "contract", req.Contract)

	if req.TraceId == "" {
		log.Warn("[CheckSufficientBalance] missing trace_id", "trace_id", req.TraceId)
		return fmt.Errorf("trace_id is required")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		log.Warn("[CheckSufficientBalance] invalid chain_code", "trace_id", req.TraceId, "chain_code", req.ChainCode, "chain_code_length", len(req.ChainCode))
		return fmt.Errorf("chain_code is required and must be 1-36 characters")
	}
	if req.CoinId == "" || len(req.CoinId) > 36 {
		log.Warn("[CheckSufficientBalance] invalid coin_id", "trace_id", req.TraceId, "coin_id", req.CoinId, "coin_id_length", len(req.CoinId))
		return fmt.Errorf("coin_id is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		log.Warn("[CheckSufficientBalance] invalid from_address", "trace_id", req.TraceId, "from_address", req.FromAddress, "from_address_length", len(req.FromAddress))
		return fmt.Errorf("from_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		log.Warn("[CheckSufficientBalance] invalid amount", "trace_id", req.TraceId, "amount", req.Amount, "amount_length", len(req.Amount))
		return fmt.Errorf("amount is required and must be 1-64 characters")
	}
	if !req.IsBasicCoin && (req.Contract == "" || len(req.Contract) > 256) {
		log.Warn("[CheckSufficientBalance] missing contract for token", "trace_id", req.TraceId, "is_basic_coin", req.IsBasicCoin, "contract", req.Contract)
		return fmt.Errorf("contract is required when is_basic_coin is false")
	}
	return nil
}

func (*TxBuilderService) validateBuildSignRawData(req *grpc.BuildSignRawDataRequest) error {
	if req.BizId == "" || len(req.BizId) > 36 {
		log.Warn("[BuildSignRawData] invalid biz_id", "biz_id", req.BizId, "biz_id_length", len(req.BizId))
		return status.Error(codes.InvalidArgument, "biz_id is required and must be 1-36 characters")
	}

	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		log.Warn("[BuildSignRawData] invalid chain_code", "biz_id", req.BizId, "chain_code", req.ChainCode, "chain_code_length", len(req.ChainCode))
		return status.Error(codes.InvalidArgument, "chain_code is required and must be 1-36 characters")
	}

	if req.CoinId == "" || len(req.CoinId) > 36 {
		log.Warn("[BuildSignRawData] invalid coin_id", "biz_id", req.BizId, "coin_id", req.CoinId, "coin_id_length", len(req.CoinId))
		return status.Error(codes.InvalidArgument, "coin_id is required and must be 1-36 characters")
	}

	if req.CoinSymbol == "" || len(req.CoinSymbol) > 36 {
		log.Warn("[BuildSignRawData] invalid coin_symbol", "biz_id", req.BizId, "coin_symbol", req.CoinSymbol, "coin_symbol_length", len(req.CoinSymbol))
		return status.Error(codes.InvalidArgument, "coin_symbol is required and must be 1-36 characters")
	}

	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		log.Warn("[BuildSignRawData] invalid from_address", "biz_id", req.BizId, "from_address", req.FromAddress, "from_address_length", len(req.FromAddress))
		return status.Error(codes.InvalidArgument, "from_address is required and must be 1-256 characters")
	}

	if req.ToAddress == "" || len(req.ToAddress) > 256 {
		log.Warn("[BuildSignRawData] invalid to_address", "biz_id", req.BizId, "to_address", req.ToAddress, "to_address_length", len(req.ToAddress))
		return status.Error(codes.InvalidArgument, "to_address is required and must be 1-256 characters")
	}

	if req.Amount == "" || len(req.Amount) > 64 {
		log.Warn("[BuildSignRawData] invalid amount", "biz_id", req.BizId, "amount", req.Amount, "amount_length", len(req.Amount))
		return status.Error(codes.InvalidArgument, "amount is required and must be 1-64 characters")
	}

	if !req.IsBasicCoin && (req.Contract == "" || len(req.Contract) > 256) {
		log.Warn("[BuildSignRawData] missing contract for token", "biz_id", req.BizId, "is_basic_coin", req.IsBasicCoin, "contract", req.Contract)
		return status.Error(codes.InvalidArgument, "contract is required when is_basic_coin is false")
	}

	if !tron.IsPureNumber(req.Amount) {
		log.Warn("[BuildSignRawData] amount is not pure number", "biz_id", req.BizId, "amount", req.Amount)
		return status.Error(codes.InvalidArgument, "amount must be a pure number")
	}

	if !tron.ValidateAddress(req.FromAddress) {
		log.Warn("[BuildSignRawData] invalid from_address", "biz_id", req.BizId, "from_address", req.FromAddress)
		return status.Error(codes.InvalidArgument, "invalid from_address")
	}
	if !tron.ValidateAddress(req.ToAddress) {
		log.Warn("[BuildSignRawData] invalid to_address", "biz_id", req.BizId, "to_address", req.ToAddress)
		return status.Error(codes.InvalidArgument, "invalid to_address")
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
