package writer

import (
	"context"
	"github.com/0x6d61/guri/internal/scheme"
)

type vertex struct {
	index, low, refs int
	onStack, cyclic  bool
}
type frame struct {
	value    scheme.Value
	children []scheme.Value
	next     int
}

func compound(v scheme.Value) bool {
	switch v.(type) {
	case *scheme.Pair, *scheme.Vector, *scheme.String, *scheme.Bytevector:
		return true
	}
	return false
}
func children(v scheme.Value) []scheme.Value {
	switch x := v.(type) {
	case *scheme.Pair:
		return []scheme.Value{x.Car(), x.Cdr()}
	case *scheme.Vector:
		return x.Values()
	}
	return nil
}

// Iterative Tarjan traversal labels all cycle members without a Go recursion
// stack. Reference counts select shared objects independently of cycle marking.
func analyze(ctx context.Context, root scheme.Value) (map[scheme.Value]*vertex, error) {
	graph := map[scheme.Value]*vertex{}
	if !compound(root) {
		return graph, nil
	}
	index := 1
	graph[root] = &vertex{index: 1, low: 1, refs: 1, onStack: true}
	stack := []scheme.Value{root}
	work := []frame{{value: root, children: children(root)}}
	for len(work) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current := &work[len(work)-1]
		info := graph[current.value]
		if current.next < len(current.children) {
			child := current.children[current.next]
			current.next++
			if !compound(child) {
				continue
			}
			next := graph[child]
			if next == nil {
				index++
				graph[child] = &vertex{index: index, low: index, refs: 1, onStack: true}
				stack = append(stack, child)
				work = append(work, frame{value: child, children: children(child)})
				continue
			}
			next.refs++
			if next.onStack && next.index < info.low {
				info.low = next.index
			}
			continue
		}
		value := current.value
		self := false
		for _, child := range current.children {
			if child == value {
				self = true
				break
			}
		}
		work = work[:len(work)-1]
		if info.low == info.index {
			start := len(stack) - 1
			for stack[start] != value {
				start--
			}
			cyclic := self || start < len(stack)-1
			for _, member := range stack[start:] {
				v := graph[member]
				v.onStack = false
				v.cyclic = cyclic
			}
			stack = stack[:start]
		}
		if len(work) > 0 {
			parent := graph[work[len(work)-1].value]
			if info.low < parent.low {
				parent.low = info.low
			}
		}
	}
	return graph, nil
}
