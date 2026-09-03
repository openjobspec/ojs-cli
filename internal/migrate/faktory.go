package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/openjobspec/ojs-cli/internal/redact"
)

const maxFaktoryResponseBytes = 8 << 20

// FaktorySource reads jobs from a Faktory server via its HTTP API.
type FaktorySource struct {
	baseURL  string
	password string
	client   *http.Client
}

// NewFaktorySource creates a source that reads jobs from a Faktory server.
func NewFaktorySource(baseURL, password string) (*FaktorySource, error) {
	if baseURL == "" {
		baseURL = "http://localhost:7420"
	}
	return &FaktorySource{
		baseURL:  baseURL,
		password: password,
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Close is a no-op for Faktory (HTTP-based, no persistent connection).
func (f *FaktorySource) Close() error {
	return nil
}

type faktoryInfo struct {
	Faktory faktoryState `json:"faktory"`
}

type faktoryState struct {
	Queues map[string]int `json:"queues"`
	Tasks  map[string]int `json:"tasks"`
}

type faktoryJob struct {
	JID        string          `json:"jid"`
	Type       string          `json:"jobtype"`
	Args       json.RawMessage `json:"args"`
	Queue      string          `json:"queue"`
	Priority   int             `json:"priority,omitempty"`
	At         string          `json:"at,omitempty"`
	ReserveFor int             `json:"reserve_for,omitempty"`
	Retry      int             `json:"retry,omitempty"`
	Custom     map[string]any  `json:"custom,omitempty"`
}

func (f *FaktorySource) doRequest(path string, target any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if f.password != "" {
		req.SetBasicAuth("", f.password)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request %s returned status %d", path, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFaktoryResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxFaktoryResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxFaktoryResponseBytes)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (f *FaktorySource) Analyze() (*AnalysisResult, error) {
	var info faktoryInfo
	if err := f.doRequest("/api/info", &info); err != nil {
		return nil, fmt.Errorf("fetch faktory info: %w", err)
	}

	result := &AnalysisResult{
		Source:     "faktory",
		Connection: redact.URL(f.baseURL),
	}

	queueNames := make([]string, 0, len(info.Faktory.Queues))
	for name := range info.Faktory.Queues {
		queueNames = append(queueNames, name)
	}
	sort.Strings(queueNames)
	for _, name := range queueNames {
		count := info.Faktory.Queues[name]
		qa := QueueAnalysis{
			Name:        name,
			PendingJobs: count,
			JobTypes:    make(map[string]int),
		}
		result.Queues = append(result.Queues, qa)
		result.TotalJobs += count
	}

	result.Summary = fmt.Sprintf("Found %d queues, %d total jobs",
		len(result.Queues), result.TotalJobs)

	return result, nil
}

func (f *FaktorySource) Export() ([]ExportedJob, error) {
	var info faktoryInfo
	if err := f.doRequest("/api/info", &info); err != nil {
		return nil, fmt.Errorf("fetch faktory info: %w", err)
	}

	var exported []ExportedJob

	queueNames := make([]string, 0, len(info.Faktory.Queues))
	for queueName := range info.Faktory.Queues {
		queueNames = append(queueNames, queueName)
	}
	sort.Strings(queueNames)
	for _, queueName := range queueNames {
		var jobs []faktoryJob
		if err := f.doRequest("/api/queues/"+url.PathEscape(queueName), &jobs); err != nil {
			return nil, fmt.Errorf("fetch Faktory queue %s: %w", queueName, err)
		}

		for i := range jobs {
			ej := parseFaktoryJob(&jobs[i])
			exported = append(exported, *ej)
		}
	}

	return exported, nil
}

// ParseFaktoryJob converts a Faktory job JSON string into an ExportedJob.
// Exported for testing.
func ParseFaktoryJob(raw string) (*ExportedJob, error) {
	var fj faktoryJob
	if err := json.Unmarshal([]byte(raw), &fj); err != nil {
		return nil, fmt.Errorf("parse faktory job: %w", err)
	}
	return parseFaktoryJob(&fj), nil
}

func parseFaktoryJob(fj *faktoryJob) *ExportedJob {
	ej := &ExportedJob{
		Type:  fj.Type,
		Queue: fj.Queue,
		Args:  fj.Args,
		Meta: map[string]any{
			"faktory_jid": fj.JID,
		},
	}

	if fj.Priority > 0 {
		p := fj.Priority
		ej.Priority = &p
	}

	if fj.At != "" {
		ej.ScheduledAt = fj.At
	}

	if fj.Queue == "" {
		ej.Queue = "default"
	}

	if fj.Custom != nil {
		ej.Meta["faktory_custom"] = fj.Custom
	}

	return ej
}
