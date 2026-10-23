package reader_test

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func TestCherokeeCaseFoldingIsIdempotent(t *testing.T) {
	symbols := scheme.NewSymbolTable()
	nodes, err := reader.ReadAll("fold.scm", "#!fold-case Ꭰ ꭰ", symbols)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if got := node.Datum().(*scheme.Symbol).Name(); got != "Ꭰ" {
			t.Fatalf("got %q want Cherokee uppercase", got)
		}
	}
}
