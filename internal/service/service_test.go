package service

import (
	"context"
	"testing"

	"github.com/koku-web3/txbuilder-tron/grpc"
	"github.com/koku-web3/txbuilder-tron/internal/config"
)

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
			address:   "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid address - starts with 3",
			address:   "3NsVCWoo9REhQVKbwuhSkxRYRAxTDagMqn",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "missing trace_id",
			address:   "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
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

			req := &grpc.VerifyAddressRequest{
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
			address:   "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid address",
			address:   "3NsVCWoo9REhQVKbwuhSkxRYRAxTDagMqn",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "missing trace_id",
			address:   "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
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

			req := &grpc.VerifyContractAddressRequest{
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
		req     *grpc.CheckSufficientBalanceRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "",
				ChainCode:   "tron",
				Coin:        "trx",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing chain_code",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "",
				Coin:        "trx",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing coin_id",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing from_address",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				IsBasicCoin: true,
				FromAddress: "",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing amount",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "",
			},
			wantErr: true,
		},
		{
			name: "token without contract",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "usdt",
				IsBasicCoin: false,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "100",
				Contract:    "",
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
		req     *grpc.BuildSignRawDataRequest
		wantErr bool
	}{
		{
			name: "missing biz_id",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "",
				ChainCode:   "tron",
				Coin:        "trx",
				IsBasicCoin: true,
				CoinSymbol:  "TRX",
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				ToAddress:   "TLa2fv6PDJNH1u2mp3CJ7S6HdUQbX7X5h3",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "invalid from_address",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				IsBasicCoin: true,
				CoinSymbol:  "TRX",
				FromAddress: "invalid_address",
				ToAddress:   "TLa2fv6PDJNH1u2mp3CJ7S6HdUQbX7X5h3",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "invalid to_address",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				IsBasicCoin: true,
				CoinSymbol:  "TRX",
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				ToAddress:   "invalid_address",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "amount with decimal point",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "tron",
				Coin:        "trx",
				IsBasicCoin: true,
				CoinSymbol:  "TRX",
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				ToAddress:   "TLa2fv6PDJNH1u2mp3CJ7S6HdUQbX7X5h3",
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

	tests := []struct {
		name    string
		req     *grpc.TxBroadcastRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "",
				RawData:   "0a86010a028f1622080223083f23e5c3ba40c0c5b3b3fd335a68080112640a2d747970652e676f6f676c65617069732e636f6d2f70726f746f636f6c2e5472616e73666572436f6e747261637412330a1541a162cbc2a988d8bf99506def3942c9e20aa0634a121541d3682962027e721c5247a9faf7865fe4a71d54381880ade20470c8f9afb3fd33124174807df06df3c6ef3071a24311b430e309d4974054000e725ab68f6135ad53190d1a0a6be185f2f0603b1bae7ac2fd206e77047023ac379df0483e8983aa18d501",
				Signature: "abcd1234",
			},
			wantErr: true,
		},
		{
			name: "missing raw_data",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "test-trace-id",
				RawData:   "",
				Signature: "abcd1234",
			},
			wantErr: true,
		},
		{
			name: "missing signature",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "test-trace-id",
				RawData:   "0a86010a028f1622080223083f23e5c3ba40c0c5b3b3fd335a68080112640a2d747970652e676f6f676c65617069732e636f6d2f70726f746f636f6c2e5472616e73666572436f6e747261637412330a1541a162cbc2a988d8bf99506def3942c9e20aa0634a121541d3682962027e721c5247a9faf7865fe4a71d54381880ade20470c8f9afb3fd33124174807df06df3c6ef3071a24311b430e309d4974054000e725ab68f6135ad53190d1a0a6be185f2f0603b1bae7ac2fd206e77047023ac379df0483e8983aa18d501",
				Signature: "",
			},
			wantErr: true,
		},
		{
			name: "raw_data too long",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "test-trace-id",
				RawData:   string(make([]byte, 4097)),
				Signature: "abcd1234",
			},
			wantErr: true,
		},
		{
			name: "signature too long",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "test-trace-id",
				RawData:   "0a86010a028f1622080223083f23e5c3ba40c0c5b3b3fd335a68080112640a2d747970652e676f6f676c65617069732e636f6d2f70726f746f636f6c2e5472616e73666572436f6e747261637412330a1541a162cbc2a988d8bf99506def3942c9e20aa0634a121541d3682962027e721c5247a9faf7865fe4a71d54381880ade20470c8f9afb3fd33124174807df06df3c6ef3071a24311b430e309d4974054000e725ab68f6135ad53190d1a0a6be185f2f0603b1bae7ac2fd206e77047023ac379df0483e8983aa18d501",
				Signature: string(make([]byte, 513)),
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
		req     *grpc.ConvertAddressRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &grpc.ConvertAddressRequest{
				TraceId: "",
				Keys: []*grpc.PublicKeysRequest{
					{
						AccountIndex:  0,
						PkixPubkeyPem: "-----BEGIN PUBLIC KEY-----\nMFYwEAYHKoZIzj0CAQYFK4EEAAoDQgAE9G3bKKkF5xIHuJ2lM9V6f0q2aFzEqYvP1vQdK3jW8g==\n-----END PUBLIC KEY-----\n",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing keys",
			req: &grpc.ConvertAddressRequest{
				TraceId: "test-trace-id",
				Keys:    nil,
			},
			wantErr: true,
		},
		{
			name: "empty keys",
			req: &grpc.ConvertAddressRequest{
				TraceId: "test-trace-id",
				Keys:    []*grpc.PublicKeysRequest{},
			},
			wantErr: true,
		},
		{
			name: "missing pkix_pubkey_pem in key",
			req: &grpc.ConvertAddressRequest{
				TraceId: "test-trace-id",
				Keys: []*grpc.PublicKeysRequest{
					{
						AccountIndex:  0,
						PkixPubkeyPem: "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid PEM string",
			req: &grpc.ConvertAddressRequest{
				TraceId: "test-trace-id",
				Keys: []*grpc.PublicKeysRequest{
					{
						AccountIndex:  0,
						PkixPubkeyPem: "not a valid pem string at all",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid PEM format but wrong key type",
			req: &grpc.ConvertAddressRequest{
				TraceId: "test-trace-id",
				Keys: []*grpc.PublicKeysRequest{
					{
						AccountIndex:  0,
						PkixPubkeyPem: "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE9G3bKKkF5xIHuJ2lM9V6f0q2aFzEqYvP1vQdK3jW8g==\n-----END PUBLIC KEY-----\n",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "pkix_pubkey_pem too long",
			req: &grpc.ConvertAddressRequest{
				TraceId: "test-trace-id",
				Keys: []*grpc.PublicKeysRequest{
					{
						AccountIndex:  0,
						PkixPubkeyPem: string(make([]byte, 8193)),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple keys with one invalid",
			req: &grpc.ConvertAddressRequest{
				TraceId: "test-trace-id",
				Keys: []*grpc.PublicKeysRequest{
					{
						AccountIndex:  0,
						PkixPubkeyPem: "-----BEGIN PUBLIC KEY-----\nMFYwEAYHKoZIzj0CAQYFK4EEAAoDQgAE9G3bKKkF5xIHuJ2lM9V6f0q2aFzEqYvP1vQdK3jW8g==\n-----END PUBLIC KEY-----\n",
					},
					{
						AccountIndex:  1,
						PkixPubkeyPem: "invalid",
					},
				},
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
