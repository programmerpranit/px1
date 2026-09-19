# Operational Guidelines for AI Agents

This document defines critical instructions, architectural principles, and documentation maintenance workflows for AI coding agents working on px1.

## 1. Core Architectural Tenets

1. Direct, Narrow Editing: px1 writes files itself. `handleFileSave`/`handleFileRename` in [`fileops.go`](../../fileops.go) are the only two places px1 touches disk, gated by `localPost` (same-origin POST only) and an optimistic-concurrency check against the file's mtime/size. Do not widen this into a general file-management API.
1. Zero Runtime and Single Binary Footprint: Any change must compile into a single static binary (`go:embed` for web assets). Do not introduce runtime dependencies (no Node.js/npm runtime requirement, no external database, no CGO dependencies). The one exception is the commit-message generator, which shells out to the `claude` CLI if present on `PATH` and fails quietly otherwise.
1. Stateless on Disk: px1 leaves zero configuration or temporary cache artifacts on the user filesystem (no local `.px1/` folders or cache files). Indexes and caches live in memory for the life of the process.
1. Performance Budgets: Indexing must complete in milliseconds using bounded concurrency (`NumCPU * 4`). File open must remain $O(1)$ relative to file length using windowed chunking (`hlChunk = 1000`) and browser DOM virtualization. Maintain explicit memory reclamation (`debug.FreeOSMemory()` on idle).

## 2. Mandatory Documentation Maintenance Protocol

Whenever modifying, adding, or refactoring code in this repository, you must audit and update the documentation accordingly:

### Documentation Mapping Matrix

| Component Modified                   | Primary Source Files                                             | Docs to Update                                                                                                   |
| ------------------------------------ | ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| System Architecture / Optimizations  | All `.go` files, `web/app.js`                                    | [`docs/internals/architecture.md`](../internals/architecture.md)                                                 |
| Indexing / Tree Walk / Gitignore     | `index.go`, `ignore.go`                                          | [`docs/internals/indexing-and-ignore.md`](../internals/indexing-and-ignore.md), [`README.md`](../../README.md)    |
| Fuzzy File Finder                    | `fuzzy.go`                                                       | [`docs/internals/fuzzy-search.md`](../internals/fuzzy-search.md)                                                 |
| Search / Regex                       | `search.go`                                                       | [`docs/internals/workspace-search.md`](../internals/workspace-search.md), [`BENCHMARKS.md`](../../BENCHMARKS.md) |
| Syntax Highlighting & Lexing         | `highlight.go`                                                   | [`docs/internals/syntax-highlighting.md`](../internals/syntax-highlighting.md), [`README.md`](../../README.md)   |
| Git Integration & Diffing / Staging  | `git.go`, `commitmsg.go`, `web/src/scm.js`                       | [`docs/internals/git-integration.md`](../internals/git-integration.md)                                           |
| Direct File Editing                  | `fileops.go`, `web/src/edit.js`                                  | [`README.md`](../../README.md)                                                                                   |
| Frontend UI / Virtualization         | `web/app.js`, `web/index.html`, `web/style.css`                  | [`docs/internals/editor-virtualization.md`](../internals/editor-virtualization.md), [`README.md`](../../README.md)|
| Themes / Colour Tokens               | `web/themes/*.css`, `web/style.css`, `web/src/theme.js`          | [`docs/internals/styling-and-themes.md`](../internals/styling-and-themes.md)                                    |
| User-Facing Features / Workflows    | All features, UX, and controls                                   | [`docs/features/README.md`](../features/README.md)                                                               |
| CLI Flags                            | `main.go`                                                         | [`README.md`](../../README.md)                                                                                   |
| Performance Metrics / Scripts        | `benchmark.sh`                                                   | [`BENCHMARKS.md`](../../BENCHMARKS.md)                                                                           |
| Release Workflow                     | `Makefile`, `build.sh`, `scripts/build-web.js`                   | [`PUBLISHING.md`](../../PUBLISHING.md)                                                                           |

## 3. Checklist for Agents Prior to Submitting Work

- Verification: Ran `go test ./...` and confirmed all unit/regression tests pass (`ok px1`).
- Build Integrity: Verified successful build with `go build -o px1 .`.
- Web Bundling: If modifying `web/src/`, verified bundle update with `./scripts/build-web.js`.
- Architecture Sync: Any new optimization, algorithmic adjustment, or structural change is documented in the corresponding [`docs/internals/`](../internals/README.md) write-up.
- Flag & Shortcut Sync: Any new keyboard shortcut, UI behavior, or CLI flag is reflected in [`README.md`](../../README.md).
- Benchmark Alignment: If search, highlight, or index performance characteristics change, verify whether [`BENCHMARKS.md`](../../BENCHMARKS.md) requires updated notes or numbers.

## 4. Frontend Architecture & Code Map for Agents

To quickly locate and modify UI features, refer to this structured section index of [`web/index.html`](../../web/index.html) and `web/app.js`:

### HTML Structure

| Section / Element ID          | Description                                                                                                                                                                                                                 |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `<aside id="side">`             | Sidebar with two tabs: `#side-tab-files` (`#tree`, the file explorer) and `#side-tab-scm` (`#scm`, the Source Control panel: `#scm-message`, `#scm-generate`, `#scm-commit-btn`, `#scm-groups`). `.panel-head` shows the workspace name (`#root-name`), the changed-files filter (`#btn-changed`), and re-index (`#btn-reindex`). `.side-foot` holds the px1 wordmark, version (`#st-ver`), GitHub link, and theme button (`#btn-theme`). |
| `<div id="resizer">`            | Draggable splitter between sidebar and main editor viewport.                                                                                                                                                              |
| `<div id="tabs">`               | Open file tabs bar.                                                                                                                                                                                                       |
| `<div id="view-switches">`      | Holds `#diff-switch` (`#diff-source` / `#diff-btn`), shown only when the active file has a diff available, toggling between source and diff view (`Cmd/Ctrl+D`).                                                        |
| `<div id="editor">`             | Core editor container: `#viewport`/`#sizer`/`#rows` (virtualized rows) and `#caret`; `#edit-banner` (save-conflict banner); `#diffview`/`#diffcontent` (diff overlay); `#imgview` (image tab viewer); `#minimap-hits`. |
| `<div id="empty">`              | Welcome / splash screen shown when no files are open.                                                                                                                                                                     |
| `<div id="findbar">`            | In-file search overlay (`Cmd/Ctrl+F`): `#find-input`, `#find-count`, `#find-prev`/`#find-next`.                                                                                                                           |
| `<div id="sel-menu">`           | Right-click menu on a selection: copy reference / copy with context.                                                                                                                                                      |
| `<div id="tree-menu">`          | Right-click menu on a file-tree row.                                                                                                                                                                                      |
| `<div id="toast">`              | Floating bottom notification toast confirming actions.                                                                                                                                                                    |
| `<footer id="status">`          | Bottom status bar: language, lines, size, cursor pos. While code is selected, `#footer-sel` replaces the left-side buttons and `#sel-stats` shows selection stats. Never wraps: `fitStatus()` in `status.js` adds cumulative `fit-1`..`fit-6` classes to hide detail as width runs out. |
| `<div id="right-resizer">` + `<aside id="right-side">` | Right-side panel hosting workspace search (`#pane-right-search`: `#q`, `#o-case`/`#o-word`/`#o-re`, `#glob`, `#results`), toggled by `Cmd/Ctrl+Shift+F`.                                                |
| `<div id="overlay">`            | Modal overlay hosting Quick Open / Command Palette (`#palette`: `#pal`, `#pal-list`, `#pal-hint`).                                                                                                                        |
| `<div id="helpsheet">`          | Keyboard shortcuts cheat-sheet modal (`?`).                                                                                                                                                                               |

### Frontend Modules (`web/src/`)

The frontend is modularized into ES modules under `web/src/` and bundled into `web/app.js` by `scripts/build-web.js`. This is the full module list — there is no LSP, markdown, agent-harness, telemetry, settings, or outline module; do not reintroduce imports of files by those names.

| Module                 | Primary Responsibilities & Key Exports                                                                                                                                                       |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `web/src/state.js`     | Core state object `S`, `doc_()`, `api()`/`apiPost()`/`apiPostJson()`, `esc()`. Per-OS shortcut labels: `keyLabel()`, `keyCaps()`, `withKeys()`, `applyKeyLabels()`.                          |
| `web/src/ui.js`        | Shared DOM refs (`vp`, `sizer`, `rowsEl`, `editor`, `toastEl`), `showToast()`, `copyToClipboard()`.                                                                                          |
| `web/src/renderer.js`  | `measure()`, `layout()`, `render()`, `paint()`, `placeCaret()`, `toggleWordWrap()`, `ensureChunks()`/`refineChunk()` (on-demand highlight chunk fetching).                                    |
| `web/src/cursor.js`    | Caret movement (`moveCol`, `moveWord`, `caretToEdge`), click / double-click selection, `revealCaretX()`, occurrence highlighting.                                                            |
| `web/src/edit.js`      | In-place editing: `insertText()`, `backspace()`, `deleteForward/BackwardWord()`, `toggleLineComment()`, `undo()`/`redo()`, `saveBuffer()`, the on-disk-conflict banner.                       |
| `web/src/diff.js`      | `toggleDiff()`, `setDiffMode()`, `syncDiffView()`: the split diff view against `HEAD` (`Cmd/Ctrl+D`).                                                                                         |
| `web/src/scm.js`       | Source Control panel: `loadScmStatus()`, stage/unstage/discard (single + bulk), `Generate`/`Commit` actions.                                                                                 |
| `web/src/tabs.js`      | `openFile()`, `closeTab()`, `switchTab()`, `drawTabs()`, `drawCrumbs()`, `reloadOpenTabs()` (in-place tab refresh on reindex), `reopenClosedTab()` (Alt+Shift+T), `loadGutter()`.             |
| `web/src/tree.js`      | `drawTree()`, `fileKind()`, `revealDir()`/`revealFile()`, explorer tree click handlers.                                                                                                       |
| `web/src/search.js`    | `runSearch()`, `renderResults()`, `displayPath()`: the right-side workspace search panel.                                                                                                     |
| `web/src/find.js`      | `openFind()`, `runFind()`, `jumpToHit()`, minimap hit dots — in-file find (`Cmd/Ctrl+F`).                                                                                                     |
| `web/src/palette.js`   | `openPalette()`, `refreshPalette()`, `COMMANDS`: fuzzy file/command finder (`Cmd/Ctrl+K`, `Cmd/Ctrl+P`).                                                                                       |
| `web/src/panels.js`    | `showPanel()`, sidebar tab switching, reindex trigger, draggable sidebar resizer.                                                                                                             |
| `web/src/shortcuts.js` | `SHORTCUTS` (source of truth for the help sheet), `showHelp()`, the keydown dispatcher.                                                                                                       |
| `web/src/theme.js`     | `listThemes()`, `setTheme()`, `cycleTheme()`, `initTheme()`: theme discovery from loaded CSS and `localStorage` persistence.                                                                  |
| `web/src/status.js`    | `updateStatus()`, `setStatusNote()`, `fmtBytes()`, `fitStatus()` (progressive status-bar truncation).                                                                                         |
| `web/src/selbar.js`    | Selection-mode status bar: `updateSelectionBar()`, `runSelectionAction()`, whole-file selection (`selectAll()`), right-click menu (`closeSelMenu()`).                                        |
| `web/src/history.js`   | `pushHistory()`, `go()`: jump history back/forward (Alt+Left / Alt+Right).                                                                                                                    |
| `web/src/main.js`      | Module initialization and the application `boot()` sequence.                                                                                                                                  |
