package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const (
	daemonChildEnv  = "PX1_DAEMON_CHILD"
	daemonStatusEnv = "PX1_DAEMON_STATUS_FILE"
)

type daemonStatus struct {
	PID     int    `json:"pid"`
	URL     string `json:"url"`
	Root    string `json:"root"`
	Version string `json:"version"`
}

// isDaemonChild reports whether this process is the re-exec'd background
// child spawned by runDetached, as opposed to the process the user typed.
func isDaemonChild() bool {
	return os.Getenv(daemonChildEnv) != ""
}

// reportDaemonReady is called by the child once it has bound a listener, so
// the parent (still attached to the user's terminal) can pick up the real
// URL and print it before handing the shell back.
func reportDaemonReady(root, url string) {
	path := os.Getenv(daemonStatusEnv)
	if path == "" {
		return
	}
	data, err := json.Marshal(daemonStatus{PID: os.Getpid(), URL: url, Root: root, Version: version})
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// runDetached re-launches the current command as a background, session-detached
// process (stdin closed, stdout/stderr to a log file in the OS temp dir), waits
// briefly for it to report the URL it bound, prints that, and exits — handing
// the shell back to the user instead of blocking it for the life of the server.
func runDetached() {
	self, err := os.Executable()
	if err != nil {
		fatal(fmt.Errorf("-d: %w", err))
	}

	var args []string
	for _, a := range os.Args[1:] {
		if a == "-d" {
			continue
		}
		args = append(args, a)
	}

	statusFile, err := os.CreateTemp("", "px1-status-*.json")
	if err != nil {
		fatal(fmt.Errorf("-d: %w", err))
	}
	statusPath := statusFile.Name()
	statusFile.Close()
	os.Remove(statusPath)
	defer os.Remove(statusPath)

	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("px1-%d.log", time.Now().UnixNano()))
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fatal(fmt.Errorf("-d: %w", err))
	}
	defer logf.Close()

	cmd := exec.Command(self, args...)
	cmd.Env = append(os.Environ(), daemonChildEnv+"=1", daemonStatusEnv+"="+statusPath)
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = detachSysProcAttr()

	if err := cmd.Start(); err != nil {
		fatal(fmt.Errorf("-d: %w", err))
	}

	var st daemonStatus
	found := false
	for i := 0; i < 150; i++ { // poll up to ~3s
		data, err := os.ReadFile(statusPath)
		if err == nil && len(data) > 0 && json.Unmarshal(data, &st) == nil {
			found = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !found {
		fmt.Fprintf(os.Stdout, "px1 is starting in the background (pid %d) — if it doesn't come up, check %s\n", cmd.Process.Pid, logPath)
		os.Exit(0)
	}

	uiHeading("px1 "+st.Version, nil, os.Stdout)
	uiKV("workspace", st.Root, 11, os.Stdout)
	uiKV("url", uiAccent(st.URL, os.Stdout), 11, os.Stdout)
	uiKV("pid", strconv.Itoa(st.PID), 11, os.Stdout)
	uiKV("log", logPath, 11, os.Stdout)
	uiHint("running in the background — px1 kill "+strconv.Itoa(st.PID)+" (or px1 kill-all) to stop it", os.Stdout)
	os.Exit(0)
}
