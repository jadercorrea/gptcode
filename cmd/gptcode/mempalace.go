package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var memCmd = &cobra.Command{
	Use:   "mem [command]",
	Short: "AI memory system (MemPalace)",
	Long: `MemPalace - AI Memory System

The highest-scoring AI memory system ever benchmarked (96.6% LongMemEval).

Commands:
  gt mem init <dir>     Initialize a new palace
  gt mem mine <dir>     Mine project files or conversations
  gt mem search <query> Search memories by meaning
  gt mem ask <question> Search and output results for context
  gt mem status         Show palace overview
  gt mem up             Load context into memory
  gt mem mcp            Show MemPalace help and MCP tools
  gt mem compress       Compress storage using AAAK dialect
  gt mem repair         Rebuild vector index

Usage for context:
  # Add to your prompt:
  echo "$(gt mem ask what is antigravity)"

Learn more: https://github.com/milla-jovovich/mempalace`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		execPath, err := findMempalace()
		if err != nil {
			return runMempalaceInstall()
		}
		runCmd := exec.Command(execPath, args...)
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		runCmd.Stdin = os.Stdin
		return runCmd.Run()
	},
}

var memInitCmd = &cobra.Command{
	Use:   "init <directory>",
	Short: "Initialize a new palace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		if yes {
			return runMempalaceCommand("init", args[0], "--yes")
		}
		return runMempalaceCommand("init", args[0])
	},
}

var memMineCmd = &cobra.Command{
	Use:   "mine <directory>",
	Short: "Mine project files or conversations",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode, _ := cmd.Flags().GetString("mode")
		if mode != "" {
			return runMempalaceCommand("mine", args[0], "--mode", mode)
		}
		return runMempalaceCommand("mine", args[0])
	},
}

var memSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMempalaceCommand("search", strings.Join(args, " "))
	},
}

var memAskCmd = &cobra.Command{
	Use:   "ask <question>",
	Short: "Search and output results for context",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		execPath, err := findMempalace()
		if err != nil {
			return runMempalaceInstall()
		}

		searchCmd := exec.Command(execPath, "search", query, "--results", "5")
		searchCmd.Stdout = os.Stdout
		searchCmd.Stderr = os.Stderr
		return searchCmd.Run()
	},
}

var memStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show palace overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMempalaceCommand("status")
	},
}

var memUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Load context into memory (wake-up)",
	RunE: func(cmd *cobra.Command, args []string) error {
		wing, _ := cmd.Flags().GetString("wing")
		if wing != "" {
			return runMempalaceCommand("wake-up", "--wing", wing)
		}
		return runMempalaceCommand("wake-up")
	},
}

var memMcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Show MemPalace help and MCP tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMempalaceCommand("instructions", "help")
	},
}

var memCompressCmd = &cobra.Command{
	Use:   "compress",
	Short: "Compress palace storage using AAAK dialect",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMempalaceCommand("compress")
	},
}

var memRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Rebuild vector index from stored data",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMempalaceCommand("repair")
	},
}

func findMempalace() (string, error) {
	if path, err := exec.LookPath("mempalace"); err == nil {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("mempalace not found")
	}

	paths := []string{
		filepath.Join(home, ".mempalace-venv", "bin", "mempalace"),
		filepath.Join(home, ".venv", "bin", "mempalace"),
		filepath.Join(home, ".local", "bin", "mempalace"),
		filepath.Join(home, "mempalace", ".venv", "bin", "mempalace"),
		filepath.Join(home, ".local", "share", "pipx", "bin", "mempalace"),
		filepath.Join(home, ".local", "share", "pipx", "venvs", "mempalace", "bin", "mempalace"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("mempalace not found")
}

func runMempalaceInstall() error {
	fmt.Println("MemPalace not found. Please install it first:")
	fmt.Println()
	fmt.Println("Option 1 (recommended):")
	fmt.Println("  python3.11 -m venv ~/.mempalace-venv")
	fmt.Println("  ~/.mempalace-venv/bin/pip install mempalace")
	fmt.Println()
	fmt.Println("Option 2 (pipx):")
	fmt.Println("  brew install pipx")
	fmt.Println("  pipx install mempalace")
	fmt.Println()
	fmt.Println("Learn more: https://github.com/milla-jovovich/mempalace")
	fmt.Println()
	return fmt.Errorf("mempalace not installed")
}

func runMempalaceCommand(args ...string) error {
	execPath, err := findMempalace()
	if err != nil {
		return runMempalaceInstall()
	}
	runCmd := exec.Command(execPath, args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin
	return runCmd.Run()
}

func init() {
	rootCmd.AddCommand(memCmd)

	memCmd.AddCommand(memInitCmd)
	memCmd.AddCommand(memMineCmd)
	memCmd.AddCommand(memSearchCmd)
	memCmd.AddCommand(memAskCmd)
	memCmd.AddCommand(memStatusCmd)
	memCmd.AddCommand(memUpCmd)
	memCmd.AddCommand(memMcpCmd)
	memCmd.AddCommand(memCompressCmd)
	memCmd.AddCommand(memRepairCmd)

	memInitCmd.Flags().BoolP("yes", "y", false, "Skip prompts (auto-accept)")
	memMineCmd.Flags().String("mode", "", "Mining mode: projects, convos, general")
	memUpCmd.Flags().String("wing", "", "Project-specific wing")
}
