// Package writer encodes Scheme datum graphs independently of ports and CLI.
package writer

import (
	"context"
	"io"
	"strconv"
	"strings"
	"unicode"

	numeric "github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/scheme"
)

type Mode uint8

const (
	Normal Mode = iota
	Shared
	Simple
)

type task struct {
	value scheme.Value
	text  string
	tail  bool
}

func Write(ctx context.Context, out io.Writer, value scheme.Value, mode Mode) error {
	if mode > Simple {
		return scheme.ErrRange
	}
	graph, err := analyze(ctx, value)
	if err != nil {
		return err
	}
	labels := map[scheme.Value]int{}
	next := 0
	labelled := func(v scheme.Value) bool {
		info := graph[v]
		return info != nil && (mode == Shared && info.refs > 1 || mode == Normal && info.cyclic)
	}
	emit := func(text string) error {
		n, err := io.WriteString(out, text)
		if err == nil && n != len(text) {
			return io.ErrShortWrite
		}
		return err
	}
	work := []task{{value: value}}
	for len(work) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		item := work[len(work)-1]
		work = work[:len(work)-1]
		if item.value == nil {
			if err := emit(item.text); err != nil {
				return err
			}
			continue
		}
		v := item.value
		if item.tail {
			p := v.(*scheme.Pair)
			cdr := p.Cdr()
			if cdr != scheme.Null {
				if _, ok := cdr.(*scheme.Pair); ok && !labelled(cdr) {
					work = append(work, task{value: cdr, tail: true}, task{text: " "})
				} else {
					work = append(work, task{value: cdr}, task{text: " . "})
				}
			}
			work = append(work, task{value: p.Car()})
			continue
		}
		if labelled(v) {
			if label, ok := labels[v]; ok {
				if err := emit("#" + strconv.Itoa(label) + "#"); err != nil {
					return err
				}
				continue
			}
			labels[v] = next
			if err := emit("#" + strconv.Itoa(next) + "="); err != nil {
				return err
			}
			next++
		}
		switch x := v.(type) {
		case *scheme.Pair:
			if err := emit("("); err != nil {
				return err
			}
			work = append(work, task{text: ")"}, task{value: x, tail: true})
		case *scheme.Vector:
			if err := emit("#("); err != nil {
				return err
			}
			work = append(work, task{text: ")"})
			values := x.Values()
			for i := len(values) - 1; i >= 0; i-- {
				work = append(work, task{value: values[i]})
				if i > 0 {
					work = append(work, task{text: " "})
				}
			}
		default:
			text, err := atomic(v)
			if err != nil {
				return err
			}
			if err := emit(text); err != nil {
				return err
			}
		}
	}
	return nil
}
func escaped(text string, quote byte) string {
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range text {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\a':
			b.WriteString(`\a`)
		case '\b':
			b.WriteString(`\b`)
		default:
			if r == rune(quote) {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if unicode.IsControl(r) {
				b.WriteString(`\x`)
				b.WriteString(strconv.FormatInt(int64(r), 16))
				b.WriteByte(';')
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}
func bareSymbol(s string) bool {
	if s == "+" || s == "-" || s == "..." {
		return true
	}
	if s == "" {
		return false
	}
	for i, r := range s {
		initial := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || strings.ContainsRune("!$%&*/:<=>?@^_~", r)
		if !initial && (i == 0 || !(r >= '0' && r <= '9' || strings.ContainsRune("+.-", r))) {
			return false
		}
	}
	return true
}
func atomic(v scheme.Value) (string, error) {
	switch x := v.(type) {
	case scheme.Boolean:
		if x {
			return "#t", nil
		}
		return "#f", nil
	case scheme.Number:
		return numeric.Format(x, 10)
	case *scheme.String:
		return escaped(x.Text(), '"'), nil
	case *scheme.Symbol:
		if bareSymbol(x.Name()) {
			return x.Name(), nil
		}
		return escaped(x.Name(), '|'), nil
	case scheme.Character:
		names := map[rune]string{0: "null", 7: "alarm", 8: "backspace", 9: "tab", 10: "newline", 13: "return", 27: "escape", 32: "space", 127: "delete"}
		if name, ok := names[x.Rune()]; ok {
			return `#\` + name, nil
		}
		return `#\x` + strconv.FormatInt(int64(x.Rune()), 16), nil
	case *scheme.Bytevector:
		var b strings.Builder
		b.WriteString("#u8(")
		for i, value := range x.Bytes() {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strconv.Itoa(int(value)))
		}
		b.WriteByte(')')
		return b.String(), nil
	default:
		if v == scheme.Null {
			return "()", nil
		}
		return "#<" + string(v.Kind()) + ">", nil
	}
}
