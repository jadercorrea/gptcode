// Command evidence-suite repeats controlled agent experiments across a
// versioned fixture corpus and writes an aggregate, failure-inclusive report.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/evidence"
)

func main() {
	configPath := flag.String("config", "", "path to a suite JSON file")
	output := flag.String("output", "", "override suite output directory")
	repetitions := flag.Int("repetitions", 0, "override repetitions per fixture")
	verbose := flag.Bool("verbose", false, "stream agent progress while preserving evidence")
	flag.Parse()
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "usage: evidence-suite -config <suite.json>")
		os.Exit(2)
	}

	file, err := os.Open(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "opening suite configuration failed")
		os.Exit(1)
	}
	defer file.Close()

	var suite evidence.Suite
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&suite); err != nil {
		fmt.Fprintln(os.Stderr, "decoding suite configuration failed")
		os.Exit(1)
	}
	if *output != "" {
		suite.Output = *output
	}
	if *repetitions > 0 {
		suite.Repetitions = *repetitions
	}
	if *verbose {
		suite.Progress = os.Stdout
	}

	summary, err := evidence.RunSuite(context.Background(), suite)
	if err != nil {
		fmt.Fprintln(os.Stderr, "suite execution failed safely")
		os.Exit(1)
	}
	fmt.Printf(
		"suite completed: %d/%d passed (%.1f%%), median %s\n",
		summary.PassedRuns,
		summary.TotalRuns,
		summary.PassRate*100,
		summary.MedianDuration,
	)
	if summary.PassedRuns != summary.TotalRuns {
		os.Exit(1)
	}
}
