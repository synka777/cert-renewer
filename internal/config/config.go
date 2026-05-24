package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Domain   string        `toml:"domain"`
	Hosting  HostingConfig `toml:"hosting"`
	Certbot  CertbotConfig `toml:"certbot"`
	Services []string      `toml:"services"`
}

type HostingConfig struct {
	Username string `toml:"username"`
	Password string `toml:"password"`
}

type CertbotConfig struct {
	Email   string `toml:"email"`
	WebRoot string `toml:"webroot"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// %w: verb that wraps the original error, meaning users can us errors.Is() or errors.As() to inspect the underlying error later.
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if cfg.Hosting.Username == "" || cfg.Hosting.Password == "" {
		return fmt.Errorf("hosting username and password are required")
	}
	if cfg.Certbot.Email == "" {
		return fmt.Errorf("certbot email is required")
	}
	return nil
}
