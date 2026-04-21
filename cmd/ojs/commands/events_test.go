package commands

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestScanSSEAcceptsEventOver64KiB(t *testing.T) {
	payload := strings.Repeat("x", 128*1024)
	var got string
	err := scanSSE(context.Background(), strings.NewReader("data: "+payload+"\n\n"), func(data string) error {
		got = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != payload {
		t.Fatalf("event payload length = %d, want %d", len(got), len(payload))
	}
}

func TestScanSSERejectsEventOverBound(t *testing.T) {
	payload := strings.Repeat("x", maxSSEEventLineBytes+1)
	called := false
	err := scanSSE(context.Background(), strings.NewReader("data: "+payload+"\n"), func(string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("scanSSE() error = nil, want over-limit error")
	}
	if called {
		t.Fatal("handler called for over-limit event")
	}
}

func TestScanSSEPropagatesMidStreamReadError(t *testing.T) {
	readErr := errors.New("connection reset")
	reader := &errorAfterReader{data: []byte("data: first\n"), err: readErr}
	var events []string
	err := scanSSE(context.Background(), reader, func(data string) error {
		events = append(events, data)
		return nil
	})
	if !errors.Is(err, readErr) {
		t.Fatalf("scanSSE() error = %v, want %v", err, readErr)
	}
	if len(events) != 1 || events[0] != "first" {
		t.Fatalf("events = %v, want [first]", events)
	}
}

func TestScanSSENormalEOFAndCancellation(t *testing.T) {
	if err := scanSSE(context.Background(), strings.NewReader("data: ok\n"), func(string) error {
		return nil
	}); err != nil {
		t.Fatalf("normal EOF returned %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := scanSSE(ctx, strings.NewReader("data: ignored\n"), func(string) error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan error = %v, want context.Canceled", err)
	}
}

type errorAfterReader struct {
	data []byte
	err  error
}

func (r *errorAfterReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	if r.err != nil {
		err := r.err
		r.err = nil
		return 0, err
	}
	return 0, io.EOF
}
