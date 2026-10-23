package core_test

import (
	"os"
	"strings"
	"testing"

	numeric "github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestStandardNumbersInVM(t *testing.T) {
	for _, src := range []string{
		`(= (+ 9223372036854775808 1) 9223372036854775809)`,
		`(> 9007199254740993 9007199254740992.0)`,
		`(= (/ 1 3) 1/3)`,
		`(call-with-values (lambda () (floor/ -5 2)) (lambda (q r) (and (= q -3) (= r 1))))`,
		`(call-with-values (lambda () (exact-integer-sqrt 100000000000000000000)) (lambda (s r) (and (= s 10000000000) (= r 0))))`,
		`(guard (e (else #t)) (/ 1 0) #f)`,
		`(guard (e (else #t)) (+ "wrong" 1) #f)`,
		`(= (string->number (number->string 3/7 16) 16) 3/7)`,
		`(= (string->number "#x10" 2) 16)`,
		`(if (string->number "bad") #f #t)`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), src); v != scheme.True {
				t.Fatal(v)
			}
		})
	}
}

func TestNumericWikiExamples(t *testing.T) {
	data, err := os.ReadFile("../../docs/wiki/Numbers.md")
	if err != nil {
		t.Fatal(err)
	}
	active := false
	source := ""
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "```scheme" {
			active = true
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
		v := eval(t, engine(), source)
		want = strings.TrimSpace(want)
		switch want {
		case "#t", "#f":
			if v != scheme.Boolean(want == "#t") {
				t.Fatalf("%s: got %v want %s", source, v, want)
			}
		default:
			n, err := scheme.ParseNumber(want)
			if err != nil {
				t.Fatal(err)
			}
			actual, ok := v.(scheme.Number)
			if !ok || !numeric.Equivalent(actual, n) {
				t.Fatalf("%s: got %v want %s", source, v, want)
			}
		}
		count++
		source = ""
	}
	if count != 19 {
		t.Fatalf("checked %d examples, update expectation when changing documentation", count)
	}
}
