package core_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
)

func TestGraphWikiExamples(t *testing.T) {
	data, err := os.ReadFile("../../docs/wiki/Graph-IO.md")
	if err != nil {
		t.Fatal(err)
	}
	var e *core.Engine
	source := ""
	active := false
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "```scheme" {
			active = true
			e = engine()
			source = ""
			continue
		}
		if line == "```" {
			active = false
			continue
		}
		if !active {
			continue
		}
		code, want, has := strings.Cut(line, "; =>")
		source += code + "\n"
		if !has {
			continue
		}
		actual := eval(t, e, source)
		expected := eval(t, e, "(quote "+strings.TrimSpace(want)+")")
		if !equality.Equal(actual, expected) {
			t.Fatal(source, actual, want)
		}
		source = ""
		count++
	}
	if count != 5 {
		t.Fatal("update example inventory", count)
	}
}
