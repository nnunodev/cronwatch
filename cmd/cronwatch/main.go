package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nnunodev/cronwatch/internal/ssh"
	"github.com/nnunodev/cronwatch/internal/view"
)

var version = "dev"

func main() {
	host := flag.String("host", "hyperion", "SSH host alias or IP")
	user := flag.String("user", "", "SSH user")
	port := flag.Int("port", 0, "SSH port")
	key := flag.String("key", "", "SSH private key path")
	simple := flag.Bool("simple", false, "Simple terminal output")
	refresh := flag.Int("refresh", 10, "Auto-refresh interval in seconds (0=disabled)")
	timeout := flag.Int("timeout", 10, "SSH command timeout in seconds")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cronwatch %s\n", version)
		return
	}

	// Track which flags were explicitly set by the user so SSH config
	// only fills in gaps, never overrides CLI arguments.
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	cfg := ssh.Config{
		Host:    *host,
		User:    *user,
		Port:    *port,
		KeyPath: *key,
		Timeout: *timeout,
	}

	// Auto-discover from ~/.ssh/config (first-match, like OpenSSH)
	if sshCfg, err := ssh.ReadSSHConfig(*host); err == nil {
		if !setFlags["user"] && sshCfg.User != "" {
			cfg.User = sshCfg.User
		}
		if !setFlags["port"] && sshCfg.Port != 0 {
			cfg.Port = sshCfg.Port
		}
		if !setFlags["key"] && sshCfg.IdentityFile != "" {
			cfg.KeyPath = sshCfg.IdentityFile
		}
		// If SSH config has a HostName, connect there instead of the alias
		if sshCfg.HostName != "" {
			cfg.Host = sshCfg.HostName
		}
	}

	// Final fallback defaults
	if cfg.User == "" {
		cfg.User = "nuno"
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}

	if *simple {
		jobs, err := ssh.FetchJobs(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		ssh.RenderSimple(jobs)
		return
	}

	model := view.NewModel(cfg, *refresh)
	p := tea.NewProgram(model)

	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
