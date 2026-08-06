package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-tron/api"
	"github.com/koku-web3/txbuilder-tron/internal/config"
)

// mockLogger is a logger that does nothing, used for testing
type mockLogger struct{}

func (m *mockLogger) With(ctx ...interface{}) logko.Logger                 { return m }
func (m *mockLogger) New(ctx ...interface{}) logko.Logger                  { return m }
func (m *mockLogger) Log(level slog.Level, msg string, ctx ...interface{}) {}
func (m *mockLogger) Trace(msg string, ctx ...interface{})                 {}
func (m *mockLogger) Debug(msg string, ctx ...interface{})                 {}
func (m *mockLogger) Info(msg string, ctx ...interface{})                  {}
func (m *mockLogger) Warn(msg string, ctx ...interface{})                  {}
func (m *mockLogger) Error(msg string, ctx ...interface{})                 {}
func (m *mockLogger) Crit(msg string, ctx ...interface{})                  {}
func (m *mockLogger) Write(level slog.Level, msg string, attrs ...any)     {}
func (m *mockLogger) Enabled(ctx context.Context, level slog.Level) bool   { return false }
func (m *mockLogger) Handler() slog.Handler                                { return nil }

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

			svc := &TxBuilderService{
				log: &mockLogger{},
			}

			req := &api.VerifyAddressRequest{
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

			svc := &TxBuilderService{
				log: &mockLogger{},
			}

			req := &api.VerifyContractAddressRequest{
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

	svc := &TxBuilderService{
		log: &mockLogger{},
	}

	tests := []struct {
		name    string
		req     *api.CheckSufficientBalanceRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &api.CheckSufficientBalanceRequest{
				TraceId:     "",
				ChainId:     "tron",
				CoinId:      "trx",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing chain_id",
			req: &api.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainId:     "",
				CoinId:      "trx",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing coin_id",
			req: &api.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainId:     "tron",
				CoinId:      "",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing from_address",
			req: &api.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainId:     "tron",
				CoinId:      "trx",
				IsBasicCoin: true,
				FromAddress: "",
				Amount:      "100",
			},
			wantErr: true,
		},
		{
			name: "missing amount",
			req: &api.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainId:     "tron",
				CoinId:      "trx",
				IsBasicCoin: true,
				FromAddress: "TQgY9QVegqL4eX8wKM7xACGCVzFZwDDy45",
				Amount:      "",
			},
			wantErr: true,
		},
		{
			name: "token without contract",
			req: &api.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainId:     "tron",
				CoinId:      "usdt",
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

	svc := &TxBuilderService{
		log: &mockLogger{},
	}

	tests := []struct {
		name    string
		req     *api.BuildSignRawDataRequest
		wantErr bool
	}{
		{
			name: "missing biz_id",
			req: &api.BuildSignRawDataRequest{
				BizId:       "",
				ChainId:     "tron",
				CoinId:      "trx",
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
			req: &api.BuildSignRawDataRequest{
				BizId:       "test-biz-id",
				ChainId:     "tron",
				CoinId:      "trx",
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
			req: &api.BuildSignRawDataRequest{
				BizId:       "test-biz-id",
				ChainId:     "tron",
				CoinId:      "trx",
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
			req: &api.BuildSignRawDataRequest{
				BizId:       "test-biz-id",
				ChainId:     "tron",
				CoinId:      "trx",
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
