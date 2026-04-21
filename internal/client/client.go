package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/redact"
)

const maxResponseBytes = 8 << 20

// Client wraps HTTP calls to an OJS server.
type Client struct {
	cfg  *config.Config
	http *http.Client
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

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	endpoint := c.cfg.BaseURL() + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
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
		return nil, 0, fmt.Errorf(
			"request failed: %w\n\nHint: verify the server is running and OJS_URL is correct (current: %s)",
			redact.RequestError(err),
			redact.URL(c.cfg.ServerURL),
		)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, resp.StatusCode, fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
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
	return c.do(context.Background(), http.MethodGet, path, nil)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(path string, body any) ([]byte, int, error) {
	return c.do(context.Background(), http.MethodPost, path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(path string) ([]byte, int, error) {
	return c.do(context.Background(), http.MethodDelete, path, nil)
}

// Patch performs a PATCH request with a JSON body.
func (c *Client) Patch(path string, body any) ([]byte, int, error) {
	return c.do(context.Background(), http.MethodPatch, path, body)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(path string, body any) ([]byte, int, error) {
	return c.do(context.Background(), http.MethodPut, path, body)
}

// errorHint returns a user-friendly suggestion for common HTTP error codes.
//
// StatusNotFound is intentionally omitted: OJS servers return a specific
// message for missing resources (e.g. "job 'xyz' not found"), so appending a
// generic hint would be redundant. Callers rely on the verbatim "code: message"
// form for not-found errors.
func errorHint(statusCode int) string {
	switch statusCode {
	case http.StatusUnauthorized:
		return "check or set OJS_AUTH_TOKEN"
	case http.StatusForbidden:
		return "your credentials lack permission for this operation"
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
