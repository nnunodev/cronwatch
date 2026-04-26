# cronwatch

Watch remote cron jobs from the terminal. Live-updating dashboard with human-readable schedules and next-run countdowns.

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
- **Next-run countdown** — "in 2h 30m" instead of raw timestamps
- **Simple mode** — plain terminal output, no TUI, script-friendly
- **Error states** — failed jobs shown in red with last error message
- **SSH config discovery** — reads `~/.ssh/config` automatically for host, user, port, and key

## Install

```bash
go install github.com/nnunodev/cronwatch/cmd/cronwatch@latest
```

Requires Go 1.21+.

## SSH setup

The easiest way to configure cronwatch is via your `~/.ssh/config`. cronwatch reads the first matching `Host` block automatically.

```bash
# Add to ~/.ssh/config on the machine running cronwatch
Host myserver
  HostName 203.0.113.10
  User admin
  IdentityFile ~/.ssh/id_ed25519
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
```

Then just run `cronwatch`. CLI flags always override anything in `~/.ssh/config`.

On Windows, place the same private key at `%USERPROFILE%\.ssh\id_ed25519`. The public key must already be on the server (`~admin/.ssh/authorized_keys`).

## Flags

```
--host string      SSH host alias or IP (default "myserver")
--user string      SSH user (default: from ~/.ssh/config, then current user)
--port int         SSH port (default: from ~/.ssh/config, then 22)
--key string       SSH private key path (default: from ~/.ssh/config)
--refresh int      Auto-refresh interval in seconds (default 10, 0=disabled)
--timeout int      SSH command timeout in seconds (default 10)
--simple           Plain terminal output instead of TUI
--version          Show version and exit
```

## Usage

### Interactive TUI (default)

```bash
cronwatch
```

Keyboard shortcuts:

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate jobs |
| `r` | Force refresh |
| `q` / `Esc` / `Ctrl+C` | Quit |

Refreshes automatically every 10 seconds.

### Plain output

```bash
cronwatch --simple
```

Good for piping, scripting, or use in other tools.

### Disable auto-refresh

```bash
cronwatch --refresh 0
```

### Custom SSH timeout

If the remote host is slow to respond (e.g. over a VPN):

```bash
cronwatch --timeout 30
```

## How it works

cronwatch connects to the remote host via SSH and reads the job list from `~/.hermes/cron/jobs.json` (the Hermes cron backend file). It parses the raw JSON into structured data:

- **ScheduleHuman** — cron expression converted to human-readable form
- **NextRunHuman** — time until next run ("in 2h 30m")
- **LastState** — `ok`, `error`, or `running`

Jobs are sorted by next run time (soonest first). Running jobs are pinned to the top. Refresh fetches fresh data on every tick.

## Build from source

```bash
git clone https://github.com/nnunodev/cronwatch
cd cronwatch
go build -o cronwatch ./cmd/cronwatch
./cronwatch
```

Set a real version at build time:

```bash
go build -ldflags="-X main.version=1.0.0" -o cronwatch ./cmd/cronwatch
```

## Requirements

- SSH access to the remote host running Hermes cron jobs
- `~/.hermes/cron/jobs.json` readable by the SSH user on the remote host
- Terminal with ANSI color support (most modern terminals)
