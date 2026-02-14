package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/openjobspec/ojs-cli/internal/config"
)

// Client wraps HTTP calls to an OJS server.
type Client struct {
	cfg    *config.Config
	http   *http.Client
}

// New creates a new OJS API client.
func New(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ErrorResponse represents an OJS error response.
type ErrorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable,omitempty"`
	} `json:"error"`
}

func (c *Client) do(method, path string, body any) ([]byte, int, error) {
	url := c.cfg.BaseURL() + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/openjobspec+json")
	req.Header.Set("Accept", "application/openjobspec+json")
	if c.cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(data, &errResp) == nil && errResp.Error.Code != "" {
			return data, resp.StatusCode, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return data, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, resp.StatusCode, nil
}

// Get performs a GET request.
func (c *Client) Get(path string) ([]byte, int, error) {
	return c.do(http.MethodGet, path, nil)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(path string, body any) ([]byte, int, error) {
	return c.do(http.MethodPost, path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(path string) ([]byte, int, error) {
	return c.do(http.MethodDelete, path, nil)
}
