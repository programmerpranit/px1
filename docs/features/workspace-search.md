# Workspace Text & Regex Search

Workspace search provides full-repository text searching across all files in your project. Accessible via `Cmd/Ctrl+Shift+F`, it allows you to query literal text phrases or complex regular expressions across millions of lines of code with instant previews.

---

## Overview & Core Purpose

When investigating an unfamiliar codebase, tracing error strings, auditing security vulnerabilities, or auditing how a configuration flag is utilized across services, searching file names alone is insufficient. Developers need to search through the entire workspace content at blazing speed.

Traditional IDEs often struggle with whole-workspace text scans, stalling the UI or spinning up CPU fans as they churn through gigabytes of code. px1 provides multi-threaded parallel grep capability that queries tens of thousands of files in tens of milliseconds without causing browser stutters or high memory consumption. Results are cleanly grouped by file in the right-side search panel, providing line numbers and contextual snippets.

---

## Key Capabilities

- **Parallel Multi-Core Grep**: Utilizes all available CPU cores on the host machine to search across repository files concurrently. Searching the entire React, Django, or Kubernetes repository takes only 25–85 milliseconds.
- **Literal & Regular Expression Support**: Supports exact string matching as well as full Go/PCRE-compatible regular expressions for matching complex code patterns, IP addresses, function declarations, and identifier conventions.
- **Case Sensitivity Toggle**: Searches are case-insensitive by default; click **Aa** (or the whole-word / regex toggles beside it) to switch modes per search.
- **Structured File Grouping**: Matches are organized hierarchically by file path in the right-side search panel.
- **Contextual Snippet Elision**: Each match displays a contextual code snippet with the matching phrase highlighted. Long lines are intelligently elided around the match so you can immediately understand the surrounding code without scrolling horizontally.
- **One-Click Navigation**: Clicking any snippet immediately opens the target file in the viewer, positions the caret at the matching row, and scrolls the line into the center of the screen.

---

## Developer Workflows & Practical Value

### Auditing Symbol Call Sites
Before refactoring a public API or modifying an exported function, you can search for the symbol name across the repository to verify every caller, mock, and test case that references it.

### Hunting Down Error Messages
When an application logs a specific error string (such as `connection refused on port 5432` or `unexpected EOF in header`), pasting that string into workspace search instantly pinpoints the exact line where the error was thrown.

### Pattern Matching with Regular Expressions
Using regex syntax, you can audit code patterns across the entire project. For example:
- `func\s+\(.*\)\s+Handle\w+` to discover all Go HTTP handler methods.
- `TODO\(.*?\):` or `FIXME` to review pending work across teams.
- `api/v[0-9]+/` to find all versioned API endpoints.

---

## Interactive Controls & Search Pane

1. Press **`Cmd/Ctrl+Shift+F`** to reveal the right-hand Inspector and focus the search input field.
2. Enter your query or regex pattern and press `Enter`.
3. As results stream in, the panel header shows the total match count and the number of affected files.
4. Click any result snippet to open the corresponding file at that line.
5. Use the collapsible chevron next to each file header to collapse files you have already reviewed.
6. Press `Esc` or click the close button to dismiss the search panel.

---

## Configuration & Tuning

There is no settings UI. `o-case` / `o-word` / `o-re` (Match case, Whole word, Regular expression) are per-search toggles in the search form itself, and `.gitignore`-excluded files are never searched (see [File Explorer](file-explorer.md)). To scope a search further, use the `files to include` glob field.

---

## Technical Architecture Deep Dive

For details on the worker pool architecture, buffer reuse mechanisms, whole-file pre-filtering, and snippet elision algorithms, see [Workspace Search & Parallel Grep Internals](../internals/workspace-search.md).
