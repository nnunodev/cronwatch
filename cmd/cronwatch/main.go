package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/bubbletea"
	"github.com/nnunodev/cronwatch/internal/ssh"
	"github.com/nnunodev/cronwatch/internal/view"
)

func main() {
	host := flag.String("host", "100.102.146.36", "Hyperion IP or hostname")
	user := flag.String("user", "root", "SSH user")
	port := flag.Int("port", 22, "SSH port")
	key := flag.String("key", "", "SSH private key path")
	simple := flag.Bool("simple", false, "Simple terminal output")
	refresh := flag.Int("refresh", 10, "Auto-refresh interval in seconds (0=disabled)")
	flag.Parse()

	cfg := ssh.Config{
		Host:    *host,
		User:    *user,
		Port:    *port,
		KeyPath: *key,
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

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		p.Quit()
	}()

	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
