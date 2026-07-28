package safestore

import (
	"bytes"
	"go/format"
	"os"
	"testing"
)

func TestStoreImplementationIsFormatted(t *testing.T) {
	source, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(source)
	if err != nil {
		t.Fatalf("format.Source() error = %v", err)
	}
	if !bytes.Equal(source, formatted) {
		t.Fatal("store.go is not gofmt-formatted")
	}
}
