package migrate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

// BullMQSource reads jobs from a BullMQ-managed Redis instance.
type BullMQSource struct {
	rdb *redis.Client
	url string
}

// NewBullMQSource creates a source that reads BullMQ data from Redis.
func NewBullMQSource(redisURL string) (*BullMQSource, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	return &BullMQSource{rdb: redis.NewClient(opts), url: redisURL}, nil
}

type bullMQJobData struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
	Opts bullMQOpts      `json:"opts"`
}

type bullMQOpts struct {
	Delay    int64 `json:"delay,omitempty"`
	Priority int   `json:"priority,omitempty"`
}

func (b *BullMQSource) Analyze() (*AnalysisResult, error) {
	ctx := context.Background()

	queueNames, err := b.discoverQueues(ctx)
	if err != nil {
		return nil, err
	}

	result := &AnalysisResult{
		Source:     "bullmq",
		Connection: b.url,
	}

	for _, q := range queueNames {
		qa, count, err := b.analyzeQueue(ctx, q)
		if err != nil {
			return nil, err
		}
		result.Queues = append(result.Queues, *qa)
		result.TotalJobs += count
	}

	result.Summary = fmt.Sprintf("Found %d queues, %d total jobs", len(queueNames), result.TotalJobs)
	return result, nil
}

func (b *BullMQSource) discoverQueues(ctx context.Context) ([]string, error) {
	seen := make(map[string]bool)
	var cursor uint64

	for {
		keys, next, err := b.rdb.Scan(ctx, cursor, "bull:*:id", 10).Result()
		if err != nil {
			return nil, fmt.Errorf("scan for bull queues: %w", err)
		}
		for _, key := range keys {
			// bull:<queue>:id → extract queue name
			parts := strings.SplitN(key, ":", 3)
			if len(parts) >= 2 {
				seen[parts[1]] = true
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	// Also scan for wait lists as a fallback
	cursor = 0
	for {
		keys, next, err := b.rdb.Scan(ctx, cursor, "bull:*:wait", 10).Result()
		if err != nil {
			break
		}
		for _, key := range keys {
			parts := strings.SplitN(key, ":", 3)
			if len(parts) >= 2 {
				seen[parts[1]] = true
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
	return queues, nil
}

func (b *BullMQSource) analyzeQueue(ctx context.Context, name string) (*QueueAnalysis, int, error) {
	// BullMQ stores waiting jobs in bull:<queue>:wait list, each entry is a job ID
	waitKey := "bull:" + name + ":wait"
	jobIDs, err := b.rdb.LRange(ctx, waitKey, 0, -1).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("read BullMQ wait list for %s: %w", name, err)
	}

	qa := &QueueAnalysis{
		Name:        name,
		PendingJobs: len(jobIDs),
		JobTypes:    make(map[string]int),
	}

	for _, id := range jobIDs {
		dataKey := "bull:" + name + ":" + id
		data, err := b.rdb.HGet(ctx, dataKey, "data").Result()
		if err != nil {
			continue
		}
		var job bullMQJobData
		if json.Unmarshal([]byte(data), &job) == nil {
			qa.JobTypes[job.Name]++
		} else {
			// data field might just be the data payload; try getting name separately
			nameVal, err := b.rdb.HGet(ctx, dataKey, "name").Result()
			if err == nil {
				qa.JobTypes[nameVal]++
			}
		}
	}

	return qa, len(jobIDs), nil
}

func (b *BullMQSource) Export() ([]ExportedJob, error) {
	ctx := context.Background()

	queueNames, err := b.discoverQueues(ctx)
	if err != nil {
		return nil, err
	}

	var exported []ExportedJob

	for _, q := range queueNames {
		waitKey := "bull:" + q + ":wait"
		jobIDs, err := b.rdb.LRange(ctx, waitKey, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, id := range jobIDs {
			dataKey := "bull:" + q + ":" + id
			fields, err := b.rdb.HGetAll(ctx, dataKey).Result()
			if err != nil {
				continue
			}
			if ej, err := parseBullMQJob(q, fields); err == nil {
				exported = append(exported, *ej)
			}
		}
	}

	return exported, nil
}

// ParseBullMQJob converts BullMQ hash fields into an ExportedJob.
// Exported for testing.
func ParseBullMQJob(queue string, raw string) (*ExportedJob, error) {
	var job bullMQJobData
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return nil, fmt.Errorf("parse bullmq job: %w", err)
	}

	ej := &ExportedJob{
		Type:  job.Name,
		Queue: queue,
		Args:  wrapInArray(job.Data),
		Meta: map[string]any{
			"bullmq_source": true,
		},
	}

	if job.Opts.Priority > 0 {
		p := job.Opts.Priority
		ej.Priority = &p
	}

	if job.Opts.Delay > 0 {
		scheduled := time.Now().Add(time.Duration(job.Opts.Delay) * time.Millisecond)
		ej.ScheduledAt = scheduled.UTC().Format(time.RFC3339)
	}

	return ej, nil
}

func parseBullMQJob(queue string, fields map[string]string) (*ExportedJob, error) {
	name := fields["name"]
	if name == "" {
		return nil, fmt.Errorf("missing job name")
	}

	ej := &ExportedJob{
		Type:  name,
		Queue: queue,
		Meta: map[string]any{
			"bullmq_source": true,
		},
	}

	if data, ok := fields["data"]; ok {
		ej.Args = wrapInArray(json.RawMessage(data))
	} else {
		ej.Args = json.RawMessage("[]")
	}

	if opts, ok := fields["opts"]; ok {
		var o bullMQOpts
		if json.Unmarshal([]byte(opts), &o) == nil {
			if o.Priority > 0 {
				p := o.Priority
				ej.Priority = &p
			}
			if o.Delay > 0 {
				scheduled := time.Now().Add(time.Duration(o.Delay) * time.Millisecond)
				ej.ScheduledAt = scheduled.UTC().Format(time.RFC3339)
			}
		}
	}

	return ej, nil
}

// wrapInArray wraps a JSON value in an array if it isn't already one.
func wrapInArray(data json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return data
	}
	wrapped, _ := json.Marshal([]json.RawMessage{data})
	return wrapped
}
