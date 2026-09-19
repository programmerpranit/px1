# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Background

px1 is a fork of [px0](https://github.com/px0-ai/px0) by Arpit Bhayani, stripped from a full code-navigator (LSP, AI pair-editing agent, markdown preview, telemetry, self-update, settings, etc.) down to 7 features: directory viewing, file navigation, live read/write editing, git stage/unstage, commit-message generation, syntax highlighting, code search. [`context.md`](context.md) has a log of that strip-down and the bug fixes that followed it. The plan that drove the strip-down is at `/Users/pranit/.claude/plans/eventual-snacking-puppy.md`, for historical reference.

**Do not `git commit` in this repo without the user's explicit go-ahead for that specific commit.**

## Commands

```bash
make build          # bundle web assets + build ./px1 for current platform
make web            # bundle web/src/*.js + web/style.css + themes into web/app.js (node ./scripts/build-web.js)
make test           # bundle web assets, then go test -v ./...
make dist           # cross-platform binaries into dist/ (./build.sh)
make clean          # remove px1 binary and dist/
```

Single test: `go test -run TestName ./...` (or `-run TestName -v` for verbose). Most test files are colocated per source file (e.g. `git_test.go` next to `git.go`).

Run locally without installing: `go run . [file-or-dir]` — add `-no-open` to skip launching a browser, `-port N` to pick a port, `-d` to run detached in the background (frees the shell; see below), `-dev web` to serve `web/` from disk instead of the embedded copy (required when iterating on `web/src/*.js` without rebuilding the bundle each time).

**Background mode (`-d`)**: `daemon.go`/`daemon_unix.go`/`daemon_windows.go` re-exec the binary as a session-detached child (stdout/stderr to a temp log file), wait for it to report the URL it bound via a temp status file, print that, and exit — the invoking shell gets control back immediately instead of blocking for the life of the server. Each instance still walks forward from its requested port if busy (`listen` in `main.go`), so running `-d` against several workspaces just stacks onto the next free port.

There is no separate lint step; `go vet ./...` and `gofmt` are the baseline. No npm/node runtime dependency — `node` is only needed at build time to run `scripts/build-web.js`.

## Architecture

**Single static Go binary.** `go:embed web` embeds the entire `web/` directory (HTML/CSS/bundled JS) into the binary; `server.go` serves it and a JSON API. No database, no local cache/config files on the user's machine — everything is in-memory for the life of the process. `main.go` is the CLI entrypoint: resolves the target file/dir into a workspace root (walking up to a git repo root when applicable), starts the index build asynchronously, binds a listener (walking forward through ports if busy), and opens a browser.

**Frontend build pipeline**: `web/src/*.js` are ES module sources — never edit `web/app.js` directly, it's generated. `scripts/build-web.js` bundles the modules into `web/app.js` only; run `make web` (or `node ./scripts/build-web.js`) after any `web/src/` change, or use `-dev web` while iterating. `web/style.css` is a plain static source file (edit it directly) served as-is at `/static/style.css`; theme files (`web/themes/*.css`) are joined into a separate `/static/themes.css` at request time by `handleThemes` in `server.go`, not at build time and not into `style.css`.

**One narrow write path.** `handleFileSave`/`handleFileRename` in `fileops.go` are the only two places px1 touches disk, gated by `localPost` (same-origin POST only, in `server.go`) and an optimistic-concurrency check against the file's mtime/size (a stale save is refused with a 409 and the current on-disk content, surfaced in the UI as a conflict banner). Any new disk-writing endpoint (e.g. git stage/unstage/commit in `git.go`, commit-message generation in `commitmsg.go`) follows the same `localPost` gating pattern.

**Backend module map** (each is a distinct concern, minimal cross-coupling):
- `index.go` / `ignore.go` — workspace file tree walk and gitignore-aware filtering, built asynchronously on startup with bounded concurrency (`NumCPU * 4`).
- `fuzzy.go` — fuzzy file matching for quick-open.
- `search.go` — workspace text/regex search.
- `highlight.go` — syntax highlighting via Chroma, windowed in chunks (`hlChunk = 1000`) so opening a huge file costs the same as a small one.
- `git.go` — git status (staged/unstaged split via porcelain v2 `XY` codes), diff (staged vs. unstaged variants), stage/unstage/commit. Unstaging in a repo with no commits yet falls back to `git rm --cached` (no HEAD to reset against).
- `commitmsg.go` — commit-message generation; shells out to the `claude` CLI on staged changes if present on `PATH`, fails quietly otherwise. This is the one intentional runtime-dependency exception to the otherwise-zero-runtime-dependency design.
- `fileops.go` — file read/save/rename (the narrow write path above).
- `server.go` — HTTP routes (`/api/*`) and static asset serving; `localPost` CSRF-style guard for mutating endpoints.

**Frontend module map** (`web/src/`, bundled by `scripts/build-web.js`):
- `state.js` — core state object `S`, `api()`/`apiPost()` fetch helpers, DOM shortcuts (`$`, `$$`), per-OS keybinding labels.
- `ui.js` — shared DOM refs, toast notifications, clipboard.
- `renderer.js` — virtualized editor rendering (`measure()`, `layout()`, `render()`, `paint()`); only visible rows are mounted, so a 400k-line file costs the same as a 10-line file.
- `tabs.js` — open-file tabs, breadcrumbs, image tab display.
- `cursor.js` — caret movement, click/double-click selection, occurrence highlighting.
- `edit.js` — in-place editing (click into an open file and type — no separate edit mode), save (`Mod+S`), the on-disk-conflict banner.
- `diff.js` — diff view against `HEAD` (`Mod+D`), staged/unstaged.
- `scm.js` — Source Control panel: staged/unstaged file groups, stage/unstage buttons, commit message box, Generate/Commit actions.
- `tree.js` — file explorer tree.
- `search.js` / `find.js` — workspace search panel and in-file find (`Mod+F`).
- `palette.js` — command palette / quick-open (`Mod+K`, `Mod+P`).
- `panels.js` — sidebar tab switching (Files / Source Control), resizer.
- `shortcuts.js` — keyboard shortcut dispatch, help sheet (`?`).
- `theme.js` — theme discovery/persistence (themes are CSS files in `web/themes/`, joined into `web/style.css` at build time).
- `status.js` — bottom status bar.
- `selbar.js` — selection-mode status bar actions (copy ref, copy with context).
- `history.js` — jump-history navigation (Alt+Left/Right).
- `main.js` — module init and `boot()` sequence.

## Performance and design constraints

- Any change must keep px1 a single static binary (`go build` only) — no new runtime dependencies (no Node/npm at runtime, no external DB, no CGO), aside from the `claude` CLI shell-out in `commitmsg.go`, which must keep failing quietly when the CLI isn't present.
- Zero on-disk state beyond the workspace itself: no `.px1/` config or cache directories. Indexes/caches are in-memory only, for the life of the process.
- Indexing and search have explicit performance budgets (millisecond-scale on typical repos) — see `BENCHMARKS.md` and `benchmark.sh` if touching `index.go`, `search.go`, `fuzzy.go`, or `highlight.go`.
- Security: reachable-by-IP (e.g. over Tailscale) allows save/stage/commit; reachable only by hostname (reverse proxy/tunnel domain) refuses writes. Don't weaken this check in `server.go` without understanding why it's there.
