package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazy-bastion/internal/config"
	sshpkg "lazy-bastion/internal/ssh"
	"lazy-bastion/internal/tunnel"
)

// ─── Messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time

type sshReadyMsg struct {
	serverIdx  int
	tunnelProc interface{ Kill() error }
}

type sshDoneMsg struct {
	tunnelProc interface{ Kill() error }
	err        error
}

type cmdErrMsg struct {
	ctx string
	err error
}

// ─── Model ───────────────────────────────────────────────────────────────────

type App struct {
	cfg     *config.Config
	mgr     *tunnel.Manager
	profile string

	cursor     int
	nTunnels   int
	nServers   int
	totalItems int

	spinner    spinner.Model
	width      int
	height     int
	statusMsg  string
	errMsg     string
	connecting bool
}

func NewApp(cfg *config.Config, profile string, mgr *tunnel.Manager) *App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = startingStyle

	return &App{
		cfg:        cfg,
		mgr:        mgr,
		profile:    profile,
		nTunnels:   len(cfg.Tunnels),
		nServers:   len(cfg.Servers),
		totalItems: len(cfg.Tunnels) + len(cfg.Servers),
		spinner:    s,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.spinner.Tick, tick())
}

func tick() tea.Cmd {
	return tea.Tick(1200*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		if a.connecting {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q":
			a.mgr.StopAll()
			return a, tea.Quit

		case "up", "k":
			if a.cursor > 0 {
				a.cursor--
			}

		case "down", "j":
			if a.cursor < a.totalItems-1 {
				a.cursor++
			}

		case "enter", " ":
			if a.cursor < a.nTunnels {
				a.mgr.Toggle(a.cursor)
				a.statusMsg = fmt.Sprintf("Toggle: %s", a.cfg.Tunnels[a.cursor].Name)
				a.errMsg = ""
			} else {
				srvIdx := a.cursor - a.nTunnels
				a.connecting = true
				a.statusMsg = fmt.Sprintf("Preparando %s...", a.cfg.Servers[srvIdx].Name)
				a.errMsg = ""
				cmds = append(cmds, a.prepareSSH(srvIdx))
			}

		case "d", "D":
			if a.cursor < a.nTunnels {
				a.mgr.Stop(a.cursor)
				a.statusMsg = fmt.Sprintf("Matando: %s", a.cfg.Tunnels[a.cursor].Name)
				a.errMsg = ""
			}

		case "a", "A":
			a.mgr.StartAll()
			a.statusMsg = "Abriendo todos los túneles..."
			a.errMsg = ""

		case "x", "X":
			a.mgr.StopAll()
			a.statusMsg = "Cerrando todos los túneles..."
			a.errMsg = ""
		}

	case tickMsg:
		cmds = append(cmds, tick())

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case sshReadyMsg:
		a.connecting = false
		srv := a.cfg.Servers[msg.serverIdx]
		sshCmd := sshpkg.BuildSSHCmd(srv, a.cfg.AWS.SSHKey)
		tunnelProc := msg.tunnelProc
		cmds = append(cmds, tea.ExecProcess(sshCmd, func(err error) tea.Msg {
			return sshDoneMsg{tunnelProc: tunnelProc, err: err}
		}))

	case sshDoneMsg:
		if msg.tunnelProc != nil {
			_ = msg.tunnelProc.Kill()
		}
		a.connecting = false
		if msg.err != nil {
			a.errMsg = fmt.Sprintf("SSH: %v", msg.err)
		} else {
			a.statusMsg = "Sesión SSH terminada."
		}

	case cmdErrMsg:
		a.connecting = false
		a.errMsg = fmt.Sprintf("%s: %v", msg.ctx, msg.err)
	}

	return a, tea.Batch(cmds...)
}

// prepareSSH is a background tea.Cmd that registers the key and opens the SSM
// tunnel to port 22, then returns sshReadyMsg when ready.
func (a *App) prepareSSH(serverIdx int) tea.Cmd {
	srv := a.cfg.Servers[serverIdx]
	profile := a.profile
	region := a.cfg.AWS.Region
	keyPath := a.cfg.AWS.SSHKey

	return func() tea.Msg {
		if err := sshpkg.EnsureKey(keyPath); err != nil {
			return cmdErrMsg{"generando llave SSH", err}
		}
		ctx := context.Background()
		if err := sshpkg.RegisterKey(ctx, profile, region, srv.InstanceID, srv.Name, keyPath); err != nil {
			return cmdErrMsg{"registrando llave en " + srv.Name, err}
		}
		proc, err := sshpkg.StartSSHTunnel(ctx, profile, region, srv)
		if err != nil {
			return cmdErrMsg{"abriendo túnel SSH", err}
		}
		if err := sshpkg.WaitForPort(srv.TunnelPort, 30*time.Second); err != nil {
			_ = proc.Kill()
			return cmdErrMsg{"esperando túnel " + srv.Name, err}
		}
		return sshReadyMsg{serverIdx: serverIdx, tunnelProc: proc}
	}
}

// ─── View ────────────────────────────────────────────────────────────────────

func (a *App) View() string {
	if a.width == 0 {
		return "  Cargando...\n"
	}

	var b strings.Builder
	inner := a.width - 4 // usable width inside 2-char left padding

	// ── Header ──
	title := fmt.Sprintf("Lazzy Bastion  ·  %s  ·  %s", a.cfg.AWS.Region, a.profile)
	b.WriteString(headerStyle.Width(a.width).Render("  "+title+"  "))
	b.WriteString("\n\n")

	// ── TUNNELS section ──
	shortcuts := helpKeyStyle.Render("a") + helpDescStyle.Render(" abrir todos  ") +
		helpKeyStyle.Render("x") + helpDescStyle.Render(" cerrar todos")
	secLabel := sectionStyle.Render("TÚNELES")
	pad := inner - lipgloss.Width(secLabel) - lipgloss.Width(shortcuts)
	if pad < 1 {
		pad = 1
	}
	b.WriteString("  " + secLabel + strings.Repeat(" ", pad) + shortcuts + "\n")
	b.WriteString("  " + dividerStyle.Render(strings.Repeat("─", inner)) + "\n")

	statuses := a.mgr.Statuses()
	for i, t := range a.cfg.Tunnels {
		st := tunnel.StatusIdle
		if i < len(statuses) {
			st = statuses[i]
		}
		b.WriteString(a.tunnelRow(i, t, st))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// ── SERVERS section ──
	b.WriteString("  " + sectionStyle.Render("SERVIDORES SSH") + "\n")
	b.WriteString("  " + dividerStyle.Render(strings.Repeat("─", inner)) + "\n")

	for i, srv := range a.cfg.Servers {
		b.WriteString(a.serverRow(a.nTunnels+i, srv))
		b.WriteString("\n")
	}

	// ── Status / error line ──
	if a.connecting {
		b.WriteString("\n  " + a.spinner.View() + " " + startingStyle.Render(a.statusMsg))
	} else if a.errMsg != "" {
		b.WriteString("\n  " + errorStyle.Render("✗  "+a.errMsg))
	} else if a.statusMsg != "" {
		b.WriteString("\n  " + infoStyle.Render(a.statusMsg))
	} else {
		b.WriteString("\n")
	}

	// ── Footer ──
	footer := helpKeyStyle.Render("↑↓/jk") + helpDescStyle.Render(" navegar  ") +
		helpKeyStyle.Render("↵/Space") + helpDescStyle.Render(" toggle / conectar  ") +
		helpKeyStyle.Render("d") + helpDescStyle.Render(" matar túnel  ") +
		helpKeyStyle.Render("a") + helpDescStyle.Render(" abrir todos  ") +
		helpKeyStyle.Render("x") + helpDescStyle.Render(" cerrar todos  ") +
		helpKeyStyle.Render("q") + helpDescStyle.Render(" salir")
	b.WriteString("\n" + footerStyle.Width(a.width).Render(footer))

	return b.String()
}

func (a *App) tunnelRow(idx int, t config.Tunnel, s tunnel.Status) string {
	cur := " "
	if a.cursor == idx {
		cur = "▶"
	}

	var dot, statusTxt string
	switch s {
	case tunnel.StatusActive:
		dot = activeStyle.Render("●")
		statusTxt = activeStyle.Render("ACTIVO")
	case tunnel.StatusStarting:
		dot = a.spinner.View()
		statusTxt = startingStyle.Render("INICIANDO...")
	case tunnel.StatusError:
		_, emsg := a.mgr.GetStatus(idx)
		dot = errorStyle.Render("✗")
		statusTxt = errorStyle.Render("ERROR")
		if emsg != "" {
			statusTxt += " " + idleStyle.Render("("+truncate(emsg, 35)+")")
		}
	default:
		dot = idleStyle.Render("○")
		statusTxt = idleStyle.Render("CERRADO")
	}

	name := padRight(t.Name, 28)
	ports := portStyle.Render(fmt.Sprintf("%5d → %-5d", t.RemotePort, t.LocalPort))
	group := labelStyle.Render(padRight(t.Group, 8))

	line := fmt.Sprintf("  %s  %s  %s  %s  %s  %s",
		cur, name, group, ports, dot, statusTxt)

	if a.cursor == idx {
		return selectedStyle.Render(line)
	}
	return line
}

func (a *App) serverRow(idx int, srv config.Server) string {
	cur := " "
	if a.cursor == idx {
		cur = "▶"
	}

	hint := helpDescStyle.Render("↵ SSH")
	line := fmt.Sprintf("  %s  %-10s  %-30s  %-24s  %s",
		cur, srv.Name, srv.Label, srv.InstanceID, hint)

	if a.cursor == idx {
		return selectedStyle.Render(line)
	}
	return line
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
