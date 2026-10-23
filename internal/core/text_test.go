package core_test

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
	"golang.org/x/text/cases"
)

func TestCharactersAndUnicodeCasing(t *testing.T) {
	for _, src := range []string{
		`(and (char? #\λ) (= (char->integer #\λ) #x3bb) (char=? (integer->char #x1f600) #\😀))`,
		`(and (char<? #\a #\b #\λ) (char<=? #\a #\a) (char>? #\b #\a) (char>=? #\a #\a))`,
		`(and (char-ci=? #\Σ #\ς #\σ) (char-ci<? #\A #\b) (char-ci<=? #\A #\a) (char-ci>? #\B #\a) (char-ci>=? #\A #\a))`,
		`(and (char=? (char-foldcase #\ẞ) #\ß) (char=? (char-foldcase #\İ) #\İ) (char=? (char-foldcase #\x0345) #\ι))`,
		`(and (char-alphabetic? #\x0345) (char-lower-case? #\x0345) (char-upper-case? #\x2160) (char-whitespace? #\x2003))`,
		`(and (char-numeric? #\٤) (= (digit-value #\٤) 4) (= (digit-value #\𝟡) 9) (not (digit-value #\Ⅳ)))`,
		`(and (char=? (char-upcase #\λ) #\Λ) (char=? (char-downcase #\Λ) #\λ))`,
		`(and (string=? (string-upcase "straße") "STRASSE") (string=? (string-downcase "ΟΣ") "ος") (string=? (string-foldcase "Straße Σς") "strasse σσ"))`,
		`(and (string-ci=? "Straße" "STRASSE") (string-ci<? "a" "B") (string-ci<=? "A" "a") (string-ci>? "B" "a") (string-ci>=? "A" "a"))`,
		`(and (string<? "a" "aa" "λ") (string<=? "a" "a") (string>? "λ" "a") (string>=? "a" "a"))`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), src); v != scheme.True {
				t.Fatal(v)
			}
		})
	}
}
func TestStringsMutationRangesAndCopies(t *testing.T) {
	for src, want := range map[string]string{
		`(string-length "Aλ😀")`: `3`, `(char->integer (string-ref "Aλ😀" 2))`: `128512`,
		`(substring "Aλ😀" 1 3)`: `"λ😀"`, `(string-copy "Aλ😀" 1 2)`: `"λ"`,
		`(list->string (list #\λ #\😀))`: `"λ😀"`, `(string #\a #\λ)`: `"aλ"`, `(make-string 2 #\λ)`: `"λλ"`,
		`(define s (string-copy "abc")) (string-set! s 1 #\😀) s`:                                      `"a😀c"`,
		`(define a (string-copy "abc")) (define b (string-copy a)) (string-fill! b #\x 1) (list a b)`: `("abc" "axx")`,
		`(define s (string-copy "abcde")) (string-copy! s 1 s 0 4) s`:                                 `"aabcd"`,
		`(define s (string-copy "abcde")) (string-copy! s 0 s 1 5) s`:                                 `"bcdee"`,
		`(string-append "a" "λ" "😀")`:                                                                 `"aλ😀"`, `(string-append)`: `""`,
		`(list->string (string->list "aλ😀" 1 3))`:                                             `"λ😀"`,
		`(define s (string-copy "abc")) (string-fill! s #\λ 1 2) s`:                           `"aλc"`,
		`(string-map char-upcase "aλ")`:                                                       `"AΛ"`,
		`(string-map (lambda (a b) b) "long" "λ😀")`:                                           `"λ😀"`,
		`(define n 0) (string-for-each (lambda (c) (set! n (+ n (char->integer c)))) "ab") n`: `195`,
	} {
		t.Run(src, func(t *testing.T) {
			if got := repr(eval(t, engine(), src)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
	for _, src := range []string{
		`(string-ref "a" 1)`, `(substring "abc" 2 1)`, `(string-copy "a" 0 2)`,
		`(string-set! "abc" 0 #\x)`, `(string-fill! "abc" #\x 1 1)`, `(string-copy! "a" 0 "" 0 0)`,
		`(integer->char #xd800)`, `(integer->char #x110000)`, `(integer->char -1)`, `(integer->char 1.0)`,
		`(string-ref "a" 0.0)`, `(list->string '(1))`, `(string-map (lambda (c) 1) "a")`,
		`(string-copy! (make-string 1) 0 "ab")`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), `(guard (e (else #t)) `+src+` #f)`); v != scheme.True {
				t.Fatal("accepted invalid input", v)
			}
		})
	}
}

func TestStringCallbackContinuationsAndDynamicScope(t *testing.T) {
	e := engine()
	first := eval(t, e, `(define saved #f) (define result (string-map (lambda (c) (if (char=? c #\b) (call/cc (lambda (k) (set! saved k) c)) c)) "abc")) result`).(*scheme.String)
	replacement, err := scheme.NewCharacter('X')
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Set(0, replacement); err != nil {
		t.Fatal(err)
	}
	if got := repr(eval(t, e, `(saved #\λ)`)); got != `"aλc"` {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(saved #\😀)`)); got != `"a😀c"` {
		t.Fatal(got)
	}
	if first.Text() != "Xbc" {
		t.Fatal("earlier map result changed", first.Text())
	}
	e = engine()
	if got := number(eval(t, e, `(define saved #f) (define n 0) (string-for-each (lambda (c) (if (char=? c #\b) (call/cc (lambda (k) (set! saved k) #f)) #f) (set! n (+ n 1))) "abc") n`)); got != 3 {
		t.Fatal(got)
	}
	if got := number(eval(t, e, `(saved #f)`)); got != 5 {
		t.Fatal(got)
	}
	if got := number(eval(t, e, `(saved #f)`)); got != 7 {
		t.Fatal(got)
	}
	for _, src := range []string{
		`(define p (make-parameter #\λ)) (string=? (parameterize ((p #\😀)) (string-map (lambda (c) (p)) "ab")) "😀😀")`,
		`(begin (string-for-each (lambda (c) (values 1 2)) "ab") #t)`,
		`(guard (e (else #t)) (string-map (lambda (c) (values c c)) "a") #f)`,
		`(= (string-length (string-map (lambda (c) c) (make-string 20000 #\λ))) 20000)`,
		`(string=? (string-map (lambda (a b) b) "" "abc") "")`,
	} {
		e := engine()
		if v := eval(t, e, src); v != scheme.True {
			t.Fatal(src, v)
		}
		if e.Stats().PeakFrames > 8 {
			t.Fatal("string iteration retained callback frames", e.Stats())
		}
	}
}

func TestUnicodeSimpleFoldExceptionsAndVersion(t *testing.T) {
	if unicode.Version != "15.0.0" || cases.UnicodeVersion != unicode.Version {
		t.Fatal("revalidate Unicode data", unicode.Version, cases.UnicodeVersion)
	}
	for _, src := range []string{
		`(char=? (char-foldcase #\ß) #\ß)`, `(char=? (char-foldcase #\x1f88) #\x1f80)`,
		`(char=? (char-foldcase #\x1fb3) #\x1fb3)`, `(char=? (char-foldcase #\x1fbc) #\x1fb3)`,
		`(char=? (char-foldcase #\x0149) #\x0149)`, `(char=? (char-foldcase #\x0131) #\x0131)`,
		`(char=? (char-foldcase #\xFB00) #\xFB00)`,
	} {
		if v := eval(t, engine(), src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
}

func TestCherokeeFoldingCanonicalUppercase(t *testing.T) {
	for _, src := range []string{
		`(char=? (char-foldcase #\Ꭰ) #\Ꭰ)`, `(char=? (char-foldcase #\ꭰ) #\Ꭰ)`,
		`(string=? (string-foldcase "Ꭰꭰ") "ᎠᎠ")`, `(string-ci=? "Ꭰ" "ꭰ")`,
	} {
		if v := eval(t, engine(), src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
}

func TestTextWikiExamplesAndBindings(t *testing.T) {
	data, err := os.ReadFile("../../docs/wiki/Characters-Strings.md")
	if err != nil {
		t.Fatal(err)
	}
	var e *core.Engine
	active := false
	source := ""
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
		if got := repr(eval(t, e, source)); got != strings.TrimSpace(want) {
			t.Fatalf("%s: got %s want %s", source, got, want)
		}
		count++
		source = ""
	}
	if count != 23 {
		t.Fatal("update example inventory", count)
	}
	file, err := os.Open("../../docs/r7rs/exports.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	e = engine()
	for _, row := range rows[1:] {
		if row[3] != "56" || seen[row[1]] {
			continue
		}
		seen[row[1]] = true
		if _, ok := eval(t, e, row[1]).(*scheme.Procedure); !ok {
			t.Fatal("not installed", row[1])
		}
	}
	if len(seen) != 50 {
		t.Fatal("update function inventory", len(seen))
	}
}
