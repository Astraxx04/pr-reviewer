package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort          string
	GitHubWebhookSecret string // loaded from DB at startup; not an env var
	GitHubClientID      string
	GitHubClientSecret  string
	DatabaseURL         string
	JWTSecret           string
	EncryptionKey       string // 64-char hex (32 bytes AES-256)
	ServerURL           string // public base URL of this server, e.g. http://localhost:8001
	FrontendURL         string
	CORSOrigins         string // comma-separated list of allowed origins; defaults to FrontendURL
	AppEnv              string
	MigrateOnly         bool // if true: run migrations then exit (used by docker-compose migrate service)
	SkipMigrations      bool // if true: the migrate step is a no-op (exits 0 without applying migrations)
	// Access control
	RequiredGithubOrg string // required: users must be members of this GitHub org to log in
	JWTTTLHours       int    // JWT lifetime in hours (default 24)
	// Invite policy
	InviteTTLHours int // invite link lifetime in hours (default 168 = 7 days)
	// API token policy
	APITokenMaxDays int // maximum token lifetime in days; 0 = no limit (default)
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		ServerPort:         getEnv("SERVER_PORT", "8001"),
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production"),
		EncryptionKey:      getEnv("ENCRYPTION_KEY", ""),
		ServerURL:          getEnv("SERVER_URL", ""),
		FrontendURL:        getEnv("FRONTEND_URL", ""),
		CORSOrigins:        getEnv("CORS_ORIGINS", getEnv("FRONTEND_URL", "")),
		AppEnv:             getEnv("APP_ENV", "development"),
		MigrateOnly:        getEnv("MIGRATE_ONLY", "") == "true",
		SkipMigrations:     getEnv("SKIP_MIGRATIONS", "") == "true",
		RequiredGithubOrg:  getEnv("REQUIRED_GITHUB_ORG", ""),
		JWTTTLHours:        getEnvInt("JWT_TTL_HOURS", 24),
		InviteTTLHours:     getEnvInt("INVITE_TTL_HOURS", 7*24),
		APITokenMaxDays:    getEnvInt("API_TOKEN_MAX_DAYS", 0),
	}, nil
}

func (c *Config) Validate() {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.EncryptionKey == "" {
		missing = append(missing, "ENCRYPTION_KEY")
	}
	if c.RequiredGithubOrg == "" {
		missing = append(missing, "REQUIRED_GITHUB_ORG")
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "fatal: required env vars not set: %s\n", strings.Join(missing, ", "))
		os.Exit(1)
	}
	if c.JWTSecret == "change-me-in-production" && c.AppEnv != "development" {
		fmt.Fprintln(os.Stderr, "fatal: JWT_SECRET must be changed from the default in production")
		os.Exit(1)
	}
	if c.APITokenMaxDays < 0 {
		fmt.Fprintln(os.Stderr, "fatal: API_TOKEN_MAX_DAYS must be 0 (no limit) or a positive integer")
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return fallback
}
