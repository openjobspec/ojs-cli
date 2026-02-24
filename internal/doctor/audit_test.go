package doctor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func healthyServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-OJS-Version", "1.0.0")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Request-Id", "test-123")

		switch r.URL.Path {
		case "/ojs/v1/health":
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		case "/ojs/manifest":
			json.NewEncoder(w).Encode(map[string]string{"version": "1.0"})
		case "/ojs/v1/queues":
			json.NewEncoder(w).Encode(map[string]any{"queues": []map[string]string{{"name": "default"}}})
		case "/ojs/v1/dead-letter":
			json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}, "pagination": map[string]int{"total": 0}})
		case "/metrics":
			w.Write([]byte("# HELP ojs_jobs_total\nojs_jobs_total 42\n"))
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestAuditHealthyServer(t *testing.T) {
	srv := healthyServer()
	defer srv.Close()

	auditor := NewAuditor(srv.URL, "")
	report := auditor.Run(context.Background())

	if report.Score <= 0 {
		t.Error("expected positive score")
	}
	if report.Grade == "" {
		t.Error("expected grade")
	}
	if len(report.Checks) != 20 {
		t.Errorf("expected 20 checks, got %d", len(report.Checks))
	}

	// Count passes
	passes := 0
	for _, c := range report.Checks {
		if c.Severity == SevPass {
			passes++
		}
	}
	if passes < 8 {
		t.Errorf("expected at least 8 passes on healthy server, got %d", passes)
	}
}

func TestAuditUnhealthyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	auditor := NewAuditor(srv.URL, "")
	report := auditor.Run(context.Background())

	criticals := 0
	for _, c := range report.Checks {
		if c.Severity == SevCritical {
			criticals++
		}
	}
	if criticals == 0 {
		t.Error("expected critical findings on unhealthy server")
	}
	if report.Grade == "A" {
		t.Error("unhealthy server should not get A grade")
	}
}

func TestAuditNoAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No auth check — always returns 200
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	auditor := NewAuditor(srv.URL, "")
	report := auditor.Run(context.Background())

	for _, c := range report.Checks {
		if c.ID == "SEC-002" {
			if c.Severity != SevCritical {
				t.Errorf("expected critical for no auth, got %s", c.Severity)
			}
			if c.Fix == "" {
				t.Error("expected fix suggestion for no auth")
			}
			return
		}
	}
	t.Error("SEC-002 check not found")
}

func TestAuditHTTPS(t *testing.T) {
	auditor := NewAuditor("https://ojs.example.com", "")
	// Can't connect, but we can test TLS detection
	report := auditor.Run(context.Background())

	for _, c := range report.Checks {
		if c.ID == "SEC-003" {
			if c.Severity != SevPass {
				t.Errorf("HTTPS should pass TLS check, got %s", c.Severity)
			}
			return
		}
	}
}

func TestAuditCategories(t *testing.T) {
	srv := healthyServer()
	defer srv.Close()

	report := NewAuditor(srv.URL, "").Run(context.Background())

	expected := []string{"security", "compliance", "operations", "observability", "reliability"}
	for _, cat := range expected {
		if _, ok := report.Categories[cat]; !ok {
			t.Errorf("expected category %s in report", cat)
		}
	}
}

func TestGrading(t *testing.T) {
	srv := healthyServer()
	defer srv.Close()

	report := NewAuditor(srv.URL, "").Run(context.Background())
	if report.MaxScore <= 0 {
		t.Error("expected positive max score")
	}
	if report.Score > report.MaxScore {
		t.Error("score should not exceed max score")
	}
}
