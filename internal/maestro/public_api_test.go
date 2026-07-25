package maestro

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublicAPIChangesDetectsAddedExportedMethod(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "store.go")
	before := `package session
type Store struct{}
func (s *Store) Put() {}
`
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotGoPublicAPI(root)
	if err != nil {
		t.Fatal(err)
	}

	after := before + "func (s *Store) Delete() {}\n"
	if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	current, err := snapshotGoPublicAPI(root)
	if err != nil {
		t.Fatal(err)
	}

	changes := publicAPIChanges(snapshot, current)
	if len(changes) != 1 || changes[0] != "added store.go:func:Store.Delete" {
		t.Fatalf("unexpected public API changes: %v", changes)
	}
}

func TestPublicAPIChangesIgnoresPrivateStructFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "store.go")
	before := "package session\ntype Store struct { sessions map[string]string }\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotGoPublicAPI(root)
	if err != nil {
		t.Fatal(err)
	}

	after := "package session\nimport \"sync\"\ntype Store struct { mu sync.RWMutex; sessions map[string]string }\n"
	if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	current, err := snapshotGoPublicAPI(root)
	if err != nil {
		t.Fatal(err)
	}
	if changes := publicAPIChanges(snapshot, current); len(changes) != 0 {
		t.Fatalf("private field change reported as public API change: %v", changes)
	}
}

func TestRequiresStablePublicAPI(t *testing.T) {
	for _, task := range []string{
		"Fix the race without changing its public API.",
		"Corrija o race sem mudar a API pública.",
	} {
		if !requiresStablePublicAPI(task) {
			t.Fatalf("constraint not detected in %q", task)
		}
	}
}
