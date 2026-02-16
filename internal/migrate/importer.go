package migrate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ImportResult holds the result of importing jobs into an OJS server.
type ImportResult struct {
	Total    int `json:"total"`
	Success  int `json:"success"`
	Failed   int `json:"failed"`
	Batches  int `json:"batches"`
}

// Poster is the interface for making POST requests to the OJS API.
type Poster interface {
	Post(path string, body any) ([]byte, int, error)
}

const importBatchSize = 100

// ImportFile reads an NDJSON file and imports jobs via the OJS API in batches.
func ImportFile(c Poster, filename string, progress func(imported, total int)) (*ImportResult, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	return importFromReader(c, f, progress)
}

func importFromReader(c Poster, r io.Reader, progress func(imported, total int)) (*ImportResult, error) {
	scanner := bufio.NewScanner(r)

	result := &ImportResult{}
	var batch []ExportedJob

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var job ExportedJob
		if err := json.Unmarshal([]byte(line), &job); err != nil {
			result.Failed++
			result.Total++
			continue
		}

		batch = append(batch, job)
		result.Total++

		if len(batch) >= importBatchSize {
			ok, fail := sendBatch(c, batch)
			result.Success += ok
			result.Failed += fail
			result.Batches++
			batch = batch[:0]

			if progress != nil {
				progress(result.Success, result.Total)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("read file: %w", err)
	}

	// Flush remaining
	if len(batch) > 0 {
		ok, fail := sendBatch(c, batch)
		result.Success += ok
		result.Failed += fail
		result.Batches++

		if progress != nil {
			progress(result.Success, result.Total)
		}
	}

	return result, nil
}

func sendBatch(c Poster, batch []ExportedJob) (success, failed int) {
	for _, job := range batch {
		body := map[string]any{
			"type": job.Type,
			"args": job.Args,
			"options": map[string]any{
				"queue": job.Queue,
			},
		}

		if job.Priority != nil {
			body["options"].(map[string]any)["priority"] = *job.Priority
		}
		if job.ScheduledAt != "" {
			body["options"].(map[string]any)["scheduled_at"] = job.ScheduledAt
		}
		if job.Meta != nil {
			body["meta"] = job.Meta
		}

		_, _, err := c.Post("/jobs", body)
		if err != nil {
			failed++
		} else {
			success++
		}
	}
	return
}
