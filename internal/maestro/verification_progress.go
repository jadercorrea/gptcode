package maestro

import (
	"regexp"
	"strings"
)

var (
	verificationDuration = regexp.MustCompile(`\d+(?:\.\d+)?s\b`)
	goTestTemporaryPath  = regexp.MustCompile(
		`(?:/private)?/var/folders/[^[:space:]]+/T/Test[^/[:space:]]+[0-9]+/[0-9]+|/tmp/Test[^/[:space:]]+[0-9]+/[0-9]+`,
	)
)

type verificationProgress struct {
	threshold   int
	lastFailure string
	consecutive int
}

func newVerificationProgress(threshold int) *verificationProgress {
	if threshold < 1 {
		threshold = 1
	}
	return &verificationProgress{threshold: threshold}
}

func (p *verificationProgress) Observe(output string) bool {
	failure := normalizeVerificationFailure(output)
	if failure == p.lastFailure && failure != "" {
		p.consecutive++
	} else {
		p.lastFailure = failure
		p.consecutive = 1
	}
	return failure != "" && p.consecutive >= p.threshold
}

func (p *verificationProgress) Consecutive() int {
	return p.consecutive
}

func normalizeVerificationFailure(output string) string {
	output = verificationDuration.ReplaceAllString(output, "(duration)")
	output = goTestTemporaryPath.ReplaceAllString(output, "(go-test-temp)")
	lines := strings.Split(output, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.Join(normalized, "\n")
}
