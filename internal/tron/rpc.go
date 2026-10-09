package tron

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-tron/internal/config"
)

type RPCClient struct {
	rpcURL string
	client *http.Client
}

func NewRPCClient() *RPCClient {
	cfg := config.GetConfig()
	return &RPCClient{
		rpcURL: strings.TrimSuffix(cfg.Chain.RPCURL, "/"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *RPCClient) call(ctx context.Context, path string, params interface{}) (json.RawMessage, error) {
	var body []byte
	var err error

	if params != nil {
		if raw, ok := params.(json.RawMessage); ok {
			body = raw
		} else {
			body, err = json.Marshal(params)
			if err != nil {
				log.Error("RPC client call failed to marshal params", "path", path, "params_type", fmt.Sprintf("%T", params), "error", err)
				return nil, fmt.Errorf("failed to marshal params: %w", err)
			}
		}
	}

	url := c.rpcURL + "/" + path

	log.Debug("RPC client call preparing request", "url", url, "path", path, "body_length", len(body))

	if len(body) > 0 {
		var prettyJSON map[string]interface{}
		if err := json.Unmarshal(body, &prettyJSON); err == nil {
			log.Debug("RPC client call request body", "url", url, "body", prettyJSON)
		} else {
			log.Debug("RPC client call request body raw", "url", url, "body_raw", string(body))
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Error("RPC client call failed to create request", "url", url, "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Debug("RPC client call sending HTTP request", "url", url, "method", http.MethodPost)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error("RPC client call failed to do request", "url", url, "error", err)
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	log.Debug("RPC client call received HTTP response", "url", url, "status_code", resp.StatusCode, "status", resp.Status)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("RPC client call failed to read response body", "url", url, "status_code", resp.StatusCode, "error", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug("RPC client call response body received", "url", url, "status_code", resp.StatusCode, "response_body_length", len(respBody))

	if resp.StatusCode != http.StatusOK {
		log.Error("RPC client call RPC call failed with non-200 status", "url", url, "path", path, "status_code", resp.StatusCode, "response_body", string(respBody))
		return nil, fmt.Errorf("rpc call failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	type tronError struct {
		Error string `json:"Error"`
	}
	var terr tronError
	if err := json.Unmarshal(respBody, &terr); err == nil && terr.Error != "" {
		log.Error("RPC client call Tron RPC returned error", "url", url, "path", path, "tron_error", terr.Error)
		return nil, fmt.Errorf("tron rpc error: %s", terr.Error)
	}

	log.Debug("RPC client call RPC call successful", "url", url, "path", path, "response_body_length", len(respBody))

	return respBody, nil
}

func (c *RPCClient) GetAccount(ctx context.Context, address string) (json.RawMessage, error) {
	log.Debug("RPC get account", "address", address)

	params := map[string]interface{}{
		"address": address,
		"visible": true,
	}
	result, err := c.call(ctx, "wallet/getaccount", params)
	if err != nil {
		log.Error("RPC get account failed", "address", address, "error", err)
		return nil, err
	}

	log.Debug("RPC get account completed", "address", address, "result_length", len(result))

	return result, nil
}

func (c *RPCClient) GetBalance(ctx context.Context, address string) (int64, error) {
	log.Debug("RPC get balance", "address", address)

	type accountResp struct {
		Balance int64 `json:"balance"`
	}

	result, err := c.call(ctx, "wallet/getaccount", map[string]interface{}{
		"address": address,
		"visible": true,
	})
	if err != nil {
		log.Error("RPC get balance failed", "address", address, "error", err)
		return 0, err
	}

	var acc accountResp
	if err := json.Unmarshal(result, &acc); err != nil {
		log.Error("RPC get balance failed to unmarshal account", "address", address, "raw_result", string(result), "error", err)
		return 0, fmt.Errorf("failed to unmarshal account: %w", err)
	}

	log.Debug("RPC get balance completed", "address", address, "balance", acc.Balance)

	return acc.Balance, nil
}

func (c *RPCClient) TriggerConstantContract(ctx context.Context, contractAddress, functionSelector, parameter, ownerAddress string) (json.RawMessage, error) {
	log.Debug("Trigger constant contract", "contract_address", contractAddress, "function_selector", functionSelector, "parameter", parameter, "owner_address", ownerAddress)

	args := map[string]interface{}{
		"contract_address":  contractAddress,
		"function_selector": functionSelector,
		"parameter":         parameter,
		"visible":           true,
		"owner_address":     ownerAddress,
	}

	result, err := c.call(ctx, "wallet/triggerconstantcontract", args)
	if err != nil {
		log.Error("Trigger constant contract failed", "contract_address", contractAddress, "function_selector", functionSelector, "error", err)
		return nil, err
	}

	type rpcResponse struct {
		ConstantResult []string `json:"constant_result"`
		Result         struct {
			Result bool `json:"result"`
		} `json:"result"`
	}
	var resp rpcResponse
	if len(result) > 0 {
		_ = json.Unmarshal(result, &resp)
		log.Debug("Trigger constant contract RPC response", "contract_address", contractAddress, "function_selector", functionSelector, "result_success", resp.Result.Result, "constant_result_count", len(resp.ConstantResult))
	}

	return result, nil
}

func (c *RPCClient) GetTransactionInfoById(ctx context.Context, txID string) (json.RawMessage, error) {
	log.Debug("RPC get transaction info by id", "tx_id", txID)

	result, err := c.call(ctx, "wallet/gettransactioninfobyid", map[string]interface{}{
		"value": txID,
	})
	if err != nil {
		log.Error("RPC get transaction info by id failed", "tx_id", txID, "error", err)
		return nil, err
	}

	log.Debug("RPC get transaction info by id completed", "tx_id", txID, "result_length", len(result))

	return result, nil
}

func (c *RPCClient) GetContract(ctx context.Context, contractAddress string) (json.RawMessage, error) {
	log.Debug("RPC get contract", "contract_address", contractAddress)

	result, err := c.call(ctx, "wallet/getcontract", map[string]interface{}{
		"address": contractAddress,
		"visible": true,
	})
	if err != nil {
		log.Error("RPC get contract failed", "contract_address", contractAddress, "error", err)
		return nil, err
	}

	log.Debug("RPC get contract completed", "contract_address", contractAddress, "result_length", len(result))

	return result, nil
}

func (c *RPCClient) GetNowBlock(ctx context.Context) (json.RawMessage, error) {
	log.Debug("RPC get now block")

	result, err := c.call(ctx, "wallet/getnowblock", nil)
	if err != nil {
		log.Error("RPC get now block failed", "error", err)
		return nil, err
	}

	log.Debug("RPC get now block completed", "result_length", len(result))

	return result, nil
}

func (c *RPCClient) Call(ctx context.Context, path string, params json.RawMessage) (json.RawMessage, error) {
	log.Debug("RPC call", "path", path, "params_length", len(params))

	result, err := c.call(ctx, path, params)
	if err != nil {
		log.Error("RPC call failed", "path", path, "error", err)
		return nil, err
	}

	log.Debug("RPC call completed", "path", path, "result_length", len(result))

	return result, nil
}

func (c *RPCClient) BroadcastTransaction(ctx context.Context, transactionHex string) (bool, error) {
	log.Debug("Broadcast transaction", "transaction_hex_length", len(transactionHex))

	txData, err := hex.DecodeString(transactionHex)
	if err != nil {
		log.Error("Broadcast transaction failed to decode transaction hex", "error", err)
		return false, fmt.Errorf("failed to decode transaction hex: %w", err)
	}

	// 解码 protobuf 交易以提取 raw_data JSON 和签名
	rawDataJSON, signatureBytes, err := decodeSignedTransaction(txData)
	if err != nil {
		log.Error("Broadcast transaction failed to decode transaction", "error", err)
		return false, fmt.Errorf("failed to decode transaction: %w", err)
	}

	log.Debug("Broadcast transaction decoded", "raw_data_json_length", len(rawDataJSON), "signature_count", len(signatureBytes))

	// 将签名转换为十六进制字符串
	signatures := make([]string, len(signatureBytes))
	for i, sig := range signatureBytes {
		signatures[i] = hex.EncodeToString(sig)
	}

	// 解析 raw_data JSON
	var rawData map[string]interface{}
	if err := json.Unmarshal(rawDataJSON, &rawData); err != nil {
		log.Error("Broadcast transaction failed to parse raw_data JSON", "error", err)
		return false, fmt.Errorf("failed to parse raw_data JSON: %w", err)
	}

	// 根据 TRON API 的正确格式构建广播请求
	args := map[string]interface{}{
		"raw_data":  rawData,
		"signature": signatures,
	}

	result, err := c.call(ctx, "wallet/broadcasttransaction", args)
	if err != nil {
		log.Error("Broadcast transaction broadcast failed", "error", err)
		return false, err
	}

	type broadcastResponse struct {
		Result  bool   `json:"result"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	var resp broadcastResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		log.Error("Broadcast transaction failed to unmarshal response", "raw_result", string(result), "error", err)
		return false, fmt.Errorf("failed to unmarshal broadcast response: %w", err)
	}

	log.Info("Broadcast transaction result", "success", resp.Result, "code", resp.Code, "message", resp.Message)

	return resp.Result, nil
}

// BroadcastTransactionWithSignature 通过组装 raw_data 和签名来广播交易
func (c *RPCClient) BroadcastTransactionWithSignature(ctx context.Context, rawDataHex, signatureHex string) (string, error) {
	log.Debug("Broadcast transaction with signature", "raw_data_hex_length", len(rawDataHex), "signature_hex_length", len(signatureHex))

	// 从十六进制解码 raw_data
	rawDataBytes, err := hex.DecodeString(rawDataHex)
	if err != nil {
		log.Error("Broadcast transaction with signature failed to decode raw_data hex", "error", err)
		return "", fmt.Errorf("failed to decode raw_data hex: %w", err)
	}

	// 从十六进制解码签名
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		log.Error("Broadcast transaction with signature failed to decode signature hex", "error", err)
		return "", fmt.Errorf("failed to decode signature hex: %w", err)
	}

	// 将 raw_data 字节转换为 JSON
	rawDataJSON, err := decodeRawDataToJSON(rawDataBytes)
	if err != nil {
		log.Error("Broadcast transaction with signature failed to decode raw_data to JSON", "error", err)
		return "", fmt.Errorf("failed to decode raw_data to JSON: %w", err)
	}

	// 解析 raw_data JSON
	var rawData map[string]interface{}
	if err := json.Unmarshal(rawDataJSON, &rawData); err != nil {
		log.Error("Broadcast transaction with signature failed to parse raw_data JSON", "error", err)
		return "", fmt.Errorf("failed to parse raw_data JSON: %w", err)
	}

	// 在修改 raw_data 之前计算 txHash = SHA256(raw_data_bytes)
	txHash := Sha256(rawDataBytes)
	txHashHex := hex.EncodeToString(txHash)

	// 构建广播请求
	args := map[string]interface{}{
		"raw_data":  rawData,
		"signature": []string{hex.EncodeToString(signatureBytes)},
	}

	success, err := c.broadcast(ctx, args)
	if err != nil {
		return "", err
	}
	if !success {
		return "", fmt.Errorf("broadcast returned failure")
	}

	return txHashHex, nil
}

// broadcast 发送交易到 TRON 网络
func (c *RPCClient) broadcast(ctx context.Context, args map[string]interface{}) (bool, error) {
	result, err := c.call(ctx, "wallet/broadcasttransaction", args)
	if err != nil {
		log.Error("Broadcast failed", "error", err)
		return false, err
	}

	type broadcastResponse struct {
		Result  bool   `json:"result"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	var resp broadcastResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		log.Error("Broadcast failed to unmarshal response", "raw_result", string(result), "error", err)
		return false, fmt.Errorf("failed to unmarshal broadcast response: %w", err)
	}

	log.Info("Broadcast completed", "success", resp.Result, "code", resp.Code, "message", resp.Message)

	return resp.Result, nil
}

// decodeSignedTransaction 从 protobuf 签名交易中提取 raw_data JSON 和签名
func decodeSignedTransaction(data []byte) ([]byte, [][]byte, error) {
	var rawDataBytes []byte
	var signatureBytes [][]byte

	pos := 0
	for pos < len(data) {
		fieldTag := data[pos]
		pos++
		fieldNum := int(fieldTag >> 3)
		wireType := fieldTag & 0x07

		switch wireType {
		case 0:
			_, consumed := decodeVarint(data[pos:])
			pos += consumed
		case 2:
			length, consumed := decodeVarint(data[pos:])
			pos += consumed
			content := data[pos : pos+length]
			pos += length

			switch fieldNum {
			case 1:
				rawDataBytes = content
			case 2:
				signatureBytes = append(signatureBytes, content)
			}
		default:
			return nil, nil, fmt.Errorf("unsupported wire type: %d", wireType)
		}
	}

	if rawDataBytes == nil {
		return nil, nil, fmt.Errorf("no raw_data found in transaction")
	}

	// 将 raw_data 从 protobuf 字节解码为 JSON
	rawDataJSON, err := decodeRawDataToJSON(rawDataBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode raw_data to JSON: %w", err)
	}

	return rawDataJSON, signatureBytes, nil
}

func decodeRawDataToJSON(data []byte) ([]byte, error) {
	rawData := make(map[string]interface{})
	contracts := []interface{}{}

	pos := 0
	for pos < len(data) {
		fieldTag := data[pos]
		pos++
		fieldNum := int(fieldTag >> 3)
		wireType := fieldTag & 0x07

		switch wireType {
		case 0:
			val, consumed := decodeVarint(data[pos:])
			pos += consumed
			switch fieldNum {
			case 2:
				rawData["ref_block_num"] = val
			case 8:
				rawData["expiration"] = val
			case 9:
				rawData["timestamp"] = val
			case 14:
				rawData["timestamp"] = val
			}
		case 2:
			length, consumed := decodeVarint(data[pos:])
			pos += consumed
			content := data[pos : pos+length]
			pos += length

			switch fieldNum {
			case 1:
				rawData["ref_block_bytes"] = hex.EncodeToString(content)
			case 3:
				rawData["ref_block_hash"] = hex.EncodeToString(content)
			case 4:
				rawData["ref_block_hash"] = hex.EncodeToString(content)
			case 10, 11:
				contract, err := decodeContractToJSON(content)
				if err != nil {
					return nil, err
				}
				contracts = append(contracts, json.RawMessage(contract))
			}
		default:
			return nil, fmt.Errorf("unsupported wire type: %d", wireType)
		}
	}

	if len(contracts) > 0 {
		rawData["contract"] = contracts
	}

	return json.Marshal(rawData)
}

func decodeContractToJSON(data []byte) ([]byte, error) {
	contract := make(map[string]interface{})
	var paramValue []byte
	var paramType string

	pos := 0
	for pos < len(data) {
		fieldTag := data[pos]
		pos++
		fieldNum := int(fieldTag >> 3)
		wireType := fieldTag & 0x07

		switch wireType {
		case 0:
			val, consumed := decodeVarint(data[pos:])
			pos += consumed
			switch fieldNum {
			case 5:
				contract["call_value"] = val
			}
		case 2:
			length, consumed := decodeVarint(data[pos:])
			pos += consumed
			content := data[pos : pos+length]
			pos += length

			switch fieldNum {
			case 1:
				paramType = string(content)
				contract["type"] = paramType
			case 2:
				paramValue = content
			}
		}
	}

	if paramValue != nil {
		typeURL := "type.googleapis.com/protocol." + paramType
		contract["parameter"] = map[string]interface{}{
			"type_url": typeURL,
			"value":    hex.EncodeToString(paramValue),
		}
	}

	return json.Marshal(contract)
}

func decodeVarint(data []byte) (int, int) {
	result := 0
	shift := 0
	for i, b := range data {
		if b < 0x80 {
			return result | int(b)<<shift, i + 1
		}
		result |= int(b&0x7f) << shift
		shift += 7
	}
	return result, len(data)
}
