package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// maxEditBytes bounds what edit mode / save will accept. px1 is a viewer with a
// small direct-edit escape hatch, not a general text editor, so this stays well
// under anything that would make a textarea unpleasant to use.
const maxEditBytes = 2 << 20 // 2MB

// writeSafePath is safePath plus a symlink-escape check. safePath is a purely
// lexical guard (Clean + ".."-rejection + prefix check), which is enough for a
// read-only handler but not for one that writes: a symlink inside the workspace
// pointing outside root passes the lexical check and would let a write land
// anywhere on disk. The target itself may not exist yet (e.g. a rename
// destination, or a save's temp file whose directory is the target's own
// parent), so this walks up to the nearest ancestor that does exist -- the
// only place a symlink could actually be planted -- and verifies that.
func (s *Server) writeSafePath(rel string) (abs, out string, ok bool) {
	abs, out, ok = s.safePath(rel)
	if !ok {
		return "", "", false
	}
	root, err := filepath.EvalSymlinks(s.ix.Root())
	if err != nil {
		return "", "", false
	}
	anchor := abs
	for {
		if _, err := os.Lstat(anchor); err == nil {
			break
		}
		parent := filepath.Dir(anchor)
		if parent == anchor {
			return "", "", false
		}
		anchor = parent
	}
	resolved, err := filepath.EvalSymlinks(anchor)
	if err != nil {
		return "", "", false
	}
	if resolved != root && !hasPathPrefix(resolved, root) {
		return "", "", false
	}
	return abs, out, true
}

func hasPathPrefix(p, root string) bool {
	return p == root || len(p) > len(root) && p[len(root)] == filepath.Separator && p[:len(root)] == root
}

func validEditableContent(b []byte) bool {
	if len(b) > maxEditBytes {
		return false
	}
	if !utf8.Valid(b) {
		return false
	}
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	return true
}

type fileSaveReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mtime   int64  `json:"mtime"`
	Size    int64  `json:"size"`
}

// handleFileSave writes an edit back to disk. Deliberately narrow: a size cap,
// a text-only check, an optimistic-concurrency check against the file's own
// mtime/size, and an atomic write.
func (s *Server) handleFileSave(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxEditBytes+1<<10))
	if err != nil {
		fail(w, 400, "failed to read body")
		return
	}
	var req fileSaveReq
	if err := json.Unmarshal(body, &req); err != nil {
		fail(w, 400, "invalid JSON: "+err.Error())
		return
	}
	abs, rel, ok := s.writeSafePath(req.Path)
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	content := []byte(req.Content)
	if !validEditableContent(content) {
		fail(w, 413, "file is too large or not plain text to edit")
		return
	}
	st, err := os.Stat(abs)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	if st.Mode().IsRegular() && (st.ModTime().UnixMilli() != req.Mtime || st.Size() != req.Size) {
		current, readErr := os.ReadFile(abs)
		if readErr != nil {
			fail(w, 500, readErr.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]any{
			"error":   "file changed on disk since it was opened",
			"content": string(current),
			"mtime":   st.ModTime().UnixMilli(),
			"size":    st.Size(),
		})
		return
	}

	dir := filepath.Dir(abs)
	tmp, err := os.CreateTemp(dir, ".px1-save-*")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	tmpPath := tmp.Name()
	_, werr := tmp.Write(content)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmpPath)
		fail(w, 500, "failed to write file")
		return
	}
	if err := os.Chmod(tmpPath, st.Mode().Perm()); err != nil {
		os.Remove(tmpPath)
		fail(w, 500, err.Error())
		return
	}
	if err := os.Rename(tmpPath, abs); err != nil {
		os.Remove(tmpPath)
		fail(w, 500, err.Error())
		return
	}

	Evict(abs)

	newSt, err := os.Stat(abs)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"ok": true, "path": rel,
		"mtime": newSt.ModTime().UnixMilli(), "size": newSt.Size(),
	})
}

// maxLiveHighlightBytes bounds what the debounced live-typing highlighter will
// tokenise per request. Chroma runs well under 1MB/s (see highlight.go), and
// this runs on every pause in typing, not once per file open like /api/file.
const maxLiveHighlightBytes = 256 << 10

type highlightReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// handleHighlight re-tokenises an in-memory buffer for the direct-edit view.
// It never touches disk: newDoc/tokenise work purely off the posted content,
// with path used only to pick a lexer by extension. Deliberately calls
// tokenise directly rather than Doc.Lines, which would kick off a cached
// background full-file pass (highlight.go's bgOnce) for a throwaway document
// that is never read again.
func (s *Server) handleHighlight(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxLiveHighlightBytes+1<<10))
	if err != nil {
		fail(w, 400, "failed to read body")
		return
	}
	var req highlightReq
	if err := json.Unmarshal(body, &req); err != nil {
		fail(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Content) > maxLiveHighlightBytes {
		fail(w, 413, "buffer too large to live-highlight")
		return
	}
	doc := newDoc(req.Content, req.Path)
	lines := doc.tokenise(req.Content, doc.Total)
	writeJSON(w, map[string]any{
		"lines": lines, "total": doc.Total, "maxCols": doc.MaxCols, "lang": doc.Lang,
	})
}

type fileRenameReq struct {
	Path    string `json:"path"`
	NewPath string `json:"newPath"`
}

// handleFileRename moves/renames a file within the workspace. It refuses to
// clobber an existing file at the destination (no force flag: this is a small,
// low-risk escape hatch, not a full file manager).
func (s *Server) handleFileRename(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		fail(w, 400, "failed to read body")
		return
	}
	var req fileRenameReq
	if err := json.Unmarshal(body, &req); err != nil {
		fail(w, 400, "invalid JSON: "+err.Error())
		return
	}
	srcAbs, srcRel, ok := s.writeSafePath(req.Path)
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	dstAbs, dstRel, ok := s.writeSafePath(req.NewPath)
	if !ok {
		fail(w, 400, "bad newPath")
		return
	}
	if _, err := os.Stat(dstAbs); err == nil {
		fail(w, 409, "a file already exists at the destination")
		return
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		fail(w, 500, err.Error())
		return
	}
	Evict(srcAbs)
	writeJSON(w, map[string]any{"ok": true, "path": srcRel, "newPath": dstRel})
}
