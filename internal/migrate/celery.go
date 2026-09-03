package migrate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/openjobspec/ojs-cli/internal/redact"
	"github.com/redis/go-redis/v9"
)

// CelerySource reads jobs from a Celery broker backed by Redis.
type CelerySource struct {
	rdb    *redis.Client
	url    string
	queues []string // queue names to scan; defaults to ["celery"]
}

// NewCelerySource creates a source that reads Celery data from Redis.
func NewCelerySource(redisURL string) (*CelerySource, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL %q: %w", redact.URL(redisURL), err)
	}
	return &CelerySource{
		rdb:    redis.NewClient(opts),
		url:    redisURL,
		queues: []string{"celery"},
	}, nil
}

// Close releases the underlying Redis connection.
func (c *CelerySource) Close() error {
	return c.rdb.Close()
}

type celeryMessage struct {
	Body    string        `json:"body"`
	Headers celeryHeaders `json:"headers"`
}

type celeryHeaders struct {
	Task string `json:"task"`
	ID   string `json:"id"`
}

func (c *CelerySource) Analyze() (*AnalysisResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := &AnalysisResult{
		Source:     "celery",
		Connection: redact.URL(c.url),
	}

	discoveredQueues, err := c.discoverQueues(ctx)
	if err != nil {
		return nil, err
	}

	for _, q := range discoveredQueues {
		qa, count, err := c.analyzeQueue(ctx, q)
		if err != nil {
			return nil, err
		}
		result.Queues = append(result.Queues, *qa)
		result.TotalJobs += count
	}

	result.Summary = fmt.Sprintf("Found %d queues, %d total jobs", len(result.Queues), result.TotalJobs)
	return result, nil
}

func (c *CelerySource) discoverQueues(ctx context.Context) ([]string, error) {
	// Celery stores tasks in list keys. Check default queue and
	// try _kombu.binding.* for additional queue names.
	seen := make(map[string]bool)
	for _, q := range c.queues {
		seen[q] = true
	}

	// Check kombu bindings for queue discovery
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, "_kombu.binding.*", 10).Result()
		if err != nil {
			return nil, fmt.Errorf("scan for Celery queues: %w", err)
		}
		for _, key := range keys {
			// _kombu.binding.<queue>
			if len(key) > len("_kombu.binding.") {
				q := key[len("_kombu.binding."):]
				seen[q] = true
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	queues := make([]string, 0, len(seen))
	for q := range seen {
		queues = append(queues, q)
	}
	sort.Strings(queues)
	return queues, nil
}

func (c *CelerySource) analyzeQueue(ctx context.Context, name string) (*QueueAnalysis, int, error) {
	messages, err := c.rdb.LRange(ctx, name, 0, -1).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("read celery queue %s: %w", name, err)
	}

	qa := &QueueAnalysis{
		Name:        name,
		PendingJobs: len(messages),
		JobTypes:    make(map[string]int),
	}

	for _, raw := range messages {
		var msg celeryMessage
		if json.Unmarshal([]byte(raw), &msg) == nil && msg.Headers.Task != "" {
			qa.JobTypes[msg.Headers.Task]++
		}
	}

	return qa, len(messages), nil
}

func (c *CelerySource) Export() ([]ExportedJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	discoveredQueues, err := c.discoverQueues(ctx)
	if err != nil {
		return nil, err
	}

	var exported []ExportedJob
	var failures []FailedRecord

	for _, q := range discoveredQueues {
		messages, err := c.rdb.LRange(ctx, q, 0, -1).Result()
		if err != nil {
			return nil, fmt.Errorf("read celery queue %s: %w", q, err)
		}
		for i, raw := range messages {
			job, parseErr := parseCeleryMessage(q, raw)
			if parseErr != nil {
				failures = append(failures, FailedRecord{
					Source: "celery", Queue: q, Structure: "list", Index: i + 1, Error: parseErr.Error(),
				})
				continue
			}
			exported = append(exported, *job)
		}
	}

	if len(failures) > 0 {
		return exported, &PartialExportError{Exported: len(exported), Failures: failures}
	}
	return exported, nil
}

// ParseCeleryMessage converts a raw Celery broker message into an ExportedJob.
// Exported for testing.
func ParseCeleryMessage(queue, raw string) (*ExportedJob, error) {
	return parseCeleryMessage(queue, raw)
}

func parseCeleryMessage(queue, raw string) (*ExportedJob, error) {
	var msg celeryMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, fmt.Errorf("parse celery message: %w", err)
	}

	if msg.Headers.Task == "" {
		return nil, fmt.Errorf("celery message missing task header")
	}

	ej := &ExportedJob{
		Type:  msg.Headers.Task,
		Queue: queue,
		Meta: map[string]any{
			"celery_task_id": msg.Headers.ID,
		},
	}

	// Decode body: base64-encoded JSON [args, kwargs, embed]
	if msg.Body != "" {
		args, kwargs, err := decodeCeleryBody(msg.Body)
		if err != nil {
			return nil, fmt.Errorf("decode celery body: %w", err)
		}
		ej.Args = args
		if kwargs != nil {
			ej.Meta["celery_kwargs"] = kwargs
		}
	} else {
		ej.Args = json.RawMessage("[]")
	}

	return ej, nil
}

// decodeCeleryBody decodes the base64 body into args and kwargs.
func decodeCeleryBody(body string) (json.RawMessage, map[string]any, error) {
	decoded, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		// Try raw JSON (some Celery configs don't base64-encode)
		return decodeCeleryBodyRaw(body)
	}
	return decodeCeleryBodyRaw(string(decoded))
}

func decodeCeleryBodyRaw(raw string) (json.RawMessage, map[string]any, error) {
	// Celery body format: [args, kwargs, embed]
	var parts []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &parts); err != nil {
		return json.RawMessage("[]"), nil, err
	}

	var args json.RawMessage
	if len(parts) > 0 {
		args = parts[0]
	} else {
		args = json.RawMessage("[]")
	}

	var kwargs map[string]any
	if len(parts) > 1 {
		if err := json.Unmarshal(parts[1], &kwargs); err != nil {
			return json.RawMessage("[]"), nil, fmt.Errorf("decode kwargs: %w", err)
		}
	}

	return args, kwargs, nil
}
