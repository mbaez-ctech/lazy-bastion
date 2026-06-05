//go:build windows

package ssh

import (
	"os/exec"
	"strconv"
)

func setProcGroup(cmd *exec.Cmd) {}

func (p *SSMProcess) Kill() error {
	if p.proc == nil {
		return nil
	}
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(p.proc.Pid)).Run()
}
