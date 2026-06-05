package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gptcode/internal/config"
	"gptcode/internal/llm"
)

var cavemanCmd = &cobra.Command{
	Use:   "caveman [level]",
	Short: "Configure or check Caveman Mode (token compression)",
	Long: `Gets or sets the Caveman Mode level for token compression.
Supported levels: off, lite, full, ultra, wenyan-lite, wenyan-full, wenyan-ultra.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Get current mode
			val, err := config.GetConfig("defaults.caveman_mode")
			if err != nil || val == "<nil>" || val == "" {
				val = "off"
			}
			fmt.Printf("Caveman Mode is currently: %s\n", val)
			return nil
		}

		level := strings.ToLower(args[0])
		if level == "on" {
			level = "full"
		}

		if level != "off" && level != "lite" && level != "full" && level != "ultra" &&
			level != "wenyan-lite" && level != "wenyan-full" && level != "wenyan-ultra" {
			return fmt.Errorf("invalid level. Choose one of: off, lite, full, ultra, wenyan-lite, wenyan-full, wenyan-ultra")
		}

		err := config.SetConfig("defaults.caveman_mode", level)
		if err != nil {
			return err
		}

		fmt.Printf("Caveman Mode set to: %s\n", level)
		return nil
	},
}

var cavemanCompressCmd = &cobra.Command{
	Use:   "compress <file>",
	Short: "Compress a context file (e.g. CLAUDE.md, README.md) into caveman style",
	Long: `Reads the target file and uses the LLM to rewrite its natural language prose
in a highly compressed Caveman format. Saves a backup of the original file
as <filename>.original.<ext>. Code blocks, URLs, and file paths are preserved byte-for-byte.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		originalContent := string(data)
		originalLen := len(originalContent)

		// Create backup
		dir := filepath.Dir(path)
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(filepath.Base(path), ext)
		backupPath := filepath.Join(dir, base+".original"+ext)

		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup file at %s: %w", backupPath, err)
		}
		fmt.Printf("✓ Created backup at %s\n", backupPath)

		fmt.Println("⚡ Compressing file using LLM...")

		_, provider, model, err := newBuilderAndLLM("", "chat", "")
		if err != nil {
			return fmt.Errorf("failed to initialize LLM provider: %w", err)
		}

		systemPrompt := `You are a text compression utility. Your goal is to rewrite the provided Markdown/text file in a highly compressed Caveman format (omitting filler, hedging, articles, using sentence fragments, and arrows for causality) to save LLM input tokens. STRICTLY PRESERVE all code blocks, URLs, file paths, and technical symbols byte-for-byte. Do not translate or change technical meanings. Respond ONLY with the compressed file content.`
		userPrompt := "Compress this file:\n\n" + originalContent

		resp, err := provider.Chat(context.Background(), llm.ChatRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			Model:        model,
			Intent:       "edit",
		})
		if err != nil {
			return fmt.Errorf("LLM completion failed: %w", err)
		}

		compressedContent := strings.TrimSpace(resp.Text)
		compressedLen := len(compressedContent)

		if err := os.WriteFile(path, []byte(compressedContent), 0644); err != nil {
			return fmt.Errorf("failed to write compressed file: %w", err)
		}

		savedChars := originalLen - compressedLen
		savedPct := 0.0
		if originalLen > 0 {
			savedPct = (float64(savedChars) / float64(originalLen)) * 100
		}

		fmt.Println("============================================================")
		fmt.Println("               Caveman Context Compression")
		fmt.Println("============================================================")
		fmt.Printf("  Original Size:   %d characters\n", originalLen)
		fmt.Printf("  Compressed Size: %d characters\n", compressedLen)
		fmt.Printf("  Savings Rate:    -%.1f%% characters\n", savedPct)
		fmt.Println("============================================================")
		fmt.Printf("✓ Overwritten %s with compressed version.\n", path)

		return nil
	},
}

func init() {
	cavemanCmd.AddCommand(cavemanCompressCmd)
}
