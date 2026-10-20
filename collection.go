package guri

func vector(value Value) (VectorValue, error) {
	v, ok := value.(VectorValue)
	if !ok {
		return nil, failure(TypeError, "expected Vector, got %T", value)
	}
	return v, nil
}

func indexValue(value Value) (int64, error) {
	n, ok := value.(int64)
	if !ok {
		return 0, failure(TypeError, "index must be Integer, got %T", value)
	}
	return n, nil
}

func cloneVector(v VectorValue) VectorValue {
	out := make(VectorValue, len(v))
	copy(out, v)
	return out
}

func (i *Interpreter) installCollections() {
	i.native("append", 1, -1, func(args []Value) (Value, error) {
		v, err := vector(args[0])
		if err != nil {
			return nil, err
		}
		return append(v, args[1:]...), nil
	})
	i.global.bindings["conj"] = &cell{i.global.bindings["append"].value}
	i.native("nth", 2, 2, func(args []Value) (Value, error) {
		v, err := vector(args[0])
		if err != nil {
			return nil, err
		}
		n, err := indexValue(args[1])
		if err != nil {
			return nil, err
		}
		if n < 0 || n >= int64(len(v)) {
			return nil, failure(IndexError, "index %d outside Vector length %d", n, len(v))
		}
		return v[n], nil
	})
	i.native("slice", 3, 3, func(args []Value) (Value, error) {
		v, err := vector(args[0])
		if err != nil {
			return nil, err
		}
		low, err := indexValue(args[1])
		if err != nil {
			return nil, err
		}
		high, err := indexValue(args[2])
		if err != nil {
			return nil, err
		}
		if low < 0 || high < low || high > int64(cap(v)) {
			return nil, failure(IndexError, "slice [%d:%d] outside capacity %d", low, high, cap(v))
		}
		return v[low:high], nil
	})
	i.native("cap", 1, 1, func(args []Value) (Value, error) {
		v, err := vector(args[0])
		if err != nil {
			return nil, err
		}
		return int64(cap(v)), nil
	})
	i.native("len", 1, 1, func(args []Value) (Value, error) {
		switch v := args[0].(type) {
		case VectorValue:
			return int64(len(v)), nil
		case ListValue:
			return int64(v.length), nil
		case string:
			return int64(len(v)), nil
		case *MapValue:
			return int64(len(v.entries)), nil
		}
		return nil, failure(TypeError, "len requires List, Vector, Map or String")
	})
	for _, name := range []string{"assoc", "assoc!"} {
		i.native(name, 3, 3, func(args []Value) (Value, error) { return associate(args[0], args[1], args[2], name == "assoc!") })
	}
	i.native("clone", 1, 1, func(args []Value) (Value, error) {
		switch v := args[0].(type) {
		case VectorValue:
			return cloneVector(v), nil
		case *MapValue:
			return cloneMap(v), nil
		}
		return nil, failure(TypeError, "clone requires Vector or Map")
	})
	i.native("reduce", 3, 3, func(args []Value) (Value, error) {
		v, err := vector(args[2])
		if err != nil {
			return nil, err
		}
		acc := args[1]
		for _, element := range v {
			acc, err = i.apply(args[0], []Value{acc, element})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	})
}

func associate(collection, key, value Value, mutate bool) (Value, error) {
	if m, ok := collection.(*MapValue); ok {
		if !mutate {
			m = cloneMap(m)
		}
		if err := m.Set(key, value); err != nil {
			return nil, err
		}
		return m, nil
	}
	v, err := vector(collection)
	if err != nil {
		return nil, err
	}
	n, err := indexValue(key)
	if err != nil {
		return nil, err
	}
	if n < 0 || n >= int64(len(v)) {
		return nil, failure(IndexError, "index %d outside Vector length %d", n, len(v))
	}
	if !mutate {
		v = cloneVector(v)
	}
	v[n] = value
	return v, nil
}
