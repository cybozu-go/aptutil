package mirror

import (
	"os"
	"syscall"
)

// Flock is a simple wrapper around *os.File to call flock(2).
type Flock struct {
	F *os.File
}

// Lock calls flock(2) with LOCK_EX|LOCK_NB
//
// If lock cannot be acquired, non-nil error will be returned.
func (fl Flock) Lock() error {
	err := syscall.Flock(int(fl.F.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	return os.NewSyscallError("flock", err)
}

// Unlock calls flock(2) with LOCK_UN
func (fl Flock) Unlock() error {
	err := syscall.Flock(int(fl.F.Fd()), syscall.LOCK_UN)
	return os.NewSyscallError("flock", err)
}
