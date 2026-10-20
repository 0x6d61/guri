package guri

import (
	"bytes"
	"errors"
	"testing"
)

func TestStringAndBytesSequences(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{`(len "日本語")`, int64(3)}, {`(len "A😀é")`, int64(3)},
		{`(len "é")`, int64(2)}, {`(len "")`, int64(0)},
		{`(nth "Alice" 0)`, "A"}, {`(nth "日本語" 1)`, "本"},
		{`(nth "A😀é" 1)`, "😀"}, {`(slice "Alice" 0 3)`, "Ali"},
		{`(slice "日本語" 1 3)`, "本語"}, {`(slice "A😀é" 1 2)`, "😀"},
		{`(slice "日本語" 3 3)`, ""}, {`(slice "" 0 0)`, ""},
		{`(len b"日本語")`, int64(9)}, {`(len b"\x00\xffA")`, int64(3)},
		{`(nth b"Alice" 0)`, int64(65)}, {`(nth b"\xff" 0)`, int64(255)},
		{`(nth b"\x00" 0)`, int64(0)}, {`(len b"")`, int64(0)},
		{`(= (slice b"Alice" 0 3) b"Ali")`, true},
		{`(= (slice b"日本" 0 1) b"\xe6")`, true},
		{`(= (slice b"" 0 0) b"")`, true},
		{`(= b"A" "A")`, false}, {`(= b"A" b"\x41")`, true},
		{`(= b"A" b"B")`, false}, {`(= b"A" 65)`, false},
		{`(get {b"key" 1 "key" 2} b"key")`, int64(1)},
		{`(get {b"key" 1 "key" 2} "key")`, int64(2)},
		{`(len {b"key" 1 "key" 2})`, int64(2)},
		{`(if b"" 1 2)`, int64(1)},
		{`(= '(b"\xff") [b"\xff"])`, false},
		{`(= 'b"\xff" b"\xff")`, true},
		{`(= '{:x b"A"} {:x b"A"})`, true},
		{`(def a "日本語") (def b (slice a 1 2)) [a b]`, VectorValue{"日本語", "本"}},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSequenceErrors(t *testing.T) {
	for _, source := range []string{`(nth "" 0)`, `(nth "本" 1)`, `(nth "A" -1)`, `(nth b"" 0)`, `(nth b"A" 1)`, `(slice "本" 0 2)`, `(slice "本" -1 0)`, `(slice "本" 1 0)`, `(slice b"A" 0 2)`, `(slice b"A" 2 1)`} {
		expectError(t, source, IndexError)
	}
	for _, source := range []string{`(nth "A" 0.0)`, `(slice b"A" 0 1.0)`, `(slice "A" nil 1)`, `(nth {} 0)`, `(slice nil 0 0)`, `(assoc! "A" 0 "B")`, `(assoc! b"A" 0 66)`, `(cap b"A")`, `(+ b"1" 1)`} {
		expectError(t, source, TypeError)
	}
	for _, source := range []string{`(nth "A")`, `(slice b"A" 0)`, `(len b"A" 1)`} {
		expectError(t, source, ArityError)
	}
}

func TestReadBytesAndDisplayRoundTrip(t *testing.T) {
	for _, source := range []string{`b""`, `b"A\"\\\n\r\t"`, `b"\x00\xff\x7f"`, `b"日本語"`} {
		forms, err := Read("bytes.guri", source)
		if err != nil {
			t.Fatal(err)
		}
		if len(forms) != 1 || forms[0].Kind != Bytes || forms[0].Position != (Position{"bytes.guri", 1, 1}) {
			t.Fatalf("unexpected syntax: %#v", forms)
		}
		value := evaluate(t, source)
		if !Equal(value, evaluate(t, Format(value))) {
			t.Fatalf("round trip failed: %s", source)
		}
	}
	for _, source := range []string{`b"open`, `b"\q"`, `b"\x"`, `b"\x0"`, `b"\xgg"`, `"\xff"`} {
		_, err := Read("bad-bytes.guri", source)
		var e *ReaderError
		if !errors.As(err, &e) || e.Position.Filename != "bad-bytes.guri" || e.Position.Line != 1 || e.Cause == "" {
			t.Fatalf("%s: expected ReaderError, got %v", source, err)
		}
	}
	var out bytes.Buffer
	i := NewInterpreter()
	i.Output = &out
	if _, err := i.Eval("bytes.guri", `(println b"A\xff")`); err != nil {
		t.Fatal(err)
	}
	if out.String() != "b\"A\\xff\"\n" {
		t.Fatalf("unexpected bytes output: %q", out.String())
	}
}

func TestAllByteValuesRoundTrip(t *testing.T) {
	data := make([]byte, 256)
	for index := range data {
		data[index] = byte(index)
	}
	want := BytesValue(data)
	if got := evaluate(t, Format(want)); !Equal(got, want) {
		t.Fatal("byte display lost data")
	}
}

func TestBytesEscapeErrorPosition(t *testing.T) {
	_, err := Read("bytes.guri", "\n  b\"\\xg0\"")
	var e *ReaderError
	if !errors.As(err, &e) || e.Position != (Position{"bytes.guri", 2, 5}) {
		t.Fatalf("unexpected error position: %v", err)
	}
}
