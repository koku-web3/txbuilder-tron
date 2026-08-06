package tron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-tron/internal/config"
)

type RPCClient struct {
	rpcURL string
	client *http.Client
	log    logko.Logger
}

func NewRPCClient() *RPCClient {
	cfg := config.GetConfig()
	return &RPCClient{
		rpcURL: strings.TrimSuffix(cfg.Chain.RPCURL, "/"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: config.GetLogger(),
	}
}

func (c *RPCClient) call(ctx context.Context, path string, params interface{}) (json.RawMessage, error) {
	log := c.log

	var body []byte
	var err error

	if params != nil {
		if raw, ok := params.(json.RawMessage); ok {
			body = raw
		} else {
			body, err = json.Marshal(params)
			if err != nil {
				log.Error("[RPCClient.call] failed to marshal params",
					"path", path,
					"params_type", fmt.Sprintf("%T", params),
					"error", err,
				)
				return nil, fmt.Errorf("failed to marshal params: %w", err)
			}
		}
	}

	url := c.rpcURL + "/" + path

	log.Debug("[RPCClient.call] preparing request",
		"url", url,
		"path", path,
		"body_length", len(body),
	)

	if len(body) > 0 {
		var prettyJSON map[string]interface{}
		if err := json.Unmarshal(body, &prettyJSON); err == nil {
			log.Debug("[RPCClient.call] request body",
				"url", url,
				"body", prettyJSON,
			)
		} else {
			log.Debug("[RPCClient.call] request body (raw)",
				"url", url,
				"body_raw", string(body),
			)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Error("[RPCClient.call] failed to create request",
			"url", url,
			"error", err,
		)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Debug("[RPCClient.call] sending HTTP request",
		"url", url,
		"method", http.MethodPost,
	)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error("[RPCClient.call] failed to do request",
			"url", url,
			"error", err,
		)
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	log.Debug("[RPCClient.call] received HTTP response",
		"url", url,
		"status_code", resp.StatusCode,
		"status", resp.Status,
	)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[RPCClient.call] failed to read response body",
			"url", url,
			"status_code", resp.StatusCode,
			"error", err,
		)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug("[RPCClient.call] response body received",
		"url", url,
		"status_code", resp.StatusCode,
		"response_body_length", len(respBody),
	)

	if resp.StatusCode != http.StatusOK {
		log.Error("[RPCClient.call] RPC call failed with non-200 status",
			"url", url,
			"path", path,
			"status_code", resp.StatusCode,
			"response_body", string(respBody),
		)
		return nil, fmt.Errorf("rpc call failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	type tronError struct {
		Error string `json:"Error"`
	}
	var terr tronError
	if err := json.Unmarshal(respBody, &terr); err == nil && terr.Error != "" {
		log.Error("[RPCClient.call] Tron RPC returned error",
			"url", url,
			"path", path,
			"tron_error", terr.Error,
		)
		return nil, fmt.Errorf("tron rpc error: %s", terr.Error)
	}

	log.Debug("[RPCClient.call] RPC call successful",
		"url", url,
		"path", path,
		"response_body_length", len(respBody),
	)

	return respBody, nil
}

func (c *RPCClient) GetAccount(ctx context.Context, address string) (json.RawMessage, error) {
	log := c.log

	log.Debug("[RPCClient.GetAccount] calling wallet/getaccount",
		"address", address,
	)

	params := map[string]interface{}{
		"address": address,
		"visible": true,
	}
	result, err := c.call(ctx, "wallet/getaccount", params)
	if err != nil {
		log.Error("[RPCClient.GetAccount] failed to get account",
			"address", address,
			"error", err,
		)
		return nil, err
	}

	log.Debug("[RPCClient.GetAccount] got account",
		"address", address,
		"result_length", len(result),
	)

	return result, nil
}

func (c *RPCClient) GetBalance(ctx context.Context, address string) (int64, error) {
	log := c.log

	log.Debug("[RPCClient.GetBalance] calling wallet/getaccount",
		"address", address,
	)

	type accountResp struct {
		Balance int64 `json:"balance"`
	}

	result, err := c.call(ctx, "wallet/getaccount", map[string]interface{}{
		"address": address,
		"visible": true,
	})
	if err != nil {
		log.Error("[RPCClient.GetBalance] failed to get account",
			"address", address,
			"error", err,
		)
		return 0, err
	}

	var acc accountResp
	if err := json.Unmarshal(result, &acc); err != nil {
		log.Error("[RPCClient.GetBalance] failed to unmarshal account",
			"address", address,
			"raw_result", string(result),
			"error", err,
		)
		return 0, fmt.Errorf("failed to unmarshal account: %w", err)
	}

	log.Debug("[RPCClient.GetBalance] got balance",
		"address", address,
		"balance", acc.Balance,
	)

	return acc.Balance, nil
}

func (c *RPCClient) TriggerConstantContract(ctx context.Context, contractAddress, functionSelector, parameter, ownerAddress string) (json.RawMessage, error) {
	log := c.log

	log.Debug("[RPCClient.TriggerConstantContract] calling wallet/triggerconstantcontract",
		"contract_address", contractAddress,
		"function_selector", functionSelector,
		"parameter", parameter,
		"owner_address", ownerAddress,
	)

	args := map[string]interface{}{
		"contract_address":  contractAddress,
		"function_selector": functionSelector,
		"parameter":         parameter,
		"visible":           true,
		"owner_address":     ownerAddress,
	}

	result, err := c.call(ctx, "wallet/triggerconstantcontract", args)
	if err != nil {
		log.Error("[RPCClient.TriggerConstantContract] failed to trigger constant contract",
			"contract_address", contractAddress,
			"function_selector", functionSelector,
			"error", err,
		)
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
		log.Info("[RPCClient.TriggerConstantContract] RPC response",
			"contract_address", contractAddress,
			"function_selector", functionSelector,
			"result_success", resp.Result.Result,
			"constant_result_count", len(resp.ConstantResult),
			"raw_result", string(result),
		)
	}

	return result, nil
}

func (c *RPCClient) GetTransactionInfoById(ctx context.Context, txID string) (json.RawMessage, error) {
	log := c.log

	log.Debug("[RPCClient.GetTransactionInfoById] calling wallet/gettransactioninfobyid",
		"tx_id", txID,
	)

	result, err := c.call(ctx, "wallet/gettransactioninfobyid", map[string]interface{}{
		"value": txID,
	})
	if err != nil {
		log.Error("[RPCClient.GetTransactionInfoById] failed to get transaction info",
			"tx_id", txID,
			"error", err,
		)
		return nil, err
	}

	log.Debug("[RPCClient.GetTransactionInfoById] got transaction info",
		"tx_id", txID,
		"result_length", len(result),
	)

	return result, nil
}

func (c *RPCClient) GetContract(ctx context.Context, contractAddress string) (json.RawMessage, error) {
	log := c.log

	log.Debug("[RPCClient.GetContract] calling wallet/getcontract",
		"contract_address", contractAddress,
	)

	result, err := c.call(ctx, "wallet/getcontract", map[string]interface{}{
		"address": contractAddress,
		"visible": true,
	})
	if err != nil {
		log.Error("[RPCClient.GetContract] failed to get contract",
			"contract_address", contractAddress,
			"error", err,
		)
		return nil, err
	}

	log.Debug("[RPCClient.GetContract] got contract",
		"contract_address", contractAddress,
		"result_length", len(result),
	)

	return result, nil
}

func (c *RPCClient) Call(ctx context.Context, path string, params json.RawMessage) (json.RawMessage, error) {
	log := c.log

	log.Debug("[RPCClient.Call] calling RPC",
		"path", path,
		"params_length", len(params),
	)

	result, err := c.call(ctx, path, params)
	if err != nil {
		log.Error("[RPCClient.Call] RPC call failed",
			"path", path,
			"error", err,
		)
		return nil, err
	}

	log.Debug("[RPCClient.Call] RPC call successful",
		"path", path,
		"result_length", len(result),
	)

	return result, nil
}
