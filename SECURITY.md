# Security Policy

## Supported Versions

| Version | Supported |
|:---|:---|
| Latest release | ✓ |
| Older releases | Fixes are backported only for critical vulnerabilities |

## Reporting a Vulnerability

Include the following in your report:

- The redis-tui version affected (`redis-tui --version`)
- A description of the vulnerability and its potential impact
- Steps to reproduce or a proof-of-concept (if applicable)

You can expect:
- An acknowledgement within **48 hours**
- A status update within **7 days**
- A coordinated disclosure once a fix is ready

## Scope

Areas of particular concern:

- Credentials (passwords, TLS keys) appearing in logs, error messages, or exported files
- Path traversal or arbitrary file write via the export/import file path input
- TLS certificate validation bypasses beyond the documented `-tls-skip-verify` flag
- Command injection through key names or values displayed in the TUI

## Out of Scope

- Vulnerabilities in the Redis server itself
- Issues requiring physical access to the machine running redis-tui
- Social engineering attacks
