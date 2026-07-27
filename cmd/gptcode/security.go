package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/security"
	"github.com/spf13/cobra"
)

var securityCmd = &cobra.Command{
	Use:   "security",
	Short: "Security scanning and vulnerability management",
	Long:  `Scan for vulnerabilities and automatically fix them.`,
}

var securityScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for security vulnerabilities",
	Long: `Scan the codebase for security vulnerabilities using language-specific tools.

Supported tools:
- Go: govulncheck
- Node.js: npm audit
- Python: safety
- Ruby: bundle audit

Examples:
  gptcode security scan           # Scan only
  gptcode security scan --fix     # Scan and auto-fix`,
	RunE: runSecurityScan,
}

var securityFix bool
var securityModel string

func init() {
	rootCmd.AddCommand(securityCmd)
	securityCmd.AddCommand(securityScanCmd)

	securityScanCmd.Flags().BoolVar(&securityFix, "fix", false, "Automatically fix vulnerabilities")
	securityCmd.PersistentFlags().StringVar(&securityModel, "model", "", "LLM model to use (default: from config)")
}

func runSecurityScan(cmd *cobra.Command, args []string) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getSecurityProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	scanner := security.NewScanner(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Println("🔒 Scanning for vulnerabilities...")

	report, err := scanner.ScanAndFix(ctx, securityFix)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(report.Vulnerabilities) == 0 {
		fmt.Println("✅ No vulnerabilities detected")
		return nil
	}

	fmt.Printf("\n⚠️  Found %d vulnerabilit(y/ies):\n", len(report.Vulnerabilities))

	criticalCount := 0
	highCount := 0
	for i, vuln := range report.Vulnerabilities {
		fmt.Printf("\n%d. ", i+1)

		if vuln.ID != "" {
			fmt.Printf("%s ", vuln.ID)
		} else if vuln.CVE != "" {
			fmt.Printf("%s ", vuln.CVE)
		}

		fmt.Printf("[%s]", vuln.Severity)

		switch vuln.Severity {
		case "Critical":
			criticalCount++
		case "High":
			highCount++
		}

		if vuln.Package != "" {
			fmt.Printf(" in %s", vuln.Package)
		}
		fmt.Println()

		if vuln.Description != "" {
			desc := vuln.Description
			if len(desc) > 100 {
				desc = desc[:97] + "..."
			}
			fmt.Printf("   %s\n", desc)
		}
	}

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("   Language: %s\n", report.Language)
	fmt.Printf("   Total: %d\n", len(report.Vulnerabilities))
	if criticalCount > 0 {
		fmt.Printf("   Critical: %d\n", criticalCount)
	}
	if highCount > 0 {
		fmt.Printf("   High: %d\n", highCount)
	}

	if securityFix {
		fmt.Printf("\n🔧 Fix Results:\n")
		fmt.Printf("   Fixed: %d\n", report.FixedCount)

		if len(report.UpdatedFiles) > 0 {
			fmt.Printf("   Updated files: %d\n", len(report.UpdatedFiles))
			for _, file := range report.UpdatedFiles {
				fmt.Printf("     - %s\n", file)
			}
		}

		if len(report.Errors) > 0 {
			fmt.Printf("\n⚠️  %d error(s) during fixing:\n", len(report.Errors))
			for _, err := range report.Errors {
				fmt.Printf("   - %v\n", err)
			}
		}

		if report.FixedCount > 0 {
			fmt.Println("\n✅ Vulnerabilities fixed")
			fmt.Println("⚠️  Run tests to verify fixes before committing")
		}
	} else {
		fmt.Println("\n💡 Run with --fix to automatically fix vulnerabilities")
	}

	return nil
}

func getSecurityProvider(setup *config.Setup) (llm.Provider, string, error) {
	model := securityModel
	backendName := setup.Defaults.Backend
	if backendName == "" {
		backendName = "anthropic"
	}

	backendCfg, ok := setup.Backend[backendName]
	if !ok {
		return nil, "", fmt.Errorf("backend %s not configured", backendName)
	}

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	if model == "" {
		model = backendCfg.GetModelForAgent("editor")
		if model == "" {
			model = backendCfg.DefaultModel
		}
	}

	if model == "" {
		return nil, "", fmt.Errorf("no model configured")
	}

	return provider, model, nil
}
