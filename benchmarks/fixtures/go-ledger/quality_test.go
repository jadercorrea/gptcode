package ledger

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"testing"
)

func TestLedgerImplementationIsFormatted(t *testing.T) {
	source, err := os.ReadFile("ledger.go")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(source)
	if err != nil {
		t.Fatalf("format.Source() error = %v", err)
	}
	if !bytes.Equal(source, formatted) {
		t.Fatal("ledger.go is not gofmt-formatted")
	}
}

func TestLedgerImplementationAvoidsUnsafe(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "ledger.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == "unsafe" {
			t.Fatal("ledger.go must not depend on package unsafe for lock ordering")
		}
	}
}
