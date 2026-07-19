package services

import (
	"fmt"
	"log"
	"os/exec"
)

// Restart reloads each service via systemctl so they pick up the new certificate.
func Restart(services []string) error {
	for _, service := range services {
		log.Printf("reloading service: %s", service)
		if err := reload(service); err != nil {
			return err
		}
	}
	return nil
}

func reload(service string) error {
	// reload-or-restart is more appropriate than a plain restart for a web server;
	// it tells systemctl to send a reload signal first (apache gracefully reloads its config and picks up
	// new certs without dropping active connections), falling back to a full restart only if the service doesn't support reloading.
	// A plain restart kills the process immediately, which is fine for a low-traffic VPS but sloppy practice.
	cmd := exec.Command("systemctl", "reload-or-restart", service)

	// cmd.CombinedOutput() is an alternative to cmd.Run() — it captures both stdout and stderr into a single []byte and returns it alongside the error. We use it here instead of wiring to os.Stdout because systemctl's output is short and we want to include it in the error message if something goes wrong, rather than just printing it to the terminal.
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("reloading %s: %w (output: %s)", service, err, string(out))
	}

	log.Printf("reloaded %s successfully", service)
	return nil
}
