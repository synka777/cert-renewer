# cert-renewer

Automates Let's Encrypt certificate renewal for a VPS using the DNS-01 challenge, with DNS managed via [1984 Hosting](https://1984.hosting).

## What it does

1. Logs into 1984 Hosting and adds a `_acme-challenge` TXT record to prove domain ownership
2. Polls 1984's authoritative nameserver until the record is visible
3. Runs `certbot renew`, which validates the challenge and issues the new certificate
4. Reloads the configured services so they pick up the new certificate
5. Removes the `_acme-challenge` TXT record

## Project structure

```
cert-renewer/
├── main.go                      # Entry point and orchestration
├── config.toml                  # Configuration file (not committed)
├── go.mod
├── go.sum
└── internal/
    ├── config/
    │   └── config.go            # TOML config loading and validation
    ├── hosting/
    │   └── client.go            # 1984 Hosting HTTP client (login, add/delete TXT records)
    ├── dns/
    │   └── poller.go            # DNS propagation polling loop
    ├── certbot/
    │   └── certbot.go           # Certbot invocation
    └── services/
        └── restarter.go         # systemctl service reloading
```

## Configuration

Create `/usr/local/bin/config.toml` on the VPS:

```toml
domain = "yourdomain.com"
services = ["apache2", "dovecot"]

[hosting]
username = "your_1984_username"
password = "your_1984_password"

[certbot]
email = "you@example.com"
```

`services` is the list of systemd services to reload after renewal. Add as many as needed.

## How the binary is invoked

The binary runs in three modes depending on the flags passed:

### Orchestrator mode (default)
```bash
cert-renewer --config /usr/local/bin/config.toml
```
The main flow. Calls certbot, which in turn calls the binary back in hook modes.

### Dry run
```bash
cert-renewer --config /usr/local/bin/config.toml --dry-run
```
Skips TXT record creation, DNS polling, and service restarts. Passes `--dry-run` to certbot so no certificate is actually modified.

### Auth hook (called by certbot, not directly)
```bash
cert-renewer --auth-hook --config /path/to/config.toml
```
Certbot calls this automatically during the challenge phase. Adds the TXT record and waits for propagation.

### Cleanup hook (called by certbot, not directly)
```bash
cert-renewer --cleanup-hook --config /path/to/config.toml
```
Certbot calls this after validation. Removes the TXT record.

## Deployment

The binary is built and deployed automatically via GitHub Actions on every push to `master`. It cross-compiles for `linux/amd64` and copies the binary to `/usr/local/bin/cert-renewer` on the VPS via SCP.

Required GitHub repository secrets:

| Secret | Description |
|---|---|
| `VPS_HOST` | VPS IP or hostname |
| `VPS_USER` | SSH user (e.g. `root`) |
| `VPS_SSH_KEY` | Private SSH key for deployment |
| `VPS_DEPLOY_PATH` | Deployment path (e.g. `/usr/local/bin`) |

## Automated renewal

A monthly cron job on the VPS triggers the renewal automatically. Certbot only renews if the certificate is within 30 days of expiry, so running monthly is safe.

```
0 3 1 * * /usr/local/bin/cert-renewer --config /usr/local/bin/config.toml >> /var/log/cert-renewer.log 2>&1
```

## DNS note

In my case `_acme-challenge.domain.com` is matched by a wildcard CNAME (`*`) in the DNS zone. To work around this, the propagation poller queries 1984's authoritative nameserver (`ns0.1984.is`) directly instead of using the system resolver. This bypasses the wildcard and sees the TXT record immediately after it is written.
