package migrate

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

type redisHook struct {
	process func(redis.Cmder) error
}

func (h redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h redisHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		return h.process(cmd)
	}
}

func (h redisHook) ProcessPipelineHook(_ redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(_ context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			if err := h.process(cmd); err != nil {
				return err
			}
		}
		return nil
	}
}

func hookedRedis(t *testing.T, process func(redis.Cmder) error) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "unused.invalid:6379"})
	client.AddHook(redisHook{process: process})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func redisCommandKey(cmd redis.Cmder) string {
	if len(cmd.Args()) < 2 {
		return ""
	}
	return fmt.Sprint(cmd.Args()[1])
}

func TestExportsPropagateRedisErrors(t *testing.T) {
	redisErr := errors.New("redis I/O failed")
	tests := []struct {
		name   string
		export func(*redis.Client) error
	}{
		{
			name: "sidekiq set",
			export: func(client *redis.Client) error {
				source := &SidekiqSource{rdb: client}
				_, err := source.Export()
				return err
			},
		},
		{
			name: "bullmq scan",
			export: func(client *redis.Client) error {
				source := &BullMQSource{rdb: client}
				_, err := source.Export()
				return err
			},
		},
		{
			name: "celery scan",
			export: func(client *redis.Client) error {
				source := &CelerySource{rdb: client, queues: []string{"celery"}}
				_, err := source.Export()
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := hookedRedis(t, func(redis.Cmder) error { return redisErr })
			if err := tt.export(client); !errors.Is(err, redisErr) {
				t.Fatalf("Export() error = %v, want wrapped Redis error", err)
			}
		})
	}
}

func TestAnalyzePropagatesRedisErrors(t *testing.T) {
	redisErr := errors.New("redis analysis failed")
	client := hookedRedis(t, func(redis.Cmder) error { return redisErr })
	sources := []struct {
		name    string
		analyze func() error
	}{
		{name: "sidekiq", analyze: func() error {
			_, err := (&SidekiqSource{rdb: client}).Analyze()
			return err
		}},
		{name: "bullmq", analyze: func() error {
			_, err := (&BullMQSource{rdb: client}).Analyze()
			return err
		}},
		{name: "celery", analyze: func() error {
			_, err := (&CelerySource{rdb: client, queues: []string{"celery"}}).Analyze()
			return err
		}},
	}
	for _, source := range sources {
		t.Run(source.name, func(t *testing.T) {
			if err := source.analyze(); !errors.Is(err, redisErr) {
				t.Fatalf("Analyze() error = %v, want Redis error", err)
			}
		})
	}
}

func TestStructureReadsPropagateErrors(t *testing.T) {
	redisErr := errors.New("structure read failed")

	t.Run("sidekiq zset", func(t *testing.T) {
		client := hookedRedis(t, func(cmd redis.Cmder) error {
			switch cmd.Name() {
			case "smembers":
				cmd.(*redis.StringSliceCmd).SetVal(nil)
				return nil
			case "zrange":
				return redisErr
			default:
				t.Fatalf("unexpected Redis command %s", cmd.String())
				return nil
			}
		})
		_, err := (&SidekiqSource{rdb: client}).Export()
		if !errors.Is(err, redisErr) {
			t.Fatalf("Export() error = %v, want zset error", err)
		}
	})

	t.Run("bullmq hash", func(t *testing.T) {
		client := hookedRedis(t, func(cmd redis.Cmder) error {
			switch cmd.Name() {
			case "scan":
				cmd.(*redis.ScanCmd).SetVal([]string{"bull:q:id"}, 0)
			case "zrange":
				cmd.(*redis.ZSliceCmd).SetVal(nil)
			case "lrange":
				cmd.(*redis.StringSliceCmd).SetVal([]string{"1"})
			case "hgetall":
				return redisErr
			default:
				t.Fatalf("unexpected Redis command %s", cmd.String())
			}
			return nil
		})
		_, err := (&BullMQSource{rdb: client}).Export()
		if !errors.Is(err, redisErr) {
			t.Fatalf("Export() error = %v, want hash error", err)
		}
	})

	t.Run("celery list", func(t *testing.T) {
		client := hookedRedis(t, func(cmd redis.Cmder) error {
			switch cmd.Name() {
			case "scan":
				cmd.(*redis.ScanCmd).SetVal(nil, 0)
				return nil
			case "lrange":
				return redisErr
			default:
				t.Fatalf("unexpected Redis command %s", cmd.String())
				return nil
			}
		})
		_, err := (&CelerySource{rdb: client, queues: []string{"celery"}}).Export()
		if !errors.Is(err, redisErr) {
			t.Fatalf("Export() error = %v, want list error", err)
		}
	})
}

func TestSidekiqExportReportsMalformedRecords(t *testing.T) {
	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "smembers":
			cmd.(*redis.StringSliceCmd).SetVal([]string{"default"})
		case "lrange":
			cmd.(*redis.StringSliceCmd).SetVal([]string{
				`{"class":"GoodJob","args":[],"jid":"good"}`,
				`{"class":`,
			})
		case "zrange":
			cmd.(*redis.ZSliceCmd).SetVal(nil)
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})

	jobs, err := (&SidekiqSource{rdb: client}).Export()
	var partial *PartialExportError
	if !errors.As(err, &partial) {
		t.Fatalf("Export() error = %v, want PartialExportError", err)
	}
	if len(jobs) != 1 || partial.Exported != 1 || len(partial.Failures) != 1 {
		t.Fatalf("jobs=%d partial=%+v, want one exported and one failure", len(jobs), partial)
	}
}

func TestCeleryExportReportsMalformedRecords(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte(`[["ok"],{},{}]`))
	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "scan":
			cmd.(*redis.ScanCmd).SetVal(nil, 0)
		case "lrange":
			cmd.(*redis.StringSliceCmd).SetVal([]string{
				`{"headers":{"task":"tasks.good","id":"1"},"body":"` + body + `"}`,
				`{"headers":`,
			})
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})

	jobs, err := (&CelerySource{rdb: client, queues: []string{"celery"}}).Export()
	var partial *PartialExportError
	if !errors.As(err, &partial) {
		t.Fatalf("Export() error = %v, want PartialExportError", err)
	}
	if len(jobs) != 1 || len(partial.Failures) != 1 {
		t.Fatalf("jobs=%d failures=%d, want one each", len(jobs), len(partial.Failures))
	}
}

func TestBullMQExportWaitDelayedDeduplicated(t *testing.T) {
	const (
		futureTarget  int64 = 1893456000001
		overdueTarget int64 = 1577836800900
	)
	fields := map[string]map[string]string{
		"future": {
			"name":      "future.job",
			"data":      `{"kind":"future"}`,
			"opts":      `{"delay":1000,"priority":3,"attempts":4}`,
			"timestamp": fmt.Sprint(futureTarget - 1000),
			"delay":     "1000",
		},
		"overdue": {
			"name":      "overdue.job",
			"data":      `{"kind":"overdue"}`,
			"opts":      `{"delay":900}`,
			"timestamp": fmt.Sprint(overdueTarget - 900),
			"delay":     "900",
		},
		"ready": {
			"name":      "ready.job",
			"data":      `{"kind":"ready"}`,
			"opts":      `{"delay":60000,"priority":7}`,
			"timestamp": "1893456000000",
			"delay":     "60000",
		},
	}

	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "scan":
			cmd.(*redis.ScanCmd).SetVal([]string{"bull:mail:id"}, 0)
		case "zrange":
			cmd.(*redis.ZSliceCmd).SetVal([]redis.Z{
				{Member: "future", Score: float64(futureTarget)*bullMQDelayedScoreFactor + 7},
				{Member: "overdue", Score: float64(overdueTarget)},
			})
		case "lrange":
			cmd.(*redis.StringSliceCmd).SetVal([]string{"ready", "future"})
		case "hgetall":
			id := strings.TrimPrefix(redisCommandKey(cmd), "bull:mail:")
			cmd.(*redis.MapStringStringCmd).SetVal(fields[id])
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})

	jobs, err := (&BullMQSource{rdb: client}).Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 {
		t.Fatalf("exported %d jobs, want 3 deduplicated jobs", len(jobs))
	}

	byType := make(map[string]ExportedJob, len(jobs))
	for _, job := range jobs {
		byType[job.Type] = job
	}
	if got := byType["future.job"].ScheduledAt; got != "2030-01-01T00:00:00.001Z" {
		t.Errorf("future scheduled_at = %q", got)
	}
	if got := byType["overdue.job"].ScheduledAt; got != "2020-01-01T00:00:00.9Z" {
		t.Errorf("overdue scheduled_at = %q", got)
	}
	if got := byType["ready.job"].ScheduledAt; got != "" {
		t.Errorf("ready scheduled_at = %q, want empty", got)
	}
	ready := byType["ready.job"]
	if ready.Queue != "mail" || ready.Priority == nil || *ready.Priority != 7 {
		t.Errorf("ready queue/priority = %q/%v", ready.Queue, ready.Priority)
	}
	if _, ok := ready.Meta["bullmq_opts"]; !ok {
		t.Error("BullMQ options were not retained in metadata")
	}
}

func TestBullMQExportReportsMalformedHash(t *testing.T) {
	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "scan":
			cmd.(*redis.ScanCmd).SetVal([]string{"bull:q:id"}, 0)
		case "zrange":
			cmd.(*redis.ZSliceCmd).SetVal(nil)
		case "lrange":
			cmd.(*redis.StringSliceCmd).SetVal([]string{"bad"})
		case "hgetall":
			cmd.(*redis.MapStringStringCmd).SetVal(map[string]string{"data": `{}`, "opts": `{}`})
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})

	jobs, err := (&BullMQSource{rdb: client}).Export()
	var partial *PartialExportError
	if !errors.As(err, &partial) {
		t.Fatalf("Export() error = %v, want PartialExportError", err)
	}
	if len(jobs) != 0 || len(partial.Failures) != 1 || partial.Failures[0].ID != "bad" {
		t.Fatalf("jobs=%v partial=%+v", jobs, partial)
	}
}

func TestBullMQTargetMillisStoredTimestampFallback(t *testing.T) {
	got, err := bullMQTargetMillis(0, map[string]string{
		"timestamp": "1893456000000",
		"delay":     "900",
	}, bullMQOpts{Delay: 900})
	if err != nil {
		t.Fatal(err)
	}
	if got != 1893456000900 {
		t.Fatalf("target millis = %d, want 1893456000900", got)
	}
}
