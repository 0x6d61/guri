package guri

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Format returns a readable representation. Map order is unspecified and cycles
// are represented by #<cycle> rather than expanded indefinitely.
func Format(value Value) string { return format(value, make(map[equalityPair]bool)) }

func format(value Value, path map[equalityPair]bool) string {
	var values []Value
	var open, close string
	var identity equalityPair
	switch v := value.(type) {
	case nil:
		return "nil"
	case bool:
		return strconv.FormatBool(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		s := strconv.FormatFloat(v, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s
	case string:
		escape := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n", "\r", "\\r", "\t", "\\t")
		return "\"" + escape.Replace(v) + "\""
	case SymbolValue:
		return string(v)
	case KeywordValue:
		return ":" + string(v)
	case *FunctionValue:
		return "#<function>"
	case ListValue:
		values = v.Values()
		open, close = "(", ")"
		identity = equalityPair{left: v.head}
	case VectorValue:
		values = v
		open, close = "[", "]"
		if len(v) > 0 {
			identity = equalityPair{left: &v[0], length: len(v)}
		}
	case *MapValue:
		identity = equalityPair{left: v}
		open, close = "{", "}"
		for key, element := range v.entries {
			values = append(values, key, element)
		}
	default:
		return "#<unknown>"
	}
	if path[identity] {
		return "#<cycle>"
	}
	path[identity] = true
	defer delete(path, identity)
	parts := make([]string, len(values))
	for index, element := range values {
		parts[index] = format(element, path)
	}
	return open + strings.Join(parts, " ") + close
}

func (i *Interpreter) installPrinting() {
	i.native("println", 0, -1, func(args []Value) (Value, error) {
		parts := make([]string, len(args))
		for index, value := range args {
			if text, ok := value.(string); ok {
				parts[index] = text
			} else {
				parts[index] = Format(value)
			}
		}
		out := i.Output
		if out == nil {
			out = io.Discard
		}
		_, err := fmt.Fprintln(out, strings.Join(parts, " "))
		return nil, err
	})
}
