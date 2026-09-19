package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSaveRoundTrip(t *testing.T) {
	s, root := newTestServer(t)
	abs := filepath.Join(root, "main.go")
	st, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}

	code, body := agentPostJSON(t, s, "/api/file/save", map[string]any{
		"path":    "main.go",
		"content": "package main\n\n// edited\nfunc main() {}\n",
		"mtime":   st.ModTime().UnixMilli(),
		"size":    st.Size(),
	})
	if code != 200 {
		t.Fatalf("save status %d body %v", code, body)
	}
	if body["ok"] != true {
		t.Fatalf("save body %v", body)
	}

	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package main\n\n// edited\nfunc main() {}\n" {
		t.Fatalf("file content = %q", got)
	}

	// Original permissions are preserved.
	newSt, _ := os.Stat(abs)
	if newSt.Mode().Perm() != st.Mode().Perm() {
		t.Errorf("mode = %v, want %v", newSt.Mode().Perm(), st.Mode().Perm())
	}
}

func TestFileSaveConflictOnStaleToken(t *testing.T) {
	s, root := newTestServer(t)
	abs := filepath.Join(root, "main.go")

	// Stale mtime/size (zero) never matches the real file.
	code, body := agentPostJSON(t, s, "/api/file/save", map[string]any{
		"path":    "main.go",
		"content": "package main\n",
		"mtime":   int64(1),
		"size":    int64(1),
	})
	if code != 409 {
		t.Fatalf("status %d, want 409 (%v)", code, body)
	}
	if body["content"] == nil {
		t.Errorf("409 body missing current content: %v", body)
	}

	// The file on disk must be untouched.
	got, _ := os.ReadFile(abs)
	if string(got) == "package main\n" {
		t.Fatalf("stale save was applied despite conflict")
	}
}

func TestFileSaveRejectsPathTraversal(t *testing.T) {
	s, _ := newTestServer(t)
	code, body := agentPostJSON(t, s, "/api/file/save", map[string]any{
		"path":    "../outside.go",
		"content": "package main\n",
	})
	if code != 400 {
		t.Fatalf("status %d, want 400 (%v)", code, body)
	}
}

func TestFileSaveRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("shh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ix := NewIndex(root)
	ix.Build()
	s := NewServer(ix, nil)

	code, body := agentPostJSON(t, s, "/api/file/save", map[string]any{
		"path":    "escape/secret.txt",
		"content": "pwned\n",
	})
	if code != 400 {
		t.Fatalf("status %d, want 400 (%v)", code, body)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "shh\n" {
		t.Fatalf("symlink escape wrote through: %q", got)
	}
}

func TestFileSaveRejectsOversizedContent(t *testing.T) {
	s, _ := newTestServer(t)
	big := make([]byte, maxEditBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	code, body := agentPostJSON(t, s, "/api/file/save", map[string]any{
		"path":    "main.go",
		"content": string(big),
	})
	if code != 413 {
		t.Fatalf("status %d, want 413 (%v)", code, body)
	}
}

func TestFileSaveRejectsBinaryContent(t *testing.T) {
	s, _ := newTestServer(t)
	code, body := agentPostJSON(t, s, "/api/file/save", map[string]any{
		"path":    "main.go",
		"content": "package main\x00binary",
	})
	if code != 413 {
		t.Fatalf("status %d, want 413 (%v)", code, body)
	}
}

func TestFileSaveRequiresLocalPost(t *testing.T) {
	s, _ := newTestServer(t)
	code, body := get(t, s, "/api/file/save?path=main.go")
	if code != 405 {
		t.Fatalf("status %d, want 405 (%v)", code, body)
	}
}

func TestFileSaveBlockedWhileAgentEditRunning(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	harness := writeHarness(t, "sleep 5\n")
	s := agentServer(t, root, harness)

	code, body := agentPostJSON(t, s, "/api/agent/edit", map[string]any{
		"path": "main.go", "l1": 1, "l2": 3, "instruction": "noop",
	})
	if code != 200 {
		t.Fatalf("agent edit start status %d (%v)", code, body)
	}

	saveCode, saveBody := agentPostJSON(t, s, "/api/file/save", map[string]any{
		"path": "main.go", "content": "package main\n",
	})
	if saveCode != 409 {
		t.Fatalf("save status %d, want 409 while agent job runs (%v)", saveCode, saveBody)
	}
}

func TestFileRenameRoundTrip(t *testing.T) {
	s, root := newTestServer(t)
	code, body := agentPostJSON(t, s, "/api/file/rename", map[string]any{
		"path": "greet.go", "newPath": "hello/greet.go",
	})
	if code != 200 {
		t.Fatalf("rename status %d (%v)", code, body)
	}
	if _, err := os.Stat(filepath.Join(root, "hello", "greet.go")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "greet.go")); !os.IsNotExist(err) {
		t.Fatalf("old path still exists: err=%v", err)
	}
}

func TestFileRenameRejectsExistingDestination(t *testing.T) {
	s, _ := newTestServer(t)
	code, body := agentPostJSON(t, s, "/api/file/rename", map[string]any{
		"path": "greet.go", "newPath": "main.go",
	})
	if code != 409 {
		t.Fatalf("status %d, want 409 (%v)", code, body)
	}
}

func TestFileRenameRejectsPathTraversal(t *testing.T) {
	s, _ := newTestServer(t)
	code, body := agentPostJSON(t, s, "/api/file/rename", map[string]any{
		"path": "greet.go", "newPath": "../outside.go",
	})
	if code != 400 {
		t.Fatalf("status %d, want 400 (%v)", code, body)
	}
}

func TestHighlightTokenisesPostedContent(t *testing.T) {
	s, _ := newTestServer(t)
	code, body := agentPostJSON(t, s, "/api/highlight", map[string]any{
		"path":    "scratch.go",
		"content": "package main\n\nfunc main() {}\n",
	})
	if code != 200 {
		t.Fatalf("status %d (%v)", code, body)
	}
	// newDoc splits on "\n", so a trailing newline yields a final empty line,
	// same as every other path through Doc (Total counts that entry too).
	lines, ok := body["lines"].([]any)
	if !ok || len(lines) != 4 {
		t.Fatalf("lines = %v, want 4 entries", body["lines"])
	}
	if body["lang"] != "Go" {
		t.Errorf("lang = %v, want Go", body["lang"])
	}
	joined := ""
	for _, l := range lines {
		joined += l.(string)
	}
	if !strings.Contains(joined, `<i class=`) {
		t.Errorf("expected highlighted token markup in %v", lines)
	}
}

func TestHighlightNeverTouchesDisk(t *testing.T) {
	s, root := newTestServer(t)
	code, _ := agentPostJSON(t, s, "/api/highlight", map[string]any{
		"path":    "main.go",
		"content": "package main\n\n// not on disk\n",
	})
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	got, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "not on disk") {
		t.Fatalf("highlight endpoint wrote to disk: %q", got)
	}
}

func TestHighlightRejectsOversizedContent(t *testing.T) {
	s, _ := newTestServer(t)
	big := make([]byte, maxLiveHighlightBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	code, body := agentPostJSON(t, s, "/api/highlight", map[string]any{
		"path": "main.go", "content": string(big),
	})
	if code != 413 {
		t.Fatalf("status %d, want 413 (%v)", code, body)
	}
}

func TestHighlightRequiresLocalPost(t *testing.T) {
	s, _ := newTestServer(t)
	code, body := get(t, s, "/api/highlight?path=main.go")
	if code != 405 {
		t.Fatalf("status %d, want 405 (%v)", code, body)
	}
}
