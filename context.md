# Session context: px1 (fork of px0)

Status as of 2026-09-20 ~00:10 IST. Written so a fresh session can resume without re-deriving the history below.

## What px1 is

A fork of [px0](https://github.com/px0-ai/px0) by Arpit Bhayani, stripped from a full code-navigator (LSP, AI pair-editing agent, markdown preview, telemetry, self-update, settings, etc.) down to 7 features:

1. Directory viewing
2. File navigation
3. Live editing (read **and** write, in place — not a separate edit mode)
4. Git stage/unstage/commit, VS Code-style, plus push when ahead of the remote
5. Commit message generator (shells out to the `claude` CLI on the staged diff)
6. Syntax highlighting
7. Code search

The strip-down plan (now executed, kept for historical reference): `/Users/pranit/.claude/plans/eventual-snacking-puppy.md`.

For architecture, module map, build commands: see `CLAUDE.md` — not duplicated here.

## Current repo state

- **Pushed to `origin/master`**: everything through commit `e259e18` (push-to-remote feature). Branch is in sync with the remote (`git status -sb` shows no ahead/behind).
- **Uncommitted right now** (5 files, all from the most recent round of UI polish in this session): `web/index.html`, `web/style.css`, `web/src/scm.js`, `web/app.js` (rebuilt bundle), `website/index.html`.
- **Rule in force**: do not `git commit` in this repo without the user's explicit go-ahead for that specific commit (see `CLAUDE.md`). The user commits and pushes work themselves from their own terminal once satisfied — see "resolved anomaly" below.

## Resolved: the "stray commit" mystery from earlier in this session

Early on, an unexplained commit (`3f59eee`) appeared in the repo mid-testing, which the plan said not to make, and its diff didn't match its own message. At the time this was flagged as an open anomaly to investigate. It's since become clear this isn't a bug: **the user has their own terminal/editor access to this same repo and commits + pushes the work themselves**, independent of this session — confirmed by two more commits (`c59bdd8` rebrand, `e259e18` push feature) appearing and getting pushed to `origin/master` without this session ever running `git commit`. The "no commit without go-ahead" rule is about *this session's* tool calls, not a claim that the repo stays untouched by the user. No further investigation needed; just keep expecting the working tree to sometimes change or advance between turns and treat that as normal, not a bug to chase.

Relatedly: earlier in the session, `highlight.go`'s `maxFileBytes` const was found commented-out and saved to disk mid-testing — almost certainly the user trying the just-shipped `Cmd+/` comment-toggle shortcut live on that exact line in the same browser session used for testing. Fixed immediately, confirmed via clean `git diff`.

## Work done this session, chronologically

1. **Strip-down to 7 features** (bulk of the work, done by a background fork agent per the plan): deleted LSP, agent/AI-pairing, markdown preview, image viewer, telemetry, metrics, self-update, settings; added `git.go` staging split, `commitmsg.go`, `/api/git/*` routes, `web/src/scm.js` SCM panel. Checkpointed first via commit `5c32fad` to protect in-flight live-editing work that predated this session. Verified manually via Chrome DevTools: highlighting, diff view, live write + save, save-conflict banner, SCM stage/unstage all working.

2. **Caret flicker + stale-highlight-lag bugfix** (`edit.js`, `renderer.js`, `cursor.js`): typing at end of a line briefly snapped the caret to line-start (`toPoint()`'s degenerate fallback landed on the wrong node type); deletions appeared to lag up to 300ms (`d.buf.highlighted` cache wasn't invalidated on edit/undo/redo). Called `advisor()` first per user's request — it caught a second latent trigger of the same `toPoint` bug and corrected the initial "rAF coalescing" misdiagnosis. Both fixed, verified via timing-controlled DOM inspection.

3. **`Cmd/Ctrl+Backspace`/`Delete` (word delete) and `Cmd/Ctrl+/` (line comment toggle)** shortcuts added to `edit.js`/`shortcuts.js`, scoped to not hijack the commit-message textarea.

4. **SCM discard buttons** (single-file + per-group "discard all") added: `gitDiscard`/`gitDiscardAll` in `git.go`, routes in `server.go`, buttons + confirm dialogs in `scm.js`/`style.css`.

5. **Stage-All bug fix**: user reported "staged only 20 of ~77 files." Root cause: the old bulk stage/unstage/discard fired one HTTP request per file, each spawning a concurrent `git add` subprocess — most lost the race on `.git/index.lock` (git doesn't queue/retry for it) and failed silently. Fixed by replacing with atomic bulk functions (`gitStageAll`/`gitUnstageAll`/`gitDiscardAll`, one git invocation per bulk action) and matching `/api/git/*-all` routes.

6. **Background mode (`-d` flag)**: `daemon.go`/`daemon_unix.go`/`daemon_windows.go` re-exec the binary as a session-detached child, report the bound URL back via a temp status file, print it, and exit — hands the shell back immediately. Multiple `-d` instances stack onto the next free port automatically.

7. **Website + full px0→px1 rebrand**: new `website/index.html` (single static page, no build step, for Vercel — root directory `website/`) + `website/install.sh` + `website/README.md` with deploy steps, domain `px1.pranitpatil.com`. Renamed px0→px1 across the entire repo: `go.mod`, binary output, `Makefile`, `build.sh`, `install.sh`, `.github/*`, all docs, and the web UI (title, favicon — now a data-URI SVG, no more hotlinked px0.ai logo images, replaced with a plain-text wordmark). Credit to the original px0 project and Arpit Bhayani kept explicitly in `README.md`, `CLAUDE.md`, and the website footer. `LICENSE` left untouched (MIT requires keeping the original copyright notice).

8. **Advisor review + Chrome DevTools check of the website** (per explicit user request): advisor caught that the rename sed pass had relabeled *old-product* claims as px1's own — a whole "Updating px1 / `px1 --update`" section that doesn't exist (no `update.go`), a `docker run px1:latest` example (no published image), stale `-no-lsp`/`symbol outline` mentions. Fixed. DevTools check found a real bug (clicking Install/Features nav landed the heading under the sticky header — `scroll-margin-top` fix) and a wrong CLAUDE.md claim (`web/style.css` is NOT generated by the build step, unlike `web/app.js` — corrected).

9. **Full docs-debt cleanup pass** (the "does not block" follow-up from step 8, done when the user asked to "add docs"): rewrote every doc still describing deleted subsystems — `docs/internals/architecture.md` (endpoint table, security model, sequence diagram), `docs/internals/README.md` (mermaid diagram), `docs/internals/git-integration.md` (split-only diff view, not split/unified), `docs/internals/workspace-search.md` (removed dead `symbols.go` section), `docs/internals/styling-and-themes.md`/`editor-virtualization.md`/`file-reload-and-updates.md` (stripped hovercard/LSP/Markdown/outline refs), `docs/agents/README.md` (HTML-structure and module tables rebuilt from actual current files), all of `docs/features/*.md`, plus `README.md`/`BENCHMARKS.md`/`CONTRIBUTING.md`/`PUBLISHING.md`. Cross-checked against real code (routes, exports, element IDs) rather than guessing.

10. **Website UI polish**: added a real product screenshot (`website/shot.webp`, 51KB, own build) in a browser-chrome frame, subtle background glow/grid, card hover states. Verified in DevTools: only same-origin requests, no console errors.

11. **Push-to-remote feature**: `gitSyncStatus`/`gitPush` in `git.go`, `/api/git/sync-status` + `/api/git/push` routes, a `#scm-sync` banner in the SCM panel (`↑N ↓M vs origin/main` + **Push** button) shown whenever the branch has a remote and is ahead. Refreshes on every SCM panel load, including right after a commit. Verified live against this real repo (was ahead 3 at the time) — did not click Push myself, left it for the user.

12. **Icon UI polish** (most recent, uncommitted): the **Generate** commit-message button is now a sparkle-icon button (was text), with a spin animation while generating. The `px1` wordmark now renders as `px` + accent-colored `1` (`.logo-accent`, uses the theme's own `--accent` token) in three places: sidebar footer logo, empty-state wordmark, website nav brand. The **Files** / **Source Control** sidebar tabs are now icon-only (folder icon, and the same branch/graph icon already used for the "changed files" filter, for visual consistency) with the change-count badge kept on the Source Control icon.

## Verification posture

`go build`/`go vet`/`go test` run clean after every change in this list. UI changes were verified live via Chrome DevTools MCP (screenshots, accessibility snapshots, network/console inspection) against real running instances, not just by reading code. No destructive git operations were run; no `git commit` or `git push` was ever executed by this session.
