package guri

import (
	"bytes"
	"errors"
	"testing"
)

func TestPrintln(t *testing.T) {
	for _, tt := range []struct{ source, want string }{
		{`(println)`, "\n"},
		{`(println "hello" 1 1.0 true false nil :name 'x)`, "hello 1 1.0 true false nil :name x\n"},
		{`(println ["a" 2] '(x "b") {:x "c"})`, "[\"a\" 2] (x \"b\") {:x \"c\"}\n"},
		{`(def a [nil]) (assoc! a 0 a) (println a)`, "[#<cycle>]\n"},
		{`(def m {}) (assoc! m :self m) (println m)`, "{:self #<cycle>}\n"},
		{`(println (fn [] nil))`, "#<function>\n"},
	} {
		t.Run(tt.source, func(t *testing.T) {
			var out bytes.Buffer
			i := NewInterpreter()
			i.Output = &out
			value, err := i.Eval("print.guri", tt.source)
			if err != nil || value != nil || out.String() != tt.want {
				t.Fatalf("value=%#v err=%v output=%q, want %q", value, err, out.String(), tt.want)
			}
		})
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("output failed") }

func TestPrintlnWriteError(t *testing.T) {
	i := NewInterpreter()
	i.Output = brokenWriter{}
	if _, err := i.Eval("print.guri", `(println "x")`); err == nil {
		t.Fatal("write error ignored")
	}
}
