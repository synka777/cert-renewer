package certbot

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// Renew runs certbot in manual DNS mode. It points --manual-auth-hook and
// --manual-cleanup-hook back at our own binary, which handles DNS and cleanup.
func Renew(domain, hookBinary, configFlag string, dryRun bool) error {
	log.Printf("running certbot renew for %s (dry-run: %v)", domain, dryRun)

	args := []string{
		"renew",
		"--manual",
		"--preferred-challenges", "dns",
		"--manual-auth-hook", hookBinary + " --auth-hook " + configFlag,
		"--manual-cleanup-hook", hookBinary + " --cleanup-hook " + configFlag,
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
