# Release Checklist & Process

This document is the authoritative guide for cutting a redis-tui release.

---

## Pre-Release Checklist

Work through this list top-to-bottom before creating the tag.

- [ ] All CI checks are green on `main` (CI badge on README)
- [ ] `CHANGELOG.md` updated — `## [Unreleased]` renamed to `## [X.Y.Z] - YYYY-MM-DD`
- [ ] Link reference added at the bottom of `CHANGELOG.md`:
  ```
  [X.Y.Z]: https://github.com/ajxv/redis-tui/releases/tag/vX.Y.Z
  ```
- [ ] README "Known Limitations" section reflects any new caveats introduced in this release
- [ ] `make build && ./redis-tui --version` prints the expected version string
- [ ] `go test -race ./...` passes locally
- [ ] GoReleaser dry-run succeeds: `goreleaser build --snapshot --clean`

---

## Release Steps

```bash
# 1. Make sure main is up to date
git checkout main && git pull

# 2. Edit CHANGELOG.md — rename [Unreleased] to [X.Y.Z] - YYYY-MM-DD

# 3. Commit the changelog
git add CHANGELOG.md
git commit -m "chore: prepare vX.Y.Z release"

# 4. Tag
git tag vX.Y.Z

# 5. Push the commit and the tag
git push origin main
git push origin vX.Y.Z
```

The `release.yml` GitHub Actions workflow triggers automatically on any `v*` tag.
GoReleaser then builds binaries for all platforms and publishes the GitHub Release.

---

## After Release

- [ ] Verify the GitHub Release page has 6 archives:
  `linux_x86_64`, `linux_arm64`, `Darwin_x86_64`, `Darwin_arm64`, `Windows_x86_64`, `Windows_arm64`
- [ ] Verify `checksums.txt` is attached
- [ ] Smoke-test the install path:
  ```bash
  go install github.com/ajxv/redis-tui/cmd/redis-tui@vX.Y.Z
  redis-tui --version
  ```
- [ ] Add a fresh `## [Unreleased]` section at the top of `CHANGELOG.md` and push to `main`

---

## Version Numbering

This project follows [Semantic Versioning](https://semver.org/).

| Tag example | When to use |
|:---|:---|
| `v1.0.0-beta` | First public beta — functionally complete but may have rough edges |
| `v1.0.0` | First stable release, post-beta feedback addressed |
| `v1.Y.0` | New backwards-compatible features |
| `v1.Y.Z` | Bug fixes only, no new features |
| `v2.0.0` | Breaking changes (e.g. incompatible export JSON format, removed CLI flags) |

---

## Hotfix Process

For a critical bug fix against an already-released tag:

```bash
# Branch from the tag you need to fix
git checkout -b hotfix/vX.Y.Z+1 vX.Y.Z

# Apply the fix, update CHANGELOG, run tests
make test

# Tag and push
git tag vX.Y.Z+1
git push origin hotfix/vX.Y.Z+1
git push origin vX.Y.Z+1

# Cherry-pick the fix back to main
git checkout main
git cherry-pick <commit-sha>
git push origin main
```

---

## Announcement Checklist (optional)

- Update the README badge if the latest release badge doesn't auto-refresh
- Post to relevant communities (Reddit r/golang, Hacker News, etc.) if it's a significant release
