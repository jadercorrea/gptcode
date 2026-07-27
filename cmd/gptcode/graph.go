package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/graph"

	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Manage dependency graph",
}

var graphBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build and index the dependency graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		fmt.Println(" Building dependency graph...")
		start := time.Now()

		builder := graph.NewBuilder(cwd)
		g, err := builder.Build()
		if err != nil {
			return fmt.Errorf("failed to build graph: %w", err)
		}

		fmt.Printf("   Nodes: %d\n", len(g.Nodes))
		fmt.Printf("   Edges: %d\n", countEdges(g))

		fmt.Println(" Calculating PageRank...")
		g.PageRank(0.85, 20)

		duration := time.Since(start)
		fmt.Printf("[OK] Done in %v\n", duration)

		return nil
	},
}

var graphQueryCmd = &cobra.Command{
	Use:   "query <terms>",
	Short: "Query the graph for relevant files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		builder := graph.NewBuilder(cwd)
		g, err := builder.Build()
		if err != nil {
			return err
		}
		g.PageRank(0.85, 20)

		query := strings.Join(args, " ")
		optimizer := graph.NewOptimizer(g)
		results := optimizer.OptimizeContext(query, 10)

		fmt.Printf("\n Query: %q\n", query)
		fmt.Println("📂 Relevant Context:")
		for _, path := range results {
			nodeID := g.Paths[path]
			score := g.Nodes[nodeID].Score
			fmt.Printf("   - %s (PR: %.4f)\n", path, score)
		}
		fmt.Println()

		return nil
	},
}

func countEdges(g *graph.Graph) int {
	count := 0
	for _, edges := range g.OutEdges {
		count += len(edges)
	}
	return count
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(graphBuildCmd)
	graphCmd.AddCommand(graphQueryCmd)
}
