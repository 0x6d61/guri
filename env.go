package guri

type cell struct{ value Value }
type environment struct {
	parent   *environment
	bindings map[string]*cell
}

func newEnvironment(parent *environment) *environment {
	return &environment{parent: parent, bindings: make(map[string]*cell)}
}

func (e *environment) lookup(name string) (*cell, bool) {
	for scope := e; scope != nil; scope = scope.parent {
		if c, ok := scope.bindings[name]; ok {
			return c, true
		}
	}
	return nil, false
}

func (e *environment) define(name string, value Value) error {
	if _, exists := e.bindings[name]; exists {
		return failure(DefinitionError, "%s is already defined; use set!", name)
	}
	e.bindings[name] = &cell{value}
	return nil
}
