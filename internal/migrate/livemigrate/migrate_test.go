package livemigrate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	rootmigrate "github.com/openjobspec/ojs-cli/internal/migrate"
)

func TestMigrationDryRun(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"jobs": []map[string]any{
				{"id": "j1", "type": "email.send", "queue": "default", "args": []string{"hello"}},
				{"id": "j2", "type": "image.resize", "queue": "media", "args": []int{100, 200}},
			},
		})
	}))
	defer source.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ojs/v1/health" {
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "new-id"})
	}))
	defer target.Close()

	m := New(&Config{
		SourceURL: source.URL,
		TargetURL: target.URL,
		DryRun:    true,
	})

	err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	stats := m.GetStats()
	if stats.Phase != PhaseComplete {
		t.Errorf("expected complete, got %s", stats.Phase)
	}
	if stats.Exported != 2 {
		t.Errorf("expected 2 exported, got %d", stats.Exported)
	}
	if stats.Imported != 2 {
		t.Errorf("expected 2 imported (dry-run), got %d", stats.Imported)
	}
}

func TestMigrationActual(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"jobs": []map[string]any{
				{"id": "j1", "type": "email.send", "queue": "default", "args": []string{"test"}},
			},
		})
	}))
	defer source.Close()

	var importCalled bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ojs/v1/health" {
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.Method == "POST" {
			importCalled = true
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]string{"id": "imported-1"})
			return
		}
	}))
	defer target.Close()

	m := New(&Config{SourceURL: source.URL, TargetURL: target.URL})
	err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !importCalled {
		t.Error("expected import to be called on target")
	}

	stats := m.GetStats()
	if stats.Imported != 1 {
		t.Errorf("expected 1 imported, got %d", stats.Imported)
	}
	if stats.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestMigrationSourceFailure(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer source.Close()

	m := New(&Config{SourceURL: source.URL, TargetURL: "http://unused:8080"})
	err := m.Run(context.Background())
	if err == nil {
		t.Error("expected error for source failure")
	}

	stats := m.GetStats()
	if stats.Phase != PhaseFailed {
		t.Errorf("expected failed phase, got %s", stats.Phase)
	}
}

func TestMigrationTargetHealthFailure(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}})
	}))
	defer source.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer target.Close()

	m := New(&Config{SourceURL: source.URL, TargetURL: target.URL})
	err := m.Run(context.Background())
	if err == nil {
		t.Error("expected error for unhealthy target")
	}
}

func TestStatsSnapshot(t *testing.T) {
	m := New(&Config{SourceURL: "http://a", TargetURL: "http://b"})
	stats := m.GetStats()
	if stats.Phase != PhaseIdle {
		t.Errorf("expected idle, got %s", stats.Phase)
	}
}

func TestMigrationMixedImportOutcomeIsPartial(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs": []map[string]any{
				{"type": "one", "queue": "default", "args": []any{}},
				{"type": "two", "queue": "default", "args": []any{}},
			},
		})
	}))
	defer source.Close()

	var posts int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			if posts == 2 {
				http.Error(w, "rejected", http.StatusUnprocessableEntity)
				return
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.Error(w, "verification should not run", http.StatusInternalServerError)
	}))
	defer target.Close()

	m := New(&Config{SourceURL: source.URL, TargetURL: target.URL})
	err := m.Run(context.Background())
	var partial *rootmigrate.PartialFailureError
	if !errors.As(err, &partial) {
		t.Fatalf("Run() error = %v, want PartialFailureError", err)
	}
	stats := m.GetStats()
	if stats.Phase != PhasePartial || stats.Imported != 1 || stats.Errors != 1 {
		t.Fatalf("stats = %+v, want partial with one imported and one error", stats)
	}
	if stats.CompletedAt == nil || len(stats.Failures) != 1 {
		t.Fatalf("terminal partial details missing: %+v", stats)
	}
}

func TestMigrationTargetConnectionFailureIsPartial(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs": []map[string]any{{"type": "one", "queue": "default", "args": []any{}}},
		})
	}))
	defer source.Close()

	target := httptest.NewServer(http.NotFoundHandler())
	targetURL := target.URL
	target.Close()

	m := New(&Config{SourceURL: source.URL, TargetURL: targetURL})
	err := m.Run(context.Background())
	var partial *rootmigrate.PartialFailureError
	if !errors.As(err, &partial) {
		t.Fatalf("Run() error = %v, want PartialFailureError", err)
	}
	if stats := m.GetStats(); stats.Phase != PhasePartial {
		t.Fatalf("phase = %s, want partial", stats.Phase)
	}
}
