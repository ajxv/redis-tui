# Redis TUI

[![CI](https://github.com/ajxv/redis-tui/actions/workflows/ci.yml/badge.svg)](https://github.com/ajxv/redis-tui/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ajxv/redis-tui)](https://goreportcard.com/report/github.com/ajxv/redis-tui)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/ajxv/redis-tui)](https://github.com/ajxv/redis-tui/releases/latest)

An interactive, fast, and lightweight Terminal User Interface (TUI) for exploring and managing your Redis databases. Built with Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework, it replaces repetitive CLI commands with an intuitive, keyboard-driven UI.

## Features

- **Interactive Database Explorer:** Browse through thousands of keys with SCAN-based pagination.
- **Context-Aware Drilling:** Navigate into Hash fields, Lists, Sets, and Sorted Sets with full CRUD support.
- **CRUD Operations Wizard:** Use guided forms to `SET`, `HSET`, `RPUSH`, `LPUSH`, `SADD`, `ZADD`, and more.
- **TTL Management:** Set, clear, or inspect key expiry. TTL is preserved when editing a value in-place.
- **Export / Import:** Dump individual keys or entire databases to portable JSON files using Redis `DUMP`/`RESTORE`.
- **TLS / SSL:** Connect to secured Redis instances (managed Redis services, Redis Cloud, AWS ElastiCache, Upstash) with full mTLS support.
- **Redis 6+ ACL Auth:** Authenticate with a username and password in addition to the legacy password-only form.
- **Reconnection with Backoff:** Automatically reconnects after a dropped connection using exponential backoff (200 ms → 25.6 s cap).
- **Lightweight & Fast:** Custom RESP protocol parser with minimal dependencies.
- **Cross-Platform:** Works on Linux, macOS, and Windows.

## Why redis-tui?

| | redis-tui | redis-cli | RedisInsight |
|:---|:---:|:---:|:---:|
| Terminal-native | ✓ | ✓ | ✗ |
| No install / single binary | ✓ | ✗ | ✗ |
| Full CRUD for all data types | ✓ | ✓ | ✓ |
| Export / Import (JSON) | ✓ | ✗ | ✓ |
| TLS / mTLS | ✓ | ✓ | ✓ |
| Keyboard-driven navigation | ✓ | ✗ | ✗ |

## Prerequisites

Redis TUI works out of the box on **macOS** and **Windows**.

On **Linux**, the clipboard copy feature (`c` key) requires one of the following:

| Display Server | Package | Install |
| :--- | :--- | :--- |
| X11 | `xclip` or `xsel` | `sudo apt install xclip` |
| Wayland | `wl-clipboard` | `sudo apt install wl-clipboard` |

All other features work on Linux without additional dependencies.

## Installation

### Option 1: Using Go Install

```bash
go install github.com/ajxv/redis-tui/cmd/redis-tui@latest
```

### Option 2: Pre-compiled Binaries (Recommended)

Download the latest binary for your OS from the [Releases page](https://github.com/ajxv/redis-tui/releases). Extract and place the `redis-tui` binary in your `PATH`.

## Usage

```bash
redis-tui
```

### Connection via URL (recommended)

The `-url` flag accepts a standard Redis connection string and overrides all individual connection flags:

```bash
# Plain TCP
redis-tui -url "redis://localhost:6379/0"

# With password
redis-tui -url "redis://:mysecret@localhost:6379"

# Redis 6+ ACL username + password
redis-tui -url "redis://alice:mysecret@localhost:6379/2"

# TLS (rediss:// implies -tls)
redis-tui -url "rediss://user:pass@my-redis.example.com:6380/0"
```

### Connection via individual flags

```bash
redis-tui -host "127.0.0.1:6379" -password "mysecret" -db 0
```

### All flags

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-url` | Redis URL: `redis://[:pass@]host[:port][/db]` or `rediss://…` | — |
| `-host` | Redis server address `host:port` | `localhost:6379` |
| `-password` | Redis password | `$REDIS_PASSWORD` env var |
| `-username` | Redis ACL username (Redis 6+) | — |
| `-db` | Redis database index | `0` |
| `-dial-timeout` | TCP connection timeout | `5s` |
| `-read-timeout` | Per-command read deadline | `10s` |
| `-tls` | Enable TLS/SSL | `false` |
| `-tls-skip-verify` | Skip TLS certificate verification *(insecure)* | `false` |
| `-tls-cert` | Path to client certificate (PEM) | — |
| `-tls-key` | Path to client private key (PEM) | — |
| `-tls-ca` | Path to CA certificate (PEM) | — |

**Tip:** Store your password in the environment to keep it out of the process list:

```bash
export REDIS_PASSWORD="mysecret"
redis-tui -host "127.0.0.1:6379"
```

### TLS examples

```bash
# Connect to a TLS-secured Redis, trust the system CA store
redis-tui -host "my-redis.example.com:6380" -tls

# Skip certificate verification (useful for self-signed certs in dev)
redis-tui -host "localhost:6380" -tls -tls-skip-verify

# mTLS with a custom CA and client certificate
redis-tui -host "my-redis.example.com:6380" \
  -tls \
  -tls-ca /etc/ssl/redis-ca.pem \
  -tls-cert /etc/ssl/redis-client.pem \
  -tls-key /etc/ssl/redis-client-key.pem

# Redis 6+ ACL user via URL (TLS implied by rediss://)
redis-tui -url "rediss://alice:secret@my-redis.example.com:6380/0"
```

> **Note:** `-tls-cert` and `-tls-key` must always be provided together.

## Keybindings

### Global

| Key | Action |
| :--- | :--- |
| `↑ / ↓` or `k / j` | Navigate lists |
| `Enter` | Select an item or submit a form |
| `Esc` | Go back or cancel |
| `Ctrl+C` | Quit |

### Key Browser (Explore mode)

| Key | Action |
| :--- | :--- |
| `Enter` | Open selected key |
| `d` | Delete key (with confirmation) |
| `r` | Rename key |
| `n` | Load next page of keys |
| `Ctrl+R` / `F5` | Refresh current view |

### Value Output

| Key | Action |
| :--- | :--- |
| `e` | Edit value in-place (TTL is preserved) |
| `c` | Copy value to clipboard |
| `x` | Set or clear TTL (enter `0` to persist) |
| `Esc` | Return to previous screen |

### Field / Member List (Hash, List, Set, Sorted Set)

| Key | Action |
| :--- | :--- |
| `Enter` | View selected field or member |
| `d` | Delete field or member (with confirmation) |
| `Ctrl+R` / `F5` | Refresh |

## Known Limitations (Beta)

- **Large collections:** Lists, Sets, and Sorted Sets load in pages of 100 items — press `n` to load the next page.
- **Offset-based list/zset paging:** If items are added or removed mid-browse, reopen the key (`Ctrl+R`) for a consistent view.
- **Redis Streams:** `XADD` / `XREAD` are not yet supported.
- **No in-browser search:** Use the pattern input (e.g. `user:*`) when entering Explore mode to narrow the key list.

## Development

```bash
# Clone
git clone https://github.com/ajxv/redis-tui.git
cd redis-tui

# Build
make build

# Run
make run

# Run all tests
make test

# Format
make fmt

# Lint (requires golangci-lint)
make lint-optional
```

## Project Layout

```
.
├── cmd/redis-tui/          # Entry point and CLI flags
├── internal/
│   ├── redis/              # RESP protocol parser
│   └── tui/                # Bubble Tea model, state machine, TLS, URL parser, export/import
├── docs/                   # Release process and maintainer guides
└── tests/
    ├── redis/              # Black-box tests for the RESP parser
    └── tui/                # Black-box integration tests for the state machine
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, test conventions, and the PR workflow.

## License

This project is licensed under the MIT License.
