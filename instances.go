package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// Registry of running px1 servers, so `px1 ps`/`px1 kill`/`px1 kill-all` can
// find and stop them. One JSON file per PID in the OS temp dir — ephemeral
// runtime bookkeeping, not workspace state, following the same pattern as
// the daemon status/log files in daemon.go.
type instanceInfo struct {
	PID       int       `json:"pid"`
	URL       string    `json:"url"`
	Root      string    `json:"root"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
}

func instancesDir() string {
	dir := filepath.Join(os.TempDir(), "px1-instances")
	_ = os.MkdirAll(dir, 0o700)
	return dir
}

func instanceFile(pid int) string {
	return filepath.Join(instancesDir(), strconv.Itoa(pid)+".json")
}

// registerInstance records this running server. Best-effort: a failure here
// never stops the server from starting.
func registerInstance(root, url string) {
	info := instanceInfo{PID: os.Getpid(), URL: url, Root: root, Version: version, StartedAt: time.Now()}
	data, err := json.Marshal(info)
	if err != nil {
		return
	}
	_ = os.WriteFile(instanceFile(info.PID), data, 0o600)
}

func unregisterInstance() {
	_ = os.Remove(instanceFile(os.Getpid()))
}

// listInstances reads the registry and drops (and removes) entries whose
// process is no longer alive, so a `kill -9` or crash self-heals the list
// instead of leaving stale ghosts behind.
func listInstances() []instanceInfo {
	dir := instancesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []instanceInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var info instanceInfo
		if json.Unmarshal(data, &info) != nil || !processAlive(info.PID) {
			os.Remove(path)
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

func runPS() {
	instances := listInstances()
	if len(instances) == 0 {
		fmt.Println("no px1 instances running")
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PID\tURL\tWORKSPACE\tUPTIME")
	for _, in := range instances {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", in.PID, in.URL, in.Root, time.Since(in.StartedAt).Round(time.Second))
	}
	tw.Flush()
}

func runKill(arg string) {
	pid, err := strconv.Atoi(arg)
	if err != nil {
		fatal(fmt.Errorf("kill: invalid pid %q", arg))
	}
	if !processAlive(pid) {
		os.Remove(instanceFile(pid))
		fatal(fmt.Errorf("kill: no running px1 instance with pid %d", pid))
	}
	if err := killProcess(pid); err != nil {
		fatal(fmt.Errorf("kill %d: %w", pid, err))
	}
	os.Remove(instanceFile(pid))
	fmt.Printf("killed %d\n", pid)
}

func runKillAll() {
	instances := listInstances()
	if len(instances) == 0 {
		fmt.Println("no px1 instances running")
		return
	}
	for _, in := range instances {
		if err := killProcess(in.PID); err != nil {
			fmt.Fprintf(os.Stderr, "px1: kill %d: %v\n", in.PID, err)
			continue
		}
		os.Remove(instanceFile(in.PID))
		fmt.Printf("killed %d (%s)\n", in.PID, in.Root)
	}
}
