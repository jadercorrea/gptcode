// Command evidence-audit produces content-free aggregate metrics from local
// agent history. It is a research utility, not part of the public GPTCode CLI.
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
	root := flag.String("codex-root", "", "path to the Codex sessions directory")
	replaySession := flag.String("replay-session", "", "Codex JSONL session to replay")
	replayTurn := flag.String("replay-turn", "", "turn ID to replay through")
	replayRoot := flag.String("replay-root", "", "detached worktree receiving the replay")
	flag.Parse()

	if *replaySession != "" || *replayTurn != "" || *replayRoot != "" {
		replay(context.Background(), *replaySession, *replayTurn, *replayRoot)
		return
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "usage: evidence-audit -codex-root <directory>")
		os.Exit(2)
	}

	summary, err := evidence.ScanCodex(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "evidence audit failed: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintf(os.Stderr, "writing evidence audit: %v\n", err)
		os.Exit(1)
	}
}

func replay(ctx context.Context, sessionPath, turnID, root string) {
	if sessionPath == "" || turnID == "" || root == "" {
		fmt.Fprintln(
			os.Stderr,
			"replay requires -replay-session, -replay-turn, and -replay-root",
		)
		os.Exit(2)
	}

	plan, err := evidence.ExtractReplayPlan(sessionPath, turnID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extracting replay plan failed safely")
		os.Exit(1)
	}
	if err := evidence.ApplyChanges(ctx, root, plan.Changes); err != nil {
		fmt.Fprintln(os.Stderr, "applying replay plan failed safely")
		os.Exit(1)
	}

	result := struct {
		AppliedChanges int `json:"applied_changes"`
	}{
		AppliedChanges: len(plan.Changes),
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "writing replay result: %v\n", err)
		os.Exit(1)
	}
}
