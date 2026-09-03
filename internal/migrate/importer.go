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
	Total    int             `json:"total"`
	Success  int             `json:"success"`
	Failed   int             `json:"failed"`
	Batches  int             `json:"batches"`
	Failures []FailureDetail `json:"failures,omitempty"`
}

// Poster is the interface for making POST requests to the OJS API.
type Poster interface {
	Post(path string, body any) ([]byte, int, error)
}

const (
	importBatchSize      = 100
	maxImportRecordBytes = 4 << 20
)

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
	scanner.Buffer(make([]byte, 64*1024), maxImportRecordBytes)

	result := &ImportResult{}
	var batch []ExportedJob
	var batchIndexes []int

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var job ExportedJob
		if err := json.Unmarshal([]byte(line), &job); err != nil {
			result.Failed++
			result.Total++
			result.Failures = append(result.Failures, FailureDetail{
				Index: result.Total,
				Error: fmt.Sprintf("decode record: %v", err),
			})
			continue
		}

		batch = append(batch, job)
		result.Total++
		batchIndexes = append(batchIndexes, result.Total)

		if len(batch) >= importBatchSize {
			ok, failures := sendBatch(c, batch, batchIndexes)
			result.Success += ok
			result.Failed += len(failures)
			result.Failures = append(result.Failures, failures...)
			result.Batches++
			batch = batch[:0]
			batchIndexes = batchIndexes[:0]

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
		ok, failures := sendBatch(c, batch, batchIndexes)
		result.Success += ok
		result.Failed += len(failures)
		result.Failures = append(result.Failures, failures...)
		result.Batches++

		if progress != nil {
			progress(result.Success, result.Total)
		}
	}

	if result.Failed > 0 {
		return result, &PartialFailureError{
			Operation: "job import",
			Total:     result.Total,
			Failed:    result.Failed,
			Details:   append([]FailureDetail(nil), result.Failures...),
		}
	}
	return result, nil
}

func sendBatch(c Poster, batch []ExportedJob, indexes []int) (int, []FailureDetail) {
	success := 0
	failures := make([]FailureDetail, 0)
	for i, job := range batch {
		options := map[string]any{"queue": job.Queue}
		body := map[string]any{
			"type":    job.Type,
			"args":    job.Args,
			"options": options,
		}

		if job.Priority != nil {
			options["priority"] = *job.Priority
		}
		if job.ScheduledAt != "" {
			options["scheduled_at"] = job.ScheduledAt
		}
		if job.Meta != nil {
			body["meta"] = job.Meta
		}

		_, _, err := c.Post("/jobs", body)
		if err != nil {
			index := i + 1
			if i < len(indexes) {
				index = indexes[i]
			}
			failures = append(failures, FailureDetail{
				Index: index,
				Type:  job.Type,
				Error: err.Error(),
			})
		} else {
			success++
		}
	}
	return success, failures
}
