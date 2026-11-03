package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/scheme"
)

var jsonDepthLimit = errors.New("JSON nesting depth limit reached")
var jsonCharacterLimit = errors.New("JSON character limit reached")

var jsonNeedCharacter = errors.New("JSON parser needs a character")
var jsonNumberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

const (
	jsonArrayFirst byte = iota
	jsonArrayValue
	jsonArraySeparator
	jsonObjectFirst
	jsonObjectKey
	jsonObjectColon
	jsonObjectValue
	jsonObjectSeparator
)

// jsonParser emits one event at a time. It retains only the open containers and
// the current scalar, and never builds a tree or reads the rest of the stream.
// Input is supplied by the VM adapter, so Scheme character generators can yield
// through ordinary VM actions without goroutines or recursive VM execution.
type jsonParser struct {
	symbols                    *scheme.SymbolTable
	stack                      []byte
	started                    bool
	mode                       byte
	token                      []rune
	literal                    string
	escaped                    bool
	ch                         rune
	pending, eof               bool
	count                      int64
	depthLimit, characterLimit scheme.Real
}

func jsonReached(n int64, limit scheme.Real) bool {
	cmp, ordered := number.Compare(scheme.NewInteger(n), limit)
	return ordered && cmp >= 0
}
func (p *jsonParser) feed(ch rune, eof bool) error {
	p.ch, p.pending, p.eof = ch, !eof, eof
	if !eof {
		p.count++
		if jsonReached(p.count, p.characterLimit) {
			return jsonCharacterLimit
		}
	}
	return nil
}
func jsonSpace(r rune) bool                                   { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }
func (p *jsonParser) take() rune                              { p.pending = false; return p.ch }
func (p *jsonParser) event(name string) (scheme.Value, error) { return p.symbols.Intern(name), nil }
func (p *jsonParser) next() (scheme.Value, error) {
	for {
		if p.mode == 0 && p.started && len(p.stack) == 0 {
			return scheme.EOF, nil
		}
		if !p.pending && !p.eof {
			return nil, jsonNeedCharacter
		}
		if p.mode == '"' {
			if p.eof {
				return nil, errors.New("unterminated JSON string")
			}
			ch := p.take()
			if ch < 0x20 {
				return nil, errors.New("unescaped control character in JSON string")
			}
			p.token = append(p.token, ch)
			if ch == '"' && !p.escaped {
				text, err := jsonDecodeString(string(p.token))
				p.mode = 0
				p.token = nil
				if err != nil {
					return nil, err
				}
				return scheme.NewString(text)
			}
			if ch == '\\' && !p.escaped {
				p.escaped = true
			} else {
				p.escaped = false
			}
			continue
		}
		if p.mode == 'n' {
			if !p.eof && strings.ContainsRune("0123456789.eE+-", p.ch) {
				p.token = append(p.token, p.take())
				continue
			}
			text := string(p.token)
			p.mode = 0
			p.token = nil
			if !jsonNumberPattern.MatchString(text) {
				return nil, fmt.Errorf("invalid JSON number: %s", text)
			}
			if len(p.stack) == 0 && !p.eof {
				if !jsonSpace(p.ch) {
					return nil, errors.New("invalid character after JSON number")
				}
				p.take() // Character generators have no unread operation; consume the separator.
			}
			n, err := scheme.ParseNumber(text)
			if err != nil {
				return nil, fmt.Errorf("unrepresentable JSON number: %s", text)
			}
			// A JSON number must remain finite after conversion to binary64.
			if f, ok := n.(scheme.Float); ok && math.IsInf(f.Float64(), 0) {
				return nil, errors.New("JSON number overflows binary64")
			}
			return n, nil
		}
		if p.mode == 'l' {
			if p.eof || p.ch != rune(p.literal[len(p.token)]) {
				return nil, errors.New("invalid JSON literal")
			}
			p.token = append(p.token, p.take())
			if len(p.token) == len(p.literal) {
				text := p.literal
				p.mode = 0
				p.token = nil
				switch text {
				case "true":
					return scheme.True, nil
				case "false":
					return scheme.False, nil
				default:
					return p.event("null")
				}
			}
			continue
		}
		if !p.eof && jsonSpace(p.ch) {
			p.take()
			continue
		}
		if p.eof {
			if !p.started {
				return scheme.EOF, nil
			}
			return nil, errors.New("unexpected end of JSON input")
		}
		state := byte(255)
		if len(p.stack) > 0 {
			state = p.stack[len(p.stack)-1]
		}
		switch state {
		case jsonArraySeparator:
			switch p.take() {
			case ',':
				p.stack[len(p.stack)-1] = jsonArrayValue
				continue
			case ']':
				p.stack = p.stack[:len(p.stack)-1]
				return p.event("array-end")
			}
			return nil, errors.New("expected comma or array end")
		case jsonObjectSeparator:
			switch p.take() {
			case ',':
				p.stack[len(p.stack)-1] = jsonObjectKey
				continue
			case '}':
				p.stack = p.stack[:len(p.stack)-1]
				return p.event("object-end")
			}
			return nil, errors.New("expected comma or object end")
		case jsonObjectColon:
			if p.take() != ':' {
				return nil, errors.New("expected colon after JSON object key")
			}
			p.stack[len(p.stack)-1] = jsonObjectValue
			continue
		case jsonObjectFirst, jsonObjectKey:
			if state == jsonObjectFirst && p.ch == '}' {
				p.take()
				p.stack = p.stack[:len(p.stack)-1]
				return p.event("object-end")
			}
			if p.ch != '"' {
				return nil, errors.New("JSON object key must be a string")
			}
			p.stack[len(p.stack)-1] = jsonObjectColon
			p.mode = '"'
			p.token = []rune{p.take()}
			continue
		case jsonArrayFirst:
			if p.ch == ']' {
				p.take()
				p.stack = p.stack[:len(p.stack)-1]
				return p.event("array-end")
			}
		}
		if len(p.stack) > 0 {
			if state == jsonObjectValue {
				p.stack[len(p.stack)-1] = jsonObjectSeparator
			} else {
				p.stack[len(p.stack)-1] = jsonArraySeparator
			}
		}
		p.started = true
		switch ch := p.ch; {
		case ch == '[' || ch == '{':
			if jsonReached(int64(len(p.stack)+1), p.depthLimit) {
				return nil, jsonDepthLimit
			}
			p.take()
			if ch == '[' {
				p.stack = append(p.stack, jsonArrayFirst)
				return p.event("array-start")
			}
			p.stack = append(p.stack, jsonObjectFirst)
			return p.event("object-start")
		case ch == '"':
			p.mode = '"'
			p.token = []rune{p.take()}
		case ch == '-' || ch >= '0' && ch <= '9':
			p.mode = 'n'
			p.token = nil
		case ch == 't' || ch == 'f' || ch == 'n':
			p.mode = 'l'
			p.token = nil
			p.literal = map[rune]string{'t': "true", 'f': "false", 'n': "null"}[ch]
		default:
			return nil, fmt.Errorf("unexpected character in JSON: %q", ch)
		}
	}
}

// encoding/json supplies JSON escaping, but intentionally replaces unpaired
// UTF-16 surrogates. Validate escape pairs first to preserve Unicode scalars.
func jsonDecodeString(raw string) (string, error) {
	hex := func(s string) (rune, bool) {
		var n rune
		for _, c := range s {
			n *= 16
			switch {
			case c >= '0' && c <= '9':
				n += c - '0'
			case c >= 'a' && c <= 'f':
				n += c - 'a' + 10
			case c >= 'A' && c <= 'F':
				n += c - 'A' + 10
			default:
				return 0, false
			}
		}
		return n, true
	}
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw)-1 {
			return "", errors.New("incomplete JSON escape")
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw)-1 {
			return "", errors.New("incomplete Unicode escape")
		}
		r, ok := hex(raw[i+1 : i+5])
		if !ok {
			return "", errors.New("invalid Unicode escape")
		}
		i += 4
		if utf16.IsSurrogate(r) {
			if r > 0xdbff || i+6 >= len(raw)-1 || raw[i+1:i+3] != "\\u" {
				return "", errors.New("unpaired Unicode surrogate")
			}
			low, ok := hex(raw[i+3 : i+7])
			if !ok || low < 0xdc00 || low > 0xdfff {
				return "", errors.New("invalid Unicode surrogate pair")
			}
			i += 6
		}
	}
	var text string
	if err := json.Unmarshal([]byte(raw), &text); err != nil {
		return "", err
	}
	return text, nil
}
