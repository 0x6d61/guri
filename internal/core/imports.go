package core

import (
	"strconv"
	"strings"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

type libraryExports map[string]*binding

type libraryPart struct {
	text    string
	integer bool
}

func parseLibraryName(n *reader.Node) (string, []libraryPart, error) {
	xs, err := nodes(n)
	if err != nil {
		return "", nil, err
	}
	if len(xs) == 0 {
		return "", nil, syntax(n, "empty library name")
	}
	var key strings.Builder
	var parts []libraryPart
	for _, x := range xs {
		part := libraryPart{}
		switch value := x.Datum().(type) {
		case *scheme.Symbol:
			part.text = value.Name()
			key.WriteByte('s')
		case *scheme.Integer:
			if value.BigInt().Sign() < 0 {
				return "", nil, syntax(x, "negative library name component")
			}
			part.integer = true
			part.text = value.BigInt().String()
			key.WriteByte('i')
		default:
			return "", nil, syntax(x, "library name needs identifiers or exact nonnegative integers")
		}
		key.WriteString(strconv.Itoa(len(part.text)))
		key.WriteByte(':')
		key.WriteString(part.text)
		parts = append(parts, part)
	}
	return key.String(), parts, nil
}
func bindingIdentity(b *binding) *binding {
	for b != nil && b.origin != nil {
		b = b.origin
	}
	return b
}
func addExport(out libraryExports, name string, b *binding, n *reader.Node) error {
	if old, found := out[name]; found && bindingIdentity(old) != bindingIdentity(b) {
		return syntax(n, "conflicting imported identifier: "+name)
	}
	out[name] = b
	return nil
}
func importIdentifier(n *reader.Node) (string, error) {
	s, ok := n.Datum().(*scheme.Symbol)
	if !ok {
		return "", syntax(n, "expected import/export identifier")
	}
	return s.Name(), nil
}
func resolveImportSet(n *reader.Node, resolve func(*reader.Node) (libraryExports, error)) (libraryExports, error) {
	xs, err := nodes(n)
	if err != nil {
		return nil, err
	}
	if len(xs) == 0 {
		return nil, syntax(n, "empty import set")
	}
	operator, ok := xs[0].Datum().(*scheme.Symbol)
	if !ok {
		return resolve(n)
	}
	name := operator.Name()
	if name != "only" && name != "except" && name != "prefix" && name != "rename" {
		return resolve(n)
	}
	if len(xs) < 2 {
		return nil, syntax(n, "import transformation needs an import set")
	}
	original, err := resolveImportSet(xs[1], resolve)
	if err != nil {
		return nil, err
	}
	result := libraryExports{}
	switch name {
	case "only", "except":
		selected := map[string]bool{}
		for _, node := range xs[2:] {
			identifier, err := importIdentifier(node)
			if err != nil {
				return nil, err
			}
			if _, found := original[identifier]; !found {
				return nil, syntax(node, "identifier is not exported: "+identifier)
			}
			selected[identifier] = true
		}
		for identifier, b := range original {
			if selected[identifier] == (name == "only") {
				result[identifier] = b
			}
		}
	case "prefix":
		if len(xs) != 3 {
			return nil, syntax(n, "prefix needs an import set and identifier")
		}
		prefix, err := importIdentifier(xs[2])
		if err != nil {
			return nil, err
		}
		for identifier, b := range original {
			result[prefix+identifier] = b
		}
	case "rename":
		replacements := map[string]string{}
		for _, node := range xs[2:] {
			pair, err := nodes(node)
			if err != nil {
				return nil, err
			}
			if len(pair) != 2 {
				return nil, syntax(node, "rename needs two identifiers")
			}
			from, err := importIdentifier(pair[0])
			if err != nil {
				return nil, err
			}
			to, err := importIdentifier(pair[1])
			if err != nil {
				return nil, err
			}
			if _, found := original[from]; !found {
				return nil, syntax(node, "identifier is not exported: "+from)
			}
			if _, found := replacements[from]; found {
				return nil, syntax(node, "duplicate rename source")
			}
			replacements[from] = to
		}
		// Renamings are simultaneous, so swapping two names is allowed.
		for identifier, b := range original {
			if to, found := replacements[identifier]; found {
				identifier = to
			}
			if err := addExport(result, identifier, b, n); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
