package synchronizer

import (
	"fmt"
	"regexp"
	"strings"

	"komodo-op/internal/config"
	"komodo-op/internal/komodoclient"
	"komodo-op/internal/logging"
	"komodo-op/internal/opclient"
)

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
var spaceRegex = regexp.MustCompile(`\s+`)

// Identifier used in description to mark variables managed by this tool
const managedByMarker = "1Password-Sync:"

// Synchronizer handles the core logic of syncing secrets from 1Password to Komodo.
type Synchronizer struct {
	opClient     *opclient.Client
	komodoClient *komodoclient.Client
	cfg          *config.Config // Keep a reference for vault UUID etc.
}

// New creates a new Synchronizer.
func New(opClient *opclient.Client, komodoClient *komodoclient.Client, cfg *config.Config) *Synchronizer {
	return &Synchronizer{
		opClient:     opClient,
		komodoClient: komodoClient,
		cfg:          cfg,
	}
}

// formatKomodoName formats the item title and field label into a Komodo variable name.
func formatKomodoName(itemName, fieldLabel string) string {
	// Keep sanitization for valid variable names but don't add prefix
	safeItemName := spaceRegex.ReplaceAllString(itemName, "-")
	safeFieldLabel := spaceRegex.ReplaceAllString(fieldLabel, "-")

	// Replace any remaining non-alphanumeric (excluding underscore) with underscore
	safeItemName = nonAlphanumericRegex.ReplaceAllString(safeItemName, "_")
	safeFieldLabel = nonAlphanumericRegex.ReplaceAllString(safeFieldLabel, "_")

	// Convert to uppercase (restoring this functionality)
	safeItemName = strings.ToUpper(safeItemName)
	safeFieldLabel = strings.ToUpper(safeFieldLabel)

	// Format is now just ITEMNAME__FIELDLABEL (without prefix)
	if fieldLabel == "" {
		return safeItemName
	}
	return fmt.Sprintf("%s__%s", safeItemName, safeFieldLabel)
}

// sanitizeNameForLog replaces the last part of a secret name (after the last __)
// with its first 2 characters followed by ***, for logging purposes.
// Updated logic: Sanitizes *each* part after the prefix.
func sanitizeNameForLog(name string) string {
	parts := strings.Split(name, "__")
	if len(parts) < 2 { // Should not happen with our format, but be safe
		return name // Return original if format is unexpected
	}

	for i := 2; i < len(parts); i++ {
		part := parts[i]
		if len(part) > 2 {
			parts[i] = part[:2] + "***"
		} else {
			// If the part is 2 chars or less, just mask it all
			parts[i] = "***"
		}
	}

	return strings.Join(parts, "__")
}

// syncKomodoSecret ensures a secret exists in Komodo with the correct value.
func (s *Synchronizer) syncKomodoSecret(name, value string) error {
	logging.Debug("Checking existence of Komodo variable '%s'", name)
	_, found, err := s.komodoClient.GetVariable(name)

	if err != nil {
		return fmt.Errorf("failed during existence check for variable '%s': %w", name, err)
	}

	if found {
		logging.Info("  Variable '%s' exists, attempting update.", sanitizeNameForLog(name))
		return s.komodoClient.UpdateVariableValue(name, value)
	} else {
		logging.Info("  Variable '%s' does not exist, attempting create.", sanitizeNameForLog(name))
		description := fmt.Sprintf("%s Synced from 1P vault '%s'", managedByMarker, s.cfg.OpVaultUUID)
		return s.komodoClient.CreateVariable(name, value, description)
	}
}

// Run executes the synchronization process.
// Returns the total number of errors encountered.
func (s *Synchronizer) Run() int {
	logging.Info("Fetching items from 1Password vault '%s'...", s.cfg.OpVaultUUID)
	items, err := s.opClient.GetItems()
	if (err != nil) {
		logging.Error("Failed to get items from 1Password: %v", err)
		return 1 // Indicate failure
	}

	if len(items) == 0 {
		logging.Info("No items found in vault '%s'. Exiting.", s.cfg.OpVaultUUID)
		return 0 // No errors, but nothing to do
	}

	expectedKomodoNames := make(map[string]bool)
	type secretToSync struct {
		name  string
		value string
	}
	secretsToSync := []secretToSync{}

	logging.Info("Processing %d items from 1Password...", len(items))
	skipped1PCount := 0
	for _, item := range items {
		logging.Debug("Processing 1P item: '%s' (ID: %s)", item.Title, item.ID)
		itemDetail, err := s.opClient.GetItemDetails(item.ID)
		if err != nil {
			logging.Error("Failed to get details for item '%s' (%s): %v", item.Title, item.ID, err)
			continue // Skip item
		}

		if len(itemDetail.Fields) == 0 {
			logging.Info("  Item '%s' has no fields. Skipping.", item.Title)
			skipped1PCount++
			continue
		}

		for _, field := range itemDetail.Fields {
			if field.Label == "" || field.Value == "" {
				logging.Debug("  Skipping field ID %s in item '%s' (label or value is empty)", field.ID, item.Title)
				skipped1PCount++
				continue
			}

			komodoName := formatKomodoName(itemDetail.Title, field.Label)
			expectedKomodoNames[komodoName] = true
			secretsToSync = append(secretsToSync, secretToSync{komodoName, field.Value})
			logging.Debug("  Added expected Komodo name: %s", komodoName)
		}
	}
	logging.Info("Finished processing 1Password items. Found %d secrets to potentially sync. Skipped %d items/fields.", len(secretsToSync), skipped1PCount)

	logging.Info("Starting synchronization (create/update) with Komodo...")
	processedCount := 0
	createUpdateErrorCount := 0

	for _, secret := range secretsToSync {
		logging.Info("  Syncing Komodo secret '%s'...", sanitizeNameForLog(secret.name))
		err := s.syncKomodoSecret(secret.name, secret.value)
		if err != nil {
			logging.Error("    Failed to sync Komodo secret '%s': %v", sanitizeNameForLog(secret.name), err)
			createUpdateErrorCount++
		} else {
			processedCount++
		}
	}
	logging.Info("Finished create/update phase. Processed: %d, Errors: %d", processedCount, createUpdateErrorCount)

	logging.Info("Checking for orphaned Komodo variables managed by this tool...")
	komodoVars, err := s.komodoClient.ListVariables()
	if err != nil {
		logging.Error("Failed to list variables from Komodo, skipping deletion phase: %v", err)
		// Return total errors accumulated so far, plus 1 for this critical failure
		return createUpdateErrorCount + 1
	}

	deleteCount := 0
	deleteErrorCount := 0
	for name, details := range komodoVars {
		if strings.Contains(details.Description, managedByMarker) && !expectedKomodoNames[name] {
			logging.Info("  Found orphaned Komodo variable '%s', attempting delete.", sanitizeNameForLog(name))
			err := s.komodoClient.DeleteVariable(name)
			if err != nil {
				logging.Error("    Failed to delete Komodo variable '%s': %v", sanitizeNameForLog(name), err)
				deleteErrorCount++
			} else {
				deleteCount++
			}
		}
	}
	logging.Info("Finished deletion phase. Deleted: %d, Errors: %d", deleteCount, deleteErrorCount)

	logging.Info("Synchronization finished.")
	logging.Info("  Secrets processed (created/updated): %d", processedCount)
	logging.Info("  Orphaned secrets deleted: %d", deleteCount)
	logging.Info("  Items/Fields skipped in 1P: %d", skipped1PCount)
	totalErrors := createUpdateErrorCount + deleteErrorCount
	logging.Info("  Total errors encountered: %d", totalErrors)

	return totalErrors
}
