package core

import (
	"context"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
)

func TestAuditCombinedControlAndData(t *testing.T) {
	cases := []string{
		`(import (scheme base) (scheme lazy))
 (define setting (make-parameter 1)) (define hooks 0) (define handled 0) (define saved #f) (define resumed #f)
 (define p (delay (dynamic-wind
  (lambda () (set! hooks (+ hooks 1)))
  (lambda () (raise-continuable (call/cc (lambda (k) (set! saved k) 'signal))))
  (lambda () (set! hooks (+ hooks 1))))))
 (define result (parameterize ((setting 42))
  (with-exception-handler (lambda (e) (set! handled (+ handled 1)) (setting)) (lambda () (force p)))))
 (if (not resumed) (begin (set! resumed #t) (saved 'again))
  (and (= result 42) (= (force p) 42) (= hooks 4) (= handled 2) (= (setting) 1)))`,
		`(import (scheme base))
 (define setting (make-parameter 1))
 (define result (parameterize ((setting 42))
  (call-with-values
   (lambda () (values (map (lambda (n) (with-exception-handler (lambda (e) (+ (setting) e)) (lambda () (raise-continuable n)))) '(1 2))))
   (lambda (xs) (apply + xs)))))
 (and (= result 87) (= (setting) 1))`,
		`(import (scheme base) (scheme read) (scheme write) (scheme eval))
 (define original (read (open-input-string "#0=(a . #0#)")))
 (define quoted (eval (list 'quote original) (environment '(scheme base))))
 (define out (open-output-string)) (write-shared quoted out)
 (define copy (read (open-input-string (get-output-string out))))
 (and (eq? original quoted) (eq? copy (cdr copy)) (equal? original copy))`,
		`(import (scheme base) (scheme read))
 (define closed #f)
 (guard (e ((read-error? e) closed) (else #f))
  (dynamic-wind (lambda () #t) (lambda () (read (open-input-string "("))) (lambda () (set! closed #t))))`,
	}
	for i, source := range cases {
		r := NewRuntime()
		result, err := r.Run(context.Background(), "audit-integration.scm", source)
		if err != nil {
			t.Fatal(i, err)
		}
		value, err := result.Single()
		if err != nil || value != scheme.True {
			t.Fatal(i, result, err)
		}
	}
}
func TestAuditImportedProperTailCalls(t *testing.T) {
	for _, step := range []string{
		`(apply loop (list (- n 1)))`,
		`(call/cc (lambda (saved) (loop (- n 1))))`,
		`(call-with-values (lambda () (values (- n 1))) loop)`,
	} {
		r := NewRuntime()
		result, err := r.Run(context.Background(), "audit-tail.scm", `(import (scheme base)) (let loop ((n 100000)) (if (= n 0) 42 `+step+`))`)
		if err != nil || result.Len() != 1 {
			t.Fatal(err, result)
		}
		if result.Values()[0].(*scheme.Integer).BigInt().Int64() != 42 || r.bootstrap.Stats().PeakFrames > 12 {
			t.Fatal(result, r.bootstrap.Stats())
		}
	}
}
