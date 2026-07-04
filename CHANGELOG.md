# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0-beta.1] - Unreleased

### Added
- **Redesigned UI** (GitHub-dark theme): full-screen background, a grouped command menu with section labels, color-coded data types, and key-hint footers pinned to the bottom of every screen.
- **Guided key picker** for `HSET`, `HGET`, `SADD`, `ZADD`, `RPUSH`, and `LPUSH`: pick an existing key of the matching type — or `＋ new key…` — instead of typing the name. Add commands jump straight into the add form.
- **Add field/member form** now works for Sets, Sorted Sets, and Lists (press `a` in the field browser), not just Hashes.
- **Scrollable value & `INFO` inspector**: long output (`INFO`, large JSON) now opens at the top and scrolls with `↑/↓`, `PgUp/PgDn`, and `Home/End` instead of being truncated; lines wrap to the screen width.
- **Friendlier export/import**: `EXPORT` lets you pick the key from a list and pre-fills a `./<key>.dump` destination; `EXPORT_DB` pre-fills `./redis-db<n>.json`; every file prompt now states whether it wants a source or a destination, and `Tab` completes filesystem paths.
- **Field-level export/import**: in the field browser, `x` exports the selected hash field, list element, set member, or sorted-set member to a self-describing JSON file, and `i` imports one back (value-based, since `DUMP`/`RESTORE` only work on whole keys).
- JSON values are automatically detected, pretty-printed, and syntax-highlighted in the output view (keys blue, strings green, numbers orange, booleans/null red).

### Changed
- `q` now quits immediately from the main menu; the exit-confirmation prompt was removed (`Esc` is a no-op on the menu).
- Server `INFO` output dims its `# Section` headers for readability.
- Confirmation and add-field screens are now full-screen layouts (header + content + footer) instead of centered modal boxes.

### Fixed
- Edit mode (`e` key) uses a full multi-line textarea that wraps large values instead of a single scrolling line.
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

[1.0.0-beta.1]: https://github.com/ajxv/redis-tui/releases/tag/v1.0.0-beta.1
[1.0.0-beta]: https://github.com/ajxv/redis-tui/releases/tag/v1.0.0-beta
