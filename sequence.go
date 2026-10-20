package guri

import "unicode/utf8"

func sequenceLimit(value Value, useCapacity bool) (int, error) {
	switch v := value.(type) {
	case VectorValue:
		if useCapacity {
			return cap(v), nil
		}
		return len(v), nil
	case string:
		return utf8.RuneCountInString(v), nil
	case BytesValue:
		return len(v), nil
	}
	return 0, failure(TypeError, "expected Vector, String or Bytes, got %T", value)
}

func nthValue(args []Value) (Value, error) {
	limit, err := sequenceLimit(args[0], false)
	if err != nil {
		return nil, err
	}
	n, err := indexValue(args[1])
	if err != nil {
		return nil, err
	}
	if n < 0 || n >= int64(limit) {
		return nil, failure(IndexError, "index %d outside sequence length %d", n, limit)
	}
	switch v := args[0].(type) {
	case string:
		return string([]rune(v)[n]), nil
	case BytesValue:
		return int64(v[n]), nil
	default:
		return args[0].(VectorValue)[n], nil
	}
}

func sliceValue(args []Value) (Value, error) {
	limit, err := sequenceLimit(args[0], true)
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
	if low < 0 || high < low || high > int64(limit) {
		return nil, failure(IndexError, "slice [%d:%d] outside sequence bound %d", low, high, limit)
	}
	switch v := args[0].(type) {
	case string:
		return string([]rune(v)[low:high]), nil
	case BytesValue:
		return v[low:high], nil
	default:
		return args[0].(VectorValue)[low:high], nil
	}
}
