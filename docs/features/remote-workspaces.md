# Remote Workspaces & Cloud Inspection

px1 is architected from the ground up as a remote-first code reading environment. It allows you to run a single binary on any remote server, cloud instance, Docker container, or CI runner and browse the codebase directly in your local desktop browser without SSH keys, port forwarding setups, or heavy remote extension daemons.

---

## Overview & Core Purpose

Modern software development frequently takes place across cloud instances (AWS EC2, GCP Compute Engine), remote containers, Kubernetes pods, and devboxes. Traditional remote development setups (such as VS Code Remote-SSH, remote X11 forwarding, or VNC) require multi-step authentication setups, background server daemon installations, high CPU overhead, and significant network bandwidth.

px1 simplifies remote code inspection into a single shell command. Because the entire application—Go backend, HTTP server, assets, and frontend—is compiled into one static ~9.5 MB binary with zero external dependencies, you can copy px1 to any remote Linux, macOS, or BSD machine and spin it up instantly. Connecting via Tailscale, WireGuard, private VPCs, or reverse proxies gives you a fluid, graphical code inspection console in your browser.

---

## Key Capabilities

- **Zero Remote Daemons**: No Node.js runtime, no npm packages, no Electron layers, and no background extension churn on the remote machine.
- **Single Port Operation**: px1 serves all assets, JSON APIs, and search queries over a single HTTP port (default `7777`).
- **Flexible Network Binding**:
  - Bind to localhost for private tunnels (`-host 127.0.0.1`).
  - Bind to all interfaces for Tailscale/VPN access (`-host 0.0.0.0`).
- **Headless Server Mode (`-no-open`)**: Starts the server silently on a remote machine or CI runner without attempting to invoke a local web browser.
- **Background Mode (`-d`)**: Runs detached and hands the shell back immediately — useful for leaving px1 running on a remote box across an SSH session, or for running several workspaces on one machine at once (each stacks onto the next free port).
- **Built-in Security & Sandboxing**:
  - **Path Traversal Protection**: Every client-supplied path is sandboxed to the workspace root (`safePath` in `server.go`); requests attempting to escape it with `..` are rejected outright.
  - **DNS Rebinding Defense**: Inspects incoming HTTP `Host` and `Origin` headers to prevent a malicious page from triggering writes via a rebound DNS record.
  - **Write Restrictions by Access Method**: Saving, staging, and committing are permitted only when px1 is reached by IP address or `localhost`. Reached through an external hostname (a reverse proxy or tunnel domain), writes are refused — reading and browsing still work.
- **Direct Terminal Ergonomics**: Launch px1 targeting specific files or line numbers directly from the command line:
  - `px1` (opens current directory)
  - `px1 ~/projects/kernel` (opens specified repository)
  - `px1 main.go:42` (opens directly to line 42)

---

## Developer Workflows & Setup Patterns

### 1. Cloud Devbox via Tailscale or WireGuard
Run px1 on your cloud instance bound to all interfaces:
```bash
px1 -host 0.0.0.0 -port 7777 ~/work/repo
```
Open your local browser to `http://100.x.y.z:7777` (your machine's private Tailscale IP). You get full code reading, fuzzy search, and diff inspection with zero SSH lag.

### 2. Inspecting a Running Container
px1 is a single static binary with no runtime dependencies, so `docker cp` it into a running container and run it in place — no image rebuild needed:
```bash
docker cp px1 <container>:/usr/local/bin/px1
docker exec -it <container> px1 -host 0.0.0.0 -no-open /workspace
```

### 3. CI/CD Runner Debugging
When a build or test suite fails on a remote CI runner, download px1, run it in the background, and inspect generated artifacts, failure logs, and git status directly in your browser.

---

## CLI Flag Reference

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-port N` | `7777` | Port to listen on (`0` picks an ephemeral free port) |
| `-host H` | `127.0.0.1` | Network address to bind |
| `-no-open` | `false` | Suppress automatic browser launch (ideal for servers) |
| `-no-git` | `false` | Disable Git status checks and diff viewing |
| `-d` | `false` | Run detached in the background, return the shell immediately |
| `-quiet` | `false` | Suppress CLI narration on stdout |
| `-verbose` | `false` | Log requests and searches to the terminal |
| `-version` | `false` | Print version and architecture and exit |

---

## Technical Architecture Deep Dive

For an architectural breakdown of HTTP routing, gzip connection pooling, symlink cycle immunity, and memory scavenging pipelines, see [System Architecture & Runtime Lifecycle Internals](../internals/architecture.md).
