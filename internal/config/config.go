package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	logko "github.com/koku-web3/logko"
)

type Config struct {
	Chain ChainConfig `toml:"chain"`
	GRPC  GRPCConfig  `toml:"grpc"`
	Log   LogConfig   `toml:"log"`
}

type ChainConfig struct {
	ChainID string `toml:"chain_id"`
	RPCURL  string `toml:"rpc_url"`
}

type GRPCConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type LogConfig struct {
	Rotation   bool   `toml:"rotation"`
	FilePath   string `toml:"file_path"`
	MaxSizeMB  int    `toml:"max_size_mb"`
	MaxBackups int    `toml:"max_backups"`
	MaxAge     int    `toml:"max_age"`
	Compress   bool   `toml:"compress"`
	Format     string `toml:"format"`
	Verbosity  int    `toml:"verbosity"`
	VModule    string `toml:"vmodule"`
}

const (
	DEFAULT_PATH = "config/config.toml"
)

var (
	cfg *Config
)

func LoadConfig(path string) error {
	configData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	cfg = &Config{}
	if err := toml.Unmarshal(configData, cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

func GetConfig() *Config {
	return cfg
}

func SetConfigForTest(c *Config) {
	cfg = c
}

func InitLogger() error {
	logConfig := GetConfig().Log

	var opts []logko.Option
	if logConfig.Rotation {
		opts = append(opts, logko.WithRotation())
	}
	if logConfig.FilePath != "" {
		opts = append(opts, logko.WithFilePath(logConfig.FilePath))
	}
	if logConfig.MaxSizeMB > 0 {
		opts = append(opts, logko.WithMaxSizeMB(logConfig.MaxSizeMB))
	}
	if logConfig.MaxBackups > 0 {
		opts = append(opts, logko.WithMaxBackups(logConfig.MaxBackups))
	}
	if logConfig.MaxAge > 0 {
		opts = append(opts, logko.WithMaxAge(logConfig.MaxAge))
	}
	if logConfig.Compress {
		opts = append(opts, logko.WithCompress())
	}
	if logConfig.Format != "" {
		opts = append(opts, logko.WithFormat(logConfig.Format))
	}
	if logConfig.Verbosity > 0 {
		opts = append(opts, logko.WithVerbosity(logConfig.Verbosity))
	}
	if logConfig.VModule != "" {
		opts = append(opts, logko.WithVmodule(logConfig.VModule))
	}

	return logko.Setup(nil, opts...)
}

func GetLogger() logko.Logger {
	return logko.Root()
}
