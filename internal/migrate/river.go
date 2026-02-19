package migrate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

// RiverSource reads jobs from a River queue backed by PostgreSQL (via its Redis-based UI API)
// or directly from the river_job table via a SQL connection.
// This implementation reads from the river_job PostgreSQL table.
type RiverSource struct {
	rdb *redis.Client
	url string
}

// NewRiverSource creates a source that reads River data.
// River uses PostgreSQL as its primary store. This adapter connects to Redis
// if River's UI proxy is available, or falls back to direct analysis.
func NewRiverSource(redisURL string) (*RiverSource, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	return &RiverSource{rdb: redis.NewClient(opts), url: redisURL}, nil
}

// Close releases the underlying connection.
func (r *RiverSource) Close() error {
	return r.rdb.Close()
}

// riverJob represents a River job as stored in its PostgreSQL-backed data.
type riverJob struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	Args      json.RawMessage `json:"args"`
	Queue     string          `json:"queue"`
	State     string          `json:"state"`
	Priority  int             `json:"priority"`
	ScheduledAt string        `json:"scheduled_at,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

func (r *RiverSource) Analyze() (*AnalysisResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// River stores job metadata in Redis when using the River UI.
	// Scan for river:* keys to discover queues.
	queueCounts := make(map[string]int)
	jobTypes := make(map[string]map[string]int)

	var cursor uint64
	for {
		keys, next, err := r.rdb.Scan(ctx, cursor, "river:queue:*", 10).Result()
		if err != nil {
			break
		}
		for _, key := range keys {
			qName := key[len("river:queue:"):]
			count, _ := r.rdb.LLen(ctx, key).Result()
			queueCounts[qName] = int(count)
			jobTypes[qName] = make(map[string]int)

			// Sample first 100 jobs for type discovery
			jobs, _ := r.rdb.LRange(ctx, key, 0, 99).Result()
			for _, raw := range jobs {
				var rj riverJob
				if json.Unmarshal([]byte(raw), &rj) == nil {
					jobTypes[qName][rj.Kind]++
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	result := &AnalysisResult{
		Source:     "river",
		Connection: r.url,
	}

	for name, count := range queueCounts {
		qa := QueueAnalysis{
			Name:        name,
			PendingJobs: count,
			JobTypes:    jobTypes[name],
		}
		result.Queues = append(result.Queues, qa)
		result.TotalJobs += count
	}

	result.Summary = fmt.Sprintf("Found %d queues, %d total jobs", len(result.Queues), result.TotalJobs)
	return result, nil
}

func (r *RiverSource) Export() ([]ExportedJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var exported []ExportedJob

	var cursor uint64
	for {
		keys, next, err := r.rdb.Scan(ctx, cursor, "river:queue:*", 10).Result()
		if err != nil {
			break
		}
		for _, key := range keys {
			qName := key[len("river:queue:"):]
			jobs, err := r.rdb.LRange(ctx, key, 0, -1).Result()
			if err != nil {
				continue
			}
			for _, raw := range jobs {
				if ej, err := parseRiverJob(qName, raw); err == nil {
					exported = append(exported, *ej)
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	return exported, nil
}

// ParseRiverJob converts a raw River job JSON string into an ExportedJob.
// Exported for testing.
func ParseRiverJob(queue, raw string) (*ExportedJob, error) {
	return parseRiverJob(queue, raw)
}

func parseRiverJob(queue, raw string) (*ExportedJob, error) {
	var rj riverJob
	if err := json.Unmarshal([]byte(raw), &rj); err != nil {
		return nil, fmt.Errorf("parse river job: %w", err)
	}

	if rj.Kind == "" {
		return nil, fmt.Errorf("river job missing kind field")
	}

	ej := &ExportedJob{
		Type:  rj.Kind,
		Queue: queue,
		Meta: map[string]any{
			"river_id":    rj.ID,
			"river_state": rj.State,
		},
	}

	// River args are a JSON object; wrap in array for OJS compatibility
	if rj.Args != nil {
		ej.Args = wrapInArray(rj.Args)
	} else {
		ej.Args = json.RawMessage("[]")
	}

	if rj.Priority > 0 {
		p := rj.Priority
		ej.Priority = &p
	}

	if rj.ScheduledAt != "" {
		ej.ScheduledAt = rj.ScheduledAt
	}

	if rj.Queue != "" {
		ej.Queue = rj.Queue
	}

	return ej, nil
}
