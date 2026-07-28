package expiringcache

import (
	"bytes"
	"go/format"
	"os"
	"testing"
)

func TestCacheImplementationIsFormatted(t *testing.T) {
	source, err := os.ReadFile("cache.go")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(source)
	if err != nil {
		t.Fatalf("format.Source() error = %v", err)
	}
	if !bytes.Equal(source, formatted) {
		t.Fatal("cache.go is not gofmt-formatted")
	}
}
