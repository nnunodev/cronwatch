package ssh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host    string
	User    string
	Port    int
	KeyPath string
	Timeout int // SSH command timeout in seconds
}

type Job struct {
	Name        string    // e.g. "Memory Backup"
	State       string    // "scheduled", "running", "paused"
	Schedule    string    // human: "daily 9:00"
	NextRun     string    // human: "in 2h 30m"
	NextRunAt   string    // raw ISO: "2026-04-10T03:00:00+01:00"
	nextRun     time.Time // parsed NextRunAt; used for sorting
	LastRunAt   string    // raw ISO: "2026-04-09T03:01:00+01:00"
	LastRunAtH  string    // human: "09:01", relative "today 03:01"
	LastState   string    // "ok", "error", "running"
	LastError   string    // error message if LastState == "error"
	RepeatTimes *int64    // null if unlimited
	RepeatDone  int64     // completed run count
	Enabled     bool
}

// FetchJobs fetches job list from a Hermes agent via SSH, parsing the jobs.json dump
func FetchJobs(cfg Config) ([]Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	connectTimeout := cfg.Timeout / 2
	if connectTimeout < 2 {
		connectTimeout = 2
	}
	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "LogLevel=ERROR",
		"-o", fmt.Sprintf("ConnectTimeout=%d", connectTimeout),
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=2",
		"-p", strconv.Itoa(cfg.Port),
	)
	if cfg.KeyPath != "" {
		cmd.Args = append(cmd.Args, "-i", cfg.KeyPath)
	}
	cmd.Args = append(cmd.Args,
		fmt.Sprintf("%s@%s", cfg.User, cfg.Host),
		"python3 -c \"import json, os; print(json.dumps(json.load(open(os.path.expanduser('~/.hermes/cron/jobs.json')))))\"",
	)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("ssh timeout after %ds (host: %s:%d)", cfg.Timeout, cfg.Host, cfg.Port)
		}
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ssh failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("ssh failed: %w", err)
	}

	return ParseJobs(out.Bytes())
}

// rawJob matches the Hermes agent jobs.json structure
type rawJob struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Enabled  bool   `json:"enabled"`
	Schedule struct {
		Expr string `json:"expr"`
	} `json:"schedule"`
	ScheduleDisplay string  `json:"schedule_display"`
	NextRunAt       string  `json:"next_run_at"`
	LastRunAt       string  `json:"last_run_at"`
	LastStatus      string  `json:"last_status"`
	LastError       *string `json:"last_error"`
	Repeat          struct {
		Times     *int64 `json:"times"`
		Completed int64  `json:"completed"`
	} `json:"repeat"`
}

type rawJobs struct {
	Jobs []rawJob `json:"jobs"`
}

func ParseJobs(data []byte) ([]Job, error) {
	var rj rawJobs
	if err := json.Unmarshal(data, &rj); err != nil {
		return nil, fmt.Errorf("failed to parse jobs.json: %w", err)
	}

	jobs := make([]Job, 0, len(rj.Jobs))
	for _, r := range rj.Jobs {
		if !r.Enabled {
			continue
		}
		j := Job{
			Name:        r.Name,
			State:       r.State,
			NextRunAt:   r.NextRunAt,
			LastRunAt:   r.LastRunAt,
			LastState:   r.LastStatus,
			RepeatTimes: r.Repeat.Times,
			RepeatDone:  r.Repeat.Completed,
			Enabled:     r.Enabled,
		}

		// Schedule human
		if r.ScheduleDisplay != "" {
			j.Schedule = humanizeSchedule(r.Schedule.Expr)
			if j.Schedule == r.Schedule.Expr {
				j.Schedule = r.ScheduleDisplay
			}
		} else {
			j.Schedule = humanizeSchedule(r.Schedule.Expr)
		}

		// Next run human
		j.NextRun = humanDuration(j.NextRunAt)
		j.nextRun, _ = time.Parse(time.RFC3339, r.NextRunAt)

		// Last run human (relative time + clock)
		j.LastRunAtH = lastRunHuman(j.LastRunAt)

		// Last error
		if r.LastError != nil {
			j.LastError = *r.LastError
		}

		jobs = append(jobs, j)
	}

	sortJobs(jobs)
	return jobs, nil
}

// sortJobs: running first, then by NextRunAt ascending.
func sortJobs(jobs []Job) {
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].State == "running" && jobs[j].State != "running" {
			return true
		}
		if jobs[i].State != "running" && jobs[j].State == "running" {
			return false
		}
		return jobs[i].nextRun.Before(jobs[j].nextRun)
	})
}

// humanizeSchedule converts a cron expr to a human-readable string.
func humanizeSchedule(schedule string) string {
	fields := strings.Fields(schedule)
	if len(fields) < 5 {
		return schedule
	}

	min := fields[0]
	hour := fields[1]
	dom := fields[2]
	month := fields[3]
	dow := fields[4]

	if min == "0" && strings.HasPrefix(hour, "*/") {
		return fmt.Sprintf("every %sh", strings.TrimPrefix(hour, "*/"))
	}
	if min == "0" && strings.Contains(hour, ",") {
		var formatted []string
		for _, p := range strings.Split(hour, ",") {
			if len(p) == 1 {
				p = "0" + p
			}
			formatted = append(formatted, fmt.Sprintf("%s:00", p))
		}
		return "(" + strings.Join(formatted, ", ") + ")"
	}
	if dom == "*" && month == "*" && dow != "*" {
		return fmt.Sprintf("weekly (%s)", dow)
	}
	if dom == "*" && month == "*" && dow == "*" {
		if min == "0" {
			return fmt.Sprintf("daily %s:00", hour)
		}
		return fmt.Sprintf("daily %s:%s", hour, min)
	}
	return schedule
}

// humanDuration converts ISO timestamp to "in Xh Ym" or "in Nm"
func humanDuration(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	diff := time.Until(t)
	if diff < 0 {
		return "overdue"
	}

	h := int(diff.Hours())
	m := int(diff.Minutes()) % 60

	if h >= 24 {
		d := h / 24
		h = h % 24
		if h > 0 && m > 0 {
			return fmt.Sprintf("%dd %dh %dm", d, h, m)
		}
		if h > 0 {
			return fmt.Sprintf("%dd %dh", d, h)
		}
		if m > 0 {
			return fmt.Sprintf("%dd %dm", d, m)
		}
		return fmt.Sprintf("%dd", d)
	}
	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return "<1m"
}

// lastRunHuman returns just the clock time
func lastRunHuman(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.In(time.Now().Location()).Format("15:04")
}


