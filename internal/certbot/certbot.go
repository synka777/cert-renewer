package certbot

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

const (
	authHookPath    = "/tmp/cert-renewer-auth-hook.sh"
	cleanupHookPath = "/tmp/cert-renewer-cleanup-hook.sh"
	hookScript      = "#!/bin/sh\nexit 0\n"
)

// Renew runs certbot in manual DNS mode. By the time this is called,
// the TXT record must already be propagated.
func Renew(domain string, dryRun bool) error {
	if err := writeHooks(); err != nil {
		return err
	}

	// defer removeHooks() ensures the temp scripts are cleaned up even if certbot fails.
	defer removeHooks()

	log.Printf("running certbot renew for %s (dry-run: %v)", domain, dryRun)
	args := []string{
		"certbot", "renew",
		"--manual",
		"--preferred-challenges", "dns",
		"--manual-auth-hook", authHookPath,
		"--manual-cleanup-hook", cleanupHookPath,
		"--cert-name", domain, // Tells certbot which certificate to renew by name (matching what's in /etc/letsencrypt/live/).
		"--non-interactive",
	}

	if dryRun {
		args = append(args, "--dry-run")
	}

	// exec.Command is Go's way of running external processes. It doesn't invoke a shell,
	// it executes the binary directly with the arguments you pass as separate strings.
	// This is safer than shell invocation (no injection risks) but means you can't use shell features like pipes or globs directly.
	cmd := exec.Command("certbot", args...)

	// Wires certbot's output directly to our process's stdout/stderr.
	// Without this, certbot's output is silently swallowed, you'd have no idea what it's doing.
	// For a long-running command like certbot this is important.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// cmd.Run() vs cmd.Start()
	// Run() blocks until the process exits and returns an error if the exit code is non-zero.
	// Start() launches the process and returns immediately, letting you do other things while it runs and call cmd.Wait() later.
	// We use Run() here since we want to wait for certbot to finish before proceeding.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("certbot renew failed: %w", err)
	}

	log.Printf("certbot renew completed successfully")
	return nil
}

func writeHooks() error {
	for _, path := range []string{authHookPath, cleanupHookPath} {
		if err := os.WriteFile(path, []byte(hookScript), 0755); err != nil {
			return fmt.Errorf("writing hook script %s: %w", path, err)
		}
	}
	return nil
}

func removeHooks() {
	for _, path := range []string{authHookPath, cleanupHookPath} {
		if err := os.Remove(path); err != nil {
			// Only log a warning if removal fails rather than returning an error; this is intentional.
			// Cleanup failures are worth knowing about but shouldn't mask the real error that caused the function to return.
			log.Printf("warning: failed to remove hook script %s: %v", path, err)
		}
	}
}
