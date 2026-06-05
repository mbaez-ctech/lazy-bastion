//go:build !windows

package tunnel

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

func killOrphansOnPort(port int) {
	out, err := exec.Command("lsof", "-ti", "-sTCP:LISTEN",
		fmt.Sprintf("tcp:%d", port)).Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && pid > 1 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}
