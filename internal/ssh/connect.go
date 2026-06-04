package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"lazy-bastion/internal/config"
)

// SSMProcess wraps an SSM tunnel process and kills the entire process group on Kill,
// ensuring the session-manager-plugin child is also terminated.
type SSMProcess struct {
	proc *os.Process
}

func (p *SSMProcess) Kill() error {
	if p.proc == nil {
		return nil
	}
	return syscall.Kill(-p.proc.Pid, syscall.SIGKILL)
}

// EnsureKey creates an Ed25519 SSH key at keyPath if it doesn't already exist.
func EnsureKey(keyPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return os.Chmod(keyPath, 0600)
	}

	user := os.Getenv("USER")
	if user == "" {
		user = "user"
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "host"
	}
	comment := fmt.Sprintf("lzb-%s@%s", user, hostname)

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", comment, "-f", keyPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-keygen: %w", err)
	}
	return os.Chmod(keyPath, 0600)
}

// RegisterKey pushes the public key to the ubuntu user's authorized_keys via SSM.
// The operation is idempotent.
func RegisterKey(ctx context.Context, profile, region, instanceID, serverName, keyPath string) error {
	pubBytes, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return fmt.Errorf("leyendo llave pública: %w", err)
	}
	pubKey := strings.TrimSpace(string(pubBytes))

	commands := []string{
		"install -d -m700 -o ubuntu -g ubuntu /home/ubuntu/.ssh",
		"touch /home/ubuntu/.ssh/authorized_keys",
		fmt.Sprintf(
			"grep -qxF '%s' /home/ubuntu/.ssh/authorized_keys || echo '%s' >> /home/ubuntu/.ssh/authorized_keys",
			pubKey, pubKey,
		),
		"chown ubuntu:ubuntu /home/ubuntu/.ssh/authorized_keys",
		"chmod 600 /home/ubuntu/.ssh/authorized_keys",
	}

	params, err := json.Marshal(map[string]interface{}{"commands": commands})
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "lzb-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(params); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	out, err := exec.CommandContext(ctx, "aws", "ssm", "send-command",
		"--profile", profile,
		"--region", region,
		"--instance-ids", instanceID,
		"--document-name", "AWS-RunShellScript",
		"--parameters", "file://"+tmp.Name(),
		"--query", "Command.CommandId",
		"--output", "text",
	).Output()
	if err != nil {
		return fmt.Errorf("send-command: %w", err)
	}
	cmdID := strings.TrimSpace(string(out))

	for range 20 {
		time.Sleep(2 * time.Second)
		st, err := exec.CommandContext(ctx, "aws", "ssm", "get-command-invocation",
			"--profile", profile,
			"--region", region,
			"--command-id", cmdID,
			"--instance-id", instanceID,
			"--query", "Status",
			"--output", "text",
		).Output()
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(st)) {
		case "Success":
			return nil
		case "Failed", "Cancelled", "TimedOut":
			return fmt.Errorf("registro de llave: %s", strings.TrimSpace(string(st)))
		}
	}
	return fmt.Errorf("timeout esperando registro de llave")
}

// StartSSHTunnel opens an SSM port forwarding tunnel to port 22 of the instance.
// Returns an SSMProcess that kills the entire process group on Kill.
func StartSSHTunnel(ctx context.Context, profile, region string, srv config.Server) (*SSMProcess, error) {
	cmd := exec.CommandContext(ctx, "aws", "ssm", "start-session",
		"--target", srv.InstanceID,
		"--document-name", "AWS-StartPortForwardingSession",
		"--parameters", fmt.Sprintf("portNumber=22,localPortNumber=%d", srv.TunnelPort),
		"--profile", profile,
		"--region", region,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logPath := fmt.Sprintf("/tmp/lzb-ssh-%s.log", srv.Name)
	if f, err := os.Create(logPath); err == nil {
		cmd.Stdout = f
		cmd.Stderr = f
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("abriendo túnel SSH: %w", err)
	}
	return &SSMProcess{proc: cmd.Process}, nil
}

// WaitForPort blocks until localhost:port is reachable or timeout expires.
func WaitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("timeout esperando el puerto %d", port)
}

// BuildSSHCmd returns an exec.Cmd ready to connect to the server over the SSM tunnel.
func BuildSSHCmd(srv config.Server, keyPath string) *exec.Cmd {
	return exec.Command("ssh",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", fmt.Sprintf("%d", srv.TunnelPort),
		fmt.Sprintf("%s@127.0.0.1", srv.SSHUser),
	)
}
