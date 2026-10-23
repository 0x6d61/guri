package vm

import "github.com/0x6d61/guri/internal/scheme"

// advanceDynamic performs one exit/entry. Each callback resumes through an
// immutable Action, so capture/escape within a hook cannot hide work in Go.
func (m *Machine) advanceDynamic(finish Action) Action {
	target := finish.dynamic
	ancestors := map[*Dynamic]bool{nil: true}
	for d := target; d != nil; d = d.parent {
		ancestors[d] = true
	}
	if !ancestors[m.dynamic] {
		leaving := m.dynamic
		m.dynamic = leaving.parent
		if leaving.after == nil {
			return finish
		}
		return Invoke(leaving.after, nil, func(_ *Context, _ scheme.Results) (Action, error) { return finish, nil })
	}
	entering := target
	for entering.parent != m.dynamic {
		entering = entering.parent
	}
	if entering.before == nil {
		m.dynamic = entering
		return finish
	}
	return Invoke(entering.before, nil, func(_ *Context, _ scheme.Results) (Action, error) {
		return Action{kind: enteredAction, dynamic: entering, next: &finish}, nil
	})
}
