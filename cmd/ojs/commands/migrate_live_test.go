package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openjobspec/ojs-go-backend-common/migration"
)

func newTestProxy(t *testing.T) *MigrationProxy {
	t.Helper()
	session, err := migration.NewSession("test", migration.SourceSidekiq)
	if err != nil {
		t.Fatal(err)
	}
	session.StartDualRun(100) // Route all to OJS for testing
	return &MigrationProxy{
		session:    session,
		ojsURL:     "http://ojs-test:8080",
		httpClient: &http.Client{Timeout: time.Second},
	}
}

func TestHandleHealth(t *testing.T) {
	proxy := newTestProxy(t)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	proxy.HandleHealth(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected ok, got %q", resp["status"])
	}
}

func TestHandleStatus(t *testing.T) {
	proxy := newTestProxy(t)
	req := httptest.NewRequest("GET", "/migrate/status", nil)
	w := httptest.NewRecorder()
	proxy.HandleStatus(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["source"] != "sidekiq" {
		t.Errorf("expected sidekiq, got %v", resp["source"])
	}
}

func TestHandleJobMethodNotAllowed(t *testing.T) {
	proxy := newTestProxy(t)
	req := httptest.NewRequest("GET", "/migrate/jobs", nil)
	w := httptest.NewRecorder()
	proxy.HandleJob(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleJobTranslationError(t *testing.T) {
	proxy := newTestProxy(t)
	req := httptest.NewRequest("POST", "/migrate/jobs", strings.NewReader("not valid json"))
	w := httptest.NewRecorder()
	proxy.HandleJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleJobRoutedToOJS(t *testing.T) {
	// Set up a mock OJS server
	ojsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "job-123", "state": "available"})
	}))
	defer ojsMock.Close()

	session, _ := migration.NewSession("test", migration.SourceSidekiq)
	session.StartDualRun(100)

	proxy := &MigrationProxy{
		session:    session,
		ojsURL:     ojsMock.URL,
		httpClient: &http.Client{Timeout: time.Second},
	}

	body := `{"class":"EmailWorker","args":["test@example.com"],"jid":"abc123","queue":"mailers"}`
	req := httptest.NewRequest("POST", "/migrate/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()
	proxy.HandleJob(w, req)

	if w.Code != 201 {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Migration-Routed") != "ojs" {
		t.Error("expected X-Migration-Routed: ojs header")
	}
}

func TestHandleJobRoutedToLegacy(t *testing.T) {
	session, _ := migration.NewSession("test", migration.SourceSidekiq)
	session.StartDualRun(0) // 0% to OJS = all legacy

	proxy := &MigrationProxy{
		session:    session,
		ojsURL:     "http://unused",
		httpClient: &http.Client{Timeout: time.Second},
	}

	body := `{"class":"Worker","args":[],"jid":"x"}`
	req := httptest.NewRequest("POST", "/migrate/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()
	proxy.HandleJob(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["routed"] != "legacy" {
		t.Errorf("expected legacy routing, got %v", resp["routed"])
	}
}

func TestHandleSetPercentage(t *testing.T) {
	proxy := newTestProxy(t)
	body := `{"percentage": 50}`
	req := httptest.NewRequest("POST", "/migrate/percentage", strings.NewReader(body))
	w := httptest.NewRecorder()
	proxy.HandleSetPercentage(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCutover(t *testing.T) {
	proxy := newTestProxy(t)
	req := httptest.NewRequest("POST", "/migrate/cutover", nil)
	w := httptest.NewRecorder()
	proxy.HandleCutover(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRollback(t *testing.T) {
	proxy := newTestProxy(t)
	body := `{"reason":"too many errors"}`
	req := httptest.NewRequest("POST", "/migrate/rollback", strings.NewReader(body))
	w := httptest.NewRecorder()
	proxy.HandleRollback(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCutoverInvalidState(t *testing.T) {
	session, _ := migration.NewSession("test", migration.SourceSidekiq)
	// Not in dual_run state, so cutover should fail
	proxy := &MigrationProxy{session: session, ojsURL: "http://unused", httpClient: &http.Client{}}
	req := httptest.NewRequest("POST", "/migrate/cutover", nil)
	w := httptest.NewRecorder()
	proxy.HandleCutover(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 conflict, got %d", w.Code)
	}
}
