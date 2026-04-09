package ssh

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"; "strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type LoadingMsg     struct{}
type JobsLoadedMsg   struct{ Jobs []Job }
type ErrorMsg       struct{ Error string }

type Config struct {
	Host    string
	User    string
	Port    int
	KeyPath string
}

type Job struct {
	ID             string
	Name           string
	Schedule       string
	ScheduleHuman  string
	NextRun        string
	NextRunHuman   string
	Deliver        string
	DeliverTag     string
	LastRun        string
	LastState      string
}

func FetchJobs(cfg Config) ([]Job, error) {
	cmd := exec.Command("ssh",
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
		"hermes cron list",
	)

	var out bytes.Buffer
	// Silencing SSH warnings — host-key warnings are expected on first connect
	// and don't affect functionality. Real errors still cause cmd.Run to fail.
	cmd.Stdout = &out
	cmd.Stderr = bytes.NewBuffer(nil)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ssh failed: %w", err)
	}

	return ParseJobs(out.String())
}

func ParseJobs(raw string) ([]Job, error) {
	lines := strings.Split(raw, "\n")
	var jobs []Job
	var buf bytes.Buffer

	for _, line := range lines {
		stripped := strings.TrimSpace(line)

		if strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "└") ||
			strings.HasPrefix(line, "│") {
			continue
		}

		if stripped == "" {
			if buf.Len() > 0 {
				if job, err := parseJobBlock(buf.String()); err == nil && job.Name != "" {
					jobs = append(jobs, job)
				}
				buf.Reset()
			}
			continue
		}

		if strings.HasPrefix(stripped, "Scheduled Jobs") {
			continue
		}

		if len(stripped) >= 13 && strings.Contains(stripped, "[active]") {
			if buf.Len() > 0 {
				if job, err := parseJobBlock(buf.String()); err == nil && job.Name != "" {
					jobs = append(jobs, job)
				}
				buf.Reset()
			}
			id := strings.Fields(stripped)[0]
			buf.WriteString("ID: " + id + "\n")
			continue
		}

		if strings.HasPrefix(stripped, "Name:") {
			buf.WriteString(strings.TrimPrefix(stripped, "Name:") + "\n")
			continue
		}
		if strings.HasPrefix(stripped, "Schedule:") {
			buf.WriteString("SCHEDULE|" + strings.TrimPrefix(stripped, "Schedule:") + "\n")
			continue
		}
		if strings.HasPrefix(stripped, "Next run:") {
			buf.WriteString("NEXT|" + strings.TrimPrefix(stripped, "Next run:") + "\n")
			continue
		}
		if strings.HasPrefix(stripped, "Deliver:") {
			buf.WriteString("DELIVER|" + strings.TrimPrefix(stripped, "Deliver:") + "\n")
			continue
		}
		if strings.HasPrefix(stripped, "Last run:") {
			buf.WriteString("LAST|" + strings.TrimPrefix(stripped, "Last run:") + "\n")
			continue
		}
	}

	if buf.Len() > 0 {
		if job, err := parseJobBlock(buf.String()); err == nil && job.Name != "" {
			jobs = append(jobs, job)
		}
	}

	jobs = sortJobsByNextRun(jobs); return jobs, nil
}

func parseJobBlock(block string) (Job, error) {
	job := Job{}
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID:") {
			job.ID = strings.TrimSpace(strings.TrimPrefix(line, "ID:"))
		} else if strings.HasPrefix(line, "SCHEDULE|") {
			job.Schedule = strings.TrimSpace(strings.TrimPrefix(line, "SCHEDULE|"))
			job.ScheduleHuman = cronToHuman(job.Schedule)
		} else if strings.HasPrefix(line, "NEXT|") {
			nextRaw := strings.TrimSpace(strings.TrimPrefix(line, "NEXT|"))
			job.NextRun = nextRaw
			job.NextRunHuman = humanDuration(nextRaw)
		} else if strings.HasPrefix(line, "DELIVER|") {
			job.Deliver = strings.TrimSpace(strings.TrimPrefix(line, "DELIVER|"))
			job.DeliverTag = deliverTag(job.Deliver)
		} else if strings.HasPrefix(line, "LAST|") {
			val := strings.TrimPrefix(line, "LAST|")
			parts := strings.SplitN(val, "  ", 2)
			job.LastRun = strings.TrimSpace(parts[0])
			if len(parts) >= 2 {
				state := strings.TrimSpace(parts[1])
				if strings.HasPrefix(state, "ok") {
					job.LastState = "ok"
				} else if strings.HasPrefix(state, "error") {
					job.LastState = "error"
				} else {
					job.LastState = state
				}
			}
		} else if line != "" {
			job.Name += line + " "
		}
	}

	job.Name = strings.TrimSpace(job.Name)
	return job, nil
}

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

func RenderSimple(jobs []Job) {
	orange := lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	amber := lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)

	fmt.Println()
	fmt.Println(orange.Render("  SCHEDULED JOBS"))
	fmt.Println(muted.Render("  " + strings.Repeat("─", 65)))
	for _, j := range jobs {
		state := green.Render("ok")
		if j.LastState == "error" {
			state = red.Render("error")
		}
		fmt.Printf("  %-45s %s  %s\n", j.Name, amber.Render(j.ScheduleHuman), cyan.Render(j.NextRunHuman))
		fmt.Printf("  %s  %s\n\n", muted.Render("→ "+j.DeliverTag), state)
	}
}

func sortJobsByNextRun(jobs []Job) []Job {
	sorted := make([]Job, len(jobs))
	copy(sorted, jobs)
	sort.Slice(sorted, func(i, j int) bool {
		ti, err := time.Parse(time.RFC3339, sorted[i].NextRun)
		if err != nil {
			ti2, err2 := time.Parse("2006-01-02T15:04:05Z07:00", sorted[i].NextRun)
			if err2 != nil {
				return false
			}
			ti = ti2
		}
		tj, err := time.Parse(time.RFC3339, sorted[j].NextRun)
		if err != nil {
			tj2, err2 := time.Parse("2006-01-02T15:04:05Z07:00", sorted[j].NextRun)
			if err2 != nil {
				return false
			}
			tj = tj2
		}
		return ti.Before(tj)
	})
	return sorted
}
