# Redis TUI

An interactive, fast, and lightweight Terminal User Interface (TUI) for exploring and managing your Redis databases. Built with Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework, it replaces repetitive CLI commands with an intuitive, Vim-friendly UI.

*(Insert Demo GIF screenshot here)*

## Features

- **Interactive Database Explorer:** Browse through thousands of keys seamlessly.
- **Context-Aware Drilling:** Navigate effortlessly into Hash fields, Lists, Sets, and Sorted Sets.
- **CRUD Operations Wizard:** Use forms to `SET`, `HSET`, `RPUSH`, `SADD`, `ZADD`, etc.
- **Lightweight & Fast:** Custom RESP parser under the hood with minimal dependencies.
- **Cross-Platform:** Works on Linux, macOS, and Windows environments.

## Installation

### Option 1: Using Go Install

If you have Go 1.25+ installed, it's a one-liner:

```bash
go install github.com/ajxv/redis-tui/cmd/redis-tui@latest
```

### Option 2: Pre-compiled Binaries (Recommended)

Download the latest pre-compiled binary for your operating system from the [Releases page](https://github.com/ajxv/redis-tui/releases). Extract the archive and place the `redis-tui` binary in your PATH.

## Usage

Simply run the tool in your terminal:

```bash
redis-tui
```

### Command Line Flags

You can customize the connection parameters:

```bash
redis-tui -host "127.0.0.1:6379" -password "mysecr3t" -db 0
```

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-host` | Redis server address `<host:port>` | `localhost:6379` |
| `-password` | Redis server password | `""` (Empty string) |
| `-db` | Redis Database index | `0` |

## Shortcuts & Keybindings

* `↑ / ↓` or `k / j`: Navigate lists
* `Enter`: Select an item or submit a form
* `Esc`: Go back or cancel an operation
* `e`: Edit the current value (only available when viewing a value)
* `d`: Delete the highlighted key or field (with confirmation)
* `n`: Load more keys (pagination in Explorer mode)
* `Ctrl+C`: Quit the application

## Development

If you want to contribute or build from source:

```bash
# Clone the repository
git clone https://github.com/ajxv/redis-tui.git
cd redis-tui

# Build the binary
make build

# Run the project
make run

# Run tests
make test
```

## License

This project is licensed under the MIT License.