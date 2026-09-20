//go:build !windows

package main

import (
	"os"
	"syscall"
)

// detachSysProcAttr puts the background child in its own session so it
// survives the parent shell exiting and is not killed by the terminal's
// process-group signals (e.g. Ctrl-C in the launching shell).
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// processAlive reports whether pid names a live process, via the standard
// unix trick of sending signal 0 (no-op, but fails with ESRCH if the pid is
// gone).
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// killProcess sends SIGTERM so the target gets px1's normal graceful-shutdown
// path (main.go's signal handler) instead of being killed mid-write.
func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}
