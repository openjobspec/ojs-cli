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

func TestPrintResultAndMessages(t *testing.T) {
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	t.Cleanup(func() {
		os.Stdout, os.Stderr = oldOut, oldErr
	})

	Format = "table"
	err = PrintResult([]any{"one", "two"}, []string{"VALUE"}, func(value any) []string {
		return []string{value.(string)}
	})
	if err != nil {
		t.Fatal(err)
	}
	Success("done %d", 2)
	Warn("warning %s", "here")

	Format = "json"
	if err := PrintResult(map[string]int{"count": 2}, nil, nil); err != nil {
		t.Fatal(err)
	}

	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if _, err := stdout.ReadFrom(outR); err != nil {
		t.Fatal(err)
	}
	if _, err := stderr.ReadFrom(errR); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "one") || !strings.Contains(stdout.String(), "✓ done 2") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "⚠ warning here") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
