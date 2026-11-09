//go:build windows

package api

import (
	"errors"
	"syscall"
)

// emUso: no Windows, arquivo aberto por outro processo (ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION).
func emUso(err error) bool {
	var e syscall.Errno
	return errors.As(err, &e) && (e == 32 || e == 33)
}
