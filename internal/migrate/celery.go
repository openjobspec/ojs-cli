package migrate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
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
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	return &CelerySource{
		rdb:    redis.NewClient(opts),
		url:    redisURL,
		queues: []string{"celery"},
	}, nil
}

type celeryMessage struct {
	Body    string         `json:"body"`
	Headers celeryHeaders  `json:"headers"`
}

type celeryHeaders struct {
	Task string `json:"task"`
	ID   string `json:"id"`
}

func (c *CelerySource) Analyze() (*AnalysisResult, error) {
	ctx := context.Background()

	result := &AnalysisResult{
		Source:     "celery",
		Connection: c.url,
	}

	// Try to discover queues by scanning known Celery key patterns
	discoveredQueues := c.discoverQueues(ctx)
	if len(discoveredQueues) == 0 {
		discoveredQueues = c.queues
	}

	for _, q := range discoveredQueues {
		qa, count, err := c.analyzeQueue(ctx, q)
		if err != nil {
			continue
		}
		result.Queues = append(result.Queues, *qa)
		result.TotalJobs += count
	}

	result.Summary = fmt.Sprintf("Found %d queues, %d total jobs", len(result.Queues), result.TotalJobs)
	return result, nil
}

func (c *CelerySource) discoverQueues(ctx context.Context) []string {
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
			break
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
	return queues
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
	ctx := context.Background()

	discoveredQueues := c.discoverQueues(ctx)
	if len(discoveredQueues) == 0 {
		discoveredQueues = c.queues
	}

	var exported []ExportedJob

	for _, q := range discoveredQueues {
		messages, err := c.rdb.LRange(ctx, q, 0, -1).Result()
		if err != nil {
			continue
		}
		for _, raw := range messages {
			if ej, err := parseCeleryMessage(q, raw); err == nil {
				exported = append(exported, *ej)
			}
		}
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
		if err == nil {
			ej.Args = args
			if kwargs != nil {
				ej.Meta["celery_kwargs"] = kwargs
			}
		} else {
			ej.Args = json.RawMessage("[]")
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
		json.Unmarshal(parts[1], &kwargs)
	}

	return args, kwargs, nil
}
