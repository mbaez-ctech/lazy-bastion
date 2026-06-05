//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"lazy-bastion/internal/tunnel"
)

func watchSignals(mgr *tunnel.Manager, p *tea.Program) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		mgr.StopAll()
		p.Kill()
	}()
}
