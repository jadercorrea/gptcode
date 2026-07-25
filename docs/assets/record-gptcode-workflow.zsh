#!/usr/bin/env zsh

set -e

type_command() {
  local command="$1"

  print -n -- "\e[38;5;111m$ "
  for ((i = 1; i <= ${#command}; i++)); do
    print -n -- "${command[$i]}"
    sleep 0.1
  done
  print "\e[0m"
  sleep 0.5
}

print -n "\e[2J\e[H"
print "\e[1;36mGPTCode · repository evidence\e[0m"
print

type_command "gt detect-language"
gt detect-language | sed -n '1,5p'
sleep 2

type_command "gt skills show go | sed -n '1,24p'"
gt skills show go | sed -n '1,24p'
sleep 2

type_command "go test -v ./..."
GOROOT=/Users/jadercorrea/.local/share/mise/installs/go/1.22.2 \
  /Users/jadercorrea/.local/share/mise/installs/go/1.22.2/bin/go test -v ./...
sleep 2

print
print "\e[1;32m✓ Repository detected · rules inspected · tests passed\e[0m"
sleep 3
