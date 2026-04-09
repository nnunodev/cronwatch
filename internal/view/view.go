package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nnunodev/cronwatch/internal/ssh"
)

type Model struct {
	jobs           []ssh.Job
	isLoading     bool
	lastRefresh   string
	lastError     string
	selectedIndex int
	cfg           ssh.Config
	refreshSec    int
	refreshFrame  int
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
	return m.fetchJobs()
}

func (m *Model) fetchJobs() tea.Cmd {
	return func() tea.Msg {
		jobs, err := ssh.FetchJobs(m.cfg)
		if err != nil {
			return ssh.ErrorMsg{Error: err.Error()}
		}
		return ssh.JobsLoadedMsg{Jobs: jobs}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ssh.JobsLoadedMsg:
		m.jobs = msg.Jobs
		m.isLoading = false
		m.lastError = ""
		m.lastRefresh = time.Now().Format("15:04:05")
		if m.refreshSec > 0 {
			return m, m.autoRefresh()
		}
		return m, nil

	case ssh.ErrorMsg:
		m.lastError = msg.Error
		m.isLoading = false
		return m, nil

	case ssh.RefreshTickMsg:
		m.refreshFrame++
		if !m.isLoading && m.refreshSec > 0 {
			m.isLoading = true
			return m, m.fetchJobs()
		}
		return m, nil

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
			m.isLoading = true
			m.refreshFrame = 0
			return m, m.fetchJobs()
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) autoRefresh() tea.Cmd {
	return tea.Tick(time.Duration(m.refreshSec)*time.Second, func(t time.Time) tea.Msg {
		return ssh.RefreshTickMsg{}
	})
}

// ─── Styles ───────────────────────────────────────────────────────────────

var (
	bg          = lipgloss.NewStyle().Background(lipgloss.Color("#0b1020"))
	muted       = lipgloss.NewStyle().Foreground(lipgloss.Color("#4b5563"))
	dimText     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	white       = lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))
	whiteBold   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9fafb")).Bold(true)
	orange      = lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	cyan        = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	amber       = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	green       = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	greenBold   = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	red         = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
	accent      = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	tagStyle    = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d1d5db")).
			Background(lipgloss.Color("#1f2937")).
			Padding(0, 1).
			MarginLeft(1)
	tagErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fecaca")).
			Background(lipgloss.Color("#7f1d1d")).
			Padding(0, 1).
			MarginLeft(1)
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

	// Header
	b.WriteString(bg.Render(" "))
	b.WriteString(orange.Render("SCHEDULED JOBS"))
	b.WriteString(dimText.Render("  ·  hyperion"))
	b.WriteString(bg.Render("\n"))

	// Column labels
	b.WriteString(bg.Render(" "))
	b.WriteString(dimText.Render("  "))
	b.WriteString(whiteBold.Render(pad("JOB", 38)))
	b.WriteString(dimText.Render("  "))
	b.WriteString(whiteBold.Render("NEXT"))
	b.WriteString(dimText.Render("   "))
	b.WriteString(whiteBold.Render(center("EVERY", 13)))
	b.WriteString(dimText.Render("  "))
	b.WriteString(whiteBold.Render(center("STATUS", 7)))
	b.WriteString(bg.Render("\n"))

	// Divider
	b.WriteString(bg.Render(" "))
	b.WriteString(dimText.Render("  " + strings.Repeat("─", 38) + "  " + strings.Repeat("─", 9) + "  " + center(strings.Repeat("─", 13), 13) + "  " + center(strings.Repeat("─", 7), 7)))
	b.WriteString(bg.Render("\n"))

	// Jobs
	for i, job := range m.jobs {
		b.WriteString(jobRow(job, i == m.selectedIndex))
		b.WriteString(bg.Render("\n"))
	}

	// Footer
	b.WriteString(bg.Render("\n"))
	b.WriteString(green.Render("● "+m.lastRefresh))
	b.WriteString(dimText.Render("  ·  "))
	b.WriteString(dimText.Render(fmt.Sprintf("%d jobs", len(m.jobs))))
	b.WriteString(dimText.Render("  ·  ↑↓ navigate  r refresh  q quit"))
	b.WriteString(bg.Render("\n"))

	return b.String()
}

func jobRow(job ssh.Job, selected bool) string {
	// Status
	dot := greenBold.Render("●")
	statusText := greenBold.Render("ok")
	if job.LastState == "error" {
		dot = red.Render("●")
		statusText = red.Render("error")
	}

	// Target tag
	tag := tagStyle.Render(job.DeliverTag)
	if job.LastState == "error" {
		tag = tagErrorStyle.Render("✗")
	}

	// Every column
	every := job.ScheduleHuman

	// Name
	nameStyle := white
	if selected {
		nameStyle = accent
	}

	prefix := " "
	if selected {
		prefix = ">"
	}

	return bg.Render(prefix) +
		nameStyle.Render(pad(trunc(job.Name, 37), 38)) +
		bg.Render("  ") +
		cyan.Render(pad(job.NextRunHuman, 9)) +
		bg.Render("  ") +
		amber.Render(pad(every, 13)) +
		bg.Render("  ") +
		dot +
		statusText +
		tag
}

func (m *Model) loadingView() string {
	frames := []rune{'-', '\\', '|', '/'}
	return bg.Render("\n\n  ") + cyan.Render(fmt.Sprintf("loading from hyperion %c", frames[m.refreshFrame%4])) + bg.Render("\n\n")
}

func (m *Model) errorView() string {
	return bg.Render("\n  ") + red.Render("✗ "+m.lastError) + bg.Render("\n\n  ") + dimText.Render("r — retry  ·  q — quit\n\n")
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



