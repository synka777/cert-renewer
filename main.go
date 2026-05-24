package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/synka777/cert-renewer/internal/config"
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

	fmt.Printf("Loaded config for domain: %s \n", cfg.Domain)
	fmt.Printf("Services to restart: %v\n", cfg.Services)
	os.Exit(0)
}
