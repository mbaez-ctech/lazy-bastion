//go:build !windows

package ssh

import (
	"os/exec"
	"syscall"
)

func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func (p *SSMProcess) Kill() error {
	if p.proc == nil {
		return nil
	}
	return syscall.Kill(-p.proc.Pid, syscall.SIGKILL)
}
