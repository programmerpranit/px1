# px1

px1 is a fast, ultra-light, remote-first IDE designed for instant code navigation and review in your browser. Booting in under 1 ms and using ~20 MB of RAM, it turns your browser into a zero-latency inspection console with symbol-level navigation, deep search, and syntax highlighting across massive codebases.

## Optimized for Reads

More and more code generation happens directly in the terminal—driven by coding agents, CLI tools, and background orchestrators. Developers spend significantly less time typing boilerplate and more time reviewing, auditing, and navigating.

Because speed of access is everything when inspecting code, **px1 is obsessively optimized for reads, with a small, direct escape hatch for writes.** You don't need a heavy editing environment with background extension churn just to verify code; you need a sub-millisecond, zero-latency window into the repository, especially across remote machines. When something needs to change, click into the file and type, then stage and commit from the same tab.

### Where px1 fits in best:

- **Verifying AI Agent Output**: Inspect live git diffs against `HEAD`, review generated code, fix it directly in the tab, stage and commit, and close it without leaving your terminal flow.
- **Remote & Cloud Server Inspection**: Spin up on any remote server, VM, or CI runner and browse the codebase instantly from your local browser—no SSH keys, no port forwarding hassle, and no heavy remote desktop/daemons.
- **Auditing Large Repositories**: Read through massive, 50,000+ file codebases on a laptop without background indexers hogging RAM or spinning up fans.
- **Sidecar to Terminal Editors**: Keep lightweight editors (like Vim, Neovim, or Helix) in the terminal for typing, while using px1 as a high-density, rich graphical inspection and diff console.

## Installation

### Quick Install (macOS, Linux, BSD)

```bash
curl -fsSL https://px1.pranitpatil.com/install.sh | sh
```

### Build from Source

Requires Go 1.24+. No npm, node, CGO, or external dependencies:

```bash
git clone https://github.com/programmerpranit/px1.git
cd px1
make build
install -d ~/.local/bin && install px1 ~/.local/bin/
```

To cross-compile binaries for all supported platforms:

```bash
make dist
```

## Features

> For in-depth guides and workflows for every feature, see the [Features Documentation](docs/features/README.md).

- **Blazing Fast Navigation**: Fuzzy file search (`Cmd/Ctrl+P`), quick palette (`Cmd/Ctrl+K`), and workspace regex search (`Cmd/Ctrl+Shift+F`) in milliseconds.
- **Remote-First, Zero SSH Hassle**: Spin up on any remote server, cloud instance, or runner in < 1 ms. Inspect remote code in your local browser over a single port (Tailscale, WireGuard, reverse proxy, or tunnel) without SSH key setups, port forwarding churn, or remote extension daemons.
- **Rich Syntax Highlighting**: Native tokenization for ~280 languages via Chroma with windowed rendering.
- **Git Awareness & Visual Diffs**: Status badges (`M`, `A`, `D`, `U`, `R`), dirty folder ancestry propagation, changed-files filter, and side-by-side diffs vs `HEAD` (`Cmd/Ctrl+D`).
- **Direct Editing**: Click into an open file and type — no separate edit mode. `Mod+S` to save, with a conflict check against concurrent on-disk changes.
- **Git Staging, VS Code-Style**: A Source Control panel lists staged and unstaged changes separately; stage, unstage, and commit without leaving the browser, with a commit-message generator built in.
- **Custom Themes**: 14 built-in themes (GitHub Dark, Tokyo Night, Catppuccin, Dracula, Gruvbox, Nord, Solarized, and more).
- **Virtual DOM / Zero Overhead**: Opening a 400,000-line file costs the same as a 10-line file; only visible rows are mounted. Reclaims memory after 15 seconds of inactivity.
- **Completely Self-Contained**: Single static binary embedding all web assets. Zero runtime dependencies, no Electron, no Node, no cloud phone-homes.

## Editing Files Directly

px1 edits files itself: click into an open file and type. There is no separate edit mode — `Mod+S` saves, `Mod+Z` / `Mod+Shift+Z` undo/redo. A save is refused (409, with the current on-disk content returned) if the file changed on disk since it was opened, so a concurrent edit from elsewhere is never silently overwritten.

## Git: Stage, Unstage, Commit

The **Source Control** tab in the sidebar (next to **Files**) lists every changed file split into **Staged Changes** and **Changes**, the same grouping VS Code uses. Click **+** to stage a file, **−** to unstage it, click the file itself to open it (press `Mod+D` to see its diff against `HEAD`).

Write a commit message and click **Commit**, or click **Generate** to have it written for you: px1 shells out to the `claude` CLI on staged changes and fills the message box with a one-line Conventional Commits summary. Generation needs the `claude` CLI on `PATH`; if it isn't installed, `Generate` reports that plainly and you write the message yourself.

## Why a Dedicated Code Viewer?

Traditional IDEs carry tens of thousands of authoring features, Electron runtimes, background indexers, and gigabytes of memory overhead. In modern AI-assisted workflows, developers spend significantly more time reviewing code than typing it.

| Parameter | Traditional IDE (e.g., VS Code) | px1 (Code Viewer) |
| --- | --- | --- |
| Primary Purpose | Manual code authoring & plugin host | Instant code reading & navigation |
| Base Memory (RSS) | ~1,440 MB (1.4+ GB) | ~20 MB (~70x lighter) |
| Active Startup CPU Spike | 35% - 50% | < 1% |
| Cold Startup Time | Several seconds | Sub-millisecond |
| Process Tree | 15+ Node.js/Electron processes | 1 single static Go binary |
| Workspace Indexing | Multi-second background churn | 0 - 45 ms for entire repositories |
| Setup and Config | Config files, plugins, node, npm | Zero config, zero runtime |

## Key Numbers and Benchmarks

All metrics are measured on real-world repositories and reproducible using [`./benchmark.sh`](benchmark.sh).

### Real Corpus Performance (px1 standalone)

| Repository   | Source Size | Files Indexed | Index Time | Fuzzy Search | Full-Tree Regex Scan | Resident RAM (RSS) |
| ------------ | ----------- | ------------- | ---------- | ------------ | -------------------- | ------------------ |
| flask        | 3 MB        | 235           | 1 ms       | 0.8 ms       | 2.3 ms               | 20 MB              |
| redis        | 26 MB       | 1,855         | 13 ms      | 1.0 ms       | 18.2 ms              | 17 MB              |
| react        | 63 MB       | 7,178         | 52 ms      | 2.7 ms       | 32.2 ms              | 21 MB              |
| django       | 74 MB       | 7,014         | 39 ms      | 1.3 ms       | 26.8 ms              | 20 MB              |
| kubernetes   | 370 MB      | 25,926        | 150 ms     | 13.5 ms      | 84.6 ms              | 30 MB              |
| TypeScript   | 414 MB      | 66,533        | 566 ms     | 6.2 ms       | 150.3 ms             | 69 MB              |
| linux kernel | 1,809 MB    | 95,710        | 370 ms     | 6.0 ms       | 451.8 ms             | 55 MB              |

### Head-to-Head: px1 vs. VS Code

Run `./benchmark.sh --vscode .` to measure both on your active machine:

```text
### px1 vs. VS Code Comparison

| Metric / Parameter | px1                    | VS Code (Server/Remote) | Notes                   |
| ------------------ | ---------------------- | ----------------------- | ----------------------- |
| Memory (RSS)       | 20 MB                  | 1,166 - 1,440 MB        | ~70x lighter            |
| Instant CPU %      | 0.0%                   | 4.0% - 39.0%            | Minimal CPU churn       |
| Index Time         | < 1 ms                 | ~4 - 10 s               | px1 is instantaneous    |
| Process Count      | 1 single Go binary     | 15+ processes           | Multi-process Node tree |
```

## Usage

Run `px1` with an optional file or directory:

```bash
px1                     # view current workspace
px1 ~/src/kernel        # view another repository
px1 web/src/main.js     # view a file in its project workspace
px1 main.go:42          # open directly to a line number
px1 -d ~/work/repo      # run in the background, hand the shell back immediately
```

Running with `-d` frees your terminal right away instead of blocking it, so you can open several workspaces at once — each instance walks forward to the next free port automatically:

```bash
px1 -d ~/work/api       # -> http://127.0.0.1:7777
px1 -d ~/work/frontend  # -> http://127.0.0.1:7778
```

### Remote & Cloud Workspaces

Spin up on any remote server, VM, or container and view code directly in your local browser without SSH shell management, X11 forwarding, or remote extension daemons:

```bash
# Bind all interfaces on a remote machine / cloud instance
px1 -host 0.0.0.0 -port 7777 ~/work/repo

# Headless / server mode without opening local browser
px1 -no-open -port 8080 /workspace

# CI runner
px1 -d -no-open -port 8080 /workspace
```

Access securely over Tailscale, WireGuard, reverse proxy, or Cloudflare Tunnel with zero remote setup overhead and sandboxing (path traversal protection & DNS rebinding checks). Anyone who can reach px1 can save, stage, or commit when it is opened by IP address (for example over Tailscale), so bind to a private network. Opened through a hostname, such as a reverse proxy or tunnel domain, writes are refused.

### CLI Flags

| Flag         | Default     | Description                                                     |
| ------------ | ----------- | --------------------------------------------------------------- |
| `-port N`    | `7777`      | Port to listen on (`0` picks an ephemeral free port)            |
| `-host H`    | `127.0.0.1` | Local address to bind                                           |
| `-no-open`   | `false`     | Do not launch the web browser automatically                     |
| `-no-git`    | `false`     | Disable git awareness (tree status badges, diff view, staging)  |
| `-d`         | `false`     | Run detached in the background and return control to the shell  |
| `-no-color`  | `false`     | Strip ANSI escape sequences from terminal output                |
| `-quiet`     | `false`     | Suppress CLI narration (errors still print to stderr)           |
| `-verbose`   | `false`     | Log requests and searches to the terminal                       |
| `-version`   | `false`     | Print version and architecture and exit                         |

## Keyboard Shortcuts

`Cmd` on macOS, `Ctrl` on Windows and Linux; `Alt` is `Option` on a Mac. The in-app sheet (`?`), footer hints and tooltips show each key the way your keyboard labels it (`Cmd+Shift+F` on a Mac, `Ctrl+Shift+F` elsewhere).

| Key                                                    | Action                                                                                     |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------ |
| `Cmd/Ctrl+K`                                           | Universal palette / quick open                                                             |
| `Cmd/Ctrl+P`                                           | Go to file                                                                                 |
| `Cmd/Ctrl+Shift+P`                                     | Command palette                                                                            |
| `Cmd/Ctrl+Shift+F`                                     | Full workspace search                                                                      |
| `Cmd/Ctrl+F`                                           | Find in active file (seeded with the current editor selection)                             |
| `Cmd/Ctrl+G`                                           | Jump to line                                                                               |
| `Cmd/Ctrl+D`                                           | Toggle git diff of the active file against `HEAD`                                          |
| `Cmd/Ctrl+S`                                           | Save the active file                                                                       |
| `Cmd/Ctrl+Z` / `Cmd/Ctrl+Shift+Z`                      | Undo / redo an edit                                                                        |
| `Cmd/Ctrl+Backspace` / `Cmd/Ctrl+Delete`               | Delete the word before / after the caret                                                   |
| `Cmd/Ctrl+/`                                           | Toggle line comment                                                                        |
| `Cmd/Ctrl+A`                                           | Select the whole file                                                                      |
| `Enter` / `Shift+Enter`                                | Next / previous match while finding                                                        |
| `Left` / `Right`, `Home` / `End` (`Cmd+Left` / `Cmd+Right` on macOS) | Move the caret along the line; click places it                                             |
| `Ctrl+Home` / `Ctrl+End` (`Cmd+Up` / `Cmd+Down` on macOS)            | Top / bottom of file                                                                       |
| `Alt+Z`                                                | Toggle word wrap                                                                           |
| `Alt+C` / `Alt+A`                                      | With code selected: copy reference / copy with file and line context                       |
| `Right click`                                          | On a selection: the same actions in a menu at the pointer                                  |
| `Alt+Left` / `Alt+Right`                               | Navigate back / forward in history                                                         |
| `Cmd/Ctrl+B`                                           | Toggle file tree sidebar                                                                   |
| `Cmd/Ctrl+Shift+R`                                     | Refresh workspace                                                                          |
| `Alt+W`                                                | Close active tab (`Cmd/Ctrl+W` too, where the browser lets a page have it)                 |
| `Alt+Shift+T`                                          | Reopen the last closed tab                                                                 |
| `Ctrl+Tab`                                             | Switch to next tab                                                                         |
| `Alt+1` ... `Alt+9`                                    | Select tab by position                                                                     |
| `?`                                                    | Show all keyboard shortcuts                                                                |

## Philosophy and Design Principles

- **Optimized for Reads**: px1 does not attempt to be a heavy code editor with plugins, panes, and background extensions. It focuses on the reader experience and keeps writes to a narrow, direct save/stage/commit path.
- **Remote-First & SSH-Free**: Works seamlessly whether inspecting a local directory or a cloud instance over Tailscale/VPN—no remote daemons, no X11 forwarding, and no SSH session maintenance.
- **Private & Sandboxed**: Zero accounts, zero cloud dependencies. Code and queries stay on the running machine. Protected by path traversal guards and DNS rebinding prevention.
- **Reclaims Memory**: Automatically recovers memory after 15 seconds of inactivity so idle sessions don't hoard host RAM.

## Reproducing Benchmarks

All benchmark figures can be measured directly on your own system:

```bash
# 1. Fetch benchmark corpus (~3 GB shallow clones of Linux, K8s, TypeScript, etc.)
./benchmark.sh --clone

# 2. Run the full benchmark suite
./benchmark.sh

# 3. Compare px1 directly against VS Code process tree on your workspace
./benchmark.sh --vscode .

# 4. Profile memory lifecycle across index, search, and idle recovery
./benchmark.sh --memory bench-repos/linux
```

See [Performance Benchmarks](BENCHMARKS.md) for full methodology and detailed charts.

## Contributing

Contributions that keep px1 fast, minimal, and dependable are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting issues or pull requests.

### Development Workflow

1. Clone the repository:
  ```bash
  git clone https://github.com/programmerpranit/px1.git
  cd px1
  ```
1. Run tests:
  ```bash
  make test
  # or go test ./...
  ```
1. Live frontend development (serves `web/` assets from disk without rebuilding the binary):
  ```bash
  go run . -dev . .
  ```
1. Verify CLI formatting and builds:
  ```bash
  go vet ./...
  make dist
  ```

### Architecture & Internals

For comprehensive technical deep-dives into the architecture, indexing, virtualized rendering, and syntax highlighting subsystems, see the [Internals Documentation](docs/internals/README.md).

- `main.go` / `ui.go`: CLI entrypoint, flag parsing, signal management, Ape terminal experience.
- `server.go`: HTTP routes, JSON API, gzip compression, and embedded asset serving.
- `fileops.go`: Direct file save/rename, gated by same-origin `localPost` and an optimistic-concurrency check.
- `index.go`: Concurrently walks workspace, honors `.gitignore` (ignored files stay visible but dimmed in the explorer, and are never indexed or searched), builds in-memory path and trie structures in milliseconds.
- `search.go` / `fuzzy.go`: High-performance substring and fuzzy file/symbol matching algorithms.
- `git.go`: Status, diff, staging, unstaging, and committing via porcelain `git`.
- `commitmsg.go`: Commit-message generation by shelling out to the `claude` CLI on staged changes.
- `web/`: Native zero-dependency ES module frontend (custom virtual scroll, syntax highlight rendering, tab manager, source-control panel).
- `web/themes/`: One CSS file per colour theme, joined by the server into `/static/themes.css`. Token reference in [Styling & Themes](docs/internals/styling-and-themes.md).

## Credits

px1 is a fork of [px0](https://github.com/px0-ai/px0) ([px0.ai](https://px0.ai)) by [Arpit Bhayani](https://arpitbhayani.me), trimmed down to a smaller feature set: directory viewing, file navigation, live read/write editing, git stage/unstage, a commit-message generator, syntax highlighting, and code search. All credit for the original architecture, virtualized rendering engine, and design philosophy goes to the upstream project.

## License

[MIT License](LICENSE) (c) 2026 Arpit Bhayani
