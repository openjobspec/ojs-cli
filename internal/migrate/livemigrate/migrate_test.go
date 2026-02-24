package livemigrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

	m := New(Config{
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

	m := New(Config{SourceURL: source.URL, TargetURL: target.URL})
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

	m := New(Config{SourceURL: source.URL, TargetURL: "http://unused:8080"})
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

	m := New(Config{SourceURL: source.URL, TargetURL: target.URL})
	err := m.Run(context.Background())
	if err == nil {
		t.Error("expected error for unhealthy target")
	}
}

func TestStatsSnapshot(t *testing.T) {
	m := New(Config{SourceURL: "http://a", TargetURL: "http://b"})
	stats := m.GetStats()
	if stats.Phase != PhaseIdle {
		t.Errorf("expected idle, got %s", stats.Phase)
	}
}
