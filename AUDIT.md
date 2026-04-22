# ojs-cli — Clean-Code / SRP Final Audit

- **Repository:** `ojs-cli` (independent Git repository)
- **Branch:** `refactor/clean-code-srp`
- **Base:** `54dc405` (`v0.4.1`)
- **Date:** 2026-08-12
- **Policy:** existing unstaged changes were preserved; no commits, staging, dependency
  upgrades, sibling edits, or unrelated formatting sweeps.

## Result

All locally executable repository gates are green with `GOWORK=off`:

- standalone build and local install
- all 13 package test targets with race detection
- **80.6% total statement coverage** (CI threshold: 80%)
- vet and pinned `golangci-lint v1.64.8` (**0 issues**, from 74)
- `make test`, `make lint`, and `make build`
- `govulncheck v1.1.4` (no vulnerabilities)
- six cross-platform builds
- GoReleaser snapshot archives, DEB/RPM packages, checksums, and Homebrew formula
- CLI smoke scenarios and clean `ojs-go-backend-common v0.4.1` compatibility test

The one external blocker is public module installation: the compatible
`github.com/openjobspec/ojs-go-backend-common@v0.4.1` tag is not publicly
resolvable/checksummable. Side-by-side source builds work; `go install
github.com/openjobspec/ojs-cli/cmd/ojs@latest` cannot work until that dependency is
published. `README.md` now states this release prerequisite.

## Implemented findings

### Standalone module and packaging

- Added the explicit compatible requirement:
  `github.com/openjobspec/ojs-go-backend-common v0.4.1`.
- Added the workspace-standard local replacement:
  `replace github.com/openjobspec/ojs-go-backend-common => ../ojs-go-backend-common`.
- Added the required transitive sums without upgrading versions; removed the now-unused
  direct `golang.org/x/net` dependency after moving to standard-library `context`.
- Verified the full suite against a clean archive of the common module's `v0.4.1` tag,
  not only the sibling's dirty working tree.
- Removed the tracked root Mach-O `ojs`; added `/ojs` to `.gitignore`.
- Confirmed Make writes `bin/ojs`; GoReleaser packages `ojs` from `cmd/ojs`.
- Removed the unused GoReleaser `main.commit` linker injection and migrated archive
  format keys to the current schema.
- Made generated source, scaffold, migration-converter, migration-plan, export, and
  observability writes atomic.

### Migration correctness and failure semantics

- Sidekiq `at` values now use exact decimal parsing and canonical RFC3339Nano UTC.
  Fractions such as `.001` and `.900` are preserved; whole seconds remain fractionless.
- BullMQ export now:
  - discovers both `wait` lists and `delayed` ZSETs;
  - gives delayed membership precedence and deduplicates IDs;
  - decodes current `timestamp * 4096 + sequence` scores;
  - supports legacy direct-millisecond scores and stored `timestamp + delay` fallback;
  - preserves queue, priority, job ID, structure, and raw options metadata;
  - keeps overdue original target times;
  - never reapplies `opts.delay` to a ready wait job.
- The tested and production BullMQ parsers now share one conversion path.
- Sidekiq, BullMQ, Celery, River, and Faktory reads propagate list/ZSET/hash/scan/HTTP
  errors instead of continuing silently.
- Malformed records produce `FailedRecord` diagnostics and typed
  `PartialExportError` results.
- Partial source exports do not replace an existing output unless the explicit
  `--allow-partial` policy is selected; an allowed partial file is still accompanied
  by a typed nonzero error.
- NDJSON and OJS migration exports use atomic replacement.
- Import validation, per-job imports, legacy batch imports, and live migration return
  typed `PartialFailureError` values after rendering counts.
- Live migration now persists `partial`/`failed` terminal phases, completion time, and
  failure details; rejected jobs can no longer produce `PhaseComplete`.
- `migrate shadow` fails immediately with `ErrShadowNotImplemented` and never emits a
  success report.
- The unwired migration proxy no longer claims an unforwarded legacy job succeeded;
  that route returns HTTP 501.

### Output, redaction, streaming, and terminal safety

- `output.Format` is initialized from validated `OJS_OUTPUT`; `--json` wins over the
  environment. Unsupported values return a CLI error.
- Added a central URL/error redactor covering Redis, AMQP, and HTTP userinfo,
  passwords, and sensitive query keys.
- Applied redaction to human and JSON migration analysis, import/export, proxy, and
  live-migration output while retaining originals only in connection state.
- SSE now uses a named 1 MiB line bound, accepts valid events above 64 KiB, propagates
  scanner/network errors, uses request cancellation, and distinguishes EOF from
  cancellation.
- Monitor rejects zero/negative intervals before constructing a ticker.
- Monitor clear-screen escapes are emitted only for terminal output.
- HTTP, migration, audit, import, validation, and proxy reads now have explicit bounds.
- HTTP requests use contexts; response decode and close/write errors are handled.

### Lint, correctness, and SRP cleanup

- Cleared all pinned lint findings: errcheck, noctx, staticcheck, gocritic, gocyclo,
  unparam, unused, gosec review items, and related resource/error handling.
- Split main global-option parsing from map-based command dispatch.
- Split create, enqueue, migration import/export, status, system maintenance, doctor,
  setup-observability, and completion generation into parse/execute/render or
  artifact-specific units.
- Replaced duplicated response decoding with shared checked JSON helpers.
- Split observability option parsing, directory creation, template data, artifact
  generation, atomic writing, and reporting.
- Split completion generation by shell; made output deterministic, prevented flag-slice
  mutation, and added all currently dispatched top-level commands plus migration/bulk
  subcommands.
- Fixed additional clear bugs found during the pass:
  - release version linker injection now works;
  - debug replay no longer double-encodes the request as base64 JSON;
  - short failure IDs no longer panic debug output;
  - doctor uses the actual `/ojs/v1` API paths;
  - Prometheus targets no longer include an invalid URL scheme;
  - `migrate generate --format yaml` now emits YAML rather than JSON with a YAML suffix;
  - generated files and command output failures are surfaced.

## Exact gate results

| Gate | Command | Result |
|---|---|---|
| CI toolchain | `GOTOOLCHAIN=go1.24.4 go version` | PASS (`go1.24.4 darwin/arm64`) |
| Standalone build | `GOWORK=off go build ./...` | PASS |
| Race + coverage | `GOWORK=off go test ./... -race -coverprofile=coverage.out` | PASS, 13 packages, **80.6%** |
| Vet | `GOWORK=off go vet ./...` | PASS |
| Pinned lint | `golangci-lint v1.64.8 run ./...` | PASS, **0 issues** |
| Module verification | `GOWORK=off go mod verify` | PASS (`all modules verified`) |
| Tidy check | `GOWORK=off go mod tidy -diff` | PASS, no diff |
| Make test | `GOWORK=off make test` | PASS, race enabled |
| Make lint | `GOWORK=off make lint` | PASS |
| Make build | `GOWORK=off make build` | PASS, `bin/ojs` Mach-O arm64 |
| Vulnerabilities | `govulncheck v1.1.4 ./...` | PASS (`No vulnerabilities found.`) |
| Version injection | `go build -ldflags '-X main.version=9.9.9-test'` | PASS, prints `ojs version 9.9.9-test` |
| Cross builds | linux/darwin/windows × amd64/arm64, `CGO_ENABLED=0` | PASS, 6/6 |
| Local install | `GOWORK=off go install ./cmd/ojs` | PASS |
| Clean common tag | full tests with replacement pointed at archived `v0.4.1` | PASS |
| GoReleaser | Go 1.24.4, `goreleaser release --snapshot --clean --skip=publish` | PASS |
| Package integrity | SHA-256 checksums + Linux tar + Windows ZIP listing | PASS |
| CLI smoke | completion, shadow failure, monitor interval, config rejection, observability | PASS |
| Formatting | `gofmt` on every changed Go file; `git diff --check` | PASS |

GoReleaser produced six platform archives, four Linux packages (DEB/RPM for amd64 and
arm64), checksums, and a Homebrew formula. It emitted non-fatal deprecation notices for
the legacy `brews` publisher and missing NFPM maintainer metadata; changing formula
publishing to a cask and selecting a package-maintainer identity are release-policy
decisions.

## Genuine external blocker / deferred product decisions

1. **Public Go install:** `go install ...@latest` fails because
   `ojs-go-backend-common@v0.4.1` returns a public proxy/sumdb 404 and cannot be cloned
   anonymously. Publish that module before the next CLI module release, then remove the
   local `replace` from the published module (or make CI/release jobs check out the
   sibling at `../ojs-go-backend-common`).
2. **Live proxy wiring:** `MigrateLive`/`MigrationProxy` remains an exported, tested
   helper but is not wired into `ojs migrate`; the legacy forwarding destination and
   dual-write acknowledgement contract are undefined product/API decisions. Unsafe
   fake success was removed.
3. **Legacy OJS export/import helpers:** `RunMigrateImport` and `RunMigrateExport`
   remain exported/tested rather than being silently attached to the existing
   source-adapter `migrate import/export` command names. Choosing one public file format
   and command surface requires a compatibility decision.
4. **Golden suite:** no repository-owned golden-test command or golden files exist.
   Deterministic completion and package-content assertions were added to the normal test
   suite instead.
