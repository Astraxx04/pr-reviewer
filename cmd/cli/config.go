package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// config is the persisted CLI configuration.
type config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

const defaultServer = "http://localhost:8001"

// configPath returns the resolved config file path, honouring --config, then
// falling back to ~/.config/pr-reviewer/config.json.
func configPath() string {
	if configFlag != "" {
		return configFlag
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "pr-reviewer", "config.json")
}

func loadConfig() config {
	cfg := config{Server: defaultServer}
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	if cfg.Server == "" {
		cfg.Server = defaultServer
	}
	return cfg
}

func saveConfig(cfg config) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// resolveConfig applies precedence: --server flag > config file > default. The
// token is only ever read from the config file, written by `auth login`.
func resolveConfig() config {
	cfg := loadConfig()
	if serverFlag != "" {
		cfg.Server = serverFlag
	}
	return cfg
}
