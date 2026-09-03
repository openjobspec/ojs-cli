package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/redact"
	"github.com/redis/go-redis/v9"
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
		return nil, fmt.Errorf("invalid redis URL %q: %w", redact.URL(redisURL), err)
	}
	return &SidekiqSource{rdb: redis.NewClient(opts), url: redisURL}, nil
}

// Close releases the underlying Redis connection.
func (s *SidekiqSource) Close() error {
	return s.rdb.Close()
}

type sidekiqJob struct {
	Class      string          `json:"class"`
	Args       json.RawMessage `json:"args"`
	Queue      string          `json:"queue"`
	Retry      any             `json:"retry"`
	JID        string          `json:"jid"`
	EnqueuedAt json.Number     `json:"enqueued_at"`
	At         json.Number     `json:"at,omitempty"`
}

func (s *SidekiqSource) Analyze() (*AnalysisResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queues, err := s.rdb.SMembers(ctx, "queues").Result()
	if err != nil {
		return nil, fmt.Errorf("read queues set: %w", err)
	}
	sort.Strings(queues)

	result := &AnalysisResult{
		Source:     "sidekiq",
		Connection: redact.URL(s.url),
	}

	for _, queue := range queues {
		analysis, count, err := s.analyzeQueue(ctx, queue)
		if err != nil {
			return nil, err
		}
		result.Queues = append(result.Queues, *analysis)
		result.TotalJobs += count
	}

	scheduledCount, err := s.rdb.ZCard(ctx, "schedule").Result()
	if err != nil {
		return nil, fmt.Errorf("count Sidekiq schedule set: %w", err)
	}
	retryCount, err := s.rdb.ZCard(ctx, "retry").Result()
	if err != nil {
		return nil, fmt.Errorf("count Sidekiq retry set: %w", err)
	}
	result.TotalJobs += int(scheduledCount) + int(retryCount)
	result.Summary = fmt.Sprintf(
		"Found %d queues, %d total jobs (%d scheduled, %d in retry)",
		len(queues), result.TotalJobs, scheduledCount, retryCount,
	)

	return result, nil
}

func (s *SidekiqSource) analyzeQueue(ctx context.Context, name string) (*QueueAnalysis, int, error) {
	jobs, err := s.rdb.LRange(ctx, "queue:"+name, 0, -1).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("read queue %s: %w", name, err)
	}

	analysis := &QueueAnalysis{
		Name:        name,
		PendingJobs: len(jobs),
		JobTypes:    make(map[string]int),
	}
	for _, raw := range jobs {
		var job sidekiqJob
		if json.Unmarshal([]byte(raw), &job) == nil && job.Class != "" {
			analysis.JobTypes[job.Class]++
		}
	}

	return analysis, len(jobs), nil
}

func (s *SidekiqSource) Export() ([]ExportedJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queues, err := s.rdb.SMembers(ctx, "queues").Result()
	if err != nil {
		return nil, fmt.Errorf("read queues set: %w", err)
	}
	sort.Strings(queues)

	var exported []ExportedJob
	var failures []FailedRecord
	for _, queue := range queues {
		jobs, err := s.rdb.LRange(ctx, "queue:"+queue, 0, -1).Result()
		if err != nil {
			return nil, fmt.Errorf("read queue %s: %w", queue, err)
		}
		for i, raw := range jobs {
			job, parseErr := parseSidekiqRecord(raw, 0)
			if parseErr != nil {
				failures = append(failures, FailedRecord{
					Source: "sidekiq", Queue: queue, Structure: "wait", Index: i + 1, Error: parseErr.Error(),
				})
				continue
			}
			exported = append(exported, *job)
		}
	}

	exported, failures, err = s.exportSortedSet(ctx, "schedule", exported, failures)
	if err != nil {
		return nil, err
	}
	exported, failures, err = s.exportSortedSet(ctx, "retry", exported, failures)
	if err != nil {
		return nil, err
	}

	if len(failures) > 0 {
		return exported, &PartialExportError{Exported: len(exported), Failures: failures}
	}
	return exported, nil
}

func (s *SidekiqSource) exportSortedSet(
	ctx context.Context,
	structure string,
	exported []ExportedJob,
	failures []FailedRecord,
) ([]ExportedJob, []FailedRecord, error) {
	records, err := s.rdb.ZRangeWithScores(ctx, structure, 0, -1).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("read Sidekiq %s set: %w", structure, err)
	}
	for i, record := range records {
		raw, memberErr := redisMemberString(record.Member)
		if memberErr != nil {
			failures = append(failures, FailedRecord{
				Source: "sidekiq", Structure: structure, Index: i + 1, Error: memberErr.Error(),
			})
			continue
		}
		job, parseErr := parseSidekiqRecord(raw, record.Score)
		if parseErr != nil {
			failures = append(failures, FailedRecord{
				Source: "sidekiq", Structure: structure, Index: i + 1, Error: parseErr.Error(),
			})
			continue
		}
		exported = append(exported, *job)
	}
	return exported, failures, nil
}

func redisMemberString(member any) (string, error) {
	switch value := member.(type) {
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	default:
		return "", fmt.Errorf("unsupported Redis member type %T", member)
	}
}

// ParseSidekiqJob converts a raw Sidekiq JSON string into an ExportedJob.
func ParseSidekiqJob(raw string) (*ExportedJob, error) {
	return parseSidekiqRecord(raw, 0)
}

func parseSidekiqRecord(raw string, fallbackScore float64) (*ExportedJob, error) {
	var source sidekiqJob
	if err := json.Unmarshal([]byte(raw), &source); err != nil {
		return nil, fmt.Errorf("parse sidekiq job: %w", err)
	}
	if source.Class == "" {
		return nil, fmt.Errorf("parse sidekiq job: missing class")
	}

	job := &ExportedJob{
		Type:  sidekiqClassToType(source.Class),
		Queue: source.Queue,
		Args:  source.Args,
		Meta: map[string]any{
			"sidekiq_jid":   source.JID,
			"sidekiq_class": source.Class,
		},
	}
	if job.Queue == "" {
		job.Queue = "default"
	}
	switch {
	case len(job.Args) == 0:
		job.Args = json.RawMessage("[]")
	case string(job.Args) == "null":
		job.Args = json.RawMessage("[]")
	default:
		var args []json.RawMessage
		if err := json.Unmarshal(job.Args, &args); err != nil {
			return nil, fmt.Errorf("parse sidekiq job: args must be a JSON array")
		}
	}

	at := source.At
	if at == "" && fallbackScore > 0 {
		at = json.Number(strconv.FormatFloat(fallbackScore, 'f', -1, 64))
	}
	if at != "" {
		scheduledAt, err := unixSecondsRFC3339(at)
		if err != nil {
			return nil, fmt.Errorf("parse sidekiq schedule: %w", err)
		}
		job.ScheduledAt = scheduledAt
	}

	return job, nil
}

func unixSecondsRFC3339(value json.Number) (string, error) {
	seconds := new(big.Rat)
	if _, ok := seconds.SetString(value.String()); !ok || seconds.Sign() <= 0 {
		return "", fmt.Errorf("invalid Unix timestamp %q", value)
	}

	whole := new(big.Int).Quo(seconds.Num(), seconds.Denom())
	if !whole.IsInt64() {
		return "", fmt.Errorf("Unix timestamp %q is out of range", value)
	}

	remainder := new(big.Int).Sub(seconds.Num(), new(big.Int).Mul(whole, seconds.Denom()))
	nanosNumerator := new(big.Int).Mul(remainder, big.NewInt(int64(time.Second)))
	nanos := new(big.Int)
	rounding := new(big.Int)
	nanos.QuoRem(nanosNumerator, seconds.Denom(), rounding)
	if new(big.Int).Lsh(rounding, 1).Cmp(seconds.Denom()) >= 0 {
		nanos.Add(nanos, big.NewInt(1))
	}
	if nanos.Cmp(big.NewInt(int64(time.Second))) >= 0 {
		whole.Add(whole, big.NewInt(1))
		nanos.SetInt64(0)
	}

	return time.Unix(whole.Int64(), nanos.Int64()).UTC().Format(time.RFC3339Nano), nil
}

// sidekiqClassToType converts a Ruby class name to an OJS job type.
// e.g., "EmailWorker" → "email.worker", "Mailers::WelcomeEmail" → "mailers.welcome.email"
func sidekiqClassToType(class string) string {
	value := strings.ReplaceAll(class, "::", ".")

	var result []rune
	for i, current := range value {
		if i > 0 && current >= 'A' && current <= 'Z' {
			previous := rune(value[i-1])
			if previous != '.' && previous >= 'a' && previous <= 'z' {
				result = append(result, '.')
			}
		}
		result = append(result, current)
	}

	return strings.ToLower(string(result))
}
