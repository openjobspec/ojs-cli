package migrate

import "encoding/json"

// AnalysisResult holds the result of analyzing a source system.
type AnalysisResult struct {
	Source     string          `json:"source"`
	Connection string         `json:"connection"`
	Queues     []QueueAnalysis `json:"queues"`
	TotalJobs  int            `json:"total_jobs"`
	Summary    string         `json:"summary"`
}

// QueueAnalysis holds analysis data for a single queue.
type QueueAnalysis struct {
	Name        string         `json:"name"`
	PendingJobs int            `json:"pending_jobs"`
	JobTypes    map[string]int `json:"job_types"`
}

// ExportedJob is the OJS-compatible NDJSON representation of a migrated job.
type ExportedJob struct {
	Type        string          `json:"type"`
	Queue       string          `json:"queue"`
	Args        json.RawMessage `json:"args"`
	Priority    *int            `json:"priority,omitempty"`
	ScheduledAt string          `json:"scheduled_at,omitempty"`
	Meta        map[string]any  `json:"meta,omitempty"`
}

// Source is the interface that migration source adapters implement.
type Source interface {
	Analyze() (*AnalysisResult, error)
	Export() ([]ExportedJob, error)
}
