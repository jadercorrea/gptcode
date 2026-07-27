# Quality and evidence

GPTCode treats quality claims as contracts that should point to executable
evidence.

| Claim | Evidence | Command |
| --- | --- | --- |
| The public session-store example is concurrency-safe | Behavior and concurrent-access tests run under Go's race detector | `make evidence` |
| The public example has complete statement coverage | CI rejects any value other than 100.0% for that package | `make evidence` |
| The CLI builds and the short suite passes | One repository-level quality target used locally and in CI | `make verify` |
| Research evidence stays inside repository boundaries | Tests exclude private/generated directories, environment files, and symlink escapes | `go test ./internal/modes` |
| Releases are intentional and verified | Only version tags trigger release; tests run before GoReleaser | `.github/workflows/cd.yml` |
| Public positioning remains honest | A contract test rejects retired product claims and unscoped coverage language | `make public-contract` |

## What 100% means here

The 100% statement-coverage guarantee applies only to
`examples/sessionstore`, the deliberately small fixture used by the public
demonstration. It does not describe the entire repository.

Repository-wide coverage is uneven because the project contains older and
experimental command surfaces. Coverage is being raised around maintained
workflows while those surfaces are evaluated or retired. A high percentage is
not accepted as a substitute for meaningful behavior, failure-mode, boundary,
and concurrency tests.

## Local verification

Run:

```bash
make verify
```

The target checks formatting, runs `go vet`, builds the CLI, executes the short
test suite, validates the public project contract, and verifies the published
race/coverage evidence.

Model-backed end-to-end behavior is intentionally separated from deterministic
CI because it depends on provider availability, credentials, latency, and model
behavior. The published terminal recording is generated from a real
model-backed run; the final code and verification remain independently
auditable.

## Reporting regressions

Use the bug report form for reproducible failures and GitHub Security Advisories
for vulnerabilities. Never publish credentials or proprietary repository
contents.
