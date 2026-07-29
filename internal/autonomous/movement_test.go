package autonomous

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMovementAcceptsStringAndStructuredFileReferences(t *testing.T) {
	var movement Movement
	err := json.Unmarshal([]byte(`{
		"id": "movement-1",
		"required_files": [
			"task.md",
			{"path": "cache.go", "content_type": "read"}
		],
		"output_files": [
			{"path": "cache.go"}
		]
	}`), &movement)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if want := (FileReferences{"task.md", "cache.go"}); !reflect.DeepEqual(movement.RequiredFiles, want) {
		t.Fatalf("RequiredFiles = %v, want %v", movement.RequiredFiles, want)
	}
	if want := (FileReferences{"cache.go"}); !reflect.DeepEqual(movement.OutputFiles, want) {
		t.Fatalf("OutputFiles = %v, want %v", movement.OutputFiles, want)
	}
}

func TestMovementRejectsStructuredFileWithoutPath(t *testing.T) {
	var movement Movement
	err := json.Unmarshal([]byte(`{"required_files":[{"content_type":"read"}]}`), &movement)
	if err == nil {
		t.Fatal("json.Unmarshal() error = nil, want missing path rejection")
	}
}
