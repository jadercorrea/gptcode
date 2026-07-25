#!/usr/bin/env zsh

set -euo pipefail

: ${GT_BIN:=gt}
: ${GO_BIN:=go}
: ${GO_ROOT:=}

if [[ -n "$GO_ROOT" ]]; then
  export GOROOT="$GO_ROOT"
  export PATH="$GO_ROOT/bin:$PATH"
fi

type_command() {
  local command="$1"

  print -n -- "\e[38;5;111m$ "
  for ((i = 1; i <= ${#command}; i++)); do
    print -n -- "${command[$i]}"
    sleep 0.035
  done
  print "\e[0m"
  sleep 0.7
}

print -n "\e[2J\e[H"
print "\e[1;36mGPTCode · evidence-driven repair\e[0m"
print

type_command 'gt research "Is the session store safe for concurrent access?"'
"$GT_BIN" research \
  "How does session expiration work, and is concurrent access actually safe? Cite implementation evidence and the verification command." \
  2>/dev/null |
  sed -n '/### Findings/,/### Verification/p' |
  sed -n '1,22p'
sleep 2

type_command 'gt review session/store.go --focus "concurrency correctness"'
"$GT_BIN" review session/store.go \
  --focus "concurrency correctness and public API stability" 2>/dev/null |
  sed -n '/CODE REVIEW/,$p' |
  sed -n '4,24p'
sleep 2

type_command 'gt do "Fix the race without changing the public API"'
"$GT_BIN" do \
  "Fix the session store concurrency bug without changing its public API. Verify with go test -race ./..." \
  --max-attempts 2
sleep 2

type_command "git diff -- session/store.go"
git --no-pager diff -- session/store.go | sed -n '1,34p'
sleep 2

type_command "go test -race ./..."
GOROOT="$GO_ROOT" GOCACHE="${GOCACHE:-/tmp/gptcode-demo-go-cache-124}" \
  "$GO_BIN" test -race ./...
sleep 2

print
print "\e[1;32m✓ Diagnosed · reviewed · repaired · verified\e[0m"
sleep 3
