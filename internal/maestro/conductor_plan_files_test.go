package maestro

import (
	"reflect"
	"testing"
)

func TestPlannedFilesExtractsModifyAndCreateSections(t *testing.T) {
	plan := `# Plan

## Files to modify
- ` + "`counter.go`" + ` (fix implementation)

## Files to create
- ` + "`counter_regression_test.go`" + `

## Success Criteria
- tests pass
`

	want := []string{"counter.go", "counter_regression_test.go"}
	if got := plannedFiles(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("plannedFiles() = %v, want %v", got, want)
	}
}

func TestPlannedFilesIgnoresNone(t *testing.T) {
	plan := "## Files to modify\n- None\n\n## Changes\n- Run tests"
	if got := plannedFiles(plan); len(got) != 0 {
		t.Fatalf("plannedFiles() = %v, want no files", got)
	}
}
