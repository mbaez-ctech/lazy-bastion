package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DetectProfile finds the AWS CLI profile that points to accountID.
// Respects the LZB_PROFILE env var as an override.
func DetectProfile(ctx context.Context, accountID string) (string, error) {
	if p := os.Getenv("LZB_PROFILE"); p != "" {
		return p, nil
	}

	out, err := exec.CommandContext(ctx, "aws", "configure", "list-profiles").Output()
	if err != nil {
		return "", fmt.Errorf("listando perfiles AWS: %w", err)
	}

	profiles := strings.Fields(string(out))

	// Pass 1: static config (no network)
	for _, p := range profiles {
		if profileMatchesAccount(ctx, p, accountID) {
			return p, nil
		}
	}

	// Pass 2: live STS check
	for _, p := range profiles {
		o, err := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity",
			"--profile", p, "--query", "Account", "--output", "text").Output()
		if err == nil && strings.TrimSpace(string(o)) == accountID {
			return p, nil
		}
	}

	return "", fmt.Errorf("no se encontró perfil AWS para la cuenta %s", accountID)
}

func profileMatchesAccount(ctx context.Context, profile, accountID string) bool {
	for _, key := range []string{"sso_account_id", "account_id"} {
		o, err := exec.CommandContext(ctx, "aws", "configure", "get", key, "--profile", profile).Output()
		if err == nil && strings.TrimSpace(string(o)) == accountID {
			return true
		}
	}
	return false
}

// ValidateSession returns true if the SSO session for the profile is active.
func ValidateSession(ctx context.Context, profile string) bool {
	return exec.CommandContext(ctx, "aws", "sts", "get-caller-identity",
		"--profile", profile).Run() == nil
}

// Login runs an interactive SSO browser login.
func Login(ctx context.Context, profile string) error {
	cmd := exec.CommandContext(ctx, "aws", "sso", "login", "--profile", profile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetCallerARN returns the caller ARN for the given profile.
func GetCallerARN(ctx context.Context, profile string) (string, error) {
	o, err := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity",
		"--profile", profile, "--query", "Arn", "--output", "text").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(o)), nil
}

// CheckGroup validates that the ARN belongs to Admin_Tech or BastionAccess.
func CheckGroup(arn string) error {
	switch {
	case strings.Contains(arn, "AWSReservedSSO_Admin_Tech_"):
		return nil
	case strings.Contains(arn, "AWSReservedSSO_BastionAccess_"):
		return nil
	case strings.Contains(arn, ":user/"):
		return nil
	default:
		return fmt.Errorf("identidad %q no tiene acceso (requiere Admin_Tech o BastionAccess)", arn)
	}
}
