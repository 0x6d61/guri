//go:build !windows

package subprocess

import (
	"errors"
	"syscall"
)

func closedChildInput(err error) bool { return errors.Is(err, syscall.EPIPE) }
