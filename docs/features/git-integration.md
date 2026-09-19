# Git Awareness & Visual Diff Viewer

px1 includes built-in Git awareness, a side-by-side diff viewer, and a VS Code-style Source Control panel for staging, unstaging, and committing — without leaving the browser. It highlights working-tree modifications across your file tree and editor gutters, and lets you toggle between source code and a side-by-side diff against `HEAD` with `Cmd/Ctrl+D`.

---

## Overview & Core Purpose

In modern software engineering, coding agents, background formatters, and compilers continuously generate or modify files on disk. Developers spend a significant portion of their time verifying what changed, ensuring unintended edits were not introduced, and auditing modifications prior to staging or committing.

px1 provides non-destructive, zero-latency Git awareness. It queries Git status asynchronously in the background without touching the index or slowing down viewer startup. With visual badges, ancestor dirty propagation, gutter indicators, and a full diff view, you can review changes with complete confidence — then stage, unstage, and commit right from the same tab.

---

## Key Capabilities

- **File Tree Status Badges**: The file explorer decorates changed files with colored badges indicating their Git working-tree status:
  - `M` (Modified): Working tree file differs from `HEAD`.
  - `A` (Added / Staged): Newly added file staged in the index.
  - `D` (Deleted): File removed from the working tree.
  - `U` (Untracked): New file not yet tracked by Git.
  - `R` (Renamed): File renamed or moved.
- **Dirty Ancestor Folder Propagation**: When a nested file is modified (e.g., `src/core/auth/token.go`), all parent directories in the tree (`auth/`, `core/`, `src/`) display a subtle dirty indicator badge. This allows you to spot modifications even when folder branches are collapsed.
- **Uncommitted Changes Filter**: A dedicated toggle in the explorer header lets you collapse all clean files and view only files that currently have uncommitted changes.
- **Visual Gutter Diff Indicators**: The code viewer gutter places colored indicator bars alongside line numbers to mark edits in real time:
  - Green bar for added lines.
  - Blue bar for modified lines.
  - Red triangle or marker for deleted lines.
- **Interactive Diff Viewer (`Cmd/Ctrl+D`)**: Toggle between normal source view and full Git diff with a single keystroke.
- **Side-by-Side Diff View**: Original `HEAD` code on the left, active working-tree code on the right, deletion/addition runs paired row-by-row so both columns stay vertically aligned.
- **Source Control Panel**: A second sidebar tab (next to Files) lists every changed file split into **Staged Changes** and **Changes**, the same grouping VS Code uses.
  - Click **+** to stage a file, **−** to unstage it, or the discard icon to throw away an unstaged file's changes.
  - Per-group **Stage all** / **Unstage all** / **Discard all** buttons appear on hover, each running as a single atomic operation so a large batch never partially fails.
  - Click a file in either group to open it; open its diff with `Cmd/Ctrl+D`.
  - Write a commit message and click **Commit**, or click **Generate** to have px1 draft one from the staged diff (see below).
  - When the branch has a remote tracking branch and is ahead of it, a banner (`↑N vs origin/main`) appears with a **Push** button. Behind-only shows as text with no action — px1 doesn't pull.

---

## Developer Workflows & Practical Value

### Auditing AI Agent Edits
When an AI coding agent finishes updating a component or fixing a bug:
1. Glance at the file explorer to see which files were touched.
2. Open any modified file. The gutter immediately highlights the altered lines.
3. Press **`Cmd/Ctrl+D`** to open the diff view.
4. Review the exact additions and deletions against `HEAD`.
5. If something needs adjustment, click into the file (diff view or source) and edit it directly — `Mod+S` saves.

### Stage, Generate a Commit Message, and Commit
Once the changes look right:
1. Switch to the **Source Control** tab in the sidebar.
2. Stage the files that belong in this commit (individually, or **Stage all**).
3. Click **Generate** to have px1 shell out to the `claude` CLI on the staged diff and draft a one-line Conventional Commits message — or write your own.
4. Click **Commit**.
5. If a push banner appears (the branch has a remote and is now ahead of it), click **Push**.

### Pre-Commit Review
Before committing, open px1's Source Control panel to perform a visual walk-through of all pending changes. The changed-files filter in the file explorer isolates your work, ensuring you don't commit debug logs, temporary comments, or unintended formatting tweaks.

---

## Keyboard Shortcuts & Controls

| Shortcut | Context | Action |
| :--- | :--- | :--- |
| `Cmd/Ctrl+D` | Editor | Toggle side-by-side Git Diff View vs. `HEAD` |
| Filter Icon | File Explorer | Show Only Files with Uncommitted Changes |

---

## Configuration & Preferences

There is no settings UI — behavior is fixed by design (see [Why a Dedicated Code Viewer?](../../README.md#why-a-dedicated-code-viewer)):

- The diff view is always the split, side-by-side layout — there's no unified-view toggle and no whitespace-ignore toggle.
- The gutter change bars are always on when a file has a git diff available.
- **CLI Flag `-no-git`**: Launch px1 with Git features completely disabled (`px1 -no-git`) for environments where Git is not installed or when viewing plain directory archives.

---

## Technical Architecture Deep Dive

For an explanation of how px1 executes read-only `git status --porcelain=v2` and `git diff` commands concurrently with directory indexing, see [Git Awareness & Diffing Internals](../internals/git-integration.md).
