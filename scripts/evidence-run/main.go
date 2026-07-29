// Command evidence-run executes a controlled agent experiment and writes an
// inspectable, replayable evidence bundle.
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
	configPath := flag.String("config", "", "path to an experiment JSON file")
	flag.Parse()
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "usage: evidence-run -config <experiment.json>")
		os.Exit(2)
	}

	file, err := os.Open(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "opening experiment configuration failed")
		os.Exit(1)
	}
	defer file.Close()

	var experiment evidence.Experiment
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&experiment); err != nil {
		fmt.Fprintln(os.Stderr, "decoding experiment configuration failed")
		os.Exit(1)
	}

	result, err := evidence.RunExperiment(context.Background(), experiment)
	if err != nil {
		fmt.Fprintln(os.Stderr, "experiment failed safely")
		os.Exit(1)
	}
	if !result.Passed {
		fmt.Fprintln(os.Stderr, "experiment completed with failed verification")
		os.Exit(1)
	}
	fmt.Println("experiment completed with verified evidence")
}
