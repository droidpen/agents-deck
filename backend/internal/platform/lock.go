package platform

import (
	"fmt"
	"os"
	"syscall"
)

// Lock prevents a second daemon from unlinking the first daemon's socket or
// writing the same state files. The kernel releases the lock after a crash.
func Lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("daemon already running or lock unavailable: %w", err)
	}
	return func() { _ = f.Close() }, nil
}
