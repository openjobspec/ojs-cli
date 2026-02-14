package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTable(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Table(
		[]string{"NAME", "STATUS"},
		[][]string{
			{"default", "active"},
			{"priority", "paused"},
		},
	)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "NAME") {
		t.Error("output missing header NAME")
	}
	if !strings.Contains(out, "default") {
		t.Error("output missing row 'default'")
	}
	if !strings.Contains(out, "paused") {
		t.Error("output missing row 'paused'")
	}
}

func TestJSON(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[string]string{"key": "value"}
	err := JSON(data)
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, `"key"`) {
		t.Error("output missing key field")
	}
	if !strings.Contains(out, `"value"`) {
		t.Error("output missing value field")
	}
}
