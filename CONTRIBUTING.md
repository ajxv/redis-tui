# Contributing to redis-tui

Thank you for your interest in contributing! This guide covers how to set up your dev environment, run tests, and submit changes.

## Development Setup

```bash
git clone https://github.com/ajxv/redis-tui.git
cd redis-tui

# Build
make build

# Run (connects to localhost:6379 by default)
make run

# Run all tests
make test

# Format code
make fmt

# Lint (requires golangci-lint — https://golangci-lint.run/usage/install/)
make lint-optional
```

A local Redis instance is not required for the test suite — all tests use mock connections.

## Project Layout

```
.
├── cmd/redis-tui/          # main() entry point, CLI flag parsing
├── internal/
│   ├── redis/              # Custom RESP protocol parser (no external Redis client)
│   └── tui/                # Bubble Tea state machine, browser, input, export/import, TLS
├── docs/                   # Maintainer guides (release process, etc.)
└── tests/
    ├── redis/              # Black-box tests for the RESP parser
    └── tui/                # Black-box integration tests for the TUI state machine
```

**Test placement:** Tests in `tests/` use `package redis_test` / `package tui_test` (black-box, public API only). Any test that needs access to unexported internals lives directly alongside the source in `internal/` under its own package.

## Code Style

- Run `make fmt` before committing — CI will fail if `go fmt` produces a diff.
- Run `go vet ./...` — CI runs this in the test job.
- Keep comments minimal: only document *why*, not *what*. Well-named identifiers are self-documenting.
- No backwards-compatibility shims for code that has no external callers.

## Branch and PR Workflow

1. Fork the repo and create a feature branch: `git checkout -b feat/my-feature`
2. Make your changes, run `make test` and `go vet ./...` locally.
3. Update `CHANGELOG.md` — add a line under `## [Unreleased]` describing the change.
4. Update `README.md` if the change adds or modifies user-visible behaviour (flags, keybindings, features).
5. Open a pull request against `main` and fill in the PR template.

## Commit Message Style

Use a short imperative subject line:
```
feat: add LPUSH menu item
fix: resolve ImportKeys using unresolved file path
refactor: extract field pagination into BrowserModel
```

Prefix with `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, or `chore:`.

## Issue Labels

| Label | Meaning |
|:---|:---|
| `bug` | Something is broken |
| `enhancement` | New feature or improvement |
| `good first issue` | Small, well-scoped task suitable for new contributors |
| `help wanted` | Larger task where input is welcome |
| `question` | Needs clarification before work can start |
