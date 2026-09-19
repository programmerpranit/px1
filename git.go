package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// gitDisabled turns off all git awareness (the -no-git flag). Like uiQuiet, a
// process-wide switch set once in main before anything reads it.
var gitDisabled bool

type gitInfo struct {
	ok       bool
	toplevel string // repo root as git reports it (symlinks resolved)
}

var (
	gitMu    sync.Mutex
	gitCache = map[string]gitInfo{}
)

// gitAvailable reports whether the git binary is on PATH and root sits inside a
// working tree. Memoized per root: detection shells out once. Fails quiet -- no
// git, no repo, or -no-git all yield false, never an error.
func gitAvailable(root string) bool { return gitProbe(root).ok }

func gitProbe(root string) gitInfo {
	if gitDisabled {
		return gitInfo{}
	}
	gitMu.Lock()
	defer gitMu.Unlock()
	if info, ok := gitCache[root]; ok {
		return info
	}
	var info gitInfo
	if _, err := exec.LookPath("git"); err == nil {
		if out, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output(); err == nil {
			info = gitInfo{ok: true, toplevel: strings.TrimSpace(string(out))}
		}
	}
	gitCache[root] = info
	return info
}

// gitStatus maps repo-relative-to-served-root path -> single-letter status for
// every file git considers changed. Uses porcelain v2 -z, the stable
// null-delimited format. Fails quiet: nil on any error, no repo, or disabled.
func gitStatus(root string) map[string]string {
	info := gitProbe(root)
	if !info.ok {
		return nil
	}
	out, err := exec.Command("git", "-C", root, "status", "--porcelain=v2", "-z", "-uall").Output()
	if err != nil {
		return nil
	}
	// Porcelain paths are relative to the repo root regardless of -C, so strip
	// the served root's offset within the repo to match the index's keys.
	prefix := ""
	if rel, err := filepath.Rel(info.toplevel, root); err == nil && rel != "." {
		prefix = filepath.ToSlash(rel) + "/"
	}
	key := func(p string) (string, bool) {
		if prefix == "" {
			return p, true
		}
		if !strings.HasPrefix(p, prefix) {
			return "", false // outside the served subtree
		}
		return p[len(prefix):], true
	}

	status := map[string]string{}
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "" {
			continue
		}
		switch f[0] {
		case '?': // "? <path>"
			if k, ok := key(f[2:]); ok {
				status[k] = "U" // untracked
			}
		case '1': // "1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>"
			p := strings.SplitN(f, " ", 9)
			if len(p) == 9 {
				if k, ok := key(p[8]); ok {
					status[k] = mapXY(p[1])
				}
			}
		case '2': // "2 <XY> ... <Rscore> <path>", then original path in the next field
			p := strings.SplitN(f, " ", 10)
			if len(p) == 10 {
				if k, ok := key(p[9]); ok {
					status[k] = mapXY(p[1])
				}
			}
			i++ // the original path follows as its own NUL-terminated field
		case 'u': // "u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>"
			p := strings.SplitN(f, " ", 11)
			if len(p) == 11 {
				if k, ok := key(p[10]); ok {
					status[k] = "!" // unmerged / conflict
				}
			}
		}
	}
	if len(status) == 0 {
		return nil
	}
	return status
}

// mapXY collapses a porcelain v2 two-letter XY code (X=index, Y=worktree) into
// a single status letter, preferring the staged side when both are set.
func mapXY(xy string) string {
	if len(xy) < 2 {
		return "M"
	}
	c := xy[0]
	if c == '.' {
		c = xy[1]
	}
	switch c {
	case 'A':
		return "A"
	case 'D':
		return "D"
	case 'R':
		return "R"
	case 'C':
		return "C"
	case 'U':
		return "!" // unmerged / conflict
	default: // M (modified), T (typechange) and anything else read as modified
		return "M"
	}
}

// gitDiff returns the unified diff of relpath against HEAD. relpath is relative
// to the served root; git resolves it against -C root. Fails quiet -> "".
func gitDiff(root, relpath string) string {
	if !gitAvailable(root) {
		return ""
	}
	out, err := exec.Command("git", "-C", root, "diff", "--no-color", "HEAD", "--", relpath).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// GitFileStatus is one changed file's index (staged) and worktree (unstaged)
// status, kept separate so the UI can render VS Code-style "Staged Changes"
// and "Changes" groups, with a file appearing in both at once.
type GitFileStatus struct {
	Path     string `json:"path"`
	Staged   string `json:"staged"`   // "" none, else A/M/D/R/C/!
	Unstaged string `json:"unstaged"` // "" none, else M/D/U(untracked)/!
}

// xyLetter maps one side (index or worktree) of a porcelain v2 XY code.
func xyLetter(c byte) string {
	switch c {
	case '.':
		return ""
	case 'A':
		return "A"
	case 'D':
		return "D"
	case 'R':
		return "R"
	case 'C':
		return "C"
	case 'U':
		return "!"
	default: // M (modified), T (typechange) and anything else read as modified
		return "M"
	}
}

// gitStatusXY lists every changed file with its staged and unstaged status
// kept separate, for the stage/unstage panel. Fails quiet: nil on any error,
// no repo, or disabled.
func gitStatusXY(root string) []GitFileStatus {
	info := gitProbe(root)
	if !info.ok {
		return nil
	}
	out, err := exec.Command("git", "-C", root, "status", "--porcelain=v2", "-z", "-uall").Output()
	if err != nil {
		return nil
	}
	prefix := ""
	if rel, err := filepath.Rel(info.toplevel, root); err == nil && rel != "." {
		prefix = filepath.ToSlash(rel) + "/"
	}
	key := func(p string) (string, bool) {
		if prefix == "" {
			return p, true
		}
		if !strings.HasPrefix(p, prefix) {
			return "", false
		}
		return p[len(prefix):], true
	}

	var out2 []GitFileStatus
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "" {
			continue
		}
		switch f[0] {
		case '?':
			if k, ok := key(f[2:]); ok {
				out2 = append(out2, GitFileStatus{Path: k, Unstaged: "U"})
			}
		case '1':
			p := strings.SplitN(f, " ", 9)
			if len(p) == 9 {
				if k, ok := key(p[8]); ok {
					out2 = append(out2, GitFileStatus{Path: k, Staged: xyLetter(p[1][0]), Unstaged: xyLetter(p[1][1])})
				}
			}
		case '2':
			p := strings.SplitN(f, " ", 10)
			if len(p) == 10 {
				if k, ok := key(p[9]); ok {
					out2 = append(out2, GitFileStatus{Path: k, Staged: xyLetter(p[1][0]), Unstaged: xyLetter(p[1][1])})
				}
			}
			i++ // original path follows as its own field
		case 'u':
			p := strings.SplitN(f, " ", 11)
			if len(p) == 11 {
				if k, ok := key(p[10]); ok {
					out2 = append(out2, GitFileStatus{Path: k, Staged: "!", Unstaged: "!"})
				}
			}
		}
	}
	return out2
}

// gitDiffCached returns the unified diff of relpath staged in the index
// against HEAD. Fails quiet -> "".
func gitDiffCached(root, relpath string) string {
	if !gitAvailable(root) {
		return ""
	}
	out, err := exec.Command("git", "-C", root, "diff", "--cached", "--no-color", "--", relpath).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// gitDiffCachedAll returns the unified diff of everything staged in the
// index against HEAD. Fails quiet -> "".
func gitDiffCachedAll(root string) string {
	if !gitAvailable(root) {
		return ""
	}
	out, err := exec.Command("git", "-C", root, "diff", "--cached", "--no-color").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// gitDiffUnstaged returns the unified diff of relpath's worktree contents
// against the index. Fails quiet -> "".
func gitDiffUnstaged(root, relpath string) string {
	if !gitAvailable(root) {
		return ""
	}
	out, err := exec.Command("git", "-C", root, "diff", "--no-color", "--", relpath).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// gitHasHEAD reports whether the repo has at least one commit.
func gitHasHEAD(root string) bool {
	return exec.Command("git", "-C", root, "rev-parse", "--verify", "-q", "HEAD").Run() == nil
}

// gitStage runs `git add` on relpath, staging its current worktree content.
func gitStage(root, relpath string) error {
	out, err := exec.Command("git", "-C", root, "add", "--", relpath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gitUnstage removes relpath from the index without touching the worktree.
// A repo with no commits yet has no HEAD to reset against, so a newly added
// file is un-staged with `git rm --cached` instead.
func gitUnstage(root, relpath string) error {
	var cmd *exec.Cmd
	if gitHasHEAD(root) {
		cmd = exec.Command("git", "-C", root, "reset", "-q", "HEAD", "--", relpath)
	} else {
		cmd = exec.Command("git", "-C", root, "rm", "--cached", "-q", "--", relpath)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gitDiscard reverts relpath's worktree content, throwing away unstaged
// changes. An untracked file has no index/HEAD version to revert to, so it
// is deleted outright; anything else is restored from the index. Status is
// checked server-side rather than trusted from the caller, since this is
// destructive.
func gitDiscard(root, relpath string) error {
	for _, f := range gitStatusXY(root) {
		if f.Path != relpath {
			continue
		}
		if f.Unstaged == "U" {
			if err := os.Remove(filepath.Join(root, relpath)); err != nil {
				return err
			}
			return nil
		}
		break
	}
	out, err := exec.Command("git", "-C", root, "checkout", "--", relpath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gitStageAll stages every path in one `git add`. Bulk actions must run as a
// single git invocation, not one process per file: `git add`/`reset`/`rm`
// each take the index lock, so N concurrent per-file calls (as the SCM
// panel's Stage All used to issue) mostly fail on "Unable to create
// '.git/index.lock': File exists" -- git does not retry or queue for it.
func gitStageAll(root string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"-C", root, "add", "--"}, paths...)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gitUnstageAll un-stages every path in one command; see gitUnstage for the
// no-HEAD fallback, which applies repo-wide so one branch covers the batch.
func gitUnstageAll(root string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	var args []string
	if gitHasHEAD(root) {
		args = append([]string{"-C", root, "reset", "-q", "HEAD", "--"}, paths...)
	} else {
		args = append([]string{"-C", root, "rm", "--cached", "-q", "--"}, paths...)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gitDiscardAll reverts every path's worktree content in at most two
// commands: untracked paths are deleted (batched), everything else is
// restored from the index via one `git checkout`.
func gitDiscardAll(root string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	untracked := map[string]bool{}
	for _, f := range gitStatusXY(root) {
		if f.Unstaged == "U" {
			untracked[f.Path] = true
		}
	}
	var toDelete, toCheckout []string
	for _, p := range paths {
		if untracked[p] {
			toDelete = append(toDelete, p)
		} else {
			toCheckout = append(toCheckout, p)
		}
	}
	for _, p := range toDelete {
		if err := os.Remove(filepath.Join(root, p)); err != nil {
			return err
		}
	}
	if len(toCheckout) > 0 {
		args := append([]string{"-C", root, "checkout", "--"}, toCheckout...)
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// gitCommit commits the current index with message. Errors (e.g. unset
// user.name/user.email, or nothing staged) are returned verbatim from git so
// the UI can show them.
func gitCommit(root, message string) error {
	out, err := exec.Command("git", "-C", root, "commit", "-m", message).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gitHunks parses the unified diff of relpath against HEAD into 1-based
// NEW-FILE line numbers for a change gutter: added lines, modified (replaced)
// lines, and one marker per pure-deletion run (the new-file line immediately
// preceding the removed run; 0 means "before the first line"). Fails quiet:
// empty when git is off/unavailable or the file has no diff (clean/untracked).
func gitHunks(root, relpath string) (added, modified, deleted []int) {
	diff := gitDiff(root, relpath)
	if diff == "" {
		return nil, nil, nil
	}
	newLine := 0
	inHunk := false
	// Current block: a maximal run of consecutive '+'/'-' lines.
	dels := 0
	var adds []int
	blockStart := 0 // newLine when the block began (for deletion markers)
	flush := func() {
		switch {
		case dels > 0 && len(adds) > 0:
			modified = append(modified, adds...) // replacement
		case len(adds) > 0:
			added = append(added, adds...) // pure insertion
		case dels > 0:
			deleted = append(deleted, blockStart-1) // pure deletion
		}
		dels, adds = 0, nil
	}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			flush()
			inHunk = true
			newLine = parseNewStart(line)
		case !inHunk, strings.HasPrefix(line, "\\"): // pre-hunk header / "\ No newline"
			// skip: neither +/- nor a new-file line
		case strings.HasPrefix(line, "+"):
			if dels == 0 && len(adds) == 0 {
				blockStart = newLine
			}
			adds = append(adds, newLine)
			newLine++
		case strings.HasPrefix(line, "-"):
			if dels == 0 && len(adds) == 0 {
				blockStart = newLine
			}
			dels++
		default: // context line (" ...", or the trailing empty split element)
			flush()
			newLine++
		}
	}
	flush()
	return added, modified, deleted
}

// parseNewStart pulls newStart out of a hunk header "@@ -a,b +c,d @@".
func parseNewStart(hdr string) int {
	i := strings.IndexByte(hdr, '+')
	if i < 0 {
		return 1
	}
	rest := hdr[i+1:]
	if end := strings.IndexAny(rest, ", "); end >= 0 {
		rest = rest[:end]
	}
	if n, err := strconv.Atoi(rest); err == nil {
		return n
	}
	return 1
}
