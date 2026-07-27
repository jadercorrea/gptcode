#!/usr/bin/env bash
set -euo pipefail

assert_contains() {
  local file="$1"
  local expected="$2"

  if ! grep -Fq "$expected" "$file"; then
    printf '%s must contain: %s\n' "$file" "$expected" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file="$1"
  local rejected="$2"

  if grep -Fq "$rejected" "$file"; then
    printf '%s must not contain: %s\n' "$file" "$rejected" >&2
    exit 1
  fi
}

assert_contains README.md "AI coding agents should produce evidence, not just answers."
assert_contains README.md "make evidence"
assert_contains README.md "go test -race"
assert_contains README.md "Limitations"
assert_contains go.mod "module github.com/jadercorrea/gptcode"
assert_not_contains README.md '$0-5/month'
assert_not_contains README.md "Autonomous AI Coding Assistant"
assert_not_contains README.md "Your code never stored on our servers"

assert_contains Makefile "evidence:"
assert_contains Makefile "verify:"
assert_contains .github/workflows/ci.yml "make verify"
assert_contains .github/workflows/cd.yml "make verify"

for file in LICENSE QUALITY.md SECURITY.md SUPPORT.md .github/ISSUE_TEMPLATE/bug_report.yml; do
  if [[ ! -f "$file" ]]; then
    printf 'missing public project contract: %s\n' "$file" >&2
    exit 1
  fi
done

tracked_artifacts="$(
  git ls-files -- \
    gt-test \
    gt-live-test \
    'trace_*.json' \
    debug_live.lua \
    test.txt \
    test_loop.sh \
    test_raw_job.sh \
    test_terminal.sh
)"
if [[ -n "$tracked_artifacts" ]]; then
  printf 'generated or scratch artifacts are still tracked:\n%s\n' "$tracked_artifacts" >&2
  exit 1
fi

printf 'public project contract verified\n'
