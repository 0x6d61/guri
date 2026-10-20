package guri

func mapValue(value Value) (*MapValue, error) {
	m, ok := value.(*MapValue)
	if !ok {
		return nil, failure(TypeError, "expected Map, got %T", value)
	}
	return m, nil
}

func cloneMap(m *MapValue) *MapValue {
	out := NewMap()
	for key, value := range m.entries {
		out.entries[key] = value
	}
	return out
}

func mapGet(args []Value) (Value, error) {
	m, err := mapValue(args[0])
	if err != nil {
		return nil, err
	}
	if !validKey(args[1]) {
		return nil, failure(TypeError, "invalid Map key type %T", args[1])
	}
	value, exists := m.entries[args[1]]
	if !exists && len(args) == 3 {
		return args[2], nil
	}
	return value, nil
}

func (i *Interpreter) installMaps() {
	i.native("get", 2, 3, mapGet)
	for _, name := range []string{"dissoc", "dissoc!"} {
		i.native(name, 2, -1, func(args []Value) (Value, error) {
			m, err := mapValue(args[0])
			if err != nil {
				return nil, err
			}
			for _, key := range args[1:] {
				if !validKey(key) {
					return nil, failure(TypeError, "invalid Map key type %T", key)
				}
			}
			if name == "dissoc" {
				m = cloneMap(m)
			}
			for _, key := range args[1:] {
				delete(m.entries, key)
			}
			return m, nil
		})
	}
	for _, name := range []string{"update", "update!"} {
		i.native(name, 3, -1, func(args []Value) (Value, error) {
			m, err := mapValue(args[0])
			if err != nil {
				return nil, err
			}
			if !validKey(args[1]) {
				return nil, failure(TypeError, "invalid Map key type %T", args[1])
			}
			if name == "update" {
				m = cloneMap(m)
			}
			callArgs := make([]Value, 1, len(args)-2)
			callArgs[0] = m.entries[args[1]]
			callArgs = append(callArgs, args[3:]...)
			value, err := i.apply(args[2], callArgs)
			if err != nil {
				return nil, err
			}
			m.entries[args[1]] = value
			return m, nil
		})
	}
}
