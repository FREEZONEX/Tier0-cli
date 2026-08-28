# Changelog

## Unreleased

- Added compatible UNS history count controls, automatic sparse queries, multi-field aggregation, and `first`/`last` aggregation functions.

## v0.7.0 - 2026-08-19

- Added `mqtt auth create/delete` for one-time MQTT credential creation, secure local profiles, dry-run previews, and confirmed revocation.
- Added TLS MQTT `publish` and `subscribe` with QoS, retain, JSON validation, wildcard subscriptions, bounded streaming, and NDJSON output.
- Added credential-redacted MQTT publish previews and redacted password, token, and API-key fields from HTTP debug logs.
- Added the embedded `tier0-mqtt` Skill and updated continuous UNS guidance to use MQTT instead of OpenAPI polling.

## v0.6.10 - 2026-08-17

- Hardened Windows self-uninstall by waiting for the CLI process to exit, retrying removal of the locked executable, and removing empty Tier0 directories after cleanup.
- Made the release script execute from an immutable temporary snapshot so source edits, pulls, or branch switches cannot corrupt an in-flight release.
- Added configurable GitHub API and asset-upload timeouts, progress output, low-speed detection, and bounded retries.
- Reconciled timed-out uploads against GitHub asset SHA-256 digests, accepting uploads that completed remotely and replacing incomplete or stale assets before retrying.
- Recovered cleanly when GitHub created a Release but its response was lost, and made reruns reuse an existing Release and matching assets.
- Known limitation: `uninstall --purge` can leave `~/.tier0/update-state.json` and legacy `tier0*.killed.bak` files created by older versions; remove `~/.tier0` manually after uninstall when these files are present.

## v0.6.9 - 2026-08-17

- Made Agent Skills and credentials persist by default during uninstall so they can be updated or reused independently.
- Added `--remove-skills` for explicit Agent Skill removal and retained `--purge` for credential removal.
- Made both the native CLI and npm wrapper remove the installed binary, embedded Skill baseline, version record, and global `@tier0/cli` package without recursive npm lifecycle cleanup.
- Verified Agent Skill removal instead of trusting a successful `skills remove` exit code, and report incomplete cleanup as a command error.
- Added npm-wrapper uninstall tests covering default preservation, full removal, false-success detection, and global package cleanup.

## v0.6.8 - 2026-08-17

- Made npm-based Windows upgrades atomic by staging the replacement executable beside the installed binary before activation.
- Added rollback when activation fails and best-effort cleanup for locked executables left by earlier Windows upgrade processes.
- Verified that npm upgrades installed the requested CLI version before reporting success, falling back to direct GitHub installation when verification fails.
- Added tests for atomic activation, rollback, and locked-backup handling on Windows.

## v0.6.7 - 2026-08-14

- Embedded a trusted Tier0 Skill baseline in every CLI binary so installation and repair no longer depend on a duplicate Skill directory in Release archives.
- Added `skills install`, provenance-aware `skills status`, and automatic Skill materialization during install and upgrade while preserving independently updated remote Skills.
- Made release builds synchronize the embedded snapshot from an exact `FREEZONEX/Tier0-skill` GitHub commit and require the resulting snapshot to be reviewed and committed before publishing.
- Kept independent Skill updates and Agent synchronization available through `tier0 skills update` and `tier0 skills sync`.
- Fixed npm packaging to preserve the executable `bin/tier0.js` entry point.
- Added retry handling for transient GitHub Release asset upload failures.

## v0.6.5 - 2026-08-11

- Added business-error validation to raw API, UNS browse/read/search/history, and Flow list/get/data commands so failed ResultVO responses return non-zero exit codes.
- Separated Tier0 business codes from HTTP status codes so large business codes are not incorrectly marked retryable.
- Enforced object-valued UNS writes and Metric schema fields during local create preflight, including batch namespace validation with clear `name` versus `path` errors.
- Added `--fields-file` to UNS create/update and `--clear-description` to UNS update for shell-safe PowerShell workflows.
- Made `config --json` return a stable redacted JSON object.
- Made Skill update lookup failures return a command error instead of an exit-zero result containing an `error` field.
- Added dry-run and explicit confirmation protection to object-storage deletion.
- Made a clean npx bootstrap require Tier0 Agent Skill installation and print the post-install authentication check/login workflow.
- Excluded repository-only README, changelog, and commit-message files from packaged Agent Skill assets.

## v0.6.4 - 2026-07-20

- Made the npm installer deploy the versioned local Release Skill globally and non-interactively to detected agents such as Codex and Claude Code, without downloading the Skill repository a second time.
- Made `tier0 skills update` resynchronize independently updated local Skills to detected agents while preserving the local update when Node.js is unavailable.
- Added npm release preflight checks for tests, package contents, and authentication before creating a GitHub Release, plus a `prepublishOnly` test guard for direct npm publishing.
- Fixed release packaging to use the sibling `Tier0-skill` checkout, exclude nested Git metadata and unreferenced protocol source snapshots, and fail before publishing when Skill assets are missing.
- Fixed `tier0 uninstall` on Windows by scheduling removal of the running executable after the command exits.
- Fixed Windows npm-wrapper Skill installation and removal by invoking `npx` through the shell from the local Skill directory.
- Added a shared `--dry-run` request-preview contract for raw API, UNS mutation, and Flow mutation operations.
- Made high-risk delete, restore, and deploy requests previewable without credentials or `--yes`.
- Added structured validation error `subtype` and `param` fields while preserving lower-level error causes.
- Rejected conflicting Flow flags, invalid UNS QoS values, invalid positional IDs, and no-op update requests locally.
- Stopped silently rewriting malformed JSON and now require valid JSON for raw API bodies and Flow templates.
- Fixed npm-based upgrades on macOS by repairing quarantine/signing state after installing the downloaded binary.
- Added post-npm-upgrade binary verification so `tier0 upgrade` fails clearly if the installed CLI cannot run.
- Changed `tier0 flow data --out` to write a deployable Node-RED `flows` array instead of the full API envelope.
- Made `tier0 flow deploy` accept full API envelopes, `data` objects, or pure `flows` arrays, normalizing all inputs before deployment.
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
