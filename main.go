package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	awspkg "lazy-bastion/internal/aws"
	"lazy-bastion/internal/config"
	"lazy-bastion/internal/tunnel"
	"lazy-bastion/internal/tui"
)

func main() {
	cfgPath := findConfig()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatalf("Error cargando config (%s): %v\n", cfgPath, err)
	}

	ctx := context.Background()

	// ── Detect / validate AWS profile ──
	fmt.Println("Detectando perfil AWS...")

	profile := cfg.AWS.Profile
	if profile == "" {
		profile, err = awspkg.DetectProfile(ctx, cfg.AWS.AccountID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌  %v\n", err)
			fmt.Fprintf(os.Stderr, "    Configura tu perfil:  aws configure sso  (account id %s)\n", cfg.AWS.AccountID)
			fmt.Fprintf(os.Stderr, "    O pon tu perfil en config.yml:  profile: <nombre>\n")
			os.Exit(1)
		}
	}
	fmt.Printf("   Perfil: %s (cuenta %s)\n", profile, cfg.AWS.AccountID)

	// ── Validate / refresh SSO session ──
	if !awspkg.ValidateSession(ctx, profile) {
		fmt.Printf("Sesión SSO expirada. Iniciando login (perfil %s)...\n", profile)
		if err := awspkg.Login(ctx, profile); err != nil {
			fatalf("Login falló: %v\n", err)
		}
	}
	fmt.Println("   Sesión activa ✓")

	// ── Launch TUI ──
	mgr := tunnel.NewManager(profile, cfg.AWS.Region, cfg.Tunnels)
	app := tui.NewApp(cfg, profile, mgr)

	p := tea.NewProgram(app, tea.WithAltScreen())

	// Ensure tunnels are killed when the terminal closes or the process is
	// terminated externally (SIGHUP = terminal closed, SIGTERM = kill/systemd).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		mgr.StopAll()
		p.Kill()
	}()

	if _, err := p.Run(); err != nil {
		fatalf("Error en TUI: %v\n", err)
	}

	mgr.StopAll()
}

func findConfig() string {
	if p := os.Getenv("LZB_CONFIG"); p != "" {
		return p
	}
	// Next to the binary
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "config.yml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Current directory
	if _, err := os.Stat("config.yml"); err == nil {
		return "config.yml"
	}
	// ~/.config/lazy-bastion/config.yml
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".config", "lazy-bastion", "config.yml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "config.yml"
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "❌  "+format, args...)
	os.Exit(1)
}
