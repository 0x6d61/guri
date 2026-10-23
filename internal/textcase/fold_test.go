package textcase_test

import (
	"testing"
	"unicode"

	"github.com/0x6d61/guri/internal/textcase"
	"golang.org/x/text/cases"
)

func TestUnicode15MappingsAndIdempotence(t *testing.T) {
	if unicode.Version != "15.0.0" || cases.UnicodeVersion != unicode.Version {
		t.Fatal("revalidate pinned Unicode data", unicode.Version, cases.UnicodeVersion)
	}
	for _, tc := range []struct{ input, full, simple string }{
		{"A", "a", "a"}, {"ß", "ss", "ß"}, {"ẞ", "ss", "ß"}, {"İ", "i\u0307", "İ"},
		{"\u0345", "ι", "ι"}, {"ς", "σ", "σ"}, {"ᾈ", "ἀι", "ᾀ"}, {"ᾼ", "αι", "ᾳ"},
		{"Ꭰ", "Ꭰ", "Ꭰ"}, {"ꭰ", "Ꭰ", "Ꭰ"}, {"Ᏽ", "Ᏽ", "Ᏽ"}, {"ᏽ", "Ᏽ", "Ᏽ"},
		{"ﬀ", "ff", "ﬀ"}, {"😀", "😀", "😀"},
	} {
		r := []rune(tc.input)[0]
		if got := textcase.Simple(r); string(got) != tc.simple {
			t.Fatal(tc.input, got, tc.simple)
		}
		if got := textcase.Fold(tc.input); got != tc.full {
			t.Fatal(tc.input, got, tc.full)
		}
		if got := textcase.Fold(tc.full); got != tc.full {
			t.Fatal("not idempotent", tc.input, got)
		}
	}
}
