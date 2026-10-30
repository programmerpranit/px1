//go:build windows

package main

import (
	"context"
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hideWindow keeps a child from opening its own console window. The
// background server runs detached with no console, and Windows gives every
// console program started from a console-less process a fresh visible one.
// CREATE_NO_WINDOW gives it a hidden console instead, which its own children
// (node, hooks, MCP servers) inherit.
func hideWindow(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

func quietCommand(name string, args ...string) *exec.Cmd {
	return hideWindow(exec.Command(name, args...))
}

func quietCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return hideWindow(exec.CommandContext(ctx, name, args...))
}
