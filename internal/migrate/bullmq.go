package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/redact"
	"github.com/redis/go-redis/v9"
)

const (
	bullMQDelayedScoreFactor = 0x1000
	maxUnixMilli             = 253402300799000 // 9999-12-31T23:59:59Z
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
		return nil, fmt.Errorf("invalid redis URL %q: %w", redact.URL(redisURL), err)
	}
	return &BullMQSource{rdb: redis.NewClient(opts), url: redisURL}, nil
}

// Close releases the underlying Redis connection.
func (b *BullMQSource) Close() error {
	return b.rdb.Close()
}

type bullMQOpts struct {
	Delay    int64 `json:"delay,omitempty"`
	Priority int   `json:"priority,omitempty"`
}

type bullMQRecord struct {
	ID        string
	Structure string
	Score     float64
}

func (b *BullMQSource) Analyze() (*AnalysisResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queueNames, err := b.discoverQueues(ctx)
	if err != nil {
		return nil, err
	}

	result := &AnalysisResult{
		Source:     "bullmq",
		Connection: redact.URL(b.url),
	}
	for _, queue := range queueNames {
		analysis, count, err := b.analyzeQueue(ctx, queue)
		if err != nil {
			return nil, err
		}
		result.Queues = append(result.Queues, *analysis)
		result.TotalJobs += count
	}

	result.Summary = fmt.Sprintf("Found %d queues, %d total jobs", len(queueNames), result.TotalJobs)
	return result, nil
}

func (b *BullMQSource) discoverQueues(ctx context.Context) ([]string, error) {
	seen := make(map[string]struct{})
	for _, suffix := range []string{"id", "wait", "delayed"} {
		var cursor uint64
		for {
			keys, next, err := b.rdb.Scan(ctx, cursor, "bull:*:"+suffix, 10).Result()
			if err != nil {
				return nil, fmt.Errorf("scan for BullMQ %s keys: %w", suffix, err)
			}
			for _, key := range keys {
				if queue, ok := bullMQQueueFromKey(key, suffix); ok {
					seen[queue] = struct{}{}
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}

	queues := make([]string, 0, len(seen))
	for queue := range seen {
		queues = append(queues, queue)
	}
	sort.Strings(queues)
	return queues, nil
}

func bullMQQueueFromKey(key, suffix string) (string, bool) {
	const prefix = "bull:"
	ending := ":" + suffix
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ending) {
		return "", false
	}
	queue := strings.TrimSuffix(strings.TrimPrefix(key, prefix), ending)
	return queue, queue != ""
}

func (b *BullMQSource) analyzeQueue(ctx context.Context, name string) (*QueueAnalysis, int, error) {
	records, err := b.queueRecords(ctx, name)
	if err != nil {
		return nil, 0, err
	}

	analysis := &QueueAnalysis{
		Name:        name,
		PendingJobs: len(records),
		JobTypes:    make(map[string]int),
	}
	for _, record := range records {
		fields, err := b.rdb.HGetAll(ctx, bullMQJobKey(name, record.ID)).Result()
		if err != nil {
			return nil, 0, fmt.Errorf("read BullMQ job %s/%s: %w", name, record.ID, err)
		}
		job, err := parseBullMQFields(name, record, fields)
		if err != nil {
			return nil, 0, fmt.Errorf("analyze BullMQ job %s/%s: %w", name, record.ID, err)
		}
		analysis.JobTypes[job.Type]++
	}
	return analysis, len(records), nil
}

func (b *BullMQSource) Export() ([]ExportedJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queueNames, err := b.discoverQueues(ctx)
	if err != nil {
		return nil, err
	}

	var exported []ExportedJob
	var failures []FailedRecord
	for _, queue := range queueNames {
		records, err := b.queueRecords(ctx, queue)
		if err != nil {
			return nil, err
		}
		for i, record := range records {
			fields, err := b.rdb.HGetAll(ctx, bullMQJobKey(queue, record.ID)).Result()
			if err != nil {
				return nil, fmt.Errorf("read BullMQ job %s/%s: %w", queue, record.ID, err)
			}
			job, parseErr := parseBullMQFields(queue, record, fields)
			if parseErr != nil {
				failures = append(failures, FailedRecord{
					Source: "bullmq", Queue: queue, Structure: record.Structure,
					ID: record.ID, Index: i + 1, Error: parseErr.Error(),
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

func (b *BullMQSource) queueRecords(ctx context.Context, queue string) ([]bullMQRecord, error) {
	delayed, err := b.rdb.ZRangeWithScores(ctx, "bull:"+queue+":delayed", 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("read BullMQ delayed set for %s: %w", queue, err)
	}
	waiting, err := b.rdb.LRange(ctx, "bull:"+queue+":wait", 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("read BullMQ wait list for %s: %w", queue, err)
	}

	records := make([]bullMQRecord, 0, len(delayed)+len(waiting))
	seen := make(map[string]struct{}, len(delayed)+len(waiting))
	for _, entry := range delayed {
		id, err := redisMemberString(entry.Member)
		if err != nil {
			return nil, fmt.Errorf("read BullMQ delayed member for %s: %w", queue, err)
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		records = append(records, bullMQRecord{ID: id, Structure: "delayed", Score: entry.Score})
	}
	for _, id := range waiting {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		records = append(records, bullMQRecord{ID: id, Structure: "wait"})
	}
	return records, nil
}

func bullMQJobKey(queue, id string) string {
	return "bull:" + queue + ":" + id
}

// ParseBullMQJob converts a serialized BullMQ job snapshot into an ExportedJob.
// A standalone snapshot is treated as a ready/waiting job; opts.delay is
// retained in metadata but never re-applied at migration time.
func ParseBullMQJob(queue, raw string) (*ExportedJob, error) {
	var snapshot struct {
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Data      json.RawMessage `json:"data"`
		Opts      json.RawMessage `json:"opts"`
		Timestamp json.Number     `json:"timestamp"`
		Delay     json.Number     `json:"delay"`
		Priority  json.Number     `json:"priority"`
	}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, fmt.Errorf("parse bullmq job: %w", err)
	}

	fields := map[string]string{
		"name": snapshot.Name,
		"data": string(snapshot.Data),
		"opts": string(snapshot.Opts),
	}
	if snapshot.Timestamp != "" {
		fields["timestamp"] = snapshot.Timestamp.String()
	}
	if snapshot.Delay != "" {
		fields["delay"] = snapshot.Delay.String()
	}
	if snapshot.Priority != "" {
		fields["priority"] = snapshot.Priority.String()
	}
	return parseBullMQFields(queue, bullMQRecord{ID: snapshot.ID, Structure: "wait"}, fields)
}

func parseBullMQFields(queue string, record bullMQRecord, fields map[string]string) (*ExportedJob, error) {
	name := fields["name"]
	if name == "" {
		return nil, fmt.Errorf("missing job name")
	}

	args, err := wrapInArray(json.RawMessage(fields["data"]))
	if err != nil {
		return nil, fmt.Errorf("invalid job data: %w", err)
	}

	opts := bullMQOpts{}
	optsMeta := map[string]any{}
	if rawOpts := fields["opts"]; rawOpts != "" {
		if err := json.Unmarshal([]byte(rawOpts), &opts); err != nil {
			return nil, fmt.Errorf("invalid job options: %w", err)
		}
		if err := json.Unmarshal([]byte(rawOpts), &optsMeta); err != nil {
			return nil, fmt.Errorf("invalid job options metadata: %w", err)
		}
	}

	job := &ExportedJob{
		Type:  name,
		Queue: queue,
		Args:  args,
		Meta: map[string]any{
			"bullmq_source":    true,
			"bullmq_structure": record.Structure,
		},
	}
	if record.ID != "" {
		job.Meta["bullmq_id"] = record.ID
	}
	if len(optsMeta) > 0 {
		job.Meta["bullmq_opts"] = optsMeta
	}

	priority := opts.Priority
	if fields["priority"] != "" {
		parsed, parseErr := strconv.Atoi(fields["priority"])
		if parseErr != nil {
			return nil, fmt.Errorf("invalid priority %q: %w", fields["priority"], parseErr)
		}
		priority = parsed
	}
	if priority > 0 {
		job.Priority = &priority
	}

	if record.Structure == "delayed" {
		targetMillis, err := bullMQTargetMillis(record.Score, fields, opts)
		if err != nil {
			return nil, err
		}
		job.ScheduledAt = time.UnixMilli(targetMillis).UTC().Format(time.RFC3339Nano)
	}

	return job, nil
}

func bullMQTargetMillis(score float64, fields map[string]string, opts bullMQOpts) (int64, error) {
	var scoreMillis int64
	if score > 0 {
		if score > maxUnixMilli {
			scoreMillis = int64(math.Floor(score / bullMQDelayedScoreFactor))
		} else {
			scoreMillis = int64(math.Floor(score))
		}
	}

	var storedMillis int64
	if rawTimestamp := fields["timestamp"]; rawTimestamp != "" {
		timestamp, err := strconv.ParseInt(rawTimestamp, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid BullMQ timestamp %q: %w", rawTimestamp, err)
		}
		delay := opts.Delay
		if rawDelay := fields["delay"]; rawDelay != "" {
			delay, err = strconv.ParseInt(rawDelay, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid BullMQ delay %q: %w", rawDelay, err)
			}
		}
		storedMillis = timestamp + delay
	}

	switch {
	case scoreMillis > 0:
		return scoreMillis, nil
	case storedMillis > 0:
		return storedMillis, nil
	default:
		return 0, fmt.Errorf("delayed job is missing a valid delayed score or timestamp/delay")
	}
}

func wrapInArray(data json.RawMessage) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return json.RawMessage("[]"), nil
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON")
	}
	if trimmed[0] == '[' {
		return data, nil
	}
	wrapped, err := json.Marshal([]json.RawMessage{data})
	if err != nil {
		return nil, fmt.Errorf("wrap job data: %w", err)
	}
	return wrapped, nil
}
