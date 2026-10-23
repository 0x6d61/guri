// Package textcase shares Unicode case folding between the reader and Scheme
// text procedures. Its Unicode version is pinned and tested alongside x/text.
package textcase

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
)

// Fold applies default full Unicode folding. Unicode 15 folds Cherokee to
// uppercase. x/text v0.43.0's Go 1.26 table incorrectly lowers the uppercase
// members, so normalize that script after applying its other full mappings.
func Fold(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0xAB70 && r <= 0xABBF || r >= 0x13F8 && r <= 0x13FD {
			return unicode.ToUpper(r)
		}
		return r
	}, cases.Fold().String(text))
}

// Simple applies the C/S mappings rather than full multi-character mappings.
// Full one-character mappings also apply here. When full folding expands a
// character, a nontrivial simple-fold orbit identifies the separate simple pair.
func Simple(r rune) rune {
	folded := Fold(string(r))
	if utf8.RuneCountInString(folded) == 1 {
		value, _ := utf8.DecodeRuneInString(folded)
		return value
	}
	if unicode.SimpleFold(r) != r {
		return unicode.ToLower(r)
	}
	return r
}
