package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/openjobspec/ojs-cli/internal/redact"
	"github.com/openjobspec/ojs-go-backend-common/migration"
)

const (
	maxMigrationProxyRequestBytes  = 1 << 20
	maxMigrationProxyResponseBytes = 8 << 20
)

// MigrateLive starts a live migration proxy that translates Sidekiq/BullMQ/Celery
// jobs to OJS format with configurable traffic splitting.
//
// Usage:
//
//	ojs migrate live --source=sidekiq --ojs-url=http://localhost:8080 --listen=:8090 --percentage=10
func MigrateLive(args []string) error {
	fs := flag.NewFlagSet("migrate live", flag.ContinueOnError)
	sourceName := fs.String("source", string(migration.SourceSidekiq), "Legacy source format")
	ojsURL := fs.String("ojs-url", "http://localhost:8080", "OJS target URL")
	listenAddr := fs.String("listen", ":8090", "Proxy listen address")
	percentage := fs.Int("percentage", 10, "Percentage of traffic routed to OJS")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	source := migration.Source(*sourceName)

	session, err := migration.NewSession("live-migration", source)
	if err != nil {
		return fmt.Errorf("creating migration session: %w", err)
	}

	if err := session.StartDualRun(*percentage); err != nil {
		return fmt.Errorf("starting dual run: %w", err)
	}

	proxy := &MigrationProxy{
		session:    session,
		ojsURL:     *ojsURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/migrate/jobs", proxy.HandleJob)
	mux.HandleFunc("/migrate/status", proxy.HandleStatus)
	mux.HandleFunc("/migrate/percentage", proxy.HandleSetPercentage)
	mux.HandleFunc("/migrate/cutover", proxy.HandleCutover)
	mux.HandleFunc("/migrate/rollback", proxy.HandleRollback)
	mux.HandleFunc("/health", proxy.HandleHealth)

	fmt.Printf("🔄 Migration proxy started\n")
	fmt.Printf("   Source:     %s\n", source)
	fmt.Printf("   OJS URL:    %s\n", redact.URL(*ojsURL))
	fmt.Printf("   Listen:     %s\n", *listenAddr)
	fmt.Printf("   Split:      %d%% → OJS\n", *percentage)
	fmt.Printf("\n   POST /migrate/jobs          Submit legacy jobs\n")
	fmt.Printf("   GET  /migrate/status        Migration status\n")
	fmt.Printf("   POST /migrate/percentage    Change traffic split\n")
	fmt.Printf("   POST /migrate/cutover       Route 100%% to OJS\n")
	fmt.Printf("   POST /migrate/rollback      Revert to 0%% OJS\n")

	server := &http.Server{
		Addr:         *listenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server.ListenAndServe()
}

// MigrationProxy handles HTTP requests for live migration.
type MigrationProxy struct {
	session    *migration.Session
	ojsURL     string
	httpClient *http.Client
}

// HandleJob accepts a legacy job payload and routes it based on the traffic split.
func (p *MigrationProxy) HandleJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMigrationProxyRequestBytes))
	if err != nil {
		writeHTTPJSON(w, http.StatusBadRequest, map[string]string{"error": "reading body: " + err.Error()})
		return
	}

	if p.session.Splitter.ShouldRouteToOJS() {
		// Translate and forward to OJS
		job, err := p.session.Translator.Translate(body)
		if err != nil {
			p.session.Splitter.RecordError()
			writeHTTPJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "translation failed: " + err.Error(),
				"routed": "ojs",
			})
			return
		}

		jobJSON, err := json.Marshal(map[string]interface{}{
			"type": job.Type,
			"args": json.RawMessage(job.Args),
			"options": map[string]interface{}{
				"queue": job.Queue,
			},
		})
		if err != nil {
			p.session.Splitter.RecordError()
			writeHTTPJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "encoding translated job: " + err.Error(),
				"routed": "ojs",
			})
			return
		}

		req, err := http.NewRequestWithContext(
			r.Context(),
			http.MethodPost,
			p.ojsURL+"/ojs/v1/jobs",
			bytes.NewReader(jobJSON),
		)
		if err != nil {
			p.session.Splitter.RecordError()
			writeHTTPJSON(w, http.StatusBadGateway, map[string]string{
				"error":  "creating OJS request: " + err.Error(),
				"routed": "ojs",
			})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := p.httpClient.Do(req)
		if err != nil {
			p.session.Splitter.RecordError()
			writeHTTPJSON(w, http.StatusBadGateway, map[string]string{
				"error":  "forwarding to OJS: " + redact.RequestError(err).Error(),
				"routed": "ojs",
			})
			return
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxMigrationProxyResponseBytes+1))
		if err != nil {
			p.session.Splitter.RecordError()
			writeHTTPJSON(w, http.StatusBadGateway, map[string]string{
				"error":  "reading OJS response: " + err.Error(),
				"routed": "ojs",
			})
			return
		}
		if len(respBody) > maxMigrationProxyResponseBytes {
			p.session.Splitter.RecordError()
			writeHTTPJSON(w, http.StatusBadGateway, map[string]string{
				"error":  "OJS response exceeded size limit",
				"routed": "ojs",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Migration-Routed", "ojs")
		w.WriteHeader(resp.StatusCode)
		if _, err := w.Write(respBody); err != nil {
			p.session.Splitter.RecordError()
		}
	} else {
		p.session.Splitter.RecordError()
		writeHTTPJSON(w, http.StatusNotImplemented, map[string]interface{}{
			"routed": "legacy",
			"error":  "legacy forwarding is not implemented; the job was not accepted",
			"size":   len(body),
		})
	}
}

// HandleStatus returns the current migration status and stats.
func (p *MigrationProxy) HandleStatus(w http.ResponseWriter, r *http.Request) {
	stats := p.session.Splitter.GetStats()
	writeHTTPJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": p.session.ID,
		"source":     p.session.Source,
		"status":     p.session.GetStatus(),
		"stats":      stats,
	})
}

// HandleSetPercentage changes the OJS traffic percentage.
func (p *MigrationProxy) HandleSetPercentage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Percentage int `json:"percentage"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeHTTPJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := p.session.StartDualRun(req.Percentage); err != nil {
		writeHTTPJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	writeHTTPJSON(w, http.StatusOK, map[string]interface{}{
		"percentage": req.Percentage,
		"status":     p.session.GetStatus(),
	})
}

// HandleCutover routes 100% of traffic to OJS.
func (p *MigrationProxy) HandleCutover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := p.session.Cutover(); err != nil {
		writeHTTPJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeHTTPJSON(w, http.StatusOK, map[string]string{
		"status":  string(p.session.GetStatus()),
		"message": "100% traffic now routed to OJS",
	})
}

// HandleRollback reverts to 0% OJS traffic.
func (p *MigrationProxy) HandleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req)
	if err != nil && !errors.Is(err, io.EOF) {
		writeHTTPJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Reason == "" {
		req.Reason = "manual rollback"
	}

	if err := p.session.Rollback(req.Reason); err != nil {
		writeHTTPJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeHTTPJSON(w, http.StatusOK, map[string]string{
		"status":  string(p.session.GetStatus()),
		"message": "rolled back to 0% OJS traffic",
	})
}

// HandleHealth returns a simple health check.
func (p *MigrationProxy) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeHTTPJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeHTTPJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		return
	}
}
