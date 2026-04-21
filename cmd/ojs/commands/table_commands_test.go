package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
)

func TestTableCommandRenderingPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(tableCommandFixture))
	defer server.Close()
	apiClient := client.New(&config.Config{ServerURL: server.URL})
	previous := output.Format
	output.Format = "table"
	t.Cleanup(func() { output.Format = previous })

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "health", run: func() error { return Health(apiClient, nil) }},
		{name: "jobs", run: func() error { return Jobs(apiClient, []string{"--limit", "2", "--state", "available"}) }},
		{name: "metrics", run: func() error { return Metrics(apiClient, nil) }},
		{name: "status", run: func() error { return Status(apiClient, []string{"j1"}) }},
		{name: "status detail", run: func() error { return Status(apiClient, []string{"--detail", "j1"}) }},
		{name: "result", run: func() error { return Result(apiClient, []string{"--wait", "j1"}) }},
		{name: "priority", run: func() error { return Priority(apiClient, []string{"--set", "5", "j1"}) }},
		{name: "retry", run: func() error { return Retry(apiClient, []string{"j1"}) }},
		{name: "retries", run: func() error { return Retries(apiClient, []string{"j1"}) }},
		{name: "queues list", run: func() error { return Queues(apiClient, nil) }},
		{name: "queues stats", run: func() error { return Queues(apiClient, []string{"--stats", "default"}) }},
		{name: "queues create", run: func() error { return Queues(apiClient, []string{"--create", "new", "--concurrency", "2"}) }},
		{name: "queues purge", run: func() error { return Queues(apiClient, []string{"--purge", "default"}) }},
		{name: "queues config", run: func() error { return Queues(apiClient, []string{"--config", "default", "--retention", "24h"}) }},
		{name: "workers list", run: func() error { return Workers(apiClient, nil) }},
		{name: "workers detail", run: func() error { return Workers(apiClient, []string{"--detail", "w1"}) }},
		{name: "workers quiet", run: func() error { return Workers(apiClient, []string{"--quiet"}) }},
		{name: "dead letter list", run: func() error { return DeadLetter(apiClient, []string{"--limit", "2"}) }},
		{name: "dead letter stats", run: func() error { return DeadLetter(apiClient, []string{"--stats"}) }},
		{name: "dead letter purge", run: func() error { return DeadLetter(apiClient, []string{"--purge", "--older-than", "7d"}) }},
		{name: "cron list", run: func() error { return Cron(apiClient, nil) }},
		{name: "cron register", run: func() error {
			return Cron(apiClient, []string{"--register", "--name", "daily", "--expression", "0 9 * * *", "--type", "report.run"})
		}},
		{name: "cron trigger", run: func() error { return Cron(apiClient, []string{"--trigger", "daily"}) }},
		{name: "cron history", run: func() error { return Cron(apiClient, []string{"--history", "daily"}) }},
		{name: "cron detail", run: func() error { return Cron(apiClient, []string{"--detail", "daily"}) }},
		{name: "cron update", run: func() error { return Cron(apiClient, []string{"--update", "daily", "--expression", "0 10 * * *"}) }},
		{name: "workflow create", run: func() error {
			return Workflow(apiClient, []string{"create", "--name", "flow", "--steps", `[{"id":"one","type":"job"}]`})
		}},
		{name: "workflow status", run: func() error { return Workflow(apiClient, []string{"status", "wf1"}) }},
		{name: "workflow cancel", run: func() error { return Workflow(apiClient, []string{"cancel", "wf1"}) }},
		{name: "workflow list", run: func() error { return Workflow(apiClient, []string{"list", "--limit", "2"}) }},
		{name: "rate list", run: func() error { return RateLimits(apiClient, nil) }},
		{name: "rate inspect", run: func() error { return RateLimits(apiClient, []string{"--inspect", "email"}) }},
		{name: "rate override", run: func() error { return RateLimits(apiClient, []string{"--override", "email", "--concurrency", "4"}) }},
		{name: "stats overview", run: func() error { return Stats(apiClient, nil) }},
		{name: "stats history", run: func() error { return Stats(apiClient, []string{"--history"}) }},
		{name: "system status", run: func() error { return System(apiClient, []string{"maintenance"}) }},
		{name: "system enable", run: func() error { return System(apiClient, []string{"maintenance", "--enable", "--reason", "upgrade"}) }},
		{name: "system config", run: func() error { return System(apiClient, []string{"config"}) }},
		{name: "webhook create", run: func() error {
			return Webhooks(apiClient, []string{"create", "--url", "https://example.test/hook", "--events", "job.completed"})
		}},
		{name: "webhook list", run: func() error { return Webhooks(apiClient, []string{"list"}) }},
		{name: "webhook get", run: func() error { return Webhooks(apiClient, []string{"get", "sub1"}) }},
		{name: "webhook test", run: func() error { return Webhooks(apiClient, []string{"test", "sub1"}) }},
		{name: "webhook rotate", run: func() error { return Webhooks(apiClient, []string{"rotate-secret", "sub1"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func tableCommandFixture(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/ojs/v1")
	fixture, ok := tableCommandFixtures[r.Method+" "+path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeFixtureJSON(w, fixture)
}

var tableCommandFixtures = map[string]any{
	"GET /health": map[string]any{
		"status": "ok", "version": "1", "uptime_seconds": 10,
		"backend": map[string]any{"type": "redis", "status": "connected"},
	},
	"GET /metrics": map[string]any{
		"uptime_seconds": 10, "jobs_enqueued_total": 5, "jobs_completed_total": 4,
		"jobs_failed_total": 1, "jobs_active": 1, "queues_active": 1,
		"workers_active": 1, "avg_latency_ms": 2.5, "throughput_per_second": 3.5,
	},
	"GET /jobs":                        map[string]any{"total": 1, "jobs": []map[string]any{jobFixture()}},
	"GET /jobs/j1":                     jobFixture(),
	"GET /admin/jobs/j1":               detailedJobFixture(),
	"GET /jobs/j1/result":              map[string]any{"state": "completed", "result": map[string]any{"ok": true}, "completed_at": "now"},
	"GET /jobs/j1/retries":             map[string]any{"job_id": "j1", "policy": map[string]any{"max_attempts": 3, "initial_interval": "1s", "backoff_strategy": "exponential"}, "retries": []map[string]any{{"attempt": 1, "state": "failed", "error": "bad", "started_at": "a", "failed_at": "b"}}},
	"POST /admin/jobs/j1/retry":        map[string]any{"state": "available"},
	"PATCH /jobs/j1":                   map[string]any{"id": "j1", "priority": 5},
	"GET /queues":                      map[string]any{"queues": []map[string]any{{"name": "default", "status": "active"}}},
	"POST /queues":                     map[string]any{"name": "new", "status": "active"},
	"GET /queues/default/stats":        map[string]any{"queue": "default", "status": "active", "stats": map[string]any{"available": 1, "active": 1, "completed": 2, "scheduled": 1, "retryable": 1, "dead": 0}},
	"POST /queues/default/purge":       map[string]any{"deleted": 2},
	"PUT /admin/queues/default/config": map[string]any{"name": "default"},
	"GET /admin/workers": map[string]any{
		"items":   []map[string]any{{"id": "w1", "state": "running", "directive": "run", "active_jobs": 1, "last_heartbeat": "now"}},
		"summary": map[string]any{"total": 1, "running": 1, "quiet": 0, "stale": 0},
	},
	"GET /admin/workers/w1": map[string]any{
		"id": "w1", "state": "running", "directive": "run", "active_jobs": 1,
		"queues": []string{"default", "priority"}, "hostname": "host", "pid": 1,
	},
	"POST /admin/workers/directive":                   map[string]any{"directive": "quiet"},
	"GET /dead-letter":                                map[string]any{"total": 1, "jobs": []map[string]any{{"id": "d1", "type": "bad", "queue": "default", "attempt": 3, "discarded_at": "now"}}},
	"GET /dead-letter/stats":                          map[string]any{"total": 2, "by_queue": map[string]int{"default": 2}, "by_type": map[string]int{"bad": 2}},
	"POST /dead-letter/purge":                         map[string]any{"deleted": 2},
	"GET /cron":                                       map[string]any{"cron_jobs": []map[string]any{{"name": "daily", "expression": "0 9 * * *", "enabled": true}}},
	"POST /cron":                                      map[string]any{"name": "daily"},
	"POST /cron/daily/trigger":                        map[string]any{"job_id": "j1"},
	"GET /cron/daily/history":                         map[string]any{"executions": []map[string]any{{"job_id": "j1", "state": "completed", "scheduled_at": "a"}}},
	"GET /cron/daily":                                 map[string]any{"name": "daily", "expression": "0 9 * * *", "enabled": true, "job_template": map[string]any{"type": "report.run", "options": map[string]any{"queue": "default"}}},
	"PATCH /cron/daily":                               map[string]any{"name": "daily"},
	"POST /workflows":                                 map[string]any{"id": "wf1", "state": "running"},
	"GET /workflows/wf1":                              map[string]any{"id": "wf1", "name": "flow", "state": "running", "created_at": "now", "steps": []map[string]any{{"id": "one", "type": "job", "state": "completed", "job_id": "j1"}}},
	"DELETE /workflows/wf1":                           map[string]any{"cancelled_steps": 1},
	"GET /workflows":                                  map[string]any{"total": 1, "workflows": []map[string]any{{"id": "wf1", "name": "flow", "state": "running", "step_count": 1}}},
	"GET /rate-limits":                                map[string]any{"rate_limits": []map[string]any{{"key": "email", "concurrency": 4, "active": 1, "available": 3}}},
	"GET /rate-limits/email":                          map[string]any{"key": "email", "concurrency": 4, "active": 1, "available": 3, "override": 4},
	"PUT /rate-limits/email/override":                 map[string]any{"concurrency": 4},
	"GET /admin/stats":                                map[string]any{"queues": map[string]any{"total": 1, "active": 1}, "workers": map[string]any{"total": 1, "running": 1}, "jobs": map[string]any{"available": 1, "active": 1, "completed": 2}, "throughput": map[string]any{"enqueued_per_min": 2, "completed_per_min": 1, "avg_latency_ms": 3.2}},
	"GET /admin/stats/history":                        map[string]any{"period": "1h", "data_points": []map[string]any{{"timestamp": "now", "enqueued": 2, "completed": 1}}},
	"GET /admin/maintenance":                          map[string]any{"enabled": true, "reason": "upgrade", "started_at": "now"},
	"POST /admin/maintenance":                         map[string]any{"enabled": true},
	"GET /admin/config":                               map[string]any{"backend": "redis"},
	"POST /webhooks/subscriptions":                    map[string]any{"id": "sub1"},
	"GET /webhooks/subscriptions":                     map[string]any{"total": 1, "subscriptions": []map[string]any{{"id": "sub1", "url": "https://example.test", "events": []string{"job.completed"}, "active": true}}},
	"GET /webhooks/subscriptions/sub1":                map[string]any{"id": "sub1", "url": "https://example.test", "events": []string{"job.completed"}, "active": true},
	"POST /webhooks/subscriptions/sub1/test":          map[string]any{"status_code": 200, "success": true},
	"POST /webhooks/subscriptions/sub1/rotate-secret": map[string]any{"new_secret": "new"},
}

func jobFixture() map[string]any {
	return map[string]any{
		"id": "j1", "type": "email.send", "state": "completed", "queue": "default",
		"attempt": 1, "priority": 2, "created_at": "now", "completed_at": "later",
		"progress": 1.0, "progress_data": map[string]any{"step": 1}, "args": []any{"a"},
		"meta": map[string]any{"tenant": "one"},
	}
}

func detailedJobFixture() map[string]any {
	job := jobFixture()
	job["options"] = map[string]any{"queue": "default"}
	job["result"] = map[string]any{"ok": true}
	job["errors"] = []any{"retry"}
	return job
}

func writeFixtureJSON(w http.ResponseWriter, value any) {
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
