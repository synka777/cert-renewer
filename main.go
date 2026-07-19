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

	// flag.Bool returns a *bool, not a bool.
	// That's why we dereference it with *dryRun everywhere. This is true for all flag package functions.
	// The reason is that flag.Parse() hasn't run yet when the variable is created,
	// so the package needs a pointer it can write the final value into later.
	dryRun := flag.Bool("dry-run", false, "perform a dry run without modifying the certificate")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		// log.Fatalf() calls os.Exit(1) afer printing, appropriate to use ata top level
		// script like main() when there's no recovery possible. Never call log.Fatal outside of main.
		// The %v verb stringifies the error, no further inspection is possible compafred to %w
		log.Fatalf("failed to load config: %v", err)
	}

	if *dryRun {
		log.Printf("--- DRY RUN MODE ---")
	}

	// Step 1: login to 1984 Hosting
	client, err := hosting.NewClient(cfg.Hosting.Username, cfg.Hosting.Password)
	if err != nil {
		log.Fatalf("failed to create hosting client: %v", err)
	}

	if err := client.Login(); err != nil {
		log.Fatalf("failed to login to 1984 Hosting: %v", err)
	}

	// Step 2: add the TXT record
	// In real usage certbot provides the validation value via $CERTBOT_VALIDATION.
	// For now we use a placeholder; commit 7 will wire up the real value.
	challengeValue := "placeholder-challenge-value"

	if !*dryRun {
		if err := client.AddTXTRecord(cfg.Domain, challengeValue); err != nil {
			log.Fatalf("failed to add TXT record: %v", err)
		}
		log.Printf("TXT record added for %s", cfg.Domain)
	} else {
		log.Printf("[dry-run] skipping TXT record creation")
	}

	// Step 3: wait for DNS propagation
	if !*dryRun {
		if err := dns.WaitForTXT(cfg.Domain, challengeValue); err != nil {
			log.Fatalf("DNS propagation failed: %v", err)
		}
	} else {
		log.Printf("[dry-run] skipping DNS propagation check")
	}

	// Step 4: run certbot (--dry-run is passed through if set)
	if err := certbot.Renew(cfg.Domain, *dryRun); err != nil {
		log.Fatalf("certificate renewal failed: %v", err)
	}

	// Step 5: restart services (skip on dry run)
	if !*dryRun {
		if err := services.Restart(cfg.Services); err != nil {
			log.Fatalf("failed to restart services: %v", err)
		}
	} else {
		log.Printf("[dry-run] skipping service restarts")
	}

	// Step 6: remove the TXT record
	if !*dryRun {
		if err := client.DeleteTXTRecord(cfg.Domain, challengeValue); err != nil {
			// Non-fatal: log the error but don't exit — the cert is already renewed
			log.Printf("warning: failed to delete TXT record: %v", err)
		} else {
			log.Printf("TXT record removed for %s", cfg.Domain)
		}
	} else {
		log.Printf("[dry-run] skipping TXT record deletion")
	}

	log.Printf("done")
	os.Exit(0)
}
