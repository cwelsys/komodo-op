package komodoclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"komodo-op/internal/config"
	"komodo-op/internal/logging"
	"komodo-op/internal/util"
)

// --- Komodo API Structures ---

// Request defines the generic structure for Komodo API requests.
type Request struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params,omitempty"`
}

// GetVariableParams defines parameters for the GetVariable request.
type GetVariableParams struct {
	Name string `json:"name"`
}

// UpdateVariableValueParams defines parameters for the UpdateVariableValue request.
type UpdateVariableValueParams struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DeleteVariableParams defines parameters for the DeleteVariable request.
type DeleteVariableParams struct {
	Name string `json:"name"`
}

// CreateParams defines parameters for the CreateVariable request.
type CreateParams struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
	IsSecret    bool   `json:"is_secret"`
}

// VariableResponse defines the structure of a variable returned by the Komodo API.
type VariableResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Value       string    `json:"value"` // May be masked
	Description string    `json:"description"`
	IsSecret    bool      `json:"is_secret"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ErrorResponse defines the structure of an error returned by the Komodo API.
type ErrorResponse struct {
	Error string   `json:"error"`
	Trace []string `json:"trace"`
}

// --- Komodo Client ---

// Client manages communication with the Komodo API.
type Client struct {
	httpClient *http.Client
	cfg        *config.Config
}

// NewClient creates a new Komodo API client.
func NewClient(httpClient *http.Client, cfg *config.Config) *Client {
	return &Client{
		httpClient: httpClient,
		cfg:        cfg,
	}
}

// makeRequest executes a request against the Komodo API.
func (c *Client) makeRequest(path string, payload interface{}, target interface{}) (int, []byte, error) {
	url := fmt.Sprintf("%s%s", c.cfg.KomodoHost, path) // path should start with / (e.g., /read, /write)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal Komodo request payload for %s: %w", path, err)
	}

	logging.Debug("Komodo Request URL: POST %s", url)
	logging.Debug("Komodo Request Body: %s", string(payloadBytes))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create Komodo request for %s: %w", path, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.cfg.KomodoAPIKey)
	req.Header.Set("X-Api-Secret", c.cfg.KomodoAPISecret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to execute Komodo request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := util.ReadAll(resp.Body) // Read body regardless of status code
	if readErr != nil {
		logging.Error("Failed to read Komodo response body from %s: %v", url, readErr)
		return resp.StatusCode, nil, fmt.Errorf("Komodo API request to %s returned status %s, but failed to read response body: %w", url, resp.Status, readErr)
	}

	logging.Debug("Komodo Response Status: %s", resp.Status)
	logging.Debug("Komodo Response Body: %s", string(bodyBytes))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var komodoErr ErrorResponse
		if json.Unmarshal(bodyBytes, &komodoErr) == nil && komodoErr.Error != "" {
			return resp.StatusCode, bodyBytes, fmt.Errorf("Komodo API request to %s failed with status %s: %s (Trace: %v)", url, resp.Status, komodoErr.Error, komodoErr.Trace)
		}
		return resp.StatusCode, bodyBytes, fmt.Errorf("Komodo API request to %s failed with status %s", url, resp.Status)
	}

	if target != nil {
		if err := json.Unmarshal(bodyBytes, target); err != nil {
			return resp.StatusCode, bodyBytes, fmt.Errorf("failed to decode successful Komodo response from %s: %w", url, err)
		}
	}

	return resp.StatusCode, bodyBytes, nil // Success
}

// GetVariable retrieves a Komodo variable by name.
// Returns the variable, a boolean indicating if found, and any error during the process.
func (c *Client) GetVariable(name string) (*VariableResponse, bool, error) {
	payload := Request{
		Type:   "GetVariable",
		Params: GetVariableParams{Name: name},
	}
	var response VariableResponse
	statusCode, bodyBytes, err := c.makeRequest("/read", payload, &response)

	if statusCode == http.StatusNotFound {
		logging.Debug("Variable '%s' not found (status 404)", name)
		return nil, false, nil // Not found, no error
	}

	if err != nil {
		var komodoErr ErrorResponse
		if json.Unmarshal(bodyBytes, &komodoErr) == nil {
			if strings.Contains(strings.ToLower(komodoErr.Error), "no variable found") {
				logging.Debug("Variable '%s' not found (status %d, error message: %s)", name, statusCode, komodoErr.Error)
				return nil, false, nil // Treat as Not Found
			}
		}
		logging.Error("Failed to get Komodo variable '%s': %v", name, err)
		return nil, false, fmt.Errorf("failed to get Komodo variable '%s': %w", name, err)
	}

	logging.Debug("Variable '%s' found", name)
	return &response, true, nil
}

// CreateVariable creates a new Komodo variable.
func (c *Client) CreateVariable(name, value, description string) error {
	payload := Request{
		Type: "CreateVariable",
		Params: CreateParams{
			Name:        name,
			Value:       value,
			Description: description,
			IsSecret:    true,
		},
	}
	_, _, err := c.makeRequest("/write", payload, nil)
	if err != nil {
		return fmt.Errorf("failed to create Komodo variable '%s': %w", name, err)
	}
	logging.Info("    Successfully created Komodo secret: %s", name)
	return nil
}

// UpdateVariableValue updates the value of an existing Komodo variable.
func (c *Client) UpdateVariableValue(name, value string) error {
	payload := Request{
		Type: "UpdateVariableValue",
		Params: UpdateVariableValueParams{
			Name:  name,
			Value: value,
		},
	}
	_, _, err := c.makeRequest("/write", payload, nil)
	if err != nil {
		return fmt.Errorf("failed to update Komodo variable '%s': %w", name, err)
	}
	logging.Info("    Successfully updated Komodo secret: %s", name)
	return nil
}

// DeleteVariable deletes a Komodo variable by name.
func (c *Client) DeleteVariable(name string) error {
	payload := Request{
		Type:   "DeleteVariable",
		Params: DeleteVariableParams{Name: name},
	}
	_, bodyBytes, err := c.makeRequest("/write", payload, nil)
	if err != nil {
		var komodoErr ErrorResponse
		if json.Unmarshal(bodyBytes, &komodoErr) == nil {
			if strings.Contains(strings.ToLower(komodoErr.Error), "no variable found") || strings.Contains(strings.ToLower(komodoErr.Error), "not found") {
				logging.Debug("Attempted to delete variable '%s' but it was already gone (Error: %s).", name, komodoErr.Error)
				return nil // Idempotent
			}
		} else if strings.Contains(strings.ToLower(err.Error()), "no variable found") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			logging.Debug("Attempted to delete variable '%s' but it was already gone (Error string: %s).", name, err.Error())
			return nil // Idempotent
		}
		return fmt.Errorf("failed to delete Komodo variable '%s': %w", name, err)
	}
	logging.Info("    Successfully deleted Komodo secret: %s", name)
	return nil
}

// ListVariables lists all variables from Komodo.
func (c *Client) ListVariables() (map[string]VariableResponse, error) {
	payload := Request{
		Type:   "ListVariables",
		Params: map[string]interface{}{}, // Ensure empty object is sent
	}
	var response []VariableResponse
	_, _, err := c.makeRequest("/read", payload, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to list Komodo variables: %w", err)
	}

	varsMap := make(map[string]VariableResponse)
	for _, v := range response {
		varsMap[v.Name] = v
	}
	logging.Info("Successfully listed %d variables from Komodo", len(varsMap))
	return varsMap, nil
}
