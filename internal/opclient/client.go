package opclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"komodo-op/internal/config"  // Corrected import path
	"komodo-op/internal/logging" // Corrected import path
	"komodo-op/internal/util"    // Corrected import path
)

// Vault represents a 1Password vault.
type Vault struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Item represents a 1Password item summary.
type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Field represents a field within a 1Password item.
type Field struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	Type    string `json:"type"`    // e.g., "STRING", "CONCEALED"
	Purpose string `json:"purpose"` // e.g., "USERNAME", "PASSWORD"
}

// ItemDetail represents the full details of a 1Password item.
type ItemDetail struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}

// Client manages communication with the 1Password Connect API.
type Client struct {
	httpClient *http.Client
	cfg        *config.Config
}

// NewClient creates a new 1Password Connect client.
func NewClient(httpClient *http.Client, cfg *config.Config) *Client {
	return &Client{
		httpClient: httpClient,
		cfg:        cfg,
	}
}

// makeRequestGeneric handles making generic requests to the 1Password API.
func (c *Client) makeRequestGeneric(method, path string, body io.Reader, target interface{}) error {
	url := c.cfg.OpConnectHost + path // Path should include /v1 prefix
	logging.Debug("Making 1Password request: %s %s", method, url)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create 1Password request to %s: %w", path, err)
	}
	authHeader := "Bearer " + c.cfg.OpServiceAccountToken
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json") // Only set Content-Type if there's a body
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute 1Password request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := util.ReadAll(resp.Body)
		logging.Debug("1Password Error Response Body: %s", string(bodyBytes))
		return fmt.Errorf("1Password API request to %s failed with status %s: %s", url, resp.Status, string(bodyBytes))
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			// Try reading the body for debugging even if JSON decoding fails
			bodyBytes, readErr := util.ReadAll(resp.Body) // Need to re-read or buffer earlier
			if readErr == nil {
				logging.Debug("Failed decoding response body: %s", string(bodyBytes))
			}
			return fmt.Errorf("failed to decode 1Password response from %s: %w", url, err)
		}
	}
	return nil
}

// makeVaultRequest handles requests specific to a vault context.
func (c *Client) makeVaultRequest(method, itemPath string, target interface{}) error {
	if c.cfg.OpVaultID == "" {
		return fmt.Errorf("internal error: vault ID not resolved before making vault request")
	}
	// Ensure itemPath starts with a slash if not empty, or is just empty
	if itemPath != "" && !strings.HasPrefix(itemPath, "/") {
		itemPath = "/" + itemPath
	}
	fullPath := fmt.Sprintf("/v1/vaults/%s%s", c.cfg.OpVaultID, itemPath)
	return c.makeRequestGeneric(method, fullPath, nil, target)
}

// GetItems retrieves a list of item summaries from the configured vault.
func (c *Client) GetItems() ([]Item, error) {
	var items []Item
	// Pass "/items" correctly
	err := c.makeVaultRequest("GET", "/items", &items)
	if err != nil {
		return nil, fmt.Errorf("failed to get items from 1Password vault '%s': %w", c.cfg.OpVaultUUID, err)
	}
	logging.Info("Found %d items in vault '%s'", len(items), c.cfg.OpVaultUUID)
	return items, nil
}

// GetItemDetails retrieves the full details for a specific item ID.
func (c *Client) GetItemDetails(itemID string) (*ItemDetail, error) {
	var itemDetail ItemDetail
	itemPath := fmt.Sprintf("/items/%s", itemID) // Path includes leading slash
	err := c.makeVaultRequest("GET", itemPath, &itemDetail)
	if err != nil {
		return nil, fmt.Errorf("failed to get details for item %s in vault '%s': %w", itemID, c.cfg.OpVaultUUID, err)
	}
	return &itemDetail, nil
}
