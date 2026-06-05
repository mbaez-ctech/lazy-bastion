//go:build windows

package tunnel

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func setProcGroup(cmd *exec.Cmd) {}

func killProcGroup(pid int) {
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

func killOrphansOnPort(port int) {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return
	}
	target := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, target) || !strings.Contains(line, "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pidStr := strings.TrimSpace(fields[len(fields)-1])
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 1 {
			_ = exec.Command("taskkill", "/F", "/T", "/PID", pidStr).Run()
		}
	}
}
