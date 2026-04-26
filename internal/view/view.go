package view

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nnunodev/cronwatch/internal/ssh"
)

type tickMsg time.Time

type JobsLoadedMsg struct{ Jobs []ssh.Job }
type ErrorMsg struct{ Error string }

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type Model struct {
	jobs           []ssh.Job
	isLoading      bool
	lastRefresh    string
	lastError      string
	selectedIndex  int
	cfg            ssh.Config
	refreshSec     int
	refreshFrame   int
	nextRefresh    time.Time
}

func NewModel(cfg ssh.Config, refreshSec int) *Model {
	return &Model{
		cfg:           cfg,
		refreshSec:    refreshSec,
		selectedIndex: 0,
		isLoading:     true,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.fetchJobs(), tickCmd())
}

func (m *Model) fetchJobs() tea.Cmd {
	return func() tea.Msg {
		jobs, err := ssh.FetchJobs(m.cfg)
		if err != nil {
			return ErrorMsg{Error: err.Error()}
		}
		return JobsLoadedMsg{Jobs: jobs}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case JobsLoadedMsg:
		m.jobs = msg.Jobs
		m.isLoading = false
		m.lastError = ""
		m.lastRefresh = time.Now().Format("15:04:05")
		if m.selectedIndex >= len(m.jobs) {
			m.selectedIndex = 0
		}
		if m.refreshSec > 0 {
			m.nextRefresh = time.Now().Add(time.Duration(m.refreshSec) * time.Second)
		}
		return m, nil

	case ErrorMsg:
		m.lastError = msg.Error
		m.isLoading = false
		if m.refreshSec > 0 {
			m.nextRefresh = time.Now().Add(time.Duration(m.refreshSec) * time.Second)
		}
		return m, nil

	case tickMsg:
		if m.isLoading {
			m.refreshFrame++
		}
		if !m.isLoading && m.refreshSec > 0 && time.Now().After(m.nextRefresh) {
			m.isLoading = true
			m.refreshFrame = 0
			return m, tea.Batch(m.fetchJobs(), tickCmd())
		}
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down", "tab":
			if m.selectedIndex < len(m.jobs)-1 {
				m.selectedIndex++
			}
		case "k", "up", "shift+tab":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "r":
			if !m.isLoading {
				m.isLoading = true
				m.refreshFrame = 0
				return m, m.fetchJobs()
			}
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

// ─── Styles ───────────────────────────────────────────────────────────────

var (
	muted     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4b5563"))
	dimText   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	white     = lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))
	whiteBold = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9fafb")).Bold(true)
	orange    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	cyan      = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	amber     = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	green     = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	greenBold = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	red       = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
	blue      = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true)
	accent    = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
)

// ─── View ─────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	if m.isLoading && len(m.jobs) == 0 {
		return m.loadingView()
	}
	if m.lastError != "" && len(m.jobs) == 0 {
		return m.errorView()
	}
	return m.jobsView()
}

func (m *Model) jobsView() string {
	var b strings.Builder

	if m.lastError != "" {
		b.WriteString(red.Render("  ⚠ " + m.lastError))
		b.WriteString("\n\n")
	}

	b.WriteString(" ")
	b.WriteString(orange.Render("SCHEDULED JOBS"))
	if m.isLoading {
		frames := []rune{'-', '\\', '|', '/'}
		b.WriteString(cyan.Render(fmt.Sprintf("  ⟳ %c", frames[m.refreshFrame%4])))
	}
	b.WriteString(dimText.Render("  ·  " + m.cfg.Host))
	b.WriteString("\n")

	b.WriteString(" ")
	b.WriteString(dimText.Render("  "))
	b.WriteString(whiteBold.Render(pad("JOB", 36)))
	b.WriteString(dimText.Render("  "))
	b.WriteString(whiteBold.Render(center("NEXT", 10)))
	b.WriteString(dimText.Render("   "))
	b.WriteString(whiteBold.Render(center("EVERY", 15)))
	b.WriteString(dimText.Render("  "))
	b.WriteString(whiteBold.Render(center("STATUS", 14)))
	b.WriteString(dimText.Render("  "))
	b.WriteString(whiteBold.Render(center("TRIGGERED", 14)))
	b.WriteString("\n")

	b.WriteString(" ")
	b.WriteString(dimText.Render("  " + strings.Repeat("─", 36) + "  " + strings.Repeat("─", 10) + "  " + strings.Repeat("─", 15) + "  " + strings.Repeat("─", 14) + "  " + strings.Repeat("─", 14)))
	b.WriteString("\n")

	for i, job := range m.jobs {
		b.WriteString(jobRow(job, i == m.selectedIndex))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.lastError != "" {
		b.WriteString(red.Render("● " + m.lastRefresh + " · error"))
		b.WriteString(dimText.Render("  ·  "))
		b.WriteString(dimText.Render(fmt.Sprintf("%d jobs", len(m.jobs))))
		b.WriteString(dimText.Render("  ·  ↑↓ navigate  r refresh  q quit"))
	} else {
		b.WriteString(green.Render("● " + m.lastRefresh))
		b.WriteString(dimText.Render("  ·  "))
		b.WriteString(dimText.Render(fmt.Sprintf("%d jobs", len(m.jobs))))
		b.WriteString(dimText.Render("  ·  ↑↓ navigate  r refresh  q quit"))
	}
	b.WriteString("\n")

	return b.String()
}

func jobRow(job ssh.Job, selected bool) string {
	var dot, statusText string
	if job.State == "running" {
		dot = blue.Render("●")
		statusText = blue.Render("running")
	} else if job.LastState == "error" {
		dot = red.Render("●")
		statusText = red.Render("error")
	} else {
		dot = greenBold.Render("●")
		statusText = greenBold.Render("ok")
	}
	combinedStatus := dot + statusText
	statusWidth := lipgloss.Width(combinedStatus)
	remaining := 14 - statusWidth
	leftPad := remaining / 2
	rightPad := remaining - leftPad
	padStatus := strings.Repeat(" ", leftPad) + combinedStatus + strings.Repeat(" ", rightPad)

	var nextDisplay string
	if job.State == "running" {
		nextDisplay = blue.Render(center("RUNNING", 10))
	} else {
		nextDisplay = cyan.Render(center(job.NextRun, 10))
	}

	nameStyle := white
	if selected {
		nameStyle = accent
	}

	prefix := " "
	if selected {
		prefix = ">"
	}

	triggeredRaw := center(job.LastRunAtH, 14)
	var triggered string
	if selected {
		triggered = accent.Render(triggeredRaw)
	} else {
		triggered = dimText.Render(triggeredRaw)
	}

	var repeatProgress string
	if job.RepeatTimes != nil && *job.RepeatTimes > 0 {
		repeatProgress = dimText.Render(fmt.Sprintf(" [%d/%d]", job.RepeatDone, *job.RepeatTimes))
	}

	return prefix +
		nameStyle.Render(pad(trunc(job.Name, 35), 36)) +
		"  " +
		nextDisplay +
		"  " +
		amber.Render(center(job.Schedule, 15)) +
		"  " +
		padStatus +
		"  " +
		triggered +
		repeatProgress
}

func (m *Model) loadingView() string {
	frames := []rune{'-', '\\', '|', '/'}
	return "\n\n  " + cyan.Render(fmt.Sprintf("loading from %s %c", m.cfg.Host, frames[m.refreshFrame%4])) + "\n\n"
}

func (m *Model) errorView() string {
	return "\n  " + red.Render("✗ "+m.cfg.Host+": "+m.lastError) + "\n\n  " + dimText.Render("r — retry  ·  q — quit\n\n")
}

// RenderSimple prints jobs in plain terminal format.
func RenderSimple(w io.Writer, jobs []ssh.Job) {
	orange := lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	amber := lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true)

	fmt.Fprintln(w)
	fmt.Fprintln(w, orange.Render("  SCHEDULED JOBS"))
	fmt.Fprintln(w, muted.Render("  "+strings.Repeat("─", 70)))

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

		fmt.Fprintf(w, "  %-48s %s  %s\n", j.Name, amber.Render(j.Schedule), cyan.Render(j.NextRun))
		fmt.Fprintf(w, "  %s %s\n\n", statePrefix+state, triggered)
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func trunc(s string, max int) string {
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max-1]) + "…"
	}
	return s
}

func center(s string, width int) string {
	runes := []rune(s)
	slen := len(runes)
	if slen >= width {
		return s
	}
	pad := width - slen
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func pad(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}
