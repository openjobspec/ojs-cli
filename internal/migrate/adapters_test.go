package migrate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestRedisSourceConstructorsAndClose(t *testing.T) {
	constructors := []struct {
		name string
		new  func(string) (Source, error)
	}{
		{name: "sidekiq", new: func(raw string) (Source, error) { return NewSidekiqSource(raw) }},
		{name: "bullmq", new: func(raw string) (Source, error) { return NewBullMQSource(raw) }},
		{name: "celery", new: func(raw string) (Source, error) { return NewCelerySource(raw) }},
		{name: "river", new: func(raw string) (Source, error) { return NewRiverSource(raw) }},
	}
	for _, tt := range constructors {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.new("not-a-redis-url"); err == nil {
				t.Fatal("constructor accepted invalid Redis URL")
			}
			source, err := tt.new("redis://localhost:6379/1")
			if err != nil {
				t.Fatal(err)
			}
			if err := source.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
	}
}

func TestSidekiqAnalyzeIncludesSortedSetsAndRedactsConnection(t *testing.T) {
	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "smembers":
			cmd.(*redis.StringSliceCmd).SetVal([]string{"critical"})
		case "lrange":
			cmd.(*redis.StringSliceCmd).SetVal([]string{`{"class":"EmailWorker","args":[]}`})
		case "zcard":
			if redisCommandKey(cmd) == "schedule" {
				cmd.(*redis.IntCmd).SetVal(2)
			} else {
				cmd.(*redis.IntCmd).SetVal(1)
			}
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})
	source := &SidekiqSource{rdb: client, url: "redis://user:password@redis.example:6379"}
	result, err := source.Analyze()
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalJobs != 4 || result.Queues[0].JobTypes["EmailWorker"] != 1 {
		t.Fatalf("analysis = %+v", result)
	}
	if strings.Contains(result.Connection, "password") || strings.Contains(result.Connection, "user@") {
		t.Fatalf("connection was not redacted: %s", result.Connection)
	}
}

func TestBullMQAnalyzeReadsReadyJobs(t *testing.T) {
	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "scan":
			cmd.(*redis.ScanCmd).SetVal([]string{"bull:mail:id"}, 0)
		case "zrange":
			cmd.(*redis.ZSliceCmd).SetVal(nil)
		case "lrange":
			cmd.(*redis.StringSliceCmd).SetVal([]string{"1"})
		case "hgetall":
			cmd.(*redis.MapStringStringCmd).SetVal(map[string]string{
				"name": "email.send",
				"data": `{"to":"user@example.com"}`,
				"opts": `{"priority":2}`,
			})
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})
	result, err := (&BullMQSource{rdb: client, url: "redis://localhost"}).Analyze()
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalJobs != 1 || result.Queues[0].JobTypes["email.send"] != 1 {
		t.Fatalf("analysis = %+v", result)
	}
}

func TestCeleryAnalyzeDiscoversMessageTypes(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte(`[["arg"],{},{}]`))
	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "scan":
			cmd.(*redis.ScanCmd).SetVal([]string{"_kombu.binding.priority"}, 0)
		case "lrange":
			task := strings.TrimSpace(redisCommandKey(cmd))
			cmd.(*redis.StringSliceCmd).SetVal([]string{
				fmt.Sprintf(`{"body":%q,"headers":{"task":"tasks.%s","id":"1"}}`, body, task),
			})
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})
	source := &CelerySource{rdb: client, url: "redis://localhost", queues: []string{"celery"}}
	result, err := source.Analyze()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queues) != 2 || result.TotalJobs != 2 {
		t.Fatalf("analysis = %+v", result)
	}
}

func TestRiverAnalyzeAndExport(t *testing.T) {
	raw := `{"id":7,"kind":"report.generate","args":{"id":1},"queue":"reports","state":"available","priority":3}`
	client := hookedRedis(t, func(cmd redis.Cmder) error {
		switch cmd.Name() {
		case "scan":
			cmd.(*redis.ScanCmd).SetVal([]string{"river:queue:default"}, 0)
		case "llen":
			cmd.(*redis.IntCmd).SetVal(1)
		case "lrange":
			cmd.(*redis.StringSliceCmd).SetVal([]string{raw})
		default:
			t.Fatalf("unexpected Redis command %s", cmd.String())
		}
		return nil
	})
	source := &RiverSource{rdb: client, url: "redis://user:password@localhost"}
	analysis, err := source.Analyze()
	if err != nil {
		t.Fatal(err)
	}
	if analysis.TotalJobs != 1 || analysis.Queues[0].JobTypes["report.generate"] != 1 {
		t.Fatalf("analysis = %+v", analysis)
	}
	jobs, err := source.Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Queue != "reports" || jobs[0].Priority == nil {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestFaktoryAnalyzeAndExport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"faktory": map[string]any{"queues": map[string]int{"default": 1}},
			})
		case "/api/queues/default":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"jid": "jid-1", "jobtype": "email.send", "args": []any{"a"}, "queue": "default",
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source, err := NewFaktorySource(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	analysis, err := source.Analyze()
	if err != nil {
		t.Fatal(err)
	}
	if analysis.TotalJobs != 1 {
		t.Fatalf("analysis = %+v", analysis)
	}
	jobs, err := source.Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Type != "email.send" {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestAdapterParsersRejectMalformedRecords(t *testing.T) {
	tests := []struct {
		name  string
		parse func() error
	}{
		{name: "sidekiq missing class", parse: func() error {
			_, err := ParseSidekiqJob(`{"args":[]}`)
			return err
		}},
		{name: "sidekiq invalid schedule", parse: func() error {
			_, err := ParseSidekiqJob(`{"class":"Job","args":[],"at":-1}`)
			return err
		}},
		{name: "sidekiq invalid args", parse: func() error {
			_, err := ParseSidekiqJob(`{"class":"Job","args":{}}`)
			return err
		}},
		{name: "bullmq missing name", parse: func() error {
			_, err := ParseBullMQJob("q", `{"data":{},"opts":{}}`)
			return err
		}},
		{name: "bullmq invalid data", parse: func() error {
			_, err := ParseBullMQJob("q", `{"name":"job","data":"unterminated}`)
			return err
		}},
		{name: "celery missing task", parse: func() error {
			_, err := ParseCeleryMessage("q", `{"headers":{},"body":""}`)
			return err
		}},
		{name: "celery invalid body", parse: func() error {
			_, err := ParseCeleryMessage("q", `{"headers":{"task":"job"},"body":"not-json"}`)
			return err
		}},
		{name: "river missing kind", parse: func() error {
			_, err := ParseRiverJob("q", `{"args":{}}`)
			return err
		}},
		{name: "faktory invalid JSON", parse: func() error {
			_, err := ParseFaktoryJob(`{`)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.parse(); err == nil {
				t.Fatal("parser accepted malformed record")
			}
		})
	}
}

func TestValidateFileAndReader(t *testing.T) {
	content := strings.Join([]string{
		`{"type":"valid.job","queue":"default","args":[]}`,
		`{"type":"","queue":"","args":{},"priority":-1}`,
		`not-json`,
	}, "\n")
	path := filepath.Join(t.TempDir(), "jobs.ndjson")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 || result.Valid != 1 || result.Invalid != 2 || len(result.Errors) < 3 {
		t.Fatalf("validation result = %+v", result)
	}

	large := `{"type":"large.job","queue":"default","args":["` + strings.Repeat("x", 128*1024) + `"]}`
	result, err = validateFromReader(strings.NewReader(large))
	if err != nil || result.Valid != 1 {
		t.Fatalf("large record result=%+v err=%v", result, err)
	}
}

func TestImportFileAndTypedErrorMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.ndjson")
	if err := os.WriteFile(path, []byte(`{"type":"job","queue":"default","args":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ImportFile(&sequencePoster{}, path, nil)
	if err != nil || result.Success != 1 {
		t.Fatalf("ImportFile result=%+v err=%v", result, err)
	}

	exportErr := (&PartialExportError{Exported: 2, Failures: []FailedRecord{{Error: "bad"}}}).Error()
	if !strings.Contains(exportErr, "failed 1") {
		t.Fatalf("PartialExportError = %q", exportErr)
	}
	partialErr := (&PartialFailureError{Operation: "test", Total: 3, Failed: 1}).Error()
	if !strings.Contains(partialErr, "1 of 3") {
		t.Fatalf("PartialFailureError = %q", partialErr)
	}
}

func TestBullMQParsingBranches(t *testing.T) {
	job, err := ParseBullMQJob("q", `{"id":"1","name":"job","data":[1,2],"opts":{"priority":4},"priority":5}`)
	if err != nil {
		t.Fatal(err)
	}
	if job.Priority == nil || *job.Priority != 5 || string(job.Args) != "[1,2]" {
		t.Fatalf("job = %+v", job)
	}
	if _, err := parseBullMQFields("q", bullMQRecord{Structure: "delayed"}, map[string]string{
		"name": "job", "data": `{}`, "opts": `{}`,
	}); err == nil {
		t.Fatal("delayed parser accepted missing schedule")
	}
	if _, err := wrapInArray(json.RawMessage(`{`)); err == nil {
		t.Fatal("wrapInArray accepted invalid JSON")
	}
	if _, err := bullMQTargetMillis(0, map[string]string{"timestamp": "bad"}, bullMQOpts{}); err == nil {
		t.Fatal("target parser accepted invalid timestamp")
	}
}

func TestRedisMemberStringVariants(t *testing.T) {
	if got, err := redisMemberString([]byte("id")); err != nil || got != "id" {
		t.Fatalf("redisMemberString bytes = %q, %v", got, err)
	}
	if _, err := redisMemberString(42); err == nil {
		t.Fatal("redisMemberString accepted integer")
	}
}

func TestFaktoryRequestErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	source, err := NewFaktorySource(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Analyze(); err == nil {
		t.Fatal("Analyze() accepted Faktory error response")
	}
}

func TestValidationScannerLimit(t *testing.T) {
	_, err := validateFromReader(strings.NewReader(strings.Repeat("x", maxValidationRecordBytes+1)))
	if err == nil {
		t.Fatal("validateFromReader() error = nil for over-limit record")
	}
}

func TestImportFileOpenError(t *testing.T) {
	if _, err := ImportFile(&sequencePoster{}, filepath.Join(t.TempDir(), "missing"), nil); err == nil {
		t.Fatal("ImportFile() error = nil for missing file")
	}
}

func TestSourceErrorTypesImplementError(t *testing.T) {
	var exportError error = &PartialExportError{}
	var partialError error = &PartialFailureError{}
	if exportError.Error() == "" || partialError.Error() == "" {
		t.Fatal("typed errors returned empty messages")
	}
	if errors.Is(exportError, partialError) {
		t.Fatal("unrelated partial errors unexpectedly match")
	}
}
