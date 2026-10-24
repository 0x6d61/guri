package core

import (
	"context"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
)

func TestEvalTailCallsUseCurrentVM(t *testing.T) {
	r := NewRuntime()
	result, err := r.Run(context.Background(), "tail-eval.scm", `(import (scheme base) (scheme eval) (scheme repl))
 (define e (interaction-environment))
 (eval '(begin
   (define e (interaction-environment))
   (define (loop n) (if (= n 0) 42 (eval (list 'loop (- n 1)) e)))) e)
 (eval '(loop 10000) e)`)
	if err != nil || result.Len() != 1 || result.Values()[0].(*scheme.Integer).BigInt().Int64() != 42 {
		t.Fatal(result, err)
	}
	if stats := r.bootstrap.Stats(); stats.PeakFrames > 12 {
		t.Fatal("eval accumulated VM frames", stats)
	}
}
