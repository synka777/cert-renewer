package main

import (
	"flag"
	"log"
	"os"

	"github.com/synka777/cert-renewer/internal/certbot"
	"github.com/synka777/cert-renewer/internal/config"
	"github.com/synka777/cert-renewer/internal/dns"
	"github.com/synka777/cert-renewer/internal/hosting"
	"github.com/synka777/cert-renewer/internal/services"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")

	authHook := flag.Bool("auth-hook", false, "run as certbot auth hook (called by certbot, not directly)")
	cleanupHook := flag.Bool("cleanup-hook", false, "run as certbot cleanup hook (called by certbot, not directly)")

	// flag.Bool returns a *bool, not a bool.
	// That's why we dereference it with *dryRun everywhere. This is true for all flag package functions.
	// The reason is that flag.Parse() hasn't run yet when the variable is created,
	// so the package needs a pointer it can write the final value into later.
	dryRun := flag.Bool("dry-run", false, "perform a dry run without modifying the certificate")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		// log.Fatalf() calls os.Exit(1) after printing, appropriate to use at a top level
		// function like main() when there's no recovery possible. Never call log.Fatal outside of main.
		// The %v verb stringifies the error, no further inspection is possible compared to %w
		log.Fatalf("failed to load config: %v", err)
	}

	switch {
	case *authHook:
		runAuthHook(cfg)
	case *cleanupHook:
		runCleanupHook(cfg)
	default:
		runOrchestrator(cfg, *dryRun)
	}
}

func runAuthHook(cfg *config.Config) {
	// Certbot sets these env vars before calling the auth hook
	validation := os.Getenv("CERTBOT_VALIDATION")
	if validation == "" {
		log.Fatalf("CERTBOT_VALIDATION env var not set — are you running this directly?")
	}

	client, err := hosting.NewClient(cfg.Hosting.Username, cfg.Hosting.Password)
	if err != nil {
		log.Fatalf("failed to create hosting client: %v", err)
	}

	if err := client.Login(); err != nil {
		log.Fatalf("failed to login: %v", err)
	}

	if err := client.AddTXTRecord(cfg.Domain, validation); err != nil {
		log.Fatalf("failed to add TXT record: %v", err)
	}
	log.Printf("TXT record added, waiting for propagation...")

	if err := dns.WaitForTXT(cfg.Domain, validation); err != nil {
		log.Fatalf("DNS propagation failed: %v", err)
	}

	// Exit 0: certbot proceeds with validation
}

func runCleanupHook(cfg *config.Config) {
	validation := os.Getenv("CERTBOT_VALIDATION")
	if validation == "" {
		log.Fatalf("CERTBOT_VALIDATION env var not set — are you running this directly?")
	}

	client, err := hosting.NewClient(cfg.Hosting.Username, cfg.Hosting.Password)
	if err != nil {
		log.Fatalf("failed to create hosting client: %v", err)
	}

	if err := client.Login(); err != nil {
		log.Fatalf("failed to login: %v", err)
	}

	if err := client.DeleteTXTRecord(cfg.Domain, validation); err != nil {
		// Non-fatal: log the error but don't exit — the cert is already renewed at this point
		log.Printf("warning: failed to delete TXT record: %v", err)
		return
	}

	log.Printf("TXT record removed successfully")
}

func runOrchestrator(cfg *config.Config, dryRun bool) {
	if dryRun {
		log.Printf("--- DRY RUN MODE ---")
	}

	// Get our own executable path so certbot can call us back as the hook
	self, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to determine executable path: %v", err)
	}

	// Pass our config path through to the hook invocations
	configFlag := "-config=" + flag.Lookup("config").Value.String()

	// Step 1: run certbot, which will call back into this binary as auth and cleanup hooks
	if err := certbot.Renew(cfg.Domain, self, configFlag, dryRun); err != nil {
		log.Fatalf("certificate renewal failed: %v", err)
	}

	// Step 2: restart services (skip on dry run)
	if !dryRun {
		if err := services.Restart(cfg.Services); err != nil {
			log.Fatalf("failed to restart services: %v", err)
		}
	} else {
		log.Printf("[dry-run] skipping service restarts")
	}

	log.Printf("done")
	os.Exit(0)
}
