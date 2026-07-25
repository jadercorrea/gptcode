package maestro

import "testing"

func TestIsQueryTaskDoesNotTreatFailedEditAsQuery(t *testing.T) {
	c := &Conductor{}
	task := "Fix the session store concurrency bug without changing its public API."
	plan := "Run command: go test -race ./...\nFiles to modify: none"

	if c.isQueryTask(task, plan, nil) {
		t.Fatal("an explicit edit task with no modified files must not be reported as a successful query")
	}
}

func TestIsQueryTaskAllowsReadOnlyRequest(t *testing.T) {
	c := &Conductor{}
	if !c.isQueryTask("Show git status", "Run command: git status", nil) {
		t.Fatal("a read-only request should be classified as a query")
	}
}

func TestIsQueryTaskRecognizesPortugueseEditRequest(t *testing.T) {
	c := &Conductor{}
	task := "Corrija o bug de concorrência sem mudar a API pública."
	plan := "Run command: go test -race ./...\nFiles to modify: none"

	if c.isQueryTask(task, plan, nil) {
		t.Fatal("a Portuguese edit request must not be reported as a successful query")
	}
}
