# Redis TUI

An interactive, fast, and lightweight Terminal User Interface (TUI) for exploring and managing your Redis databases. Built with Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework, it replaces repetitive CLI commands with an intuitive, keyboard-driven UI.

*(Insert Demo GIF screenshot here)*

## Features

- **Interactive Database Explorer:** Browse through thousands of keys with SCAN-based pagination.
- **Context-Aware Drilling:** Navigate into Hash fields, Lists, Sets, and Sorted Sets with full CRUD support.
- **CRUD Operations Wizard:** Use guided forms to `SET`, `HSET`, `RPUSH`, `SADD`, `ZADD`, and more.
- **TTL Management:** Set, clear, or inspect key expiry. TTL is preserved when editing a value in-place.
- **Export / Import:** Dump individual keys or entire databases to portable JSON files using Redis `DUMP`/`RESTORE`.
- **Lightweight & Fast:** Custom RESP protocol parser with minimal dependencies.
- **Cross-Platform:** Works on Linux, macOS, and Windows.

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

### Command Line Flags

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-host` | Redis server address `<host:port>` | `localhost:6379` |
| `-password` | Redis server password | `$REDIS_PASSWORD` env var, or `""` |
| `-db` | Redis database index | `0` |

**Tip:** Set the `REDIS_PASSWORD` environment variable to avoid exposing your password in the process list:

```bash
export REDIS_PASSWORD="mysecr3t"
redis-tui -host "127.0.0.1:6379"
```

Or pass it directly as a flag:

```bash
redis-tui -host "127.0.0.1:6379" -password "mysecr3t" -db 0
```

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
│   └── tui/                # Bubble Tea model, state machine, export/import
└── tests/
    ├── redis/              # Black-box tests for the RESP parser
    └── tui/                # Black-box integration tests for the state machine
```

> **Note on test placement:** Go's testing convention co-locates unit tests with source (`_test.go` files).
> Tests that require access to unexported implementation details (e.g., `exportSingleKey`) remain in
> `internal/tui/` under `package tui`. Tests exercising only the public API live in `tests/` under
> `package redis_test` / `package tui_test`.

## License

This project is licensed under the MIT License.
