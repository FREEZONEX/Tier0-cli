# Changelog

## Unreleased

- CLI output is now English-only.
- Removed runtime use of the legacy `i18n.T` translation helper from command definitions and deleted the unused i18n package.
- Updated CLI README, npm wrapper docs, install scripts, release scripts, and user-facing command text to English.
- Deprecated language switching; `tier0 config --lang en` remains accepted for compatibility, while other language values are rejected.
- Updated bundled skill guidance examples so `go test ./...` validates against the current CLI command tree.
- Added `tier0 doctor` for local connectivity and authentication diagnostics.
- Improved UNS command ergonomics and batch response validation.
- Added positional topic support for `uns read`.
- Added deprecated `--topic` compatibility alias for `uns delete`; prefer `--path`.

## v0.4.11 - 2026-05-26

- Fixed write operations that ignored backend business errors.

## v0.4.10 - 2026-05-26

- Added `tier0 config --api-key` for direct API key setup.

## v0.4.9 - 2026-05-26

- Added `tier0 uninstall`.
- Fixed install version selection by using the npm package version directly.
- Fixed GitHub Release JSON payload generation in `release.sh`.

## v0.4.6 - 2026-05-26

- Renamed npm package to `@tier0/cli`.
- Added one-command install and uninstall for CLI plus Agent Skills.
- Fixed login polling type handling.

## v0.2.4 - 2026-05-18

- Fixed JSON decoding for `workspaceID` during `login --setup-code` polling.

## v0.2.3 - 2026-05-18

- Added cross-platform install scripts for macOS, Linux, and Windows.

## v0.2.2 - 2026-05-18

- Fixed `login` base URL loading from config.
- Clarified that private deployments must run `config --base-url` before `login`.

## v0.2.1 - 2026-05-18

- Added persistent `tier0 config --base-url`.
- Added `TIER0_BASE_URL` support.

## v0.2.0 - 2026-05-18

- Added `upgrade` and `skills` commands.
- Included skills documentation in release packages.

## v0.1.0 - 2026-05-17

- Initial release with Device Flow authentication, UNS API proxying, and config management.
