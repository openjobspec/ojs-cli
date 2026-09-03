package commands

import (
	"errors"

	"github.com/openjobspec/ojs-cli/internal/client"
)

// ErrShadowNotImplemented is returned until the CLI has a real source tailer
// and comparison engine. Returning immediately prevents a no-op run from being
// mistaken for a successful production validation.
var ErrShadowNotImplemented = errors.New(
	"migrate shadow is not implemented: no jobs would be mirrored; use migrate export/import instead",
)

func migrateShadow(_ *client.Client, _ []string) error {
	return ErrShadowNotImplemented
}
