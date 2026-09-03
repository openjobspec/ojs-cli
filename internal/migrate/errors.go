package migrate

import (
	"fmt"
	"strings"
)

// FailedRecord identifies a source record that could not be converted.
type FailedRecord struct {
	Source    string `json:"source"`
	Queue     string `json:"queue,omitempty"`
	Structure string `json:"structure,omitempty"`
	ID        string `json:"id,omitempty"`
	Index     int    `json:"index,omitempty"`
	Error     string `json:"error"`
}

// PartialExportError reports records skipped from an otherwise readable export.
type PartialExportError struct {
	Exported int            `json:"exported"`
	Failures []FailedRecord `json:"failures"`
}

func (e *PartialExportError) Error() string {
	return fmt.Sprintf("partial export: exported %d records and failed %d records", e.Exported, len(e.Failures))
}

// FailureDetail describes a failed item in a batch operation.
type FailureDetail struct {
	Index int    `json:"index,omitempty"`
	ID    string `json:"id,omitempty"`
	Type  string `json:"type,omitempty"`
	Error string `json:"error"`
}

// PartialFailureError reports a completed operation with one or more rejected items.
type PartialFailureError struct {
	Operation string          `json:"operation"`
	Total     int             `json:"total"`
	Failed    int             `json:"failed"`
	Details   []FailureDetail `json:"details,omitempty"`
}

func (e *PartialFailureError) Error() string {
	operation := strings.TrimSpace(e.Operation)
	if operation == "" {
		operation = "operation"
	}
	return fmt.Sprintf("%s partially failed: %d of %d items failed", operation, e.Failed, e.Total)
}
