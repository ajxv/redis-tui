# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
