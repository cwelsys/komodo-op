package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"komodo-op/internal/config"
	"komodo-op/internal/komodoclient"
	"komodo-op/internal/logging"
	"komodo-op/internal/opclient"
	"komodo-op/internal/synchronizer"
)

var Version string

func main() {
	// --- CLI Flags ---
	daemonMode := flag.Bool("daemon", false, "Run the application in daemon mode, syncing periodically.")
	intervalFlag := flag.String("interval", "", "Sync interval for daemon mode (e.g., \"30s\", \"5m\", \"1h\"). Overrides SYNC_INTERVAL env var.")
	flag.Parse()

	// --- Configuration & Logging ---
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	logging.SetLevel(cfg.LogLevel)

	// Determine the effective sync interval
	effectiveIntervalStr := cfg.SyncInterval // Start with env var or default
	if *intervalFlag != "" {
		effectiveIntervalStr = *intervalFlag // Override with flag if provided
	}

	logging.Info("Configuration loaded:")
	logging.Info("  OP_CONNECT_HOST: %s", cfg.OpConnectHost)
	logging.Info("  OP_VAULT (UUID): %s", cfg.OpVaultUUID)
	logging.Info("  KOMODO_HOST: %s", cfg.KomodoHost)
	logging.Info("  SYNC_INTERVAL: %s (effective)", effectiveIntervalStr)

	// --- Initialize Clients ---
	httpClient := &http.Client{Timeout: 60 * time.Second}
	opClient := opclient.NewClient(httpClient, cfg)
	komodoClient := komodoclient.NewClient(httpClient, cfg)
	sync := synchronizer.New(opClient, komodoClient, cfg)

	// --- Execution Mode ---
	if *daemonMode {
		// Daemon Mode
		duration, err := time.ParseDuration(effectiveIntervalStr)
		if err != nil {
			logging.Error("Invalid sync interval format '%s': %v", effectiveIntervalStr, err)
			os.Exit(1)
		}

		if duration <= 0 {
			logging.Error("Sync interval must be positive.")
			os.Exit(1)
		}

		logging.Info("Starting daemon mode with sync interval: %v", duration)

		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		stopChan := make(chan os.Signal, 1)
		signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

		// Run first sync immediately
		logging.Info("Performing initial sync...")
		initialErrors := sync.Run()
		if initialErrors > 0 {
			logging.Error("Initial sync completed with %d errors.", initialErrors)
			// Decide if we should exit or continue? For now, continue.
		} else {
			logging.Info("Initial sync completed successfully.")
		}

		// Loop, syncing on each tick or exiting on signal
		for {
			select {
			case <-ticker.C:
				logging.Info("Periodic sync triggered...")
				runErrors := sync.Run()
				if runErrors > 0 {
					logging.Error("Periodic sync completed with %d errors.", runErrors)
				} else {
					logging.Info("Periodic sync completed successfully.")
				}
			case <-stopChan:
				logging.Info("Received shutdown signal. Exiting daemon mode...")
				return // Exit main
			}
		}

	} else {
		// One-off Sync Mode (Default)
		logging.Info("Starting one-off sync...")
		totalErrors := sync.Run()
		if totalErrors > 0 {
			logging.Error("Synchronization completed with %d errors.", totalErrors)
			os.Exit(1)
		} else {
			logging.Info("Synchronization completed successfully.")
			os.Exit(0)
		}
	}
}
