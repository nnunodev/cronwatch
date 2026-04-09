package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type LoadingMsg     struct{}
type JobsLoadedMsg   struct{ Jobs []Job }
type ErrorMsg       struct{ Error string }
type RefreshTickMsg struct{ Frame int }

type Config struct {
	Host    string
	User    string
	Port    int
	KeyPath string
	Refresh int
}

type Job struct {
	ID           string
	Name         string
	Schedule     string
	Repeat       string
	NextRun      string
	NextRunHuman string // "in 2h 30m"
	Deliver      string
	DeliverTag   string
	LastRun      string
	LastState    string
}

func FetchJobs(cfg Config) ([]Job, error) {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
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
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

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
		if strings.HasPrefix(stripped, "Repeat:") {
			buf.WriteString("REPEAT|" + strings.TrimPrefix(stripped, "Repeat:") + "\n")
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

	return jobs, nil
}

func parseJobBlock(block string) (Job, error) {
	job := Job{}
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID:") {
			job.ID = strings.TrimSpace(strings.TrimPrefix(line, "ID:"))
		} else if strings.HasPrefix(line, "SCHEDULE|") {
			job.Schedule = strings.TrimSpace(strings.TrimPrefix(line, "SCHEDULE|"))
		} else if strings.HasPrefix(line, "REPEAT|") {
			job.Repeat = strings.TrimSpace(strings.TrimPrefix(line, "REPEAT|"))
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

// humanDuration converts an ISO timestamp to "in Xh Ym" or "in Nm"
func humanDuration(ts string) string {
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", ts)
	if err != nil {
		// Try alternative layout
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
		id := strings.TrimPrefix(d, "discord:")
		runes := []rune(id)
		if len(runes) > 8 {
			return "discord"
		}
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
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	cron := lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	next := lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	err := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)

	fmt.Println()
	fmt.Println(style.Render("  SCHEDULED JOBS"))
	fmt.Println(muted.Render("  " + strings.Repeat("─", 72)))
	for _, j := range jobs {
		state := ok.Render("ok")
		if j.LastState == "error" {
			state = err.Render("error")
		}
		fmt.Printf("  %-45s %s  %s\n", j.Name, cron.Render(j.Schedule), next.Render(j.NextRunHuman))
		fmt.Printf("  %s  %s\n\n", muted.Render("→ "+j.DeliverTag), state)
	}
}
