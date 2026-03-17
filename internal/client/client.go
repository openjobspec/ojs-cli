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
		return nil, 0, fmt.Errorf("request to %s failed: %w\n\nHint: verify the server is running and OJS_URL is correct (current: %s)", url, err, c.cfg.ServerURL)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(data, &errResp) == nil && errResp.Error.Code != "" {
			hint := errorHint(resp.StatusCode)
			if hint != "" {
				return data, resp.StatusCode, fmt.Errorf("%s: %s\n\nHint: %s", errResp.Error.Code, errResp.Error.Message, hint)
			}
			return data, resp.StatusCode, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return data, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, resp.StatusCode, nil
}

// BaseURL returns the raw server URL (without API prefix).
func (c *Client) BaseURL() string {
	return c.cfg.ServerURL
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

// Patch performs a PATCH request with a JSON body.
func (c *Client) Patch(path string, body any) ([]byte, int, error) {
	return c.do(http.MethodPatch, path, body)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(path string, body any) ([]byte, int, error) {
	return c.do(http.MethodPut, path, body)
}

// errorHint returns a user-friendly suggestion for common HTTP error codes.
func errorHint(statusCode int) string {
	switch statusCode {
	case http.StatusUnauthorized:
		return "check your auth token with 'ojs config show' or set OJS_AUTH_TOKEN"
	case http.StatusForbidden:
		return "your credentials lack permission for this operation"
	case http.StatusNotFound:
		return "the resource was not found; verify the job ID, queue name, or endpoint path"
	case http.StatusConflict:
		return "a conflicting operation is in progress (e.g., unique job constraint or invalid state transition)"
	case http.StatusTooManyRequests:
		return "rate limit exceeded; retry after a brief delay"
	case http.StatusServiceUnavailable:
		return "the server is temporarily unavailable; it may be starting up or overloaded"
	default:
		return ""
	}
}

