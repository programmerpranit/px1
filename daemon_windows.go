//go:build windows

package main

import (
	"os"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// detachSysProcAttr starts the background child outside the launching
// console's process group so it keeps running after the shell exits.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
}

// processAlive reports whether pid names a live process. Windows has no
// signal-0 equivalent; os.FindProcess itself opens a real handle via
// OpenProcess and fails if the pid doesn't exist.
func processAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}

// killProcess terminates the target process. Windows doesn't support SIGTERM
// through os.Process.Signal, so this is a hard TerminateProcess rather than
// px1's graceful shutdown path.
func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
