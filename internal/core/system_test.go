package core_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestEnvironmentVariableValuesAndImmutability(t *testing.T) {
	t.Setenv("GURI_TEST_SYSTEM_VALUE", "first")
	t.Setenv("GURI_TEST_SYSTEM_EMPTY", "")
	r := core.NewRuntime()
	source := `(import (scheme base) (scheme process-context))
 (define value (get-environment-variable "GURI_TEST_SYSTEM_VALUE"))
 (define item (assoc "GURI_TEST_SYSTEM_VALUE" (get-environment-variables)))
 (and (string=? value "first") (string=? (cdr item) "first")
  (string=? (get-environment-variable "GURI_TEST_SYSTEM_EMPTY") "")
  (guard (e (else (error-object? e))) (string-set! value 0 #\x) #f)
  (guard (e (else (error-object? e))) (set-cdr! item "changed") #f)
  (guard (e (else (error-object? e))) (string-set! (car item) 0 #\x) #f)
  (guard (e (else (error-object? e))) (get-environment-variable 42) #f))`
	if got := runProgram(t, r, source); got != scheme.True {
		t.Fatal(got)
	}
	t.Setenv("GURI_TEST_SYSTEM_VALUE", "second")
	if got := runProgram(t, r, `(import (scheme base) (scheme process-context)) (string=? (get-environment-variable "GURI_TEST_SYSTEM_VALUE") "second")`); got != scheme.True {
		t.Fatal(got)
	}
}
func TestSystemLibraryTypesAndFeatures(t *testing.T) {
	r := core.NewRuntime()
	source := `(import (scheme base) (scheme process-context) (scheme time))
 (define args (command-line)) (define start (current-jiffy)) (define end (current-jiffy))
 (and (pair? args) (string? (car args))
  (guard (e (else (error-object? e))) (string-set! (car args) 0 #\x) #f)
  (inexact? (current-second)) (real? (current-second))
  (exact-integer? start) (<= start end) (exact-integer? (jiffies-per-second)) (> (jiffies-per-second) 0)
  (not (memq 'r7rs (features)))
  (cond-expand ((and (library (scheme process-context)) (library (scheme time))) #t) (else #f)))`
	if got := runProgram(t, r, source); got != scheme.True {
		t.Fatal(got)
	}
}
func TestREPLExitStopsInputWithoutErrorDiagnostic(t *testing.T) {
	for _, name := range []string{"exit", "emergency-exit"} {
		var out, diagnostic bytes.Buffer
		err := core.NewRuntime().REPL(context.Background(), "exit-repl.scm", strings.NewReader("("+name+" 7)\n42\n"), &out, &diagnostic)
		var exit *core.ExitRequest
		if !errors.As(err, &exit) || exit.Code != 7 || out.Len() != 0 || diagnostic.Len() != 0 {
			t.Fatal(err, out.String(), diagnostic.String())
		}
	}
}
func TestExitThroughEvalLoadAndLibraryInitializer(t *testing.T) {
	for _, body := range []string{
		`(import (scheme base) (scheme process-context)) (exit 7) 42`,
		`(import (scheme base) (scheme eval)) (eval '(exit 7) (environment '(scheme process-context))) 42`,
	} {
		_, err := core.NewRuntime().Run(context.Background(), "exit-program.scm", body)
		var exit *core.ExitRequest
		if !errors.As(err, &exit) || exit.Code != 7 {
			t.Fatal(err)
		}
	}
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (stop) (export value) (import (scheme base) (scheme process-context)) (begin (define value (exit 7))))`)
	_, err := r.Run(context.Background(), "initializer.scm", `(import (stop)) value`)
	var exit *core.ExitRequest
	if !errors.As(err, &exit) || exit.Code != 7 {
		t.Fatal(err)
	}
}

func TestExitFromLoadedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stop.scm")
	if err := os.WriteFile(path, []byte(`(exit 7) (error "continued")`), 0600); err != nil {
		t.Fatal(err)
	}
	source := `(import (scheme base) (scheme load)) (load ` + strconv.Quote(path) + `) 42`
	_, err := core.NewRuntime().Run(context.Background(), "load-exit.scm", source)
	var exit *core.ExitRequest
	if !errors.As(err, &exit) || exit.Code != 7 {
		t.Fatal(err)
	}
}

func TestSystemInterfaceWikiExamplesAndBindings(t *testing.T) {
	t.Setenv("GURI_SYSTEM_WIKI", "example")
	doc, err := os.ReadFile("../../docs/wiki/System-Interface.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	if len(blocks) != 7 {
		t.Fatal(len(blocks))
	}
	for i, block := range blocks {
		r := core.NewRuntime()
		if i == 3 || i == 4 {
			var out, diagnostic bytes.Buffer
			err := r.REPL(context.Background(), "system-example.scm", strings.NewReader(block[1]), &out, &diagnostic)
			var exit *core.ExitRequest
			expected := "開始\n後処理\n"
			if i == 4 {
				expected = "開始\n"
			}
			if !errors.As(err, &exit) || exit.Code != 7 || exit.Emergency != (i == 4) || out.String() != expected || diagnostic.Len() != 0 {
				t.Fatal(i, err, out.String(), diagnostic.String())
			}
		} else if got := runProgram(t, r, block[1]); got != scheme.True {
			t.Fatal(i, got)
		}
	}
	f, err := os.Open("../../docs/r7rs/exports.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	libraries := map[string]bool{}
	systemCount := 0
	for _, row := range rows[1:] {
		libraries[row[0]] = true
		if row[5] != "内部Runtimeのimport実装済み・公開CLI未接続" {
			t.Fatal(row)
		}
		if row[0] != "(scheme process-context)" && row[0] != "(scheme time)" {
			continue
		}
		systemCount++
		if row[3] != "65" || (!strings.Contains(string(doc), "("+row[1]+" ") && !strings.Contains(string(doc), "("+row[1]+")")) {
			t.Fatal(row)
		}
	}
	if len(libraries) != 16 || len(rows)-1 != 557 || systemCount != 8 {
		t.Fatal(len(libraries), len(rows), systemCount)
	}
}
