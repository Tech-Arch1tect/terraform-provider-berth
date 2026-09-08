package provider

import (
	"errors"

	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

type readOutcome int

const (
	readSucceeded readOutcome = iota
	readGone
	readFailed
)

func classifyRead(err error) readOutcome {
	switch {
	case err == nil:
		return readSucceeded
	case errors.Is(err, client.ErrNotFound):
		return readGone
	default:
		return readFailed
	}
}
