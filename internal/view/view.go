package view

import (
	"fmt"
	"regexp"
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

var (
	bgStyle            = lipgloss.NewStyle().Background(lipgloss.Color("#0b1020"))
	titleStyle         = lipgloss.NewStyle().Background(lipgloss.Color("#0b1020")).Foreground(lipgloss.Color("#f97316")).Bold(true)
	cardStyle          = lipgloss.NewStyle().Background(lipgloss.Color("#0f1923")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#1f3b5b")).Padding(0, 1)
	cardSelectedStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#172033")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#22d3ee")).Padding(0, 1)
	jobNameStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#f8fafc")).Bold(true)
	jobNameMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af"))
	statusActiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	statusErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
	cronStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	nextRunStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	footerStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	labelStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	loadingStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee")).Bold(true)
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	headerAccentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	dotActiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	dotErrorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	greenStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
)

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

	b.WriteString(bgStyle.Render("\n"))
	b.WriteString(titleStyle.Render("  SCHEDULED JOBS  ") + "  ")
	b.WriteString(headerAccentStyle.Render("when") + footerStyle.Render(" — hyperion scheduler"))
	b.WriteString(bgStyle.Render("\n\n"))

	b.WriteString(labelStyle.Render("  "))
	b.WriteString(truncate("NAME", 42))
	b.WriteString(labelStyle.Render("  "))
	b.WriteString(truncate("SCHEDULE", 14))
	b.WriteString(labelStyle.Render("  "))
	b.WriteString(truncate("NEXT RUN", 22))
	b.WriteString(labelStyle.Render("  "))
	b.WriteString(truncate("STATUS", 9))
	b.WriteString(labelStyle.Render("  "))
	b.WriteString(truncate("DELIVER", 18))
	b.WriteString(bgStyle.Render("\n"))

	b.WriteString(labelStyle.Render("  ") +
		strings.Repeat("─", 42) + "  " +
		strings.Repeat("─", 14) + "  " +
		strings.Repeat("─", 22) + "  " +
		strings.Repeat("─", 9) + "  " +
		strings.Repeat("─", 18))
	b.WriteString(bgStyle.Render("\n"))

	for i, job := range m.jobs {
		style := cardStyle
		if i == m.selectedIndex {
			style = cardSelectedStyle
		}

		dot := dotActiveStyle.Render("●")
		status := statusActiveStyle.Render("ACTIVE")
		if job.LastState == "error" {
			dot = dotErrorStyle.Render("●")
			status = statusErrorStyle.Render("ERROR")
		}

		name := jobNameStyle.Render(truncate(job.Name, 42))
		if job.LastState == "error" {
			name = jobNameMutedStyle.Render(truncate(job.Name, 42))
		}

		b.WriteString(style.Render("  "))
		b.WriteString(name)
		b.WriteString(style.Render("  "))
		b.WriteString(cronStyle.Render(truncate(job.Schedule, 14)))
		b.WriteString(style.Render("  "))
		b.WriteString(nextRunStyle.Render(truncate(job.NextRun, 22)))
		b.WriteString(style.Render("  "))
		b.WriteString(dot + status)
		b.WriteString(style.Render("  "))
		b.WriteString(footerStyle.Render(truncate(formatDeliver(job.Deliver), 18)))
		b.WriteString(bgStyle.Render("\n"))

		meta := fmt.Sprintf("    last: %s  %s", job.LastRun, job.LastState)
		b.WriteString(labelStyle.Render(meta) + bgStyle.Render("\n"))
	}

	b.WriteString(bgStyle.Render("\n"))

	refreshHint := "r refresh  q quit"
	if m.refreshSec > 0 {
		refreshHint = fmt.Sprintf("auto-refresh %ds  r refresh  q quit", m.refreshSec)
	}
	b.WriteString(footerStyle.Render("  ") + greenStyle.Render("● "+m.lastRefresh))
	b.WriteString(footerStyle.Render("  ·  "))
	b.WriteString(footerStyle.Render(fmt.Sprintf("%d jobs", len(m.jobs))))
	b.WriteString(footerStyle.Render("  ·  " + refreshHint))
	b.WriteString(bgStyle.Render("\n"))

	return b.String()
}

func (m *Model) loadingView() string {
	frames := []rune{'-', '\\', '|', '/'}
	var b strings.Builder
	b.WriteString(bgStyle.Render("\n\n"))
	b.WriteString(loadingStyle.Render(fmt.Sprintf("  Loading from Hyperion %c", frames[m.refreshFrame%4])))
	b.WriteString(bgStyle.Render("\n\n"))
	return b.String()
}

func (m *Model) errorView() string {
	var b strings.Builder
	b.WriteString(bgStyle.Render("\n"))
	b.WriteString(errorStyle.Render("  ✗ "+m.lastError))
	b.WriteString(bgStyle.Render("\n\n"))
	b.WriteString(footerStyle.Render("  Press r to retry  ·  q to quit"))
	b.WriteString(bgStyle.Render("\n"))
	return b.String()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max-1]) + "…"
	}
	return s
}

var deliverDiscordRx = regexp.MustCompile(`discord:(\d+)`)

func formatDeliver(d string) string {
	m := deliverDiscordRx.FindStringSubmatch(d)
	if len(m) == 2 {
		runes := []rune(m[1])
		if len(runes) > 8 {
			return "discord:" + string(runes[:8]) + "…"
		}
		return "discord:" + m[1]
	}
	return d
}
