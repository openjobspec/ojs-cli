package migrate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

// SidekiqSource reads jobs from a Sidekiq-managed Redis instance.
type SidekiqSource struct {
	rdb *redis.Client
	url string
}

// NewSidekiqSource creates a source that reads Sidekiq data from Redis.
func NewSidekiqSource(redisURL string) (*SidekiqSource, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	return &SidekiqSource{rdb: redis.NewClient(opts), url: redisURL}, nil
}

type sidekiqJob struct {
	Class      string          `json:"class"`
	Args       json.RawMessage `json:"args"`
	Queue      string          `json:"queue"`
	Retry      any             `json:"retry"`
	JID        string          `json:"jid"`
	EnqueuedAt float64         `json:"enqueued_at"`
	At         float64         `json:"at,omitempty"`
}

func (s *SidekiqSource) Analyze() (*AnalysisResult, error) {
	ctx := context.Background()

	queues, err := s.rdb.SMembers(ctx, "queues").Result()
	if err != nil {
		return nil, fmt.Errorf("read queues set: %w", err)
	}

	result := &AnalysisResult{
		Source:     "sidekiq",
		Connection: s.url,
	}

	for _, q := range queues {
		qa, count, err := s.analyzeQueue(ctx, q)
		if err != nil {
			return nil, err
		}
		result.Queues = append(result.Queues, *qa)
		result.TotalJobs += count
	}

	// Count scheduled and retry sets
	scheduledCount, _ := s.rdb.ZCard(ctx, "schedule").Result()
	retryCount, _ := s.rdb.ZCard(ctx, "retry").Result()
	result.TotalJobs += int(scheduledCount) + int(retryCount)

	result.Summary = fmt.Sprintf("Found %d queues, %d total jobs (%d scheduled, %d in retry)",
		len(queues), result.TotalJobs, scheduledCount, retryCount)

	return result, nil
}

func (s *SidekiqSource) analyzeQueue(ctx context.Context, name string) (*QueueAnalysis, int, error) {
	key := "queue:" + name
	jobs, err := s.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("read queue %s: %w", name, err)
	}

	qa := &QueueAnalysis{
		Name:        name,
		PendingJobs: len(jobs),
		JobTypes:    make(map[string]int),
	}

	for _, raw := range jobs {
		var sj sidekiqJob
		if json.Unmarshal([]byte(raw), &sj) == nil {
			qa.JobTypes[sj.Class]++
		}
	}

	return qa, len(jobs), nil
}

func (s *SidekiqSource) Export() ([]ExportedJob, error) {
	ctx := context.Background()

	queues, err := s.rdb.SMembers(ctx, "queues").Result()
	if err != nil {
		return nil, fmt.Errorf("read queues set: %w", err)
	}

	var exported []ExportedJob

	for _, q := range queues {
		key := "queue:" + q
		jobs, err := s.rdb.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return nil, fmt.Errorf("read queue %s: %w", q, err)
		}
		for _, raw := range jobs {
			if ej, err := parseSidekiqJob(raw); err == nil {
				exported = append(exported, *ej)
			}
		}
	}

	// Scheduled jobs
	scheduled, err := s.rdb.ZRangeWithScores(ctx, "schedule", 0, -1).Result()
	if err == nil {
		for _, z := range scheduled {
			if ej, err := parseSidekiqJob(z.Member.(string)); err == nil {
				exported = append(exported, *ej)
			}
		}
	}

	// Retry jobs
	retries, err := s.rdb.ZRangeWithScores(ctx, "retry", 0, -1).Result()
	if err == nil {
		for _, z := range retries {
			if ej, err := parseSidekiqJob(z.Member.(string)); err == nil {
				exported = append(exported, *ej)
			}
		}
	}

	return exported, nil
}

// ParseSidekiqJob converts a raw Sidekiq JSON string into an ExportedJob.
// Exported for testing.
func ParseSidekiqJob(raw string) (*ExportedJob, error) {
	return parseSidekiqJob(raw)
}

func parseSidekiqJob(raw string) (*ExportedJob, error) {
	var sj sidekiqJob
	if err := json.Unmarshal([]byte(raw), &sj); err != nil {
		return nil, fmt.Errorf("parse sidekiq job: %w", err)
	}

	ej := &ExportedJob{
		Type:  sidekiqClassToType(sj.Class),
		Queue: sj.Queue,
		Args:  sj.Args,
		Meta: map[string]any{
			"sidekiq_jid":   sj.JID,
			"sidekiq_class": sj.Class,
		},
	}

	if sj.At > 0 {
		ej.ScheduledAt = fmt.Sprintf("%.0f", sj.At)
	}

	if ej.Queue == "" {
		ej.Queue = "default"
	}

	return ej, nil
}

// sidekiqClassToType converts a Ruby class name to an OJS job type.
// e.g., "EmailWorker" → "email.worker", "Mailers::WelcomeEmail" → "mailers.welcome.email"
func sidekiqClassToType(class string) string {
	// Replace :: with .
	s := strings.ReplaceAll(class, "::", ".")

	// Insert dots before uppercase letters for CamelCase
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(s[i-1])
			if prev != '.' && prev >= 'a' && prev <= 'z' {
				result = append(result, '.')
			}
		}
		result = append(result, r)
	}

	return strings.ToLower(string(result))
}
