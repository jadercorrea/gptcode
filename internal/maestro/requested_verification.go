package maestro

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var safeVerificationToken = regexp.MustCompile(`^(?:-[A-Za-z0-9=._/-]+|[A-Za-z0-9.][A-Za-z0-9=._/-]*)$`)

func requestedVerificationCommand(task string) []string {
	fields := strings.Fields(task)
	for index := 0; index+1 < len(fields); index++ {
		if !strings.EqualFold(fields[index], "go") || !strings.EqualFold(fields[index+1], "test") {
			continue
		}

		command := []string{"go", "test"}
		for _, field := range fields[index+2:] {
			token := strings.TrimRight(field, ",:")
			if token != "./..." {
				token = strings.TrimSuffix(token, ".")
			}
			if token == "" || !safeVerificationToken.MatchString(token) {
				if strings.ContainsAny(field, ";|&`$<>(){}[]") {
					return nil
				}
				break
			}
			if !strings.HasPrefix(token, "-") && !strings.Contains(token, "/") && token != "." {
				break
			}
			command = append(command, token)
		}
		if len(command) > 2 {
			return command
		}
		return nil
	}
	return nil
}

func repositoryVerificationCommand(cwd string) []string {
	if makefileHasTarget(filepath.Join(cwd, "Makefile"), "verify") {
		return []string{"make", "verify"}
	}

	candidates := []struct {
		marker  string
		command []string
	}{
		{"go.mod", []string{"go", "test", "./..."}},
		{"mix.exs", []string{"mix", "test"}},
		{"Cargo.toml", []string{"cargo", "test"}},
		{"package.json", []string{"npm", "test"}},
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(cwd, candidate.marker)); err == nil {
			return candidate.command
		}
	}
	return nil
}

func makefileHasTarget(path, target string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target) + `\s*:`)
	return pattern.Match(content)
}

func runRequestedVerification(ctx context.Context, cwd string, command []string) (string, error) {
	if len(command) == 0 {
		return "", nil
	}
	process := exec.CommandContext(ctx, command[0], command[1:]...)
	process.Dir = cwd
	output, err := process.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed: %w", strings.Join(command, " "), err)
	}
	return string(output), nil
}
