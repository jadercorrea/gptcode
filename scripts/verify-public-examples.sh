#!/usr/bin/env bash
set -euo pipefail

coverage_file="$(mktemp "${TMPDIR:-/tmp}/gptcode-example-coverage.XXXXXX")"
trap 'rm -f "$coverage_file"' EXIT

go test -race -coverprofile="$coverage_file" ./examples/sessionstore

coverage="$(
  go tool cover -func="$coverage_file" |
    awk '/^total:/ { gsub("%", "", $3); print $3 }'
)"

if [[ "$coverage" != "100.0" ]]; then
  printf 'public example coverage is %s%%; expected 100.0%%\n' "$coverage" >&2
  exit 1
fi

printf 'public examples verified: race detector passed; statement coverage 100.0%%\n'
