# cronwatch

Watch your Hermes cron jobs from the terminal. Live-updating dashboard with human-readable schedules and next-run countdowns.

```
SCHEDULED JOBS  ·  hyperion
  JOB                                     NEXT   EVERY  STATUS
  ──────────────────────────────────────  ─────────  ───────────
  Server Patch Report                     in 50m    daily 8:30    ●active  discord
  Subconscious Agent — Daily Self-Audit   in 1h 20m  daily 9:00   ●active  discord
  bootcli-research                       in 1h 50m  daily 9:30   ●active  discord
  Patch Feedback                         in 2h 20m  twice (10:00, 22:00) ●active  discord
  @bootcli Bluesky Poster                in 3h 20m  twice (11:00, 16:00) ●active  discord
  ...

● 07:10:02  ·  13 jobs  ·  ↑↓ navigate  r refresh  q quit
```

## Features

- **Live TUI dashboard** — auto-refreshes every 10s, jobs sorted by next run (soonest first)
- **Human-readable schedules** — `daily 9:30`, `twice (11:00, 16:00)`, `every 12h`, `weekly (Sun)`
- **Next-run countdown** — "in 2h 30m" instead of raw timestamps
- **Simple mode** — plain terminal output, no TUI, script-friendly
- **Error states** — failed jobs shown in red with last error message

## Install

```bash
go install github.com/nnunodev/cronwatch/cmd/cronwatch@latest
```

Requires Go 1.21+.

## SSH setup

cronwatch SSHes to your Hermes host to fetch job data. It needs passwordless SSH access:

```bash
# Add to ~/.ssh/config on the machine running cronwatch
Host hyperion
  HostName 100.102.146.36
  User root
  IdentityFile ~/.ssh/id_ed25519
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
```

Or use the `--key` flag to point to a specific key.

## Flags

```
--host string      Hyperion IP or hostname (default "100.102.146.36")
--user string      SSH user (default "root")
--port int         SSH port (default 22)
--key string       SSH private key path
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

If your Hermes host is slow to respond (e.g. over a VPN with latency):

```bash
cronwatch --timeout 30
```

## How it works

cronwatch SSHes to the Hermes host and runs `hermes cron list`, then parses the output into structured job data:

- `ScheduleHuman` — cron expression converted to human-readable form
- `NextRunHuman` — time until next run ("in 2h 30m")
- `LastState` — "ok" or parsed error message

Sorting is done server-side (soonest job first). Refresh fetches fresh data on every tick.

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

- SSH access to the Hermes host (Hyperion)
- `hermes cron list` command available on the remote host
- Terminal with ANSI color support (most modern terminals)
