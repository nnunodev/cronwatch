package ssh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type LoadingMsg   struct{}
type JobsLoadedMsg struct{ Jobs []Job }
type ErrorMsg     struct{ Error string }

type Config struct {
	Host    string
	User    string
	Port    int
	KeyPath string
	Timeout int // SSH command timeout in seconds
}

type Job struct {
	ID            string // e.g. "4bde57c2ee7a"
	Name          string // "Hermes Memory Backup"
	State         string // "scheduled", "running", "paused"
	Schedule      string // human: "daily 9:00"
	ScheduleExpr  string // raw: "0 9 * * *"
	NextRun       string // human: "in 2h 30m"
	NextRunAt     string // raw ISO: "2026-04-10T03:00:00+01:00"
	LastRunAt     string // raw ISO: "2026-04-09T03:01:00+01:00"
	LastRunAtH    string // human: "09:01", relative "today 03:01"
	LastState     string // "ok", "error", "running"
	LastError     string // error message if LastState == "error"
	Deliver       string // full: "discord:1488469942272790629"
	DeliverTag    string // short: "discord"
	RepeatTimes   *int64 // null if unlimited
	RepeatDone    int64  // completed run count
	Enabled       bool
}

// FetchJobs fetches job list from Hermes via SSH, parsing the jobs.json dump
func FetchJobs(cfg Config) ([]Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	// Dump jobs.json as clean JSON over SSH — avoids text parsing entirely
	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "LogLevel=ERROR",
		"-p", fmt.Sprintf("%d", cfg.Port),
	)
	if cfg.KeyPath != "" {
		cmd.Args = append(cmd.Args, "-i", cfg.KeyPath)
	}
	cmd.Args = append(cmd.Args,
		fmt.Sprintf("%s@%s", cfg.User, cfg.Host),
		"python3 -c \"import json; print(json.dumps(json.load(open('/root/.hermes/cron/jobs.json'))))\"",
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = bytes.NewBuffer(nil)

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("ssh timeout after %ds", cfg.Timeout)
		}
		return nil, fmt.Errorf("ssh failed: %w", err)
	}

	return ParseJobs(out.String())
}

// rawJob matches the Hermes jobs.json structure
type rawJob struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	State    string `json:"state"`
	Enabled  bool   `json:"enabled"`
	Schedule struct {
		Expr    string `json:"expr"`
		Display string `json:"display"`
	} `json:"schedule"`
	ScheduleDisplay string  `json:"schedule_display"`
	NextRunAt       string  `json:"next_run_at"`
	LastRunAt       string  `json:"last_run_at"`
	LastStatus      string  `json:"last_status"`
	LastError       *string `json:"last_error"`
	Deliver         string  `json:"deliver"`
	Repeat          struct {
		Times     *int64 `json:"times"`
		Completed int64  `json:"completed"`
	} `json:"repeat"`
}

type rawJobs struct {
	Jobs []rawJob `json:"jobs"`
}

func ParseJobs(raw string) ([]Job, error) {
	var rj rawJobs
	if err := json.Unmarshal([]byte(raw), &rj); err != nil {
		return nil, fmt.Errorf("failed to parse jobs.json: %w", err)
	}

	jobs := make([]Job, 0, len(rj.Jobs))
	for _, r := range rj.Jobs {
		if !r.Enabled {
			continue
		}
		j := Job{
			ID:           r.ID,
			Name:         r.Name,
			State:        r.State,
			ScheduleExpr: r.Schedule.Expr,
			NextRunAt:    r.NextRunAt,
			LastRunAt:    r.LastRunAt,
			LastState:    r.LastStatus,
			RepeatTimes:  r.Repeat.Times,
			RepeatDone:   r.Repeat.Completed,
			Enabled:      r.Enabled,
		}

		// Schedule human
		if r.ScheduleDisplay != "" {
			j.Schedule = cronToHuman(r.Schedule.Expr)
			if j.Schedule == r.Schedule.Expr {
				j.Schedule = r.ScheduleDisplay
			}
		} else {
			j.Schedule = cronToHuman(r.Schedule.Expr)
		}

		// Next run human
		j.NextRun = humanDuration(j.NextRunAt)

		// Last run human (relative time + clock)
		j.LastRunAtH = lastRunHuman(j.LastRunAt)

		// Last error
		if r.LastError != nil {
			j.LastError = *r.LastError
		}

		// Deliver tag
		j.Deliver = r.Deliver
		j.DeliverTag = deliverTag(r.Deliver)

		jobs = append(jobs, j)
	}

	jobs = sortJobs(jobs)
	return jobs, nil
}

// sortJobs: running first, then by NextRunAt ascending
func sortJobs(jobs []Job) []Job {
	sorted := make([]Job, len(jobs))
	copy(sorted, jobs)
	sort.Slice(sorted, func(i, j int) bool {
		// Running always first
		if sorted[i].State == "running" && sorted[j].State != "running" {
			return true
		}
		if sorted[i].State != "running" && sorted[j].State == "running" {
			return false
		}
		// Both running: sort by NextRunAt
		if sorted[i].State == "running" && sorted[j].State == "running" {
			return sorted[i].NextRunAt < sorted[j].NextRunAt
		}
		// Otherwise: sort by NextRunAt ascending
		ti, err := time.Parse(time.RFC3339, sorted[i].NextRunAt)
		if err != nil {
			return false
		}
		tj, err := time.Parse(time.RFC3339, sorted[j].NextRunAt)
		if err != nil {
			return true
		}
		return ti.Before(tj)
	})
	return sorted
}

// cronToHuman converts cron expr to human-readable string
func cronToHuman(schedule string) string {
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
		return "twice (" + strings.Join(formatted, ", ") + ")"
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
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	diff := time.Until(t)
	if diff < 0 {
		return "now"
	}

	h := int(diff.Hours())
	m := int(diff.Minutes()) % 60

	if h > 24 {
		d := h / 24
		h = h % 24
		if h > 0 {
			return fmt.Sprintf("in %dd %dh", d, h)
		}
		return fmt.Sprintf("in %dd", d)
	}
	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("in %dh %dm", h, m)
		}
		return fmt.Sprintf("in %dh", h)
	}
	if m > 0 {
		return fmt.Sprintf("in %dm", m)
	}
	return "in <1m"
}

// lastRunHuman returns just the clock time
func lastRunHuman(ts string) string {
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	return t.In(time.Now().Location()).Format("15:04")
}

func deliverTag(d string) string {
	if strings.HasPrefix(d, "discord:") {
		return "discord"
	}
	if d == "local" {
		return "local"
	}
	runes := []rune(d)
	if len(runes) > 10 {
		return string(runes[:10])
	}
	return d
}

// RenderSimple prints jobs in plain terminal format
func RenderSimple(jobs []Job) {
	orange := lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	amber := lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true)

	fmt.Println()
	fmt.Println(orange.Render("  SCHEDULED JOBS"))
	fmt.Println(muted.Render("  " + strings.Repeat("─", 70)))

	for _, j := range jobs {
		state := green.Render("ok")
		statePrefix := "  "
		if j.State == "running" {
			state = blue.Render("running")
			statePrefix = "● "
		} else if j.LastState == "error" || j.State == "paused" {
			state = red.Render(j.LastState)
			statePrefix = "● "
		}

		triggered := muted.Render(j.LastRunAtH)

		fmt.Printf("  %-48s %s  %s\n", j.Name, amber.Render(j.Schedule), cyan.Render(j.NextRun))
		fmt.Printf("  %s %s\n\n", statePrefix+state, triggered)
	}
}
