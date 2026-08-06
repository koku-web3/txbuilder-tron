package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	configContent := `
[chain]
chain_id = "tron"
rpc_url = "https://nile.trongrid.io"

[grpc]
host = "127.0.0.1"
port = 50051

[log]
rotation = true
file_path = "/tmp/test.log"
max_size_mb = 100
max_backups = 10
max_age = 30
compress = true
format = "terminal"
verbosity = 3
vmodule = ""
`

	tmpFile, err := os.CreateTemp("", "config_test_*.toml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	if err := LoadConfig(tmpFile.Name()); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	cfg := GetConfig()
	if cfg.Chain.ChainID != "tron" {
		t.Errorf("ChainID = %v, want %v", cfg.Chain.ChainID, "tron")
	}
	if cfg.Chain.RPCURL != "https://nile.trongrid.io" {
		t.Errorf("RPCURL = %v, want %v", cfg.Chain.RPCURL, "https://nile.trongrid.io")
	}
	if cfg.GRPC.Host != "127.0.0.1" {
		t.Errorf("GRPC.Host = %v, want %v", cfg.GRPC.Host, "127.0.0.1")
	}
	if cfg.GRPC.Port != 50051 {
		t.Errorf("GRPC.Port = %v, want %v", cfg.GRPC.Port, 50051)
	}
	if !cfg.Log.Rotation {
		t.Error("Log.Rotation = false, want true")
	}
	if cfg.Log.Verbosity != 3 {
		t.Errorf("Log.Verbosity = %v, want %v", cfg.Log.Verbosity, 3)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	err := LoadConfig("/nonexistent/config.toml")
	if err == nil {
		t.Error("LoadConfig() should return error for nonexistent file")
	}
}
