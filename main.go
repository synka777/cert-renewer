package main

import (
	"flag"
	"log"
	"os"

	"github.com/synka777/cert-renewer/internal/config"
	"github.com/synka777/cert-renewer/internal/dns"
	"github.com/synka777/cert-renewer/internal/hosting"
)

func main() {
	configPath := flag.String("config", "config.toml", "./internal/config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		// log.Fatalf() calls os.Exit(1) afer printing, appropriate to use ata top level
		// script like main() when there's no recovery possible. Never call log.Fatal outside of main.
		// The %v verb stringifies the error, no further inspection is possible compafred to %w
		log.Fatalf("failed to load config: %v", err)
	}

	client, err := hosting.NewClient(cfg.Hosting.Username, cfg.Hosting.Password)
	if err != nil {
		log.Fatalf("failed to create hosting client: %v", err)
	}

	if err := client.Login(); err != nil {
		log.Fatalf("failed to login: %v", err)
	}

	// Smoke test: poll for a fake value so we can see the loop in action
	if err := dns.WaitForTXT(cfg.Domain, "fake-test_value"); err != nil {
		log.Printf("poller stopped: %v", err)
	}

	os.Exit(0)
}
