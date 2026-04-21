// Package redact removes credentials from values that may be rendered or logged.
package redact

import (
	"errors"
	"net/url"
	"strings"
)

const replacement = "REDACTED"

var sensitiveQueryKeys = map[string]struct{}{
	"access_key":           {},
	"access_key_id":        {},
	"api_key":              {},
	"apikey":               {},
	"auth":                 {},
	"authorization":        {},
	"client_secret":        {},
	"credential":           {},
	"credentials":          {},
	"key":                  {},
	"password":             {},
	"passwd":               {},
	"pwd":                  {},
	"secret":               {},
	"signature":            {},
	"sig":                  {},
	"token":                {},
	"auth_token":           {},
	"access_token":         {},
	"refresh_token":        {},
	"session_token":        {},
	"security_token":       {},
	"x-amz-credential":     {},
	"x-amz-signature":      {},
	"x-amz-security-token": {},
}

// URL returns a URL safe for display. The original value should be retained only
// by the code that establishes the connection.
func URL(raw string) string {
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return replacement
	}

	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(replacement, replacement)
		} else {
			u.User = url.User(replacement)
		}
	}

	query := u.Query()
	for key := range query {
		if isSensitiveQueryKey(key) {
			query.Set(key, replacement)
		}
	}
	u.RawQuery = query.Encode()

	return u.String()
}

// RequestError returns an equivalent URL error with its displayed URL redacted.
func RequestError(err error) error {
	var requestErr *url.Error
	if !errors.As(err, &requestErr) {
		return err
	}
	copy := *requestErr
	copy.URL = URL(copy.URL)
	return &copy
}

func isSensitiveQueryKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	_, ok := sensitiveQueryKeys[normalized]
	return ok
}
