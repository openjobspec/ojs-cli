package redact

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "redis password",
			raw:  "redis://worker:p%40ss@redis.example:6379/2",
			want: "redis://REDACTED:REDACTED@redis.example:6379/2",
		},
		{
			name: "amqp userinfo",
			raw:  "amqp://guest:secret@mq.example:5672/vhost",
			want: "amqp://REDACTED:REDACTED@mq.example:5672/vhost",
		},
		{
			name: "http query secrets",
			raw:  "https://api.example/jobs?token=abc&region=eu&client_secret=xyz",
			want: "https://api.example/jobs?client_secret=REDACTED&region=eu&token=REDACTED",
		},
		{
			name: "username only",
			raw:  "redis://operator@redis.example:6379",
			want: "redis://REDACTED@redis.example:6379",
		},
		{
			name: "plain URL",
			raw:  "http://localhost:8080/ojs/v1",
			want: "http://localhost:8080/ojs/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := URL(tt.raw)
			if got != tt.want {
				t.Fatalf("URL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
			for _, secret := range []string{"p%40ss", "secret@", "token=abc", "client_secret=xyz", "operator@"} {
				if strings.Contains(got, secret) {
					t.Fatalf("URL(%q) leaked %q in %q", tt.raw, secret, got)
				}
			}
		})
	}
}

func TestRequestErrorRedactsURLAndPreservesCause(t *testing.T) {
	cause := errors.New("connection refused")
	err := &url.Error{
		Op:  "Get",
		URL: "https://user:password@example.test/jobs?token=secret",
		Err: cause,
	}
	safe := RequestError(err)
	if strings.Contains(safe.Error(), "password") || strings.Contains(safe.Error(), "secret") {
		t.Fatalf("RequestError leaked credentials: %v", safe)
	}
	if !errors.Is(safe, cause) {
		t.Fatalf("RequestError no longer wraps original cause: %v", safe)
	}
}

func TestURLMalformedDoesNotEchoInput(t *testing.T) {
	const raw = "http://user:supersecret@%zz"
	if got := URL(raw); got != replacement {
		t.Fatalf("URL(malformed) = %q, want %q", got, replacement)
	}
}
