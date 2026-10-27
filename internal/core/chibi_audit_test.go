package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
	"github.com/0x6d61/guri/internal/writer"
)

func auditDisplay(v scheme.Value) string {
	var b bytes.Buffer
	_ = writer.Write(context.Background(), &b, v, writer.Shared)
	return b.String()
}
func auditNumber(n scheme.Number) complex128 {
	switch x := n.(type) {
	case *scheme.Integer:
		f, _ := x.BigInt().Float64()
		return complex(f, 0)
	case *scheme.Rational:
		f, _ := x.BigRat().Float64()
		return complex(f, 0)
	case scheme.Float:
		return complex(x.Float64(), 0)
	case *scheme.Complex:
		a, b := x.Parts()
		return complex(real(auditNumber(a)), real(auditNumber(b)))
	}
	panic("number")
}
func auditNear(a, b scheme.Value) bool {
	if equality.Equal(a, b) {
		return true
	}
	x, xok := a.(scheme.Number)
	y, yok := b.(scheme.Number)
	if !xok || !yok {
		return false
	}
	if x.Exact() && y.Exact() {
		return false
	}
	ca, cb := auditNumber(x), auditNumber(y)
	near := func(a, b float64) bool {
		return a == b || math.IsNaN(a) && math.IsNaN(b) || !math.IsInf(a, 0) && !math.IsInf(b, 0) && math.Abs(a-b) <= 1e-6*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	}
	return near(real(ca), real(cb)) && near(imag(ca), imag(cb))
}

const chibiRevision = "b2087f3a320a56f79ede5c546fd45fdde582c4ed"
const chibiSourceHash = "004d215a38f990bbf2bfaa8b4a395aa59ef98d155096f8e31cc361b7812f7d6a"
const auditTestAdapter = `(define-library (chibi test)
 (export test test-begin test-end test-assert test-error test-values)
 (import (scheme base) (guri audit-bridge))
 (begin
  (define (test-begin . ignored) #f)
  (define (test-end . ignored) #f)
  (define-syntax test (syntax-rules ()
   ((_ expected expression) (audit-check expected expression 'expression))
   ((_ name expected expression) (audit-check expected expression 'expression))))
  (define-syntax test-assert (syntax-rules ()
   ((_ expression) (audit-check #t (if expression #t #f) 'expression))
   ((_ name expression) (audit-check #t (if expression #t #f) name))))
  (define-syntax test-values (syntax-rules () ((_ expected expression)
   (test (call-with-values (lambda () expected) list)
         (call-with-values (lambda () expression) list)))))
  (define-syntax test-error (syntax-rules () ((_ expression)
   (guard (e (else (audit-check #t #t 'expression)))
    expression (audit-check #t #f 'expression)))))
 ))`

type auditObservation struct {
	Line                 int    `json:"line"`
	Section              string `json:"section"`
	Status               string `json:"status"`
	Expression           string `json:"expression"`
	Expected             string `json:"expected,omitempty"`
	Actual               string `json:"actual,omitempty"`
	Reason               string `json:"reason,omitempty"`
	AssertionsNotReached int    `json:"assertions_not_reached,omitempty"`
}
type sourceAuditReport struct {
	SchemaVersion        int                `json:"schema_version"`
	Engine               string             `json:"engine"`
	Platform             string             `json:"platform"`
	Revision             string             `json:"upstream_revision"`
	SHA256               string             `json:"upstream_sha256"`
	Comparator           string             `json:"comparator"`
	TopLevelForms        int                `json:"top_level_forms"`
	AssertionsEvaluated  int                `json:"assertions_evaluated"`
	SuitePassed          bool               `json:"suite_passed"`
	NoUnexpectedFailures bool               `json:"no_unexpected_failures"`
	Counts               map[string]int     `json:"counts"`
	Observations         []auditObservation `json:"observations"`
}

// This is an internal source audit, not the public-CLI measurement. The original
// bytes remain unchanged; only the (chibi test) test API is supplied by our own
// adapter. Forms are compiled sequentially with shared locations so a failed
// form does not prevent observing later independent forms. This is not an
// isolation guarantee for assertions that depend on earlier definitions.
func TestChibiSourceAudit(t *testing.T) {
	r := NewRuntime()
	engine := r.newEngine()
	engine.strict = false
	report := sourceAuditReport{SchemaVersion: 1, Engine: "internal/core.Runtime", Platform: runtime.GOOS + "/" + runtime.GOARCH, Revision: chibiRevision, SHA256: chibiSourceHash, Comparator: "equal?; root inexact numbers: relative/absolute tolerance 1e-6; exact numbers never rounded", Counts: map[string]int{}}
	section := ""
	topLine := 0
	record := func(o auditObservation) {
		report.Counts[o.Status]++
		report.Observations = append(report.Observations, o)
	}
	bridge := vm.Native("audit-check", 3, 3, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		report.AssertionsEvaluated++
		o := auditObservation{Line: c.Source().Line, Section: section, Status: "pass", Expression: auditDisplay(a[2]), Expected: auditDisplay(a[0]), Actual: auditDisplay(a[1])}
		if !auditNear(a[0], a[1]) {
			o.Status = "fail"
		}
		record(o)
		return vm.Return(scheme.Unspecified), nil
	})
	b := &binding{id: scheme.NewIdentifier(r.bootstrap.symbols.Intern("audit-check")), cell: scheme.NewCell(bridge), declared: true}
	bridgeNodes, err := reader.ReadAll("bridge.sld", "(guri audit-bridge)", r.bootstrap.symbols)
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := parseLibraryName(bridgeNodes[0])
	if err != nil {
		t.Fatal(err)
	}
	r.modules[key] = &library{exports: libraryExports{"audit-check": b}, prepared: true, initialized: true}
	if err := r.Register("adapter.sld", auditTestAdapter); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/r7rs/upstream/chibi/r7rs-tests.scm")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != chibiSourceHash {
		t.Fatal("upstream source changed; review classifications and provenance")
	}
	ns, err := reader.ReadAll("chibi-r7rs-tests.scm", string(data), r.bootstrap.symbols)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	r.context = ctx
	var deps []*library
	optionalMarkers := map[int]string{2356: "1s2", 2357: "1S2", 2358: "1f2", 2359: "1F2", 2360: "1d2", 2361: "1D2", 2362: "1l2", 2363: "1L2", 2400: "1s2+1.0i", 2402: "1.0+1s2i"}
	heading := regexp.MustCompile(`^[1-7](?:\.[0-9]+)*`)
	for _, n := range ns {
		topLine = n.Source().Line
		if rawForm(n, "import") {
			deps, err = r.applyImports(ctx, engine, n)
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		report.TopLevelForms++
		if rawForm(n, "test-begin") {
			parts, e := nodes(n)
			if e == nil && len(parts) == 2 {
				if title, ok := parts[1].Datum().(*scheme.String); ok {
					if h := heading.FindString(title.Text()); h != "" {
						section = h
					}
				}
			}
		}
		code, err := engine.compileEvaluation(ctx, []*reader.Node{n})
		if err == nil {
			main := vm.Closure(code)
			entry := vm.Native("audit-entry", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
				return initializeLibraries(c, dependencyOrder(deps), 0, main)
			})
			_, err = r.bootstrap.machine.Run(ctx, entry)
		}
		if err != nil {
			o := auditObservation{Line: topLine, Section: section, Status: "fail", Expression: auditDisplay(n.Datum()), Actual: err.Error()}
			if rawForm(n, "test-error") && (errors.Is(err, vm.ErrArity) || errors.Is(err, scheme.ErrType)) {
				report.AssertionsEvaluated++
				o.Status = "pass"
				o.Expected = "error detected"
				o.Reason = "error-optional: the report does not require a Scheme raise for these apply domain/arity errors"
			} else if input, ok := optionalMarkers[topLine]; ok && auditReadError(err) && rawForm(n, "test-numeric-syntax") {
				parts, e := nodes(n)
				if e == nil && len(parts) > 1 {
					if value, ok := parts[1].Datum().(*scheme.String); ok && value.Text() == input {
						o.Status = "not-adopted"
						o.Reason = "R7RS 6.2.5 optional precision exponent markers s/f/d/l are not selected in the binary64 profile"
						o.AssertionsNotReached = 2
					}
				}
			}
			record(o)
		}
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
	}
	report.SuitePassed = report.Counts["fail"] == 0 && report.Counts["not-adopted"] == 0 && report.Counts["deferred"] == 0
	report.NoUnexpectedFailures = report.Counts["fail"] == 0
	if path := os.Getenv("GURI_AUDIT_REPORT"); path != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range report.Observations {
		if o.Status == "fail" {
			t.Errorf("line %d: %s expected %s got %s", o.Line, o.Expression, o.Expected, o.Actual)
		}
	}
	if report.AssertionsEvaluated != 1214 || report.TopLevelForms < 1000 || report.Counts["not-adopted"] != 10 || report.Counts["deferred"] != 0 || report.Counts["pass"] != 1214 {
		t.Fatalf("unexpected audit coverage: forms=%d checks=%d counts=%v", report.TopLevelForms, report.AssertionsEvaluated, report.Counts)
	}
	t.Logf("forms=%d assertions=%d counts=%v raw_suite_passed=%v no_unexpected_failures=%v", report.TopLevelForms, report.AssertionsEvaluated, report.Counts, report.SuitePassed, report.NoUnexpectedFailures)
}
func auditReadError(err error) bool {
	var raised *scheme.Raised
	if !errors.As(err, &raised) {
		return false
	}
	rec, ok := raised.Object().(*scheme.Record)
	if !ok || rec.RecordType() != exceptionType {
		return false
	}
	value, e := rec.Ref(2)
	if e != nil {
		return false
	}
	return value.(*scheme.Integer).BigInt().Int64()&int64(ErrorRead) != 0
}
func TestAuditComparatorDoesNotRoundExactIntegers(t *testing.T) {
	a, err := scheme.ParseNumber("9007199254740992")
	if err != nil {
		t.Fatal(err)
	}
	b, err := scheme.ParseNumber("9007199254740993")
	if err != nil {
		t.Fatal(err)
	}
	if auditNear(a, b) {
		t.Fatal("distinct exact integers compared after rounding")
	}
	if !auditNear(scheme.NewFloat(9.728), scheme.NewFloat(9.728000255822641)) {
		t.Fatal("reported example needs numeric tolerance")
	}
	if auditNear(scheme.NewFloat(1), scheme.NewFloat(math.Inf(1))) {
		t.Fatal("finite and infinite values treated alike")
	}
}
