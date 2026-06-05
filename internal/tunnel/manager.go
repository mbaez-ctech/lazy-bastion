package tunnel

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"lazy-bastion/internal/config"
)

type Status int

const (
	StatusIdle Status = iota
	StatusStarting
	StatusActive
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusStarting:
		return "Iniciando..."
	case StatusActive:
		return "Activo"
	case StatusError:
		return "Error"
	default:
		return "Cerrado"
	}
}

type Process struct {
	Config  config.Tunnel
	Status  Status
	ErrMsg  string
	StartAt time.Time
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	mu      sync.Mutex
}

type Manager struct {
	procs   []*Process
	profile string
	region  string
	mu      sync.RWMutex
}

func NewManager(profile, region string, tunnels []config.Tunnel) *Manager {
	m := &Manager{profile: profile, region: region}
	for _, t := range tunnels {
		m.procs = append(m.procs, &Process{Config: t, Status: StatusIdle})
	}
	return m
}

func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.procs)
}

// GetStatus returns status and error message for index i.
func (m *Manager) GetStatus(i int) (Status, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if i < 0 || i >= len(m.procs) {
		return StatusIdle, ""
	}
	p := m.procs[i]
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Status, p.ErrMsg
}

// Statuses returns a snapshot of all tunnel statuses.
func (m *Manager) Statuses() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Status, len(m.procs))
	for i, p := range m.procs {
		p.mu.Lock()
		out[i] = p.Status
		p.mu.Unlock()
	}
	return out
}

// Toggle starts or stops tunnel i.
func (m *Manager) Toggle(i int) {
	m.mu.RLock()
	if i < 0 || i >= len(m.procs) {
		m.mu.RUnlock()
		return
	}
	p := m.procs[i]
	m.mu.RUnlock()

	p.mu.Lock()
	s := p.Status
	p.mu.Unlock()

	if s == StatusActive || s == StatusStarting {
		go m.stopProc(p)
	} else {
		go m.startProc(p)
	}
}

// Stop stops tunnel i if it is running.
func (m *Manager) Stop(i int) {
	m.mu.RLock()
	if i < 0 || i >= len(m.procs) {
		m.mu.RUnlock()
		return
	}
	p := m.procs[i]
	m.mu.RUnlock()

	p.mu.Lock()
	s := p.Status
	p.mu.Unlock()

	if s == StatusActive || s == StatusStarting {
		go m.stopProc(p)
	}
}

// StartAll starts all tunnels concurrently.
func (m *Manager) StartAll() {
	m.mu.RLock()
	ps := make([]*Process, len(m.procs))
	copy(ps, m.procs)
	m.mu.RUnlock()
	for _, p := range ps {
		go m.startProc(p)
	}
}

// StopAll stops all tunnels.
func (m *Manager) StopAll() {
	m.mu.RLock()
	ps := make([]*Process, len(m.procs))
	copy(ps, m.procs)
	m.mu.RUnlock()
	for _, p := range ps {
		m.stopProc(p)
	}
}

func (m *Manager) startProc(p *Process) {
	p.mu.Lock()
	if p.Status == StatusStarting || p.Status == StatusActive {
		p.mu.Unlock()
		return
	}
	p.Status = StatusStarting
	p.ErrMsg = ""
	p.StartAt = time.Now()

	// Sweep any orphaned processes on this port (e.g. previous session-manager-plugin
	// that survived after its aws parent exited with a different PGID).
	killOrphansOnPort(p.Config.LocalPort)

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	cmd := exec.CommandContext(ctx, "aws", "ssm", "start-session",
		"--target", p.Config.InstanceID,
		"--document-name", "AWS-StartPortForwardingSession",
		"--parameters", fmt.Sprintf("portNumber=%d,localPortNumber=%d", p.Config.RemotePort, p.Config.LocalPort),
		"--profile", m.profile,
		"--region", m.region,
	)
	setProcGroup(cmd)
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("lzb-%d.log", p.Config.LocalPort))
	if f, err := os.Create(logPath); err == nil {
		cmd.Stdout = f
		cmd.Stderr = f
	}
	p.cmd = cmd
	p.mu.Unlock()

	if err := cmd.Start(); err != nil {
		p.mu.Lock()
		p.Status = StatusError
		p.ErrMsg = err.Error()
		p.mu.Unlock()
		cancel()
		return
	}

	// Process watcher goroutine
	procDone := make(chan error, 1)
	go func() { procDone <- cmd.Wait() }()

	go func() {
		// Wait for port to open (max 30s)
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.NewTimer(30 * time.Second)
		defer deadline.Stop()

		for {
			select {
			case <-ctx.Done():
				p.mu.Lock()
				p.Status = StatusIdle
				p.mu.Unlock()
				return

			case <-procDone:
				p.mu.Lock()
				if p.Status == StatusStarting || p.Status == StatusActive {
					p.Status = StatusError
					p.ErrMsg = "proceso terminó inesperadamente"
				}
				p.mu.Unlock()
				return

			case <-deadline.C:
				p.mu.Lock()
				if p.Status == StatusStarting {
					p.Status = StatusError
					p.ErrMsg = "timeout esperando el puerto"
				}
				p.mu.Unlock()
				// Keep watching for process exit even after timeout
				<-procDone
				return

			case <-ticker.C:
				if isPortOpen(p.Config.LocalPort) {
					p.mu.Lock()
					if p.Status == StatusStarting {
						p.Status = StatusActive
					}
					p.mu.Unlock()
					// Now just wait for process to end or context cancel
					select {
					case <-ctx.Done():
						p.mu.Lock()
						p.Status = StatusIdle
						p.mu.Unlock()
					case <-procDone:
						p.mu.Lock()
						if p.Status == StatusActive {
							p.Status = StatusError
							p.ErrMsg = "proceso terminó inesperadamente"
						}
						p.mu.Unlock()
					}
					return
				}
			}
		}
	}()
}

func (m *Manager) stopProc(p *Process) {
	p.mu.Lock()
	// Kill the process group first so session-manager-plugin (child of aws) is
	// also killed. Cancelling the context afterwards lets the watcher goroutine
	// exit cleanly; the exec.CommandContext auto-kill becomes a harmless no-op.
	if p.cmd != nil && p.cmd.Process != nil {
		killProcGroup(p.cmd.Process.Pid)
	}
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.cmd = nil
	p.Status = StatusIdle
	port := p.Config.LocalPort
	p.mu.Unlock()

	// Sweep synchronously so the orphan is dead before StopAll returns and the
	// process exits (an async goroutine would be killed first on SIGHUP/exit).
	killOrphansOnPort(port)
}

func isPortOpen(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
