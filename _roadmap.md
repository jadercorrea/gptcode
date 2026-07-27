# GPTCode roadmap

GPTCode is maintained as an independent open-source project and personal
engineering portfolio.

## Website transition

- [x] Replace the hosted-product positioning with an open-source project page.
- [x] Remove GPTCode Live, Cloud, pricing, and enterprise-service claims.
- [x] Update repository links to `jadercorrea/gptcode`.
- [x] Configure the Jekyll source for `gptcode.dev`.
- [x] Add an automated public-site identity check.
- [x] Rebuild the homepage around verifiable agent workflows and real CLI commands.
- [x] Make the execution pipeline, inspectable evidence, and engineering philosophy central to the homepage.
- [x] Publish a real terminal recording of repository detection, active skills, and executable verification.
- [x] Refine the first fold with a paced terminal demo, evidence preview, and explicit five-stage rationale.
- [x] Publish the anchor essay explaining why the workflow is the source of truth.
- [x] Ground `research` in repository contents and bound model-backed research/review calls with explicit timeouts.
- [x] Validate `do` against a real Go data race: preserve the public API, add internal synchronization, and pass `go test -race ./...`.
- [x] Validate `research` and `review` against both vulnerable and corrected implementations.
- [x] Record the full real workflow: diagnose with `research`, confirm with `review`, fix with `do`, and show race verification passing.
- [x] Publish the site changes to the `main` branch.
- [x] Move authoritative DNS for `gptcode.dev` from GoDaddy to Cloudflare.
- [x] Associate `gptcode.dev` and `www.gptcode.dev` with Cloudflare Pages.
- [x] Publish a race-safe example with behavior tests and 100% statement coverage enforced in CI.
- [x] Replace the legacy scheduled/private release pipeline with explicit, verified tag releases.
- [x] Verify custom-domain certificate activation and HTTPS after DNS propagation.
- [x] Align the README and GitHub metadata with the repository-centered verification thesis.
- [x] Publish an executable quality contract, security policy, support policy, and contribution templates.
- [x] Use one `make verify` quality gate locally, in CI, and before tagged releases.
- [x] Remove tracked binaries, traces, and scratch scripts from the public repository.
- [x] Prevent research evidence collection from traversing private/generated directories or symlink escapes.
- [x] Exercise the new release workflow with an intentional semantic-version tag and validate its artifacts.
- [x] Align the Go module path with the public repository so `go install ...@latest` is supported.
- [x] Report module versions correctly for both GoReleaser and `go install`, and keep Go/Actions dependencies monitored.
- [ ] Retire or isolate the legacy Live, training, Supabase, and experimental command surfaces with compatibility tests.
- [ ] Raise coverage in legacy workflow packages without presenting the public fixture as repository-wide coverage.
