package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	pollInterval = 15 * time.Second
	pollTimeout  = 10 * time.Minute
)

// WaitForTXT blocks until the expected TXT record is visible in DNS,
// or until the tiemout is reached
func WaitForTXT(domain, expectedValue string) error {
	// context.WithTimeout is Go's standard way to put a deadline on any operation. It returns a new context and a cancel function.
	ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
	defer cancel() // <= this is important. Even if the context times out, we have to releases the resources that were allocated to it.

	fqdn := "_acme-challenge." + domain
	log.Printf("polling DNS for TXT record to propagate on %s every %s (timeout: %s)", fqdn, pollInterval, pollTimeout)

	// time.NewTicker vs time.Sleep. We could write the poll loop with time.Sleep(pollInterval)
	// but a Ticker is more correct — it ticks on a fixed schedule regardless of how long the work inside the loop took.
	// With Sleep, drift accumulates. Always defer ticker.Stop() to release the underlying goroutine when you're done with it.
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		// The select statement is one of Go's most distinctive features.
		// It blocks until one of its case channels is ready, then executes that branch — similar to a switch but for channel operations.
		// Here it's listening on two channels simultaneously: ctx.Done() (closed when the timeout fires) and ticker.C (receives the current time every 15 seconds).
		// Whichever fires first wins. This is how Go handles concurrent waiting without threads or callbacks.
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for TXT record to propagate on %s", fqdn)

		case <-ticker.C:
			found, err := checkTXT(fqdn, expectedValue)
			if err != nil {
				log.Printf("DNS lookup error (will retry): %v", err)
				continue
			}
			if found {
				log.Printf("TXT record propagated successfully")
				return nil
			}
			log.Printf("TXT record not yet visible, retrying in %s...", pollInterval)
		}
	}
}

func checkTXT(fqdn, expectedValue string) (bool, error) {
	records, err := net.LookupTXT(fqdn)
	if err != nil {
		// DNS error type assertion err.(*net.DNSError)
		// this is Go's way of inspecting the concrete type behind an interface.
		// net.LookupTXT returns a plain error interface, but the actual value underneath may be a *net.DNSError which has extra fields like IsNotFound.
		// The ok pattern (value, ok := x.(Type)) is a safe assertion — if the underlying type doesn't match, ok is false and the code doesn't panic.
		// We use this to distinguish "record doesn't exist yet" (normal during propagation, not an error) from a genuine network failure.
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return false, nil
		}
		return false, fmt.Errorf("looking up TXT records for %s: %w", fqdn, err)
	}

	for _, record := range records {
		if record == expectedValue {
			return true, nil
		}
	}
	return false, nil
}
