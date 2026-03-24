// Package arbiter provides an HTTP client for the Arbiter GPU inference server.
package arbiter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultURL is the default Arbiter server address on the spark machine.
const DefaultURL = "http://10.0.0.254:8400"

// Model represents a single model's status in the Arbiter system.
type Model struct {
	ID           string   `json:"id"`
	State        string   `json:"state"`
	MemoryGB     float64  `json:"memory_gb"`
	ActiveJobs   int      `json:"active_jobs"`
	QueuedJobs   int      `json:"queued_jobs"`
	IdleSeconds  *float64 `json:"idle_seconds"`
	MaxInstances int      `json:"max_instances"`
}

// Queue holds global job counts across all models.
type Queue struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

// Status is the response from GET /v1/ps.
type Status struct {
	VRAMBudgetGB      float64 `json:"vram_budget_gb"`
	VRAMUsedGB        float64 `json:"vram_used_gb"`
	VRAMConfiguredGB  float64 `json:"vram_configured_gb"`
	GPUUtilizationPct int     `json:"gpu_utilization_pct"`
	Models            []Model `json:"models"`
	Queue             Queue   `json:"queue"`
}

// Client communicates with the Arbiter server.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a Client targeting the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// PS fetches the current system status from the Arbiter server.
func (c *Client) PS() (Status, error) {
	resp, err := c.http.Get(c.baseURL + "/v1/ps")
	if err != nil {
		return Status{}, fmt.Errorf("arbiter ps request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Status{}, fmt.Errorf("arbiter ps: status %d: %s", resp.StatusCode, string(body))
	}

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return Status{}, fmt.Errorf("arbiter ps decode: %w", err)
	}

	return status, nil
}

// SetMaxInstances updates the maximum instance count for a model via
// PATCH /v1/models/{id}.
func (c *Client) SetMaxInstances(modelID string, max int) error {
	body, err := json.Marshal(map[string]int{"max_instances": max})
	if err != nil {
		return fmt.Errorf("marshal max_instances: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, c.baseURL+"/v1/models/"+modelID, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("set max instances for %s: %w", modelID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set max instances for %s: status %d: %s", modelID, resp.StatusCode, string(respBody))
	}

	return nil
}

// ClearQueue cancels all queued jobs for a model via
// DELETE /v1/models/{id}/queue.
func (c *Client) ClearQueue(modelID string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/v1/models/"+modelID+"/queue", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("clear queue for %s: %w", modelID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clear queue for %s: status %d: %s", modelID, resp.StatusCode, string(respBody))
	}

	return nil
}
