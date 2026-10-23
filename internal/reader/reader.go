// Package reader reads R7RS data into the shared Scheme value model.
package reader

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/0x6d61/guri/internal/scheme"
	"golang.org/x/text/cases"
)

// Node retains source information at every pair link and vector element.
// Car/Cdr mirror Pair data; Elements describes Vector/Bytevector syntax.
type Node struct {
	Syntax   scheme.Syntax
	Car, Cdr *Node
	Elements []*Node
}

func (n *Node) Datum() scheme.Value   { return n.Syntax.Datum() }
func (n *Node) Source() scheme.Source { return n.Syntax.Source() }

var ErrDatumLabels = errors.New("datum labels require the graph reader (#58)")

type Error struct {
	Position scheme.Source
	Message  string
	Cause    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: ReaderError: %s", e.Position.Filename, e.Position.Line, e.Position.Column, e.Message)
}
func (e *Error) Unwrap() error { return e.Cause }

type Reader struct {
	input    []rune
	offset   int
	position scheme.Source
	afterCR  bool
	symbols  *scheme.SymbolTable
	foldCase bool
	folder   cases.Caser
}

func New(filename, source string, symbols *scheme.SymbolTable) (*Reader, error) {
	r := &Reader{position: scheme.Source{Filename: filename, Line: 1, Column: 1}, symbols: symbols, folder: cases.Fold()}
	if r.symbols == nil {
		r.symbols = scheme.NewSymbolTable()
	}
	// Validate all bytes, including ignored comments, before decoding the input.
	if byteOffset := invalidUTF8Offset(source); byteOffset >= 0 {
		prefix := source[:byteOffset]
		probe := &Reader{input: []rune(prefix), position: r.position}
		for !probe.done() {
			probe.advance()
		}
		return nil, &Error{Position: probe.position, Message: "invalid UTF-8"}
	}
	r.input = []rune(source)
	return r, nil
}
func invalidUTF8Offset(source string) int {
	for i := 0; i < len(source); {
		c, size := utf8.DecodeRuneInString(source[i:])
		if c == utf8.RuneError && size == 1 {
			return i
		}
		i += size
	}
	return -1
}

func ReadAll(filename, source string, symbols *scheme.SymbolTable) ([]*Node, error) {
	r, err := New(filename, source, symbols)
	if err != nil {
		return nil, err
	}
	nodes := []*Node{}
	for {
		n, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nodes, nil
		}
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
}
func (r *Reader) Read() (*Node, error) {
	if err := r.space(); err != nil {
		return nil, err
	}
	if r.done() {
		return nil, io.EOF
	}
	return r.datum()
}
func (r *Reader) done() bool { return r.offset == len(r.input) }
func (r *Reader) peek() rune {
	if r.done() {
		return 0
	}
	return r.input[r.offset]
}
func (r *Reader) has(prefix string) bool {
	offset := r.offset
	for _, c := range prefix {
		if offset >= len(r.input) || r.input[offset] != c {
			return false
		}
		offset++
	}
	return true
}
func (r *Reader) advance() rune {
	c := r.input[r.offset]
	r.offset++
	switch c {
	case '\r':
		r.position.Line++
		r.position.Column = 1
		r.afterCR = true
	case '\n':
		if !r.afterCR {
			r.position.Line++
		}
		r.position.Column = 1
		r.afterCR = false
	default:
		r.position.Column++
		r.afterCR = false
	}
	return c
}
func (r *Reader) fail(pos scheme.Source, message string) (*Node, error) {
	return nil, &Error{Position: pos, Message: message}
}
func (r *Reader) atom(value scheme.Value, pos scheme.Source) *Node {
	return &Node{Syntax: scheme.NewSyntax(value, pos)}
}
func (r *Reader) pair(car, cdr *Node, pos scheme.Source) *Node {
	return &Node{Syntax: scheme.NewSyntax(scheme.Cons(car.Datum(), cdr.Datum()), pos), Car: car, Cdr: cdr}
}
func delimiter(c rune) bool { return unicode.IsSpace(c) || strings.ContainsRune("|()\";", c) }
func (r *Reader) token() string {
	start := r.offset
	for !r.done() && !delimiter(r.peek()) {
		r.advance()
	}
	return string(r.input[start:r.offset])
}

func (r *Reader) space() error {
	for !r.done() {
		switch {
		case unicode.IsSpace(r.peek()):
			r.advance()
		case r.peek() == ';':
			for !r.done() && r.peek() != '\r' && r.peek() != '\n' {
				r.advance()
			}
		case r.has("#|"):
			start := r.position
			r.advance()
			r.advance()
			depth := 1
			for depth > 0 {
				if r.done() {
					return &Error{Position: start, Message: "unclosed block comment"}
				}
				if r.has("#|") {
					r.advance()
					r.advance()
					depth++
				} else if r.has("|#") {
					r.advance()
					r.advance()
					depth--
				} else {
					r.advance()
				}
			}
		case r.has("#;"):
			r.advance()
			r.advance()
			if _, err := r.datum(); err != nil {
				return err
			}
		case r.has("#!"):
			pos := r.position
			directive := strings.ToLower(r.token())
			switch directive {
			case "#!fold-case":
				r.foldCase = true
			case "#!no-fold-case":
				r.foldCase = false
			default:
				return &Error{Position: pos, Message: "unknown reader directive " + directive}
			}
		default:
			return nil
		}
	}
	return nil
}

func (r *Reader) datum() (*Node, error) {
	if err := r.space(); err != nil {
		return nil, err
	}
	pos := r.position
	if r.done() {
		return r.fail(pos, "expected datum")
	}
	switch r.peek() {
	case '(':
		r.advance()
		return r.sequence(pos, "list")
	case ')', '[', ']', '{', '}':
		return r.fail(pos, "unexpected or reserved delimiter")
	case '\'', '`', ',':
		c := r.advance()
		name := map[rune]string{'\'': "quote", '`': "quasiquote", ',': "unquote"}[c]
		if c == ',' && !r.done() && r.peek() == '@' {
			r.advance()
			name = "unquote-splicing"
		}
		data, err := r.datum()
		if err != nil {
			return nil, err
		}
		keyword := r.atom(r.symbols.Intern(name), pos)
		return r.pair(keyword, r.pair(data, r.atom(scheme.Null, data.Source()), pos), pos), nil
	case '"', '|':
		quote := r.peek()
		text, err := r.quoted(quote)
		if err != nil {
			return nil, err
		}
		if quote == '|' {
			if r.foldCase {
				text = r.folder.String(text)
			}
			return r.atom(r.symbols.Intern(text), pos), nil
		}
		value, err := scheme.NewString(text)
		if err != nil {
			return r.fail(pos, err.Error())
		}
		return r.atom(value, pos), nil
	case '#':
		switch {
		case r.has("#("):
			r.advance()
			r.advance()
			return r.sequence(pos, "vector")
		case r.has("#u8(") || r.has("#U8("):
			for range 4 {
				r.advance()
			}
			return r.sequence(pos, "bytevector")
		case r.has("#\\"):
			return r.character()
		}
	}
	text := r.token()
	if text == "" {
		return r.fail(pos, "expected token")
	}
	switch strings.ToLower(text) {
	case "#t", "#true":
		return r.atom(scheme.True, pos), nil
	case "#f", "#false":
		return r.atom(scheme.False, pos), nil
	}
	if strings.HasPrefix(text, "#") && len(text) > 1 && text[1] >= '0' && text[1] <= '9' {
		return nil, &Error{Position: pos, Message: ErrDatumLabels.Error(), Cause: ErrDatumLabels}
	}
	number, numberErr := scheme.ParseNumber(text)
	if numberErr == nil {
		return r.atom(number, pos), nil
	}
	if !identifier(text) {
		return nil, &Error{Position: pos, Message: "invalid token: " + numberErr.Error(), Cause: numberErr}
	}
	if r.foldCase {
		text = r.folder.String(text)
	}
	return r.atom(r.symbols.Intern(text), pos), nil
}

func (r *Reader) sequence(pos scheme.Source, kind string) (*Node, error) {
	var elements []*Node
	var tail *Node
	for {
		if err := r.space(); err != nil {
			return nil, err
		}
		if r.done() {
			return r.fail(pos, "unclosed "+kind)
		}
		if r.peek() == ')' {
			end := r.position
			r.advance()
			if tail == nil {
				tail = r.atom(scheme.Null, end)
			}
			break
		}
		if r.peek() == '.' && (r.offset+1 == len(r.input) || delimiter(r.input[r.offset+1])) {
			if kind != "list" || len(elements) == 0 || tail != nil {
				return r.fail(r.position, "invalid dotted tail")
			}
			r.advance()
			var err error
			tail, err = r.datum()
			if err != nil {
				return nil, err
			}
			if err := r.space(); err != nil {
				return nil, err
			}
			if r.done() || r.peek() != ')' {
				return r.fail(r.position, "expected closing parenthesis after dotted tail")
			}
			r.advance()
			break
		}
		element, err := r.datum()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	switch kind {
	case "list":
		for i := len(elements) - 1; i >= 0; i-- {
			source := elements[i].Source()
			if i == 0 {
				source = pos
			}
			tail = r.pair(elements[i], tail, source)
		}
		if len(elements) == 0 {
			tail = r.atom(scheme.Null, pos)
		}
		return tail, nil
	case "vector":
		values := make([]scheme.Value, len(elements))
		for i, e := range elements {
			values[i] = e.Datum()
		}
		node := r.atom(scheme.NewVector(values...), pos)
		node.Elements = elements
		return node, nil
	case "bytevector":
		bytes := make([]byte, len(elements))
		for i, e := range elements {
			integer, ok := e.Datum().(*scheme.Integer)
			if !ok {
				return r.fail(e.Source(), "bytevector needs exact u8 integers")
			}
			n := integer.BigInt()
			if !n.IsInt64() || n.Sign() < 0 || n.Int64() > 255 {
				return r.fail(e.Source(), "bytevector value outside 0..255")
			}
			bytes[i] = byte(n.Int64())
		}
		node := r.atom(scheme.NewBytevector(bytes), pos)
		node.Elements = elements
		return node, nil
	}
	panic("reader: unknown sequence")
}

func identifier(text string) bool {
	chars := []rune(text)
	if len(chars) == 0 {
		return false
	}
	first := chars[0]
	start := 1
	switch {
	case initial(first):
	case first == '+' || first == '-':
		if len(chars) == 1 {
			return true
		}
		if chars[1] == '.' {
			if len(chars) < 3 || !dotSubsequent(chars[2]) {
				return false
			}
			start = 3
		} else if signSubsequent(chars[1]) {
			start = 2
		} else {
			return false
		}
	case first == '.':
		if len(chars) < 2 || !dotSubsequent(chars[1]) {
			return false
		}
		start = 2
	default:
		return false
	}
	for _, c := range chars[start:] {
		if !subsequent(c) {
			return false
		}
	}
	return true
}
func initial(c rune) bool {
	if c < 128 {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || strings.ContainsRune("!$%&*/:<=>?@^_~", c)
	}
	return c == 0x200c || c == 0x200d || unicode.In(c, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo, unicode.Mn, unicode.Nl, unicode.No, unicode.Pd, unicode.Pc, unicode.Po, unicode.Sc, unicode.Sm, unicode.Sk, unicode.So, unicode.Co)
}
func subsequent(c rune) bool {
	return initial(c) || c >= '0' && c <= '9' || strings.ContainsRune("+-.@", c) || c >= 128 && unicode.In(c, unicode.Nd, unicode.Mc, unicode.Me)
}
func signSubsequent(c rune) bool { return initial(c) || c == '+' || c == '-' || c == '@' }
func dotSubsequent(c rune) bool  { return signSubsequent(c) || c == '.' }
