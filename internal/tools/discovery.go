package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Stop words to filter out from search queries
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"to": true, "for": true, "in": true, "on": true, "at": true,
	"is": true, "it": true, "this": true, "that": true, "with": true,
	"as": true, "by": true, "from": true, "be": true, "are": true,
	"was": true, "were": true, "been": true, "being": true, "have": true,
	"has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "could": true, "should": true, "may": true,
	"might": true, "must": true, "can": true, "of": true, "if": true,
	"so": true, "but": true, "not": true, "no": true, "yes": true,
	"add": true, "fix": true, "issue": true, "create": true, "update": true,
	"make": true, "change": true, "modify": true, "implement": true,
}

// File type priorities for ranking (higher is better)
var fileTypePriority = map[string]int{
	".go":   10,
	".ts":   10,
	".tsx":  10,
	".py":   10,
	".rs":   10,
	".ex":   10,
	".exs":  10,
	".js":   8,
	".jsx":  8,
	".rb":   8,
	".java": 8,
	".c":    8,
	".cpp":  8,
	".h":    7,
	".hpp":  7,
	".md":   5,
	".txt":  3,
	".json": 3,
	".yaml": 3,
	".yml":  3,
}

// FileMatch represents a file with relevance info
type FileMatch struct {
	Path       string
	MatchCount int
	FirstMatch string
	Priority   int
}

// FindRelevantFiles searches for files most relevant to a query
func FindRelevantFiles(call ToolCall, workdir string) ToolResult {
	query, ok := call.Arguments["query"].(string)
	if !ok || query == "" {
		return ToolResult{Tool: "find_relevant_files", Error: "query parameter required"}
	}

	limit := 10
	if l, ok := call.Arguments["limit"].(float64); ok {
		limit = int(l)
	}

	fileTypes := ""
	if ft, ok := call.Arguments["file_types"].(string); ok {
		fileTypes = ft
	}

	// Extract keywords from query
	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		return ToolResult{Tool: "find_relevant_files", Error: "no searchable keywords found in query"}
	}

	// Build regex pattern for ripgrep
	pattern := strings.Join(keywords, "|")

	// Use ripgrep to find files with matches
	// -l: only print file names
	// -i: case insensitive
	// -c: count matches per file
	args := []string{"-i", "-c", pattern}

	// Add file type filters if specified
	if fileTypes != "" {
		for _, ext := range strings.Split(fileTypes, ",") {
			ext = strings.TrimSpace(ext)
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			args = append(args, "-g", "*"+ext)
		}
	}

	// Exclude common directories
	args = append(args, "-g", "!node_modules", "-g", "!vendor", "-g", "!.git", "-g", "!dist", "-g", "!build")
	args = append(args, workdir)

	cmd := exec.Command("rg", args...)
	output, err := cmd.CombinedOutput()

	// If ripgrep not found, fall back to grep
	if err != nil && strings.Contains(err.Error(), "executable file not found") {
		return findRelevantFilesWithGrep(query, keywords, workdir, limit)
	}

	// Parse ripgrep output (format: file:count)
	var matches []FileMatch
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		filePath := parts[0]
		count := 0
		fmt.Sscanf(parts[len(parts)-1], "%d", &count)

		relPath, _ := filepath.Rel(workdir, filePath)
		ext := filepath.Ext(filePath)
		priority := fileTypePriority[ext]
		if priority == 0 {
			priority = 1
		}

		matches = append(matches, FileMatch{
			Path:       relPath,
			MatchCount: count,
			Priority:   priority,
		})
	}

	// Sort by relevance: match count * priority
	sort.Slice(matches, func(i, j int) bool {
		scoreI := matches[i].MatchCount * matches[i].Priority
		scoreJ := matches[j].MatchCount * matches[j].Priority
		return scoreI > scoreJ
	})

	// Limit results
	if len(matches) > limit {
		matches = matches[:limit]
	}

	// Get first matching line for each file
	for i := range matches {
		matches[i].FirstMatch = getFirstMatch(workdir, matches[i].Path, keywords)
	}

	// Format output
	var result strings.Builder
	fmt.Fprintf(&result, "Found %d relevant files for query: %q\n", len(matches), query)
	fmt.Fprintf(&result, "Keywords: %s\n\n", strings.Join(keywords, ", "))

	for i, m := range matches {
		fmt.Fprintf(&result, "%d. %s (%d matches)\n", i+1, m.Path, m.MatchCount)
		if m.FirstMatch != "" {
			fmt.Fprintf(&result, "   Preview: %s\n", m.FirstMatch)
		}
	}

	if len(matches) == 0 {
		result.WriteString("No files found matching the query.\n")
		result.WriteString("Try using different keywords or broader search terms.\n")
	}

	return ToolResult{
		Tool:   "find_relevant_files",
		Result: result.String(),
	}
}

// extractKeywords extracts searchable keywords from a query
func extractKeywords(query string) []string {
	// Remove special characters
	re := regexp.MustCompile(`[^\w\s]`)
	query = re.ReplaceAllString(query, " ")

	// Split into words
	words := strings.Fields(strings.ToLower(query))

	// Filter stop words and short words
	var keywords []string
	seen := make(map[string]bool)
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		if stopWords[word] {
			continue
		}
		if seen[word] {
			continue
		}
		seen[word] = true
		keywords = append(keywords, word)
	}

	return keywords
}

// getFirstMatch returns the first matching line in a file
func getFirstMatch(workdir, relPath string, keywords []string) string {
	fullPath := filepath.Join(workdir, relPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	pattern := strings.Join(keywords, "|")
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return ""
	}

	for _, line := range lines {
		if re.MatchString(line) {
			line = strings.TrimSpace(line)
			if len(line) > 80 {
				line = line[:77] + "..."
			}
			return line
		}
	}
	return ""
}

// findRelevantFilesWithGrep is a fallback when ripgrep is not available
func findRelevantFilesWithGrep(query string, keywords []string, workdir string, limit int) ToolResult {
	pattern := strings.Join(keywords, "\\|")

	cmd := exec.Command("grep", "-r", "-i", "-l", "-c", pattern, workdir)
	output, _ := cmd.CombinedOutput()

	var matches []FileMatch
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		filePath := parts[0]
		count := 0
		fmt.Sscanf(parts[1], "%d", &count)

		relPath, _ := filepath.Rel(workdir, filePath)
		ext := filepath.Ext(filePath)
		priority := fileTypePriority[ext]
		if priority == 0 {
			priority = 1
		}

		matches = append(matches, FileMatch{
			Path:       relPath,
			MatchCount: count,
			Priority:   priority,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].MatchCount*matches[i].Priority > matches[j].MatchCount*matches[j].Priority
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Found %d relevant files (using grep fallback)\n", len(matches))
	fmt.Fprintf(&result, "Keywords: %s\n\n", strings.Join(keywords, ", "))

	for i, m := range matches {
		fmt.Fprintf(&result, "%d. %s (%d matches)\n", i+1, m.Path, m.MatchCount)
	}

	return ToolResult{
		Tool:   "find_relevant_files",
		Result: result.String(),
	}
}
