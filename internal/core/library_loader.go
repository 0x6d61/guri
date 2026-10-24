package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

// The installer can set these absolute directories with Go linker -X flags.
// Empty defaults keep development binaries independent of an installation tree;
// all currently implemented standard libraries are compiled into the binary.
var standardLibraryDirectory string
var siteLibraryDirectory string

func configuredLibraryPaths(explicit []string) []string {
	paths := append([]string(nil), explicit...)
	paths = append(paths, filepath.SplitList(os.Getenv("GURI_LOAD_PATH"))...)
	paths = append(paths, siteLibraryDirectory, standardLibraryDirectory)
	result := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if !seen[path] {
			result = append(result, path)
			seen[path] = true
		}
	}
	return result
}

func encodedLibraryPart(p libraryPart) string {
	if p.integer {
		return p.text
	}
	if p.text == "" {
		return "%"
	}
	digits := true
	for _, c := range p.text {
		if c < '0' || c > '9' {
			digits = false
		}
	}
	upper := strings.ToUpper(p.text)
	device := upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" || len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '1' && upper[3] <= '9'
	var out strings.Builder
	for i, b := range []byte(p.text) {
		safe := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-'
		if safe && !(i == 0 && (digits || device)) {
			out.WriteByte(b)
		} else {
			fmt.Fprintf(&out, "%%%02X", b)
		}
	}
	return out.String()
}
func (r *Runtime) locate(ctx context.Context, n *reader.Node) (*library, error) {
	key, parts, err := parseLibraryName(n)
	if err != nil {
		return nil, err
	}
	if l, found := r.modules[key]; found {
		return l, nil
	}
	if r.standard[key] {
		return nil, syntax(n, "standard library is not implemented yet")
	}
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = encodedLibraryPart(p)
	}
	relative := filepath.Join(names...) + ".sld"
	for _, root := range r.paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := filepath.Join(root, relative)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, &Error{Source: n.Source(), Cause: err}
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if err := r.register(absolute, string(data)); err != nil {
			return nil, err
		}
		if l := r.modules[key]; l != nil {
			return l, nil
		}
		return nil, syntax(n, "library file does not define the requested name: "+absolute)
	}
	return nil, syntax(n, "library was not found: "+relative)
}
func (r *Runtime) available(name []scheme.Value) bool {
	var nodes []*reader.Node
	pos := scheme.Source{Filename: "<library-requirement>", Line: 1, Column: 1}
	for _, v := range name {
		nodes = append(nodes, atom(v, pos))
	}
	n := list(nodes, atom(scheme.Null, pos), pos)
	ctx := r.context
	if ctx == nil {
		ctx = context.Background()
	}
	l, err := r.locate(ctx, n)
	return err == nil && l.err == nil
}
func includePath(including, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(including), path)
	}
	return filepath.Abs(path)
}
func fileSourceLoader(ctx context.Context, including, path string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	absolute, err := includePath(including, path)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return "", "", err
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	return absolute, string(data), nil
}
