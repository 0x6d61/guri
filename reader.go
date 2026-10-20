package guri

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ReaderError describes invalid source at a particular location.
type ReaderError struct {
	Position Position
	Cause    string
}

func (e *ReaderError) Error() string {
	return fmt.Sprintf("%s:%d:%d: ReaderError: %s", e.Position.Filename, e.Position.Line, e.Position.Column, e.Cause)
}

// Read parses every top-level expression without evaluating it.
func Read(filename, source string) ([]Expr, error) {
	r := reader{filename: filename, line: 1, column: 1}
	// Validate before converting to runes, including comments and strings.
	for offset, line, column := 0, 1, 1; offset < len(source); {
		ch, size := utf8.DecodeRuneInString(source[offset:])
		if ch == utf8.RuneError && size == 1 {
			return nil, &ReaderError{Position{filename, line, column}, "invalid UTF-8"}
		}
		offset += size
		if ch == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	r.input = []rune(source)
	var forms []Expr
	for {
		r.skipSpace()
		if r.done() {
			return forms, nil
		}
		expr, err := r.readExpr()
		if err != nil {
			return nil, err
		}
		forms = append(forms, expr)
	}
}

type reader struct {
	filename string
	input    []rune
	offset   int
	line     int
	column   int
}

func (r *reader) position() Position {
	return Position{r.filename, r.line, r.column}
}

func (r *reader) done() bool { return r.offset == len(r.input) }

func (r *reader) advance() rune {
	ch := r.input[r.offset]
	r.offset++
	if ch == '\n' {
		r.line++
		r.column = 1
	} else {
		r.column++
	}
	return ch
}

func (r *reader) skipSpace() {
	for !r.done() {
		ch := r.input[r.offset]
		if unicode.IsSpace(ch) {
			r.advance()
		} else if ch == ';' {
			for !r.done() && r.input[r.offset] != '\n' {
				r.advance()
			}
		} else {
			return
		}
	}
}

func (r *reader) readExpr() (Expr, error) {
	pos := r.position()
	switch ch := r.input[r.offset]; ch {
	case '(':
		return r.readCollection(List, ')')
	case '[':
		return r.readCollection(Vector, ']')
	case '{':
		return r.readCollection(Map, '}')
	case ')', ']', '}':
		return Expr{}, &ReaderError{pos, fmt.Sprintf("unexpected closing delimiter %q", ch)}
	case '\'':
		r.advance()
		r.skipSpace()
		if r.done() {
			return Expr{}, &ReaderError{pos, "quote requires an expression"}
		}
		quoted, err := r.readExpr()
		if err != nil {
			return Expr{}, err
		}
		return Expr{Kind: List, Position: pos, Elements: []Expr{
			{Kind: Symbol, Value: "quote", Position: pos}, quoted,
		}}, nil
	case '"':
		return r.readString(String)
	default:
		if ch == 'b' && r.offset+1 < len(r.input) && r.input[r.offset+1] == '"' {
			return r.readString(Bytes)
		}
		return r.readAtom()
	}
}

func (r *reader) readCollection(kind Kind, closing rune) (Expr, error) {
	pos := r.position()
	r.advance()
	expr := Expr{Kind: kind, Position: pos}
	for {
		r.skipSpace()
		if r.done() {
			return Expr{}, &ReaderError{pos, fmt.Sprintf("unclosed %s, expected %q", kind, closing)}
		}
		if r.input[r.offset] == closing {
			r.advance()
			if kind == Map && len(expr.Elements)%2 != 0 {
				return Expr{}, &ReaderError{pos, "Map requires key/value pairs"}
			}
			return expr, nil
		}
		child, err := r.readExpr()
		if err != nil {
			return Expr{}, err
		}
		expr.Elements = append(expr.Elements, child)
	}
}

func (r *reader) readString(kind Kind) (Expr, error) {
	pos := r.position()
	if kind == Bytes {
		r.advance()
	}
	r.advance()
	var value strings.Builder
	for !r.done() {
		charPos := r.position()
		switch ch := r.advance(); ch {
		case '"':
			return Expr{Kind: kind, Value: value.String(), Position: pos}, nil
		case '\\':
			if r.done() {
				return Expr{}, &ReaderError{pos, "unclosed string"}
			}
			escaped := r.advance()
			switch escaped {
			case '"', '\\':
				value.WriteRune(escaped)
			case 'n':
				value.WriteByte('\n')
			case 'r':
				value.WriteByte('\r')
			case 't':
				value.WriteByte('\t')
			case 'x':
				if kind != Bytes {
					return Expr{}, &ReaderError{charPos, "hex byte escape is only supported in Bytes"}
				}
				if r.offset+2 > len(r.input) {
					return Expr{}, &ReaderError{charPos, "expected two hexadecimal digits after \\x"}
				}
				n, err := strconv.ParseUint(string(r.input[r.offset:r.offset+2]), 16, 8)
				if err != nil {
					return Expr{}, &ReaderError{charPos, "expected two hexadecimal digits after \\x"}
				}
				r.advance()
				r.advance()
				value.WriteByte(byte(n))
			default:
				return Expr{}, &ReaderError{charPos, fmt.Sprintf("unsupported escape \\%c", escaped)}
			}
		default:
			value.WriteRune(ch)
		}
	}
	return Expr{}, &ReaderError{pos, "unclosed string"}
}

var integerToken = regexp.MustCompile(`^[+-]?[0-9]+$`)
var floatToken = regexp.MustCompile(`^[+-]?(?:[0-9]+\.[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$|^[+-]?[0-9]+[eE][+-]?[0-9]+$`)

func (r *reader) readAtom() (Expr, error) {
	pos, start := r.position(), r.offset
	for !r.done() {
		ch := r.input[r.offset]
		if unicode.IsSpace(ch) || strings.ContainsRune("()[]{}'\";", ch) {
			break
		}
		r.advance()
	}
	token := string(r.input[start:r.offset])
	expr := Expr{Kind: Symbol, Value: token, Position: pos}
	switch token {
	case "nil":
		expr.Kind, expr.Value = Nil, nil
	case "true", "false":
		expr.Kind, expr.Value = Boolean, token == "true"
	default:
		switch {
		case strings.HasPrefix(token, ":"):
			if len(token) == 1 {
				return Expr{}, &ReaderError{pos, "Keyword requires a name"}
			}
			expr.Kind, expr.Value = Keyword, token[1:]
		case integerToken.MatchString(token):
			value, err := strconv.ParseInt(token, 10, 64)
			if err != nil {
				return Expr{}, &ReaderError{pos, "Integer outside int64 range"}
			}
			expr.Kind, expr.Value = Integer, value
		case floatToken.MatchString(token):
			value, err := strconv.ParseFloat(token, 64)
			if err != nil {
				return Expr{}, &ReaderError{pos, "Float outside finite float64 range"}
			}
			expr.Kind, expr.Value = Float, value
		}
	}
	return expr, nil
}
