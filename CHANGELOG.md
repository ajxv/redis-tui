# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0-beta] - 2026-05-30

### Added
- JSON values are now automatically detected, pretty-printed with indentation, and syntax-highlighted in the output view. Keys are colored cyan, string values yellow, numbers pink, and booleans/null red.

## [1.0.1-beta] - 2026-05-27

### Fixed
- Output screen now word-wraps large values (JSON, Redis `INFO`, errors) to the terminal width instead of overflowing.
- Confirmation dialogs (delete key/field) now wrap long key and field names within the dialog box boundary.
- Edit mode (`e` key) now uses a full multi-line textarea that word-wraps large values to the terminal width instead of a single scrolling line.
- Pressing `esc` after a successful edit (`e`) or TTL change (`x`) now correctly returns to the browser instead of showing a blank output screen.

## [1.0.0-beta] - 2026-05-17

### Added
- **Interactive Terminal User Interface (TUI)** for exploring and managing Redis databases.
- **Full CRUD Support** across all major Redis data types: Strings, Lists, Hashes, Sets, and Sorted Sets.
- **Form Wizard** for seamless key-value entry, appending, and updates without typing raw commands.
- **Pattern-based Explorer** for searching, filtering, and browsing keyspaces.
- **Automatic Pagination** for safely loading large datasets without memory exhaustion.
- **Cross-Platform Compatibility** (macOS, Linux, Windows) with zero external dependencies.
- Redis Server `INFO` dashboard.
- Key Rename functionality (`r` key in Explorer).
- TTL Management for setting/persisting expiries (`x` key).
- Explorer view refresh hotkey (`ctrl+r`).
- Clipboard copy support for returned output values (`c` key).
- Exit confirmation safeguard on the main menu.
- Visual loading spinner for background Redis operations.
- Type-safe internal event routing to ensure high application stability.
- Network dial timeouts to gracefully fail on dropped packets or unresponsive hosts.
- Dynamically centered modal dialogs that adapt to terminal resizing.
- Built-in guard rails to prevent UI deadlocks when interacting with keys that have expired in the background.
- Clean input field state management across diverse form wizards.
- Strictly enforced read-only execution limits for `INFO` screens and internal list navigation fields.
- `LPUSH` command support for prepending values to a list.
- Field-level pagination for Lists (LRANGE), Sets (SSCAN), and Sorted Sets (ZRANGE) — loads 100 items per page, press `n` for more.
- `--version` flag to print the current binary version and exit.
- Context-aware TTL input hint: "0 = remove expiry (PERSIST)" shown when setting TTL.
- TLS support with mTLS and custom CA for secure Redis connections.
- Redis URL (`redis://` / `rediss://`) connection string support.
- Export / Import keys and entire databases via JSON (`DUMP`/`RESTORE`).
- Exponential backoff reconnection (200 ms → 25.6 s cap) on connection loss.

[1.1.0-beta]: https://github.com/ajxv/redis-tui/releases/tag/v1.1.0-beta
[1.0.1-beta]: https://github.com/ajxv/redis-tui/releases/tag/v1.0.1-beta
[1.0.0-beta]: https://github.com/ajxv/redis-tui/releases/tag/v1.0.0-beta
