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

const defaultServer = "http://localhost:8080"

// configPath returns the resolved config file path, honouring --config / the
// PR_REVIEWER_CONFIG env var, then falling back to ~/.config/pr-reviewer/config.json.
func configPath() string {
	if configFlag != "" {
		return configFlag
	}
	if env := os.Getenv("PR_REVIEWER_CONFIG"); env != "" {
		return env
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

// resolveConfig applies precedence: --flag > env var > config file > default.
func resolveConfig() config {
	cfg := loadConfig()

	if env := os.Getenv("PR_REVIEWER_SERVER"); env != "" {
		cfg.Server = env
	}
	if env := os.Getenv("PR_REVIEWER_TOKEN"); env != "" {
		cfg.Token = env
	}
	if serverFlag != "" {
		cfg.Server = serverFlag
	}
	if tokenFlag != "" {
		cfg.Token = tokenFlag
	}
	return cfg
}
