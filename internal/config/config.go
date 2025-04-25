package config

import (
	"fmt"
	"os"
	"strings"
	// Import time for default duration
	// "log" // Temporarily remove direct logging, will be handled in main
)

// Config holds the application configuration.
type Config struct {
	OpConnectHost         string
	OpVaultUUID           string // User-provided UUID (or name, though we now assume UUID)
	OpServiceAccountToken string
	KomodoHost            string
	KomodoAPIKey          string
	KomodoAPISecret       string
	LogLevel              string // Keep for initial read by main
	SyncInterval          string // Interval for daemon mode (e.g., "1h", "30m")

	// Internal: Populated during load or later steps
	OpVaultID string // Resolved Vault ID (currently same as OpVaultUUID)
}

// DefaultSyncInterval defines the default sync interval if not set via env var.
const DefaultSyncInterval = "1h"

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	syncInterval := os.Getenv("SYNC_INTERVAL")
	if syncInterval == "" {
		syncInterval = DefaultSyncInterval
	}

	cfg := &Config{
		OpConnectHost:         os.Getenv("OP_CONNECT_HOST"),
		OpVaultUUID:           os.Getenv("OP_VAULT"),
		OpServiceAccountToken: strings.TrimSpace(os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")),
		KomodoHost:            os.Getenv("KOMODO_HOST"),
		KomodoAPIKey:          os.Getenv("KOMODO_API_KEY"),
		KomodoAPISecret:       os.Getenv("KOMODO_API_SECRET"),
		LogLevel:              os.Getenv("LOG_LEVEL"),
		SyncInterval:          syncInterval, // Set from env var or default
	}

	// Validate required fields
	if cfg.OpConnectHost == "" {
		return nil, fmt.Errorf("OP_CONNECT_HOST environment variable not set")
	}
	if cfg.OpVaultUUID == "" {
		return nil, fmt.Errorf("OP_VAULT environment variable (vault UUID) not set")
	}
	if cfg.OpServiceAccountToken == "" {
		return nil, fmt.Errorf("OP_SERVICE_ACCOUNT_TOKEN environment variable not set or is only whitespace")
	}
	if cfg.KomodoHost == "" {
		return nil, fmt.Errorf("KOMODO_HOST environment variable not set")
	}
	if cfg.KomodoAPIKey == "" {
		return nil, fmt.Errorf("KOMODO_API_KEY environment variable not set")
	}
	if cfg.KomodoAPISecret == "" {
		return nil, fmt.Errorf("KOMODO_API_SECRET environment variable not set")
	}

	// Resolve Vault ID (currently just using the provided UUID)
	cfg.OpVaultID = cfg.OpVaultUUID

	// Ensure hosts start with http:// or https://
	if !strings.HasPrefix(cfg.OpConnectHost, "http") {
		cfg.OpConnectHost = "http://" + cfg.OpConnectHost
	}
	if !strings.HasPrefix(cfg.KomodoHost, "http") {
		cfg.KomodoHost = "http://" + cfg.KomodoHost
	}

	// Remove trailing slashes
	cfg.OpConnectHost = strings.TrimSuffix(cfg.OpConnectHost, "/")
	cfg.KomodoHost = strings.TrimSuffix(cfg.KomodoHost, "/")

	// Logging of loaded config will be done in main after setting log level
	// We will also log the SyncInterval there.

	return cfg, nil
}
