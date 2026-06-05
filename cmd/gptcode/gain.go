package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type SavingsRecord struct {
	Timestamp   string  `json:"timestamp"`
	Tool        string  `json:"tool"`
	Command     string  `json:"command"`
	TokensSaved int     `json:"tokens_saved"`
	CostSaved   float64 `json:"cost_saved"`
}

var shareFlag bool

var gainCmd = &cobra.Command{
	Use:   "gain",
	Short: "Show cumulative token and cost savings from command optimization",
	Long: `Displays aggregate reports on tokens saved, estimated financial savings,
and count of commands optimized by the GPTCode output filtering engine (adapted from RTK).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not get home dir: %w", err)
		}

		savingsFile := filepath.Join(home, ".gptcode", "savings.jsonl")
		file, err := os.Open(savingsFile)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No token savings recorded yet. Run some commands using gt run or gt do first!")
				return nil
			}
			return err
		}
		defer file.Close()

		var totalCommands int
		var totalTokensSaved int
		var totalCostSaved float64
		cmdSavings := make(map[string]int)

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var record SavingsRecord
			if err := json.Unmarshal([]byte(scanner.Text()), &record); err != nil {
				continue
			}

			totalCommands++
			totalTokensSaved += record.TokensSaved
			totalCostSaved += record.CostSaved

			if record.Command != "" {
				// Clean up command for grouping (first 3 words)
				words := strings.Fields(record.Command)
				if len(words) > 3 {
					words = words[:3]
				}
				cmdKey := strings.Join(words, " ")
				cmdSavings[cmdKey] += record.TokensSaved
			} else {
				cmdSavings[record.Tool] += record.TokensSaved
			}
		}

		if err := scanner.Err(); err != nil {
			return err
		}

		if shareFlag {
			savingsStr := ""
			if totalTokensSaved >= 1000000 {
				savingsStr = fmt.Sprintf("%.1fM", float64(totalTokensSaved)/1000000.0)
			} else if totalTokensSaved >= 1000 {
				savingsStr = fmt.Sprintf("%.1fk", float64(totalTokensSaved)/1000.0)
			} else {
				savingsStr = fmt.Sprintf("%d", totalTokensSaved)
			}
			fmt.Printf("⛏ saved %s tokens ($%.4f) using @gptcode token optimization! why use many token when few do trick\n", savingsStr, totalCostSaved)
			return nil
		}

		fmt.Println("============================================================")
		fmt.Println("                GPTCode Token Savings Report")
		fmt.Println("============================================================")
		fmt.Printf("  Total Optimized Commands:   %d\n", totalCommands)
		fmt.Printf("  Total Tokens Saved (RTK):   %d\n", totalTokensSaved)
		fmt.Printf("  Estimated Cost Saved (USD): $%.4f\n", totalCostSaved)
		fmt.Println("============================================================")

		// Print top optimized command categories
		if len(cmdSavings) > 0 {
			fmt.Println("\nTop Optimized Commands:")
			fmt.Println("------------------------------------------------------------")
			for cmdKey, saved := range cmdSavings {
				fmt.Printf("  %-35s  %d tokens saved\n", cmdKey, saved)
			}
		}

		return nil
	},
}

func init() {
	gainCmd.Flags().BoolVarP(&shareFlag, "share", "s", false, "Print a shareable/tweetable savings badge line")
}
