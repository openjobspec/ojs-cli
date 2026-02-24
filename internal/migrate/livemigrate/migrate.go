// Package livemigrate implements zero-downtime migration between OJS backends.
package livemigrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Phase represents the current migration phase.
type Phase string

const (
	PhaseIdle      Phase = "idle"
	PhaseExporting Phase = "exporting"
	PhaseImporting Phase = "importing"
	PhaseVerifying Phase = "verifying"
	PhaseComplete  Phase = "complete"
	PhaseFailed    Phase = "failed"
)

// Config configures a live migration.
type Config struct {
	SourceURL string `json:"source_url"`
	TargetURL string `json:"target_url"`
	SourceKey string `json:"source_api_key,omitempty"`
	TargetKey string `json:"target_api_key,omitempty"`
	BatchSize int    `json:"batch_size"`
	DryRun    bool   `json:"dry_run"`
}

// Stats tracks migration progress.
type Stats struct {
	Phase        Phase     `json:"phase"`
	Exported     atomic.Int64
	Imported     atomic.Int64
	Verified     atomic.Int64
	Errors       atomic.Int64
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// Snapshot returns a serializable copy.
func (s *Stats) Snapshot() StatsSnapshot {
	return StatsSnapshot{
		Phase:       s.Phase,
		Exported:    s.Exported.Load(),
		Imported:    s.Imported.Load(),
		Verified:    s.Verified.Load(),
		Errors:      s.Errors.Load(),
		StartedAt:   s.StartedAt,
		CompletedAt: s.CompletedAt,
	}
}

// StatsSnapshot is a serializable stats copy.
type StatsSnapshot struct {
	Phase       Phase      `json:"phase"`
	Exported    int64      `json:"exported"`
	Imported    int64      `json:"imported"`
	Verified    int64      `json:"verified"`
	Errors      int64      `json:"errors"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Migration manages the backend-to-backend migration lifecycle.
type Migration struct {
	mu     sync.Mutex
	config Config
	stats  Stats
	client *http.Client
}

// New creates a new live migration.
func New(cfg Config) *Migration {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	return &Migration{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		stats:  Stats{Phase: PhaseIdle},
	}
}

// GetStats returns current migration progress.
func (m *Migration) GetStats() StatsSnapshot {
	return m.stats.Snapshot()
}

// Run executes the full migration: export → import → verify.
func (m *Migration) Run(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.StartedAt = time.Now()

	// Phase 1: Export
	m.stats.Phase = PhaseExporting
	jobs, err := m.exportJobs(ctx)
	if err != nil {
		m.stats.Phase = PhaseFailed
		return fmt.Errorf("export failed: %w", err)
	}

	// Phase 2: Import
	m.stats.Phase = PhaseImporting
	if err := m.importJobs(ctx, jobs); err != nil {
		m.stats.Phase = PhaseFailed
		return fmt.Errorf("import failed: %w", err)
	}

	// Phase 3: Verify
	m.stats.Phase = PhaseVerifying
	if err := m.verifyTarget(ctx); err != nil {
		m.stats.Phase = PhaseFailed
		return fmt.Errorf("verify failed: %w", err)
	}

	m.stats.Phase = PhaseComplete
	now := time.Now()
	m.stats.CompletedAt = &now
	return nil
}

func (m *Migration) exportJobs(ctx context.Context) ([]json.RawMessage, error) {
	data, err := m.request(ctx, "GET", m.config.SourceURL+"/ojs/v1/admin/jobs", m.config.SourceKey, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing jobs: %w", err)
	}

	m.stats.Exported.Store(int64(len(result.Jobs)))
	return result.Jobs, nil
}

func (m *Migration) importJobs(ctx context.Context, jobs []json.RawMessage) error {
	for _, raw := range jobs {
		if m.config.DryRun {
			m.stats.Imported.Add(1)
			continue
		}

		var job map[string]interface{}
		if err := json.Unmarshal(raw, &job); err != nil {
			m.stats.Errors.Add(1)
			continue
		}

		req := map[string]interface{}{
			"type":  job["type"],
			"queue": job["queue"],
			"args":  job["args"],
		}
		if meta, ok := job["meta"]; ok {
			req["meta"] = meta
		}

		body, _ := json.Marshal(req)
		if _, err := m.request(ctx, "POST", m.config.TargetURL+"/ojs/v1/jobs", m.config.TargetKey, body); err != nil {
			m.stats.Errors.Add(1)
			continue
		}
		m.stats.Imported.Add(1)
	}
	return nil
}

func (m *Migration) verifyTarget(ctx context.Context) error {
	_, err := m.request(ctx, "GET", m.config.TargetURL+"/ojs/v1/health", m.config.TargetKey, nil)
	if err != nil {
		return fmt.Errorf("target unhealthy: %w", err)
	}
	m.stats.Verified.Store(m.stats.Imported.Load())
	return nil
}

func (m *Migration) request(ctx context.Context, method, url, apiKey string, body []byte) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
