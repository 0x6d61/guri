package reader

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/0x6d61/guri/internal/scheme"
)

func (r *Reader) quoted(quote rune) (string, error) {
	start := r.position
	r.advance()
	var out strings.Builder
	for !r.done() {
		pos := r.position
		c := r.advance()
		if c == quote {
			return out.String(), nil
		}
		if c == '\\' {
			if r.done() {
				return "", &Error{Position: start, Message: "unclosed quoted token"}
			}
			escaped := r.advance()
			switch escaped {
			case 'a':
				out.WriteByte('\a')
			case 'b':
				out.WriteByte('\b')
			case 't':
				out.WriteByte('\t')
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case '"', '\\', '|':
				out.WriteRune(escaped)
			case 'x', 'X':
				first := r.offset
				for !r.done() && r.peek() != ';' {
					r.advance()
				}
				if r.done() {
					return "", &Error{Position: pos, Message: "hex escape needs terminating semicolon"}
				}
				code, err := hexScalar(string(r.input[first:r.offset]))
				if err != nil {
					return "", &Error{Position: pos, Message: err.Error()}
				}
				r.advance()
				out.WriteRune(code)
			default:
				if quote != '"' || !intraline(escaped) && escaped != '\r' && escaped != '\n' {
					return "", &Error{Position: pos, Message: "unknown escape"}
				}
				for intraline(escaped) && !r.done() {
					escaped = r.advance()
				}
				if escaped != '\r' && escaped != '\n' {
					return "", &Error{Position: pos, Message: "line continuation needs a line ending"}
				}
				if escaped == '\r' && !r.done() && r.peek() == '\n' {
					r.advance()
				}
				for !r.done() && intraline(r.peek()) {
					r.advance()
				}
			}
		} else if c == '\r' && quote == '"' {
			if !r.done() && r.peek() == '\n' {
				r.advance()
			}
			out.WriteByte('\n')
		} else {
			out.WriteRune(c)
		}
	}
	return "", &Error{Position: start, Message: "unclosed quoted token"}
}
func intraline(c rune) bool { return c == ' ' || c == '\t' }
func hexScalar(text string) (rune, error) {
	code, err := strconv.ParseUint(text, 16, 32)
	if err != nil || !utf8.ValidRune(rune(code)) {
		return 0, scheme.ErrRange
	}
	return rune(code), nil
}

func (r *Reader) character() (*Node, error) {
	pos := r.position
	r.advance()
	r.advance()
	if r.done() {
		return r.fail(pos, "missing character")
	}
	var text string
	if delimiter(r.peek()) {
		text = string(r.advance())
		if !r.done() && !delimiter(r.peek()) {
			return r.fail(pos, "character needs a delimiter")
		}
	} else {
		text = r.token()
	}
	chars := []rune(text)
	var code rune
	switch {
	case len(chars) == 1:
		code = chars[0]
	case len(text) > 1 && (text[0] == 'x' || text[0] == 'X'):
		var err error
		code, err = hexScalar(text[1:])
		if err != nil {
			return r.fail(pos, "invalid character scalar")
		}
	default:
		if r.foldCase {
			text = r.folder.String(text)
		}
		names := map[string]rune{"alarm": 7, "backspace": 8, "delete": 127, "escape": 27, "newline": 10, "null": 0, "return": 13, "space": 32, "tab": 9}
		var ok bool
		code, ok = names[text]
		if !ok {
			return r.fail(pos, "unknown character name "+text)
		}
	}
	value, err := scheme.NewCharacter(code)
	if err != nil {
		return r.fail(pos, err.Error())
	}
	return r.atom(value, pos), nil
}
