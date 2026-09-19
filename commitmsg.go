package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// generateCommitMessage asks the claude CLI, if installed, to write a commit
// message for the currently staged changes. It shells out rather than
// embedding a harness-selection system: one CLI, one call, no state.
func generateCommitMessage(ctx context.Context, root string) (string, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return "", fmt.Errorf("claude CLI not found on PATH")
	}
	diff := gitDiffCachedAll(root)
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("no staged changes to summarize")
	}
	const maxDiff = 20000 // keep the prompt small; a huge diff still needs a short message
	if len(diff) > maxDiff {
		diff = diff[:maxDiff] + "\n... (diff truncated)"
	}

	prompt := "Write a single concise git commit message (Conventional Commits style, one line, no body, no quotes, no markdown) " +
		"for the staged diff on stdin. Output only the message text, nothing else.\n\n" + diff

	cmd := exec.CommandContext(ctx, "claude", "-p", prompt)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("claude: %s", msg)
	}

	msg := strings.TrimSpace(stdout.String())
	if msg == "" {
		return "", fmt.Errorf("claude returned no message")
	}
	// Keep only the first line in case the model added extra chatter.
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = strings.TrimSpace(msg[:i])
	}
	return msg, nil
}
