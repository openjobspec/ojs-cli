package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type sequencePoster struct {
	errors []error
	calls  int
}

func (p *sequencePoster) Post(_ string, _ any) ([]byte, int, error) {
	index := p.calls
	p.calls++
	if index < len(p.errors) && p.errors[index] != nil {
		return nil, 0, p.errors[index]
	}
	return []byte(`{"id":"ok"}`), 201, nil
}

func TestImportFromReaderReturnsPartialFailureAfterMixedResults(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"one","queue":"default","args":[]}`,
		`{"type":"two","queue":"default","args":[]}`,
		`{"type":"three","queue":"default","args":[]}`,
	}, "\n")
	poster := &sequencePoster{errors: []error{nil, errors.New("rejected"), nil}}

	result, err := importFromReader(poster, strings.NewReader(input), nil)
	var partial *PartialFailureError
	if !errors.As(err, &partial) {
		t.Fatalf("importFromReader() error = %v, want PartialFailureError", err)
	}
	if result.Success != 2 || result.Failed != 1 || result.Total != 3 {
		t.Fatalf("result = %+v, want success=2 failed=1 total=3", result)
	}
	if partial.Failed != 1 || len(partial.Details) != 1 || partial.Details[0].Index != 2 {
		t.Fatalf("partial = %+v, want failure at index 2", partial)
	}
}

func TestImportFromReaderReportsMalformedRecord(t *testing.T) {
	input := "{\"type\":\"ok\",\"queue\":\"default\",\"args\":[]}\n{\"type\":"
	result, err := importFromReader(&sequencePoster{}, strings.NewReader(input), nil)
	var partial *PartialFailureError
	if !errors.As(err, &partial) {
		t.Fatalf("importFromReader() error = %v, want PartialFailureError", err)
	}
	if result.Success != 1 || result.Failed != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestImportFromReaderAcceptsLargeBoundedRecord(t *testing.T) {
	payload, err := json.Marshal(strings.Repeat("x", 128*1024))
	if err != nil {
		t.Fatal(err)
	}
	input := fmt.Sprintf(`{"type":"large","queue":"default","args":[%s]}`, payload)
	result, err := importFromReader(&sequencePoster{}, strings.NewReader(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success != 1 {
		t.Fatalf("success = %d, want 1", result.Success)
	}
}
