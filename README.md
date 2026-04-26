# cronwatch

Watch remote cron jobs from the terminal. Live-updating dashboard with human-readable schedules and next-run countdowns.

> **Note:** cronwatch is a client for the [Hermes agent cron scheduler](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron). It reads `~/.hermes/cron/jobs.json` from a remote host over SSH. If you don't have the Hermes agent running on a server, this tool won't show anything useful.

```
SCHEDULED JOBS
  JOB                                     NEXT       EVERY           STATUS
  ──────────────────────────────────────  ─────────  ─────────────── ───────────
  Server Patch Report                     in 50m     daily 8:30    ● ok
  Daily Self-Audit                        in 1h 20m  daily 9:00    ● ok
  Backup Job                              in 1h 50m  daily 9:30    ● ok
  Health Check                            in 2h 20m  twice daily   ● ok
  Social Poster                           in 3h 20m  twice daily   ● ok
  ...

● 07:10:02  ·  13 jobs  ·  ↑↓ navigate  r refresh  q quit
```

## Features

- **Live TUI dashboard** — auto-refreshes every 10s, jobs sorted by next run (soonest first)
- **Human-readable schedules** — `daily 9:30`, `twice (11:00, 16:00)`, `every 12h`, `weekly (Sun)`
- **Next-run countdown** — "in 2h 30m", "1d 4h", or "overdue" instead of raw timestamps
- **Simple mode** — plain terminal output, no TUI, script-friendly
- **Error states** — failed jobs shown in red; refresh failures visible in the banner and footer
- **Visual refresh feedback** — spinning `⟳` indicator in the header while data is loading
- **Staleness warning** — if auto-refresh fails, cached jobs stay visible with a red `· error` footer
- **SSH config discovery** — reads `~/.ssh/config` automatically for host, user, port, and key

## Prerequisites

- A remote server running the [Hermes agent](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron) with cron jobs configured.
- SSH access to that server (key-based auth recommended).
- The Hermes agent cron file `~/.hermes/cron/jobs.json` must be readable by the SSH user.
- Go 1.24+ (if installing from source).

## Install

```bash
go install github.com/nnunodev/cronwatch/cmd/cronwatch@latest
```

Or build from source:

```bash
git clone https://github.com/nnunodev/cronwatch
cd cronwatch
go build -ldflags="-X main.version=1.0.0" -o cronwatch ./cmd/cronwatch
```

## Quick start

The fastest way to get started is with explicit flags:

```bash
cronwatch --host 203.0.113.10 --user admin --key ~/.ssh/id_ed25519
```

If you have a `Host` block in `~/.ssh/config`, you can use the alias instead:

```bash
# ~/.ssh/config
Host myserver
    HostName 203.0.113.10
    User admin
    IdentityFile ~/.ssh/id_ed25519
```

```bash
cronwatch --host myserver
```

> **Security note:** The examples above include `StrictHostKeyChecking no` only for convenience in isolated environments. In production, keep host key verification enabled and use a proper `KnownHostsFile`.

## Flags

```
--host string      SSH host alias or IP (required)
--user string      SSH user (default: from ~/.ssh/config, then current user)
--port int         SSH port (default: from ~/.ssh/config, then 22)
--key string       SSH private key path (default: from ~/.ssh/config)
--refresh int      Auto-refresh interval in seconds (default 10, 0=disable)
--timeout int      SSH command timeout in seconds (default 10, must be > 0)
--simple           Plain terminal output instead of TUI
--version          Show version and exit
```

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate jobs |
| `r` | Force refresh (shows `⟳` spinner in header while fetching) |
| `q` / `Esc` / `Ctrl+C` | Quit |

## Usage examples

### Interactive TUI (default)

```bash
cronwatch --host myserver
```

Refreshes automatically every 10 seconds.

### Plain output

```bash
cronwatch --host myserver --simple
```

Good for piping, scripting, or use in other tools.

### Disable auto-refresh

```bash
cronwatch --host myserver --refresh 0
```

### Custom SSH timeout

If the remote host is slow to respond (e.g. over a VPN):

```bash
cronwatch --host myserver --timeout 30
```

### Override SSH config settings

```bash
cronwatch --host myserver --user root --port 2222
```

## How it works

cronwatch connects to the remote host via SSH and reads the job list from `~/.hermes/cron/jobs.json` (the Hermes agent cron backend file). It parses the raw JSON into structured data:

- **Schedule** — cron expression converted to a human-readable form
- **NextRun** — time until next run (e.g. `"in 2h 30m"`, `"1d 4h"`, overdue)
- **LastState** — `ok`, `error`, or `running`

Jobs are sorted by next run time (soonest first). Running jobs are pinned to the top. Refresh fetches fresh data on every tick. If a background refresh fails, the existing table stays visible with a red error banner and a red `· error` footer dot so you know the data is stale.

## Requirements

- SSH access to the remote host running the Hermes agent
- `~/.hermes/cron/jobs.json` readable by the SSH user on the remote host
- Terminal with ANSI color support (most modern terminals)

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `Error: --host is required` | You didn't pass a host. | Add `--host <host>` |
| `Error: could not determine SSH user` | No `--user`, no SSH config User, and `osuser.Current()` failed. | Pass `--user <name>` |
| `ssh failed: ... No such file or directory` | `~/.hermes/cron/jobs.json` doesn't exist on the server. | Make sure the [Hermes agent](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron) is installed and has written the jobs file. |
| `ssh failed: ... python3: command not found` | The remote server doesn't have `python3` in `PATH`. | Install Python 3 on the remote host. |
| `ssh timeout after 10s` | Network is slow or the host is unreachable. | Increase `--timeout` |
| Auth failures / `Permission denied` | Wrong key, wrong user, or key not in `authorized_keys`. | Check `--user`, `--key`, and server `~/.ssh/authorized_keys` |
| Blank screen after quitting TUI | Terminal didn't clear alt-screen buffer. | Press `Ctrl+L` or run `reset` |

## Windows notes

- Works in Windows Terminal, PowerShell, and most modern terminals.
- Store your SSH private key at `%USERPROFILE%\.ssh\id_ed25519`.
- The `go install` and `go build` commands work identically on Windows.
