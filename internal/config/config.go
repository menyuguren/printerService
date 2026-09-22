package config

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
)

type Config struct {
	ControlBind       string `json:"control_bind"`
	ControlPort       int    `json:"control_port"`
	DataBind          string `json:"data_bind"`
	DataPort          int    `json:"data_port"`
	PrinterName       string `json:"printer_name"`
	UseDefaultPrinter bool   `json:"use_default_printer"`
	LogLevel          string `json:"log_level"`
}

const (
	LogLevelInfo  = "info"
	LogLevelDebug = "debug"
)

func Default() Config {
	return Config{
		ControlBind:       "127.0.0.1",
		ControlPort:       8080,
		DataBind:          "0.0.0.0",
		DataPort:          9100,
		UseDefaultPrinter: true,
		LogLevel:          LogLevelInfo,
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func Save(path string, cfg Config) error {
	applyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func Validate(cfg Config) error {
	if net.ParseIP(cfg.ControlBind) == nil {
		return errors.New("invalid control bind address")
	}
	if net.ParseIP(cfg.DataBind) == nil {
		return errors.New("invalid data bind address")
	}
	if cfg.ControlPort < 1 || cfg.ControlPort > 65535 {
		return errors.New("invalid control port")
	}
	if cfg.DataPort < 1 || cfg.DataPort > 65535 {
		return errors.New("invalid data port")
	}
	if cfg.ControlBind == cfg.DataBind && cfg.ControlPort == cfg.DataPort {
		return errors.New("control port and data port must differ")
	}
	if cfg.LogLevel != "" && cfg.LogLevel != LogLevelInfo && cfg.LogLevel != LogLevelDebug {
		return errors.New("invalid log level: must be info or debug")
	}
	return nil
}

func applyDefaults(cfg *Config) {
	def := Default()
	if cfg.ControlBind == "" {
		cfg.ControlBind = def.ControlBind
	}
	if cfg.ControlPort == 0 {
		cfg.ControlPort = def.ControlPort
	}
	if cfg.DataBind == "" {
		cfg.DataBind = def.DataBind
	}
	if cfg.DataPort == 0 {
		cfg.DataPort = def.DataPort
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = def.LogLevel
	}
	if cfg.PrinterName == "" {
		cfg.UseDefaultPrinter = true
	}
}
