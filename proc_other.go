//go:build !windows

package main

import (
	"context"
	"os/exec"
)

// quietCommand and quietCommandContext are plain exec wrappers off Windows,
// where child processes never open their own console window.
func quietCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func quietCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
