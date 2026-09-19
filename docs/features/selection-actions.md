# Selection Toolbar & Context Actions

px1 features a dedicated footer selection toolbar and contextual right-click menu for pulling code out of the editor and into somewhere else. When code is selected, px1 replaces standard footer clutter with two purposeful actions: Copy Reference (`Alt+C`) and Copy with Context (`Alt+A`).

---

## Overview & Core Purpose

In typical developer workflows, sharing code snippets with teammates or pasting code into an external AI chat (ChatGPT, Claude, a terminal coding agent) involves repetitive manual labor: selecting lines, copying them, manually writing down the file path and line numbers, and formatting markdown code fences.

px1 turns code selection into a small launchpad for that. Instead of popping up an obstructive floating tooltip that obscures adjacent lines of code, px1 smoothly transitions the left section of the fixed bottom status bar into an action bar the moment text is highlighted. The same two actions are simultaneously accessible via a right-click context menu and direct keyboard shortcuts.

---

## Key Actions & Capabilities

### 1. Copy Reference (`Alt+C`)
Copies a concise pointer to the selected code, formatted as:
`path/to/file.go:42` for a single line, or `path/to/file.go:42-68` for a range.
- **Practical Use**: Paste directly into pull request review comments, Slack messages, or GitHub issues so teammates can immediately locate the exact lines.

### 2. Copy with Context (`Alt+A`)
Copies a self-contained Markdown block meant for pasting into an AI chat:
- A `@path/to/file.go line(s) N` or `@path/to/file.go line(s) N-M` header line.
- The selected text enclosed in a language-tagged fenced code block (the language tag comes from the file extension).

---

## Split Diff Selection

When reviewing changes in the split (side-by-side) git diff viewer, selecting code on either the left (`HEAD`) or right (working tree) side requires special care. Traditional editors sweep up line numbers and diff markers (`+`/`-`) into the clipboard.

Every rendered diff row carries its working-tree line in a `data-l` attribute (or `data-at` for a deleted row, the line it sat before) that `selbar.js` reads directly, so:
- Gutter line numbers and diff signs are cleanly excluded from what gets copied.
- The selected text is mapped to working-tree coordinates before either action runs.

---

## Keyboard Shortcuts & Interaction Matrix

| Action | Shortcut | Context Menu | Status Bar Button | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Copy Reference** | `Alt+C` | Right-Click | `[Copy Ref]` | Copies a `path:line` or `path:line-line` pointer |
| **Copy with Context** | `Alt+A` | Right-Click | `[Copy Context]` | Copies a fenced code block with a file/line header, for pasting into an AI chat |

---

## Non-Intrusive Ergonomics

- **No Code Occlusion**: Floating toolbars often pop up directly over the line above or below your selection, hiding the very code you are trying to read. px1 mounts action buttons in the bottom status bar, leaving the editor viewport 100% unobstructed.
- **Automatic State Restoration**: As soon as you click elsewhere or collapse the selection, the status bar smoothly restores normal file coordinates, line/column counters, and Git branch details.
