//go:build !windows

package main

import "syscall"

// detachSysProcAttr puts the background child in its own session so it
// survives the parent shell exiting and is not killed by the terminal's
// process-group signals (e.g. Ctrl-C in the launching shell).
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
