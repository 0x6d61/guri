package core

import (
	"context"
	"io"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func (r *Runtime) included(ctx context.Context, n *reader.Node, files []*reader.Node, fold bool) ([]*reader.Node, error) {
	if len(files) == 0 {
		return nil, syntax(n, "include needs at least one filename")
	}
	var result []*reader.Node
	for _, file := range files {
		path, ok := file.Datum().(*scheme.String)
		if !ok {
			return nil, syntax(file, "include filename must be a string")
		}
		filename, source, err := fileSourceLoader(ctx, n.Source().Filename, path.Text())
		if err != nil {
			return nil, &Error{Source: file.Source(), Cause: err}
		}
		parser, err := reader.New(filename, source, r.bootstrap.symbols)
		if err != nil {
			return nil, err
		}
		parser.SetFoldCase(fold)
		for {
			node, err := parser.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			result = append(result, node)
		}
	}
	return result, nil
}
func (r *Runtime) declarations(ctx context.Context, e *Engine, input []*reader.Node, stack map[string]bool) ([]*reader.Node, error) {
	var result []*reader.Node
	for _, n := range input {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		xs, err := nodes(n)
		if err != nil {
			return nil, err
		}
		if len(xs) == 0 || xs[0].Datum().Kind() != scheme.KindSymbol {
			return nil, syntax(n, "invalid library declaration")
		}
		switch xs[0].Datum().(*scheme.Symbol).Name() {
		case "import", "export", "begin", "include", "include-ci":
			result = append(result, n)
		case "include-library-declarations":
			if len(xs) < 2 {
				return nil, syntax(n, "include-library-declarations needs filenames")
			}
			for _, file := range xs[1:] {
				included, err := r.included(ctx, n, []*reader.Node{file}, false)
				if err != nil {
					return nil, err
				}
				// Include paths are absolute in every resulting node. Track even empty
				// includes by their resolved filename to diagnose recursive declarations.
				path := file.Datum().(*scheme.String).Text()
				filename, err := includePath(n.Source().Filename, path)
				if err != nil {
					return nil, err
				}
				if stack[filename] {
					return nil, syntax(file, "cyclic include-library-declarations")
				}
				stack[filename] = true
				nested, err := r.declarations(ctx, e, included, stack)
				delete(stack, filename)
				if err != nil {
					return nil, err
				}
				result = append(result, nested...)
			}
		case "cond-expand":
			b := &builder{engine: e, env: &scope{}, ctx: ctx}
			for i, clause := range xs[1:] {
				parts, err := nodes(clause)
				if err != nil {
					return nil, err
				}
				if len(parts) == 0 {
					return nil, syntax(clause, "empty library cond-expand clause")
				}
				yes := symbolNamed(parts[0], "else")
				if yes && i != len(xs)-2 {
					return nil, syntax(clause, "else clause must be last")
				}
				if !yes {
					yes, err = b.feature(parts[0])
					if err != nil {
						return nil, err
					}
				}
				if yes {
					nested, err := r.declarations(ctx, e, parts[1:], stack)
					if err != nil {
						return nil, err
					}
					result = append(result, nested...)
					break
				}
			}
		default:
			return nil, syntax(n, "unknown library declaration")
		}
	}
	return result, nil
}
