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

// Styles — no borders, just clean colored text
var (
	bg              = lipgloss.NewStyle().Background(lipgloss.Color("#0b1020"))
	dim             = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	dimText         = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	mutedText       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af"))
	white           = lipgloss.NewStyle().Foreground(lipgloss.Color("#f3f4f6"))
	whiteBold       = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9fafb")).Bold(true)
	orange          = lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	cyan            = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	amber           = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	green           = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	greenBold       = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	red             = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
	accent          = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))
	selectedBg      = lipgloss.NewStyle().Background(lipgloss.Color("#172033"))
	divider         = lipgloss.NewStyle().Foreground(lipgloss.Color("#1f3b5b"))
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

	// ── Header ──────────────────────────────────────────
	b.WriteString(bg.Render(" "))
	b.WriteString(orange.Render("SCHEDULED JOBS"))
	b.WriteString(dimText.Render("  ·  hyperion cronwatch"))
	b.WriteString(bg.Render("\n"))

	// Column headers
	b.WriteString(bg.Render(" "))
	b.WriteString(whiteBold.Render(padRight("NAME", 44)))
	b.WriteString(whiteBold.Render(padRight("SCHEDULE", 14)))
	b.WriteString(whiteBold.Render(padRight("NEXT RUN", 26)))
	b.WriteString(whiteBold.Render(padRight("STATUS", 8)))
	b.WriteString(whiteBold.Render("DELIVER"))
	b.WriteString(bg.Render("\n"))

	b.WriteString(bg.Render(" "))
	b.WriteString(dim.Render(strings.Repeat("─", 44) + "  " + strings.Repeat("─", 14) + "  " + strings.Repeat("─", 26) + "  " + strings.Repeat("─", 8) + "  " + strings.Repeat("─", 24)))
	b.WriteString(bg.Render("\n"))

	// ── Job rows ───────────────────────────────────────
	for i, job := range m.jobs {
		prefix := " "
		nameStyle := white
		if i == m.selectedIndex {
			prefix = ">"
			nameStyle = accent
		}

		dot := green.Render("●")
		status := greenBold.Render("ACTIVE")
		if job.LastState == "error" {
			dot = red.Render("●")
			status = red.Render("ERROR")
			if i != m.selectedIndex {
				nameStyle = dimText
			}
		}

		name := nameStyle.Render(padRight(trunc(job.Name, 43), 44))
		sched := amber.Render(padRight(job.Schedule, 14))
		next := cyan.Render(padRight(trunc(job.NextRun, 25), 26))
		deliver := mutedText.Render(trunc(formatDeliver(job.Deliver), 24))

		b.WriteString(bg.Render(prefix))
		b.WriteString(name)
		b.WriteString(sched)
		b.WriteString(next)
		b.WriteString(dot)
		b.WriteString(status)
		b.WriteString("  ")
		b.WriteString(deliver)
		b.WriteString(bg.Render("\n"))

		// last run meta on next line, indented
		lastState := dimText.Render(job.LastState)
		if job.LastState == "error" {
			lastState = red.Render(job.LastState)
		}
		b.WriteString(bg.Render(" "))
		b.WriteString(dimText.Render("  last:  " + job.LastRun))
		b.WriteString("  ")
		b.WriteString(lastState)
		b.WriteString(bg.Render("\n"))
	}

	// ── Footer ────────────────────────────────────────
	b.WriteString(bg.Render(" "))
	b.WriteString(green.Render("● "+m.lastRefresh))
	b.WriteString(dimText.Render("  ·  "))
	b.WriteString(dimText.Render(fmt.Sprintf("%d jobs", len(m.jobs))))
	b.WriteString(dimText.Render("  ·  "))
	b.WriteString(dimText.Render("↑↓ navigate  r refresh  q quit"))
	b.WriteString(bg.Render("\n"))

	return b.String()
}

func (m *Model) loadingView() string {
	frames := []rune{'-', '\\', '|', '/'}
	b := bg.Render("\n\n  ") + cyan.Render(fmt.Sprintf("Loading from Hyperion %c", frames[m.refreshFrame%4])) + bg.Render("\n\n")
	return b
}

func (m *Model) errorView() string {
	b := bg.Render("\n  ") + red.Render("✗ "+m.lastError) + bg.Render("\n\n  ") + dimText.Render("r — retry  ·  q — quit\n\n")
	return b
}

func trunc(s string, max int) string {
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max-1]) + "…"
	}
	return s
}

func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}

var deliverDiscordRx = regexp.MustCompile(`discord:(\d+)`)

func formatDeliver(d string) string {
	m := deliverDiscordRx.FindStringSubmatch(d)
	if len(m) == 2 {
		runes := []rune(m[1])
		if len(runes) > 10 {
			return "discord:" + string(runes[:10]) + "…"
		}
		return "discord:" + m[1]
	}
	return d
}
