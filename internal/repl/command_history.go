package repl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// CommandEntry represents a single command executed in the session
type CommandEntry struct {
	ID       string            `json:"id"`
	Command  string            `json:"command"`
	Output   string            `json:"output"`
	Error    string            `json:"error,omitempty"`
	ExitCode int               `json:"exit_code"`
	Dir      string            `json:"dir"`
	Env      map[string]string `json:"env"`
	Time     time.Time         `json:"time"`
}

// CommandHistory tracks command execution in a session
type CommandHistory struct {
	Commands    []CommandEntry    `json:"commands"`
	MaxEntries  int               `json:"max_entries"`
	CurrentDir  string            `json:"current_dir"`
	Environment map[string]string `json:"environment"`
}

// NewCommandHistory creates a new command history tracker
func NewCommandHistory(maxEntries int) *CommandHistory {
	return &CommandHistory{
		Commands:    make([]CommandEntry, 0),
		MaxEntries:  maxEntries,
		CurrentDir:  "",
		Environment: make(map[string]string),
	}
}

// AddCommand adds a command to the history
func (h *CommandHistory) AddCommand(cmd, output, errorStr string, exitCode int) {
	entry := CommandEntry{
		ID:       fmt.Sprintf("%d", len(h.Commands)+1),
		Command:  cmd,
		Output:   output,
		Error:    errorStr,
		ExitCode: exitCode,
		Dir:      h.CurrentDir,
		Env:      make(map[string]string),
		Time:     time.Now(),
	}
	// Copy environment snapshot
	for k, v := range h.Environment {
		entry.Env[k] = v
	}

	h.Commands = append(h.Commands, entry)

	// Trim history if needed
	if len(h.Commands) > h.MaxEntries {
		h.Commands = h.Commands[1:]
	}
}

// GetCommand returns a command by ID
func (h *CommandHistory) GetCommand(id string) (*CommandEntry, error) {
	for _, cmd := range h.Commands {
		if cmd.ID == id {
			return &cmd, nil
		}
	}
	return nil, fmt.Errorf("command not found: %s", id)
}

// GetLastCommand returns the most recent command
func (h *CommandHistory) GetLastCommand() *CommandEntry {
	if len(h.Commands) == 0 {
		return nil
	}
	return &h.Commands[len(h.Commands)-1]
}

// SetDirectory updates the current working directory
func (h *CommandHistory) SetDirectory(dir string) {
	h.CurrentDir = dir
}

// SetEnvironment sets an environment variable
func (h *CommandHistory) SetEnvironment(key, value string) {
	h.Environment[key] = value
}

// GetEnvironment gets an environment variable
func (h *CommandHistory) GetEnvironment(key string) string {
	return h.Environment[key]
}

// GetCommandsList returns the list of command IDs
func (h *CommandHistory) GetCommandsList() []string {
	ids := make([]string, len(h.Commands))
	for i, cmd := range h.Commands {
		ids[i] = cmd.ID
	}
	return ids
}

// String returns a formatted representation for display
func (h *CommandHistory) String() string {
	if len(h.Commands) == 0 {
		return "No commands executed yet."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Command history (%d/%d):\n", len(h.Commands), h.MaxEntries)
	for _, cmd := range h.Commands {
		fmt.Fprintf(&sb, "  [%s] %s", cmd.ID, cmd.Command)
		if cmd.ExitCode != 0 {
			fmt.Fprintf(&sb, " (exit: %d)", cmd.ExitCode)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// SaveSession saves the current session to JSON
func (h *CommandHistory) SaveSession(filename string) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	return saveToFile(filename, data)
}

// LoadSession loads a session from JSON
func (h *CommandHistory) LoadSession(filename string) error {
	data, err := loadFromFile(filename)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	var loaded CommandHistory
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to unmarshal session: %w", err)
	}

	h.Commands = loaded.Commands
	h.MaxEntries = loaded.MaxEntries
	h.CurrentDir = loaded.CurrentDir
	h.Environment = loaded.Environment

	return nil
}

// GetLastN returns the last N commands
func (h *CommandHistory) GetLastN(n int) []CommandEntry {
	if len(h.Commands) <= n {
		result := make([]CommandEntry, len(h.Commands))
		copy(result, h.Commands)
		return result
	}

	start := len(h.Commands) - n
	result := make([]CommandEntry, n)
	copy(result, h.Commands[start:])
	return result
}

// saveToFile writes data to a file
func saveToFile(filename string, data []byte) error {
	error := os.WriteFile(filename, data, 0644)
	if error != nil {
		return fmt.Errorf("failed to write file: %w", error)
	}
	return nil
}

// loadFromFile reads data from a file
func loadFromFile(filename string) ([]byte, error) {
	data, error := os.ReadFile(filename)
	if error != nil {
		return nil, fmt.Errorf("failed to read file: %w", error)
	}
	return data, nil
}
