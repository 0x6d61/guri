package subprocess

import (
	"errors"
	"syscall"
)

func closedChildInput(err error) bool {
	const errorNoData = syscall.Errno(232)
	return errors.Is(err, syscall.ERROR_BROKEN_PIPE) || errors.Is(err, errorNoData)
}
