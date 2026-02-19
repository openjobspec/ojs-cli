package migrate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ValidationResult holds the outcome of validating an exported NDJSON file.
type ValidationResult struct {
	Total    int              `json:"total"`
	Valid    int              `json:"valid"`
	Invalid  int              `json:"invalid"`
	Errors   []ValidationError `json:"errors,omitempty"`
}

// ValidationError describes a single validation failure.
type ValidationError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ValidateFile reads an NDJSON file and validates each line as a valid OJS job.
func ValidateFile(filename string) (*ValidationResult, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	return validateFromReader(f)
}

func validateFromReader(r io.Reader) (*ValidationResult, error) {
	scanner := bufio.NewScanner(r)
	result := &ValidationResult{}
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		result.Total++

		var job ExportedJob
		if err := json.Unmarshal([]byte(line), &job); err != nil {
			result.Invalid++
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Message: fmt.Sprintf("invalid JSON: %s", err),
			})
			continue
		}

		if errs := validateJob(&job, lineNum); len(errs) > 0 {
			result.Invalid++
			result.Errors = append(result.Errors, errs...)
			continue
		}

		result.Valid++
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("read file: %w", err)
	}

	return result, nil
}

func validateJob(job *ExportedJob, line int) []ValidationError {
	var errs []ValidationError

	if job.Type == "" {
		errs = append(errs, ValidationError{
			Line:    line,
			Message: "missing required field: type",
		})
	}

	if job.Queue == "" {
		errs = append(errs, ValidationError{
			Line:    line,
			Message: "missing required field: queue",
		})
	}

	if job.Args == nil {
		errs = append(errs, ValidationError{
			Line:    line,
			Message: "missing required field: args",
		})
	} else {
		// Validate args is a JSON array
		trimmed := json.RawMessage(job.Args)
		var arr []json.RawMessage
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			errs = append(errs, ValidationError{
				Line:    line,
				Message: "args must be a JSON array",
			})
		}
	}

	if job.Priority != nil && *job.Priority < 0 {
		errs = append(errs, ValidationError{
			Line:    line,
			Message: "priority must be non-negative",
		})
	}

	return errs
}
