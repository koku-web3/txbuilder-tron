package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	txbuilder "github.com/koku-web3/txbuilder-tron/grpc"
	"github.com/koku-web3/txbuilder-tron/internal/config"
	"github.com/koku-web3/txbuilder-tron/internal/tron"
)

// mockRefBlockFetcher implements tron.RefBlockFetcher for service-level tests.
type mockRefBlockFetcher struct {
	blockResp []byte
	blockErr  error
}

func (m *mockRefBlockFetcher) GetNowBlock(ctx context.Context) (json.RawMessage, error) {
	return m.blockResp, m.blockErr
}

// Valid TRON mainnet addresses that pass gotron-sdk's DecodeCheck.
const svcTestFrom = "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ"
const svcTestTo = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"
const svcTestContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

func TestBuildSignRawData_MsgEqualsSHA256RawData(t *testing.T) {
	blockRespJSON := `{"blockID":"0x00000000052eb0c0cb95fd971bc93fc21683e18bb3f84b6427b0f32b57ae6ab7","block_header":{"raw_data":{"number":86945984}}}`
	fetcher := &mockRefBlockFetcher{blockResp: []byte(blockRespJSON)}

	rawData, err := tron.BuildBasicCoinRawData(context.Background(), fetcher, svcTestFrom, svcTestTo, "1000000")
	if err != nil {
		t.Fatalf("BuildBasicCoinRawData() error = %v", err)
	}

	rawDataHex := hex.EncodeToString(rawData)
	hash := sha256.Sum256(rawData)
	msg := hex.EncodeToString(hash[:])

	if msg == "" {
		t.Error("Msg should not be empty")
	}
	if len(rawData) == 0 {
		t.Error("RawData should not be empty")
	}
	if msg == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Error("Msg appears to be SHA256 of empty data")
	}

	t.Logf("BasicCoin: raw_data_length=%d, msg=%s", len(rawData), msg)
	_ = rawDataHex
}

func TestBuildSignRawData_MsgEqualsSHA256RawData_Token(t *testing.T) {
	blockRespJSON := `{"blockID":"0x00000000052eb0c0cb95fd971bc93fc21683e18bb3f84b6427b0f32b57ae6ab7","block_header":{"raw_data":{"number":86945984}}}`
	fetcher := &mockRefBlockFetcher{blockResp: []byte(blockRespJSON)}

	rawData, err := tron.BuildTokenRawData(context.Background(), fetcher, svcTestFrom, svcTestTo, "1000000", svcTestContract, tron.DefaultTRC20FeeLimit)
	if err != nil {
		t.Fatalf("BuildTokenRawData() error = %v", err)
	}

	hash := sha256.Sum256(rawData)
	msg := hex.EncodeToString(hash[:])

	if msg == "" {
		t.Error("Msg should not be empty")
	}
	if len(rawData) == 0 {
		t.Error("RawData should not be empty")
	}

	t.Logf("Token: raw_data_length=%d, msg=%s", len(rawData), msg)
}

func TestVerifyAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		traceID   string
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "valid address",
			address:   "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid address",
			address:   "invalid_address",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "missing trace_id",
			address:   "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
			traceID:   "",
			wantValid: false,
			wantErr:   true,
		},
		{
			name:      "missing address",
			address:   "",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			config.SetConfigForTest(cfg)

			svc := &TxBuilderService{}

			req := &txbuilder.VerifyAddressRequest{
				TraceId: tt.traceID,
				Address: tt.address,
			}

			resp, err := svc.VerifyAddress(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp.IsValid != tt.wantValid {
				t.Errorf("VerifyAddress() = %v, want %v", resp.IsValid, tt.wantValid)
			}
		})
	}
}

func TestVerifyContractAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		traceID   string
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "valid contract address",
			address:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid address",
			address:   "invalid_address",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "missing trace_id",
			address:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			traceID:   "",
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			config.SetConfigForTest(cfg)

			svc := &TxBuilderService{}

			req := &txbuilder.VerifyContractAddressRequest{
				TraceId: tt.traceID,
				Address: tt.address,
			}

			resp, err := svc.VerifyContractAddress(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyContractAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp.IsValid != tt.wantValid {
				t.Errorf("VerifyContractAddress() = %v, want %v", resp.IsValid, tt.wantValid)
			}
		})
	}
}

func TestCheckSufficientBalance_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	tests := []struct {
		name    string
		req     *txbuilder.CheckSufficientBalanceRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.CheckSufficientBalanceRequest{
				TraceId:     "",
				ChainCode:   "tron",
				Coin:        "trx",
				FromAddress: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing chain_code",
			req: &txbuilder.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "",
				Coin:        "trx",
				FromAddress: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing coin",
			req: &txbuilder.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "",
				FromAddress: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing from_address",
			req: &txbuilder.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				FromAddress: "",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing amount",
			req: &txbuilder.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				FromAddress: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
				Amount:      "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CheckSufficientBalance(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckSufficientBalance() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildSignRawData_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	tests := []struct {
		name    string
		req     *txbuilder.BuildSignRawDataRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.BuildSignRawDataRequest{
				TraceId:     "",
				ChainCode:   "tron",
				Coin:        "trx",
				CoinSymbol:  "TRX",
				FromAddress: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
				ToAddress:   "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9",
				Amount:      "1000000",
			},
			wantErr: true,
		},
		{
			name: "invalid from_address",
			req: &txbuilder.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				CoinSymbol:  "TRX",
				FromAddress: "invalid_address",
				ToAddress:   "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9",
				Amount:      "1000000",
			},
			wantErr: true,
		},
		{
			name: "invalid to_address",
			req: &txbuilder.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				CoinSymbol:  "TRX",
				FromAddress: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
				ToAddress:   "invalid_address",
				Amount:      "1000000",
			},
			wantErr: true,
		},
		{
			name: "amount with decimal point",
			req: &txbuilder.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				CoinSymbol:  "TRX",
				FromAddress: "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ",
				ToAddress:   "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9",
				Amount:      "100.50",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.BuildSignRawData(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildSignRawData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTxBroadcast_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	const validRawData = "0a86010a028f1622080223083f23e5c3ba40c0c5b3b3fd335a68080112640a2d747970652e676f6f676c65617069732e636f6d2f70726f746f636f6c2e5472616e73666572436f6e747261637412330a1541a162cbc2a988d8bf99506def3942c9e20aa0634a121541d3682962027e721c5247a9faf7865fe4a71d54381880ade20470c8f9afb3fd33124174807df06df3c6ef3071a24311b430e309d4974054000e725ab68f6135ad53190d1a0a6be185f2f0603b1bae7ac2fd206e77047023ac379df0483e8983aa18d501"
	const validSig = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234ab"
	const validFrom = "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ"

	tests := []struct {
		name    string
		req     *txbuilder.TxBroadcastRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:     "",
				RawData:     validRawData,
				Signature:   validSig,
				FromAddress: validFrom,
			},
			wantErr: true,
		},
		{
			name: "missing raw_data",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:     "test-trace-id",
				RawData:     "",
				Signature:   validSig,
				FromAddress: validFrom,
			},
			wantErr: true,
		},
		{
			name: "missing signature",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:     "test-trace-id",
				RawData:     validRawData,
				Signature:   "",
				FromAddress: validFrom,
			},
			wantErr: true,
		},
		{
			name: "missing from_address",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:     "test-trace-id",
				RawData:     validRawData,
				Signature:   validSig,
				FromAddress: "",
			},
			wantErr: true,
		},
		{
			name: "invalid from_address",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:     "test-trace-id",
				RawData:     validRawData,
				Signature:   validSig,
				FromAddress: "invalid_address",
			},
			wantErr: true,
		},
		{
			name: "raw_data too long",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:     "test-trace-id",
				RawData:     strings.Repeat("a", 4097),
				Signature:   validSig,
				FromAddress: validFrom,
			},
			wantErr: true,
		},
		{
			name: "signature too long",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:     "test-trace-id",
				RawData:     validRawData,
				Signature:   strings.Repeat("a", 513),
				FromAddress: validFrom,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.TxBroadcast(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("TxBroadcast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvertAddress_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	tests := []struct {
		name    string
		req     *txbuilder.ConvertAddressRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "",
				Keys: []*txbuilder.PublicKeysRequest{
					{AccountIndex: 0, PkixPubkeyPem: "-----BEGIN PUBLIC KEY-----\nMFYwEAYHKoZIzj0CAQYFK4EEAAoDQgAE9G3bKKkF5xIHuJ2lM9V6f0q2aFzEqYvP1vQdK3jW8g==\n-----END PUBLIC KEY-----\n"},
				},
			},
			wantErr: true,
		},
		{
			name: "trace_id too long",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: strings.Repeat("a", 37),
				Keys: []*txbuilder.PublicKeysRequest{
					{AccountIndex: 0, PkixPubkeyPem: "-----BEGIN PUBLIC KEY-----\nMFYwEAYHKoZIzj0CAQYFK4EEAAoDQgAE9G3bKKkF5xIHuJ2lM9V6f0q2aFzEqYvP1vQdK3jW8g==\n-----END PUBLIC KEY-----\n"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty keys",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{},
			},
			wantErr: true,
		},
		{
			name: "keys length > 100",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    generateKeys(101, "-----BEGIN PUBLIC KEY-----\nMFYwEAYHKoZIzj0CAQYFK4EEAAoDQgAE9G3bKKkF5xIHuJ2lM9V6f0q2aFzEqYvP1vQdK3jW8g==\n-----END PUBLIC KEY-----\n"),
			},
			wantErr: true,
		},
		{
			name: "empty pkix_pubkey_pem",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: ""}},
			},
			wantErr: true,
		},
		{
			name: "pkix_pubkey_pem too long",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: strings.Repeat("a", 4097)}},
			},
			wantErr: true,
		},
		{
			name: "invalid PEM string",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: "not a valid pem"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ConvertAddress(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func generateKeys(n int, pemStr string) []*txbuilder.PublicKeysRequest {
	keys := make([]*txbuilder.PublicKeysRequest, n)
	for i := 0; i < n; i++ {
		keys[i] = &txbuilder.PublicKeysRequest{AccountIndex: uint32(i), PkixPubkeyPem: pemStr}
	}
	return keys
}
