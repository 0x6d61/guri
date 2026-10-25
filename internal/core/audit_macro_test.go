// Regression cases adapted from Chibi r7rs-tests.scm, revision
// b2087f3a320a56f79ede5c546fd45fdde582c4ed, lines 557-592.
// Includes additional Guri-authored variants for values, records and isolation.
//
// Copyright (c) 2009-2021 Alex Shinn
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. The name of the author may not be used to endorse or promote products
//    derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS OR
// IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES
// OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
// IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT,
// INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT
// NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
// THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package core_test

import (
	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func TestIntroducedGlobalDefinitionsShareForwardCells(t *testing.T) {
	e := core.NewCore()
	source := `(define-syntax make-functions
 (syntax-rules () ((_ f) (begin (define (f x) (helper x)) (define (helper x) (* x x))))))
 (make-functions square-it)
 (square-it 10)`
	if got := number(eval(t, e, source)); got != 100 {
		t.Fatal(got)
	}
}
func TestNestedMacroPatternVariablesUseIdentifierContext(t *testing.T) {
	source := `(let-syntax ((m (syntax-rules ()
 ((_ x) (let-syntax ((n (syntax-rules (k) ((n x) 'matched) ((n y) 'unmatched)))) (n z)))))) (m k))`
	got := eval(t, core.NewCore(), source)
	if got.(*scheme.Symbol).Name() != "matched" {
		t.Fatal(got)
	}
}

func TestIntroducedGlobalsKeepPublicBindingsSeparate(t *testing.T) {
	source := `(define (helper x) -1)
 (define-syntax make-functions (syntax-rules () ((_ f) (begin (define (f x) (helper x)) (define (helper x) (* x x))))))
 (make-functions square-it)
 (+ (square-it 10) (helper 0))`
	if got := number(eval(t, core.NewCore(), source)); got != 99 {
		t.Fatal(got)
	}
}
func TestIntroducedGlobalValuesShareForwardCells(t *testing.T) {
	source := `(define-syntax make-value (syntax-rules () ((_ f) (begin (define (f) private-value) (define-values (private-value) (values 42))))))
 (make-value answer) (answer)`
	if got := number(eval(t, core.NewCore(), source)); got != 42 {
		t.Fatal(got)
	}
}

func TestIntroducedRecordProceduresShareForwardCells(t *testing.T) {
	source := `(define-syntax make-record-functions (syntax-rules () ((_ f) (begin (define (f) (private-ref (private-new 42))) (define-record-type private-type (private-new value) private? (value private-ref))))))
 (make-record-functions answer) (answer)`
	if got := number(eval(t, core.NewCore(), source)); got != 42 {
		t.Fatal(got)
	}
}
