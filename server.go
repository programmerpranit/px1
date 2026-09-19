package main

import (
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed web
var embedded embed.FS

// assets is the embedded web/ directory, or the one on disk under -dev.
var assets fs.FS = embedded

// useDiskAssets serves web/ from the filesystem so the UI can be edited without
// rebuilding. Development convenience only.
func useDiskAssets(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "web", "index.html")); err != nil {
		return err
	}
	assets = os.DirFS(dir)
	return nil
}

type Server struct {
	ix  *Index
	mux *http.ServeMux

	lastReq atomic.Int64 // unix nanos of the most recent request
}

func NewServer(ix *Index) *Server {
	s := &Server{ix: ix, mux: http.NewServeMux()}
	sub, _ := fs.Sub(assets, "web")
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	s.mux.HandleFunc("/static/themes.css", s.handleThemes)
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/meta", s.handleMeta)
	s.mux.HandleFunc("/api/tree", s.handleTree)
	s.mux.HandleFunc("/api/find", s.handleFind)
	s.mux.HandleFunc("/api/file", s.handleFile)
	s.mux.HandleFunc("/api/close", s.handleClose)
	s.mux.HandleFunc("/api/raw", s.handleRaw)
	s.mux.HandleFunc("/api/diff", s.handleDiff)
	s.mux.HandleFunc("/api/gutter", s.handleGutter)
	s.mux.HandleFunc("/api/search", s.handleSearch)
	s.mux.HandleFunc("/api/reindex", s.handleReindex)
	s.mux.HandleFunc("/api/file/save", s.handleFileSave)
	s.mux.HandleFunc("/api/file/rename", s.handleFileRename)
	s.mux.HandleFunc("/api/highlight", s.handleHighlight)
	s.mux.HandleFunc("/api/git/status", s.handleGitStatus)
	s.mux.HandleFunc("/api/git/diff", s.handleGitDiffScoped)
	s.mux.HandleFunc("/api/git/stage", s.handleGitStage)
	s.mux.HandleFunc("/api/git/unstage", s.handleGitUnstage)
	s.mux.HandleFunc("/api/git/discard", s.handleGitDiscard)
	s.mux.HandleFunc("/api/git/stage-all", s.handleGitStageAll)
	s.mux.HandleFunc("/api/git/unstage-all", s.handleGitUnstageAll)
	s.mux.HandleFunc("/api/git/discard-all", s.handleGitDiscardAll)
	s.mux.HandleFunc("/api/git/commit", s.handleGitCommit)
	s.mux.HandleFunc("/api/git/commit-message", s.handleGitCommitMessage)
	s.mux.HandleFunc("/api/git/sync-status", s.handleGitSyncStatus)
	s.mux.HandleFunc("/api/git/push", s.handleGitPush)
	s.lastReq.Store(time.Now().UnixNano())
	go s.scavenge()
	return s
}

// scavenge hands freed pages back to the OS once nobody is asking for anything.
// Reading a large tree churns through a lot of short-lived memory, and the Go
// runtime is in no hurry to return it. That is harmless but it makes a process
// that is doing nothing look like it is holding hundreds of megabytes.
func (s *Server) scavenge() {
	const idleFor = 15 * time.Second
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	done := true // nothing to release before the first request
	for range tick.C {
		idle := time.Since(time.Unix(0, s.lastReq.Load()))
		if idle < idleFor {
			done = false
			continue
		}
		if done {
			continue // already released since the last burst of work
		}
		debug.FreeOSMemory()
		done = true
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.lastReq.Store(time.Now().UnixNano())
	w.Header().Set("Cache-Control", "no-store")
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		s.mux.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(w)
	defer func() { gz.Close(); gzipPool.Put(gz) }()
	s.mux.ServeHTTP(gzipWriter{ResponseWriter: w, w: gz}, r)
}

var gzipPool = sync.Pool{New: func() any {
	w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return w
}}

type gzipWriter struct {
	http.ResponseWriter
	w *gzip.Writer
}

func (g gzipWriter) Write(b []byte) (int, error) { return g.w.Write(b) }

// safePath resolves a client-supplied relative path inside the root, refusing
// anything that escapes it.
func (s *Server) safePath(rel string) (string, string, bool) {
	rel = strings.TrimPrefix(strings.TrimSpace(rel), "/")
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." {
		return s.ix.Root(), "", true
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", "", false
	}
	abs := filepath.Join(s.ix.Root(), clean)
	if abs != s.ix.Root() && !strings.HasPrefix(abs, s.ix.Root()+string(filepath.Separator)) {
		return "", "", false
	}
	return abs, filepath.ToSlash(clean), true
}

// resolvePath resolves a client-supplied relative path inside the root.
func (s *Server) resolvePath(p string) (string, string, bool) {
	return s.safePath(p)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	// Highlighted lines are already HTML-escaped by the time they get here, so
	// the extra \u003c encoding only inflates the payload and makes the API
	// awkward to read with anything but a JSON parser.
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// localPost admits a request that changes the machine only when it is a POST
// from px1's own page. Browsers send Origin on every POST, so a page from
// another site cannot pass. Requiring the Host to be an IP address or
// localhost also shuts out DNS rebinding, where an attacker's domain is
// pointed at this machine and its Origin would otherwise match.
func localPost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "POST only")
		return false
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host != "localhost" && net.ParseIP(host) == nil {
		fail(w, http.StatusForbidden, "open px1 by IP address or localhost to change files")
		return false
	}
	if o, err := url.Parse(r.Header.Get("Origin")); err != nil || o.Host != r.Host {
		fail(w, http.StatusForbidden, "request did not come from px1")
		return false
	}
	return true
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := fs.ReadFile(assets, "web/index.html")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

// handleThemes joins web/themes/*.css into one stylesheet in file name order, so
// adding a theme means adding a file: there is no list to keep in sync.
func (s *Server) handleThemes(w http.ResponseWriter, r *http.Request) {
	names, err := fs.Glob(assets, "web/themes/*.css")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	var css strings.Builder
	for _, name := range names {
		b, err := fs.ReadFile(assets, name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		css.WriteString("/* " + strings.TrimPrefix(name, "web/") + " */\n")
		css.Write(b)
		css.WriteString("\n")
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	io.WriteString(w, css.String())
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	n, at, ms := s.ix.Stats()
	writeJSON(w, map[string]any{
		"root":    s.ix.Root(),
		"name":    filepath.Base(s.ix.Root()),
		"files":   n,
		"indexMs": ms,
		"builtAt": at,
		"ready":   s.ix.Ready(),
		"git":     gitAvailable(s.ix.Root()),
		"version": version,
	})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	dir := strings.Trim(r.URL.Query().Get("dir"), "/")
	kids, ok := s.ix.Children(dir)
	if !ok && !s.ix.Ready() {
		// If indexing is still in flight, wait up to 300ms for this directory to be scanned
		for i := 0; i < 30; i++ {
			time.Sleep(10 * time.Millisecond)
			if kids, ok = s.ix.Children(dir); ok {
				break
			}
			if s.ix.Ready() {
				kids, ok = s.ix.Children(dir)
				break
			}
		}
	}
	if !ok {
		fail(w, 404, "not indexed: "+dir)
		return
	}
	writeJSON(w, map[string]any{"dir": dir, "children": kids})
}

func (s *Server) handleFind(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	res := FuzzyFind(s.ix.Files(), q, limit)
	if res == nil {
		res = []FuzzyResult{}
	}
	if uiVerbose && q != "" {
		dur := fmtDuration(time.Since(start))
		uiStatus("info", "find", fmt.Sprintf("%q · %d files  (%s)", q, len(res), dur), 0, os.Stdout)
	}
	writeJSON(w, map[string]any{"results": res})
}

var imageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".svg": true, ".ico": true, ".bmp": true, ".avif": true,
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	abs, rel, ok := s.resolvePath(q.Get("path"))
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	st, err := os.Stat(abs)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	if imageExt[strings.ToLower(filepath.Ext(rel))] {
		if uiVerbose {
			uiStatus("info", "view", fmt.Sprintf("%s · image (%s)", rel, formatBytes(st.Size())), 0, os.Stdout)
		}
		writeJSON(w, map[string]any{"path": rel, "image": true, "size": st.Size()})
		return
	}

	d, err := Open(abs, rel)
	if err != nil {
		fail(w, 415, err.Error())
		return
	}
	start, _ := strconv.Atoi(q.Get("start"))
	count, _ := strconv.Atoi(q.Get("count"))
	if count <= 0 {
		count = hlChunk
	}
	if start < 0 {
		start = 0
	}
	if start > d.Total {
		start = d.Total
	}
	if uiVerbose && start == 0 {
		uiStatus("info", "view", fmt.Sprintf("%s · %d lines (%s)", rel, d.Total, formatBytes(st.Size())), 0, os.Stdout)
	}
	lines, exact := d.Lines(start, start+count)
	_, coming := d.Exact()
	diffAvail := false
	if gitAvailable(s.ix.Root()) {
		diffAvail = gitDiff(s.ix.Root(), rel) != ""
	}
	writeJSON(w, map[string]any{
		"path": rel, "lang": d.Lang, "total": d.Total, "maxCols": d.MaxCols,
		"start": start, "lines": lines, "size": st.Size(),
		"exact": exact, "refine": !exact && coming,
		"diffAvailable": diffAvail,
		"mtime":         st.ModTime().UnixMilli(),
		"editable":      st.Size() <= maxEditBytes,
	})
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	abs, rel, ok := s.resolvePath(q.Get("path"))
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	Evict(abs)
	debug.FreeOSMemory()
	writeJSON(w, map[string]any{"ok": true, "path": rel})
}

func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	abs, rel, ok := s.safePath(r.URL.Query().Get("path"))
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(rel)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeFile(w, r, abs)
}

// handleDiff returns the unified diff of a file against HEAD. available is false
// (with an empty diff and 200) when git is off/absent or the file is unchanged.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	_, rel, ok := s.resolvePath(r.URL.Query().Get("path"))
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	diff := gitDiff(s.ix.Root(), rel)
	if uiVerbose {
		status := "clean"
		if diff != "" {
			lines := strings.Count(diff, "\n")
			status = fmt.Sprintf("%d diff lines", lines)
		}
		uiStatus("info", "diff", fmt.Sprintf("%s · %s", rel, status), 0, os.Stdout)
	}
	writeJSON(w, map[string]any{"path": rel, "diff": diff, "available": diff != ""})
}

// handleGutter returns per-file changed-line ranges (new-file line numbers) for
// a VS Code-style change gutter. available is false (200, empty arrays) when
// git is off/absent or the file is unchanged/untracked; never 500 for those.
func (s *Server) handleGutter(w http.ResponseWriter, r *http.Request) {
	_, rel, ok := s.resolvePath(r.URL.Query().Get("path"))
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	added, modified, deleted := gitHunks(s.ix.Root(), rel)
	nz := func(v []int) []int { // marshal as [] not null
		if v == nil {
			return []int{}
		}
		return v
	}
	writeJSON(w, map[string]any{
		"path":      rel,
		"available": added != nil || modified != nil || deleted != nil,
		"added":     nz(added),
		"modified":  nz(modified),
		"deleted":   nz(deleted),
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()
	opts := SearchOpts{
		Query: q.Get("q"),
		Regex: q.Get("re") == "1",
		Case:  q.Get("case") == "1",
		Word:  q.Get("word") == "1",
		Glob:  q.Get("glob"),
	}
	res, truncated, err := SearchContext(r.Context(), s.ix, opts)
	if err != nil {
		if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
			return
		}
		fail(w, 400, err.Error())
		return
	}
	total := 0
	for _, f := range res {
		total += len(f.Matches)
	}
	if res == nil {
		res = []FileMatches{} // an empty result is [], never null
	}
	if uiVerbose && opts.Query != "" {
		dur := fmtDuration(time.Since(start))
		matchStr := "matches"
		if total == 1 {
			matchStr = "match"
		}
		fileStr := "files"
		if len(res) == 1 {
			fileStr = "file"
		}
		truncStr := ""
		if truncated {
			truncStr = " (truncated)"
		}
		uiStatus("info", "search", fmt.Sprintf("%q · %d %s in %d %s%s  (%s)", opts.Query, total, matchStr, len(res), fileStr, truncStr, dur), 0, os.Stdout)
	}
	writeJSON(w, map[string]any{"results": res, "files": len(res), "total": total, "truncated": truncated})
}

func (s *Server) handleReindex(w http.ResponseWriter, r *http.Request) {
	EvictAll()
	s.ix.Build()
	n, _, ms := s.ix.Stats()
	writeJSON(w, map[string]any{"files": n, "indexMs": ms})
}

// handleGitStatus lists every changed file with staged/unstaged status
// separated, for the source-control panel.
func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	files := gitStatusXY(s.ix.Root())
	if files == nil {
		files = []GitFileStatus{}
	}
	writeJSON(w, map[string]any{"files": files, "available": gitAvailable(s.ix.Root())})
}

// handleGitDiffScoped is separate from handleDiff (which diffs a file against
// HEAD for the gutter/diff-view) because the SCM panel needs staged and
// unstaged changes to the same file shown independently. scope=staged|worktree.
func (s *Server) handleGitDiffScoped(w http.ResponseWriter, r *http.Request) {
	_, rel, ok := s.safePath(r.URL.Query().Get("path"))
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	var diff string
	if r.URL.Query().Get("scope") == "staged" {
		diff = gitDiffCached(s.ix.Root(), rel)
	} else {
		diff = gitDiffUnstaged(s.ix.Root(), rel)
	}
	writeJSON(w, map[string]any{"path": rel, "diff": diff, "available": diff != ""})
}

func (s *Server) handleGitStage(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	_, rel, ok := s.safePath(r.URL.Query().Get("path"))
	if !ok || rel == "" {
		fail(w, 400, "bad path")
		return
	}
	if err := gitStage(s.ix.Root(), rel); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "path": rel})
}

func (s *Server) handleGitUnstage(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	_, rel, ok := s.safePath(r.URL.Query().Get("path"))
	if !ok || rel == "" {
		fail(w, 400, "bad path")
		return
	}
	if err := gitUnstage(s.ix.Root(), rel); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "path": rel})
}

func (s *Server) handleGitDiscard(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	_, rel, ok := s.safePath(r.URL.Query().Get("path"))
	if !ok || rel == "" {
		fail(w, 400, "bad path")
		return
	}
	if err := gitDiscard(s.ix.Root(), rel); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "path": rel})
}

// readGitPaths resolves a POST body of {"paths": [...]} into repo-relative
// paths, rejecting the request if any entry falls outside the served root.
func (s *Server) readGitPaths(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		fail(w, 400, "failed to read body")
		return nil, false
	}
	var payload struct {
		Paths []string `json:"paths"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			fail(w, 400, "invalid JSON: "+err.Error())
			return nil, false
		}
	}
	rels := make([]string, 0, len(payload.Paths))
	for _, p := range payload.Paths {
		_, rel, ok := s.safePath(p)
		if !ok || rel == "" {
			fail(w, 400, "bad path: "+p)
			return nil, false
		}
		rels = append(rels, rel)
	}
	return rels, true
}

func (s *Server) handleGitStageAll(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	paths, ok := s.readGitPaths(w, r)
	if !ok {
		return
	}
	if err := gitStageAll(s.ix.Root(), paths); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(paths)})
}

func (s *Server) handleGitUnstageAll(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	paths, ok := s.readGitPaths(w, r)
	if !ok {
		return
	}
	if err := gitUnstageAll(s.ix.Root(), paths); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(paths)})
}

func (s *Server) handleGitDiscardAll(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	paths, ok := s.readGitPaths(w, r)
	if !ok {
		return
	}
	if err := gitDiscardAll(s.ix.Root(), paths); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(paths)})
}

func (s *Server) handleGitCommit(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(w, 400, "failed to read body")
		return
	}
	var payload struct {
		Message string `json:"message"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			fail(w, 400, "invalid JSON: "+err.Error())
			return
		}
	}
	if strings.TrimSpace(payload.Message) == "" {
		fail(w, 400, "empty commit message")
		return
	}
	if err := gitCommit(s.ix.Root(), payload.Message); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleGitCommitMessage(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	msg, err := generateCommitMessage(ctx, s.ix.Root())
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"message": msg})
}

// handleGitSyncStatus reports how the current branch compares to its
// upstream, so the SCM panel can offer to push right after a commit.
func (s *Server) handleGitSyncStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, gitSyncStatus(s.ix.Root()))
}

func (s *Server) handleGitPush(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	if err := gitPush(s.ix.Root()); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
