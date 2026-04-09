package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Msg types for Bubble Tea
type LoadingMsg       struct{}
type JobsLoadedMsg     struct{ Jobs []Job }
type ErrorMsg         struct{ Error string }
type RefreshTickMsg   struct{ Frame int }

type Config struct {
	Host    string
	User    string
	Port    int
	KeyPath string
	Refresh int
}

type Job struct {
	ID        string
	Name      string
	Schedule  string
	Repeat    string
	NextRun   string
	Deliver   string
	LastRun   string
	LastState string
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

		// Skip decorative lines
		if strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "└") ||
			strings.HasPrefix(line, "│") {
			continue
		}

		// Empty line — flush buffer if we have content
		if stripped == "" {
			if buf.Len() > 0 {
				if job, err := parseJobBlock(buf.String()); err == nil && job.Name != "" {
					jobs = append(jobs, job)
				}
				buf.Reset()
			}
			continue
		}

		// Skip header
		if strings.HasPrefix(stripped, "Scheduled Jobs") {
			continue
		}

		// Job ID line: "  4bde57c2ee7a [active]"
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
			job.NextRun = strings.TrimSpace(strings.TrimPrefix(line, "NEXT|"))
		} else if strings.HasPrefix(line, "DELIVER|") {
			job.Deliver = strings.TrimSpace(strings.TrimPrefix(line, "DELIVER|"))
		} else if strings.HasPrefix(line, "LAST|") {
			// Format: "Last run:  2026-04-08T15:43:33.134405+01:00  ok"
			// or:      "Last run:  2026-04-08T15:43:33.134405+01:00  error: message..."
			val := strings.TrimPrefix(line, "LAST|")
			parts := strings.SplitN(val, "  ", 2)
			job.LastRun = strings.TrimSpace(parts[0])
			if len(parts) >= 2 {
				state := strings.TrimSpace(parts[1])
				// Extract just "ok" or "error" prefix
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

func RenderSimple(jobs []Job) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	cron := lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	next := lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	err := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)

	fmt.Println()
	fmt.Println(style.Render("  SCHEDULED JOBS"))
	fmt.Println(muted.Render("  " + strings.Repeat("─", 65)))
	for _, j := range jobs {
		state := ok.Render("ok")
		if j.LastState == "error" {
			state = err.Render("error")
		}
		fmt.Printf("  %-45s %s\n", j.Name, cron.Render(j.Schedule))
		fmt.Printf("    Next: %s  %s\n\n", next.Render(j.NextRun), state)
	}
}
