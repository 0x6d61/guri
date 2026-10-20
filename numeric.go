package guri

import (
	"math"
	"math/big"
)

func (i *Interpreter) native(name string, min, max int, call func([]Value) (Value, error)) {
	i.global.bindings[name] = &cell{&FunctionValue{call: func(args []Value) (Value, error) {
		if err := arity(name, len(args), min, max); err != nil {
			return nil, err
		}
		return call(args)
	}}}
}

func (i *Interpreter) installNumbers() {
	for _, op := range []string{"+", "-", "*", "/"} {
		min := 0
		if op == "-" {
			min = 1
		}
		if op == "/" {
			min = 2
		}
		i.native(op, min, -1, func(args []Value) (Value, error) { return arithmetic(op, args) })
	}
	i.native("inc", 1, 1, func(args []Value) (Value, error) { return arithmetic("+", []Value{args[0], int64(1)}) })
	i.native("not", 1, 1, func(args []Value) (Value, error) { return !Truthy(args[0]), nil })
	i.native("=", 2, -1, func(args []Value) (Value, error) {
		for _, v := range args[1:] {
			if !Equal(args[0], v) {
				return false, nil
			}
		}
		return true, nil
	})
	i.native("!=", 2, 2, func(args []Value) (Value, error) { return !Equal(args[0], args[1]), nil })
	for _, op := range []string{"<", "<=", ">", ">="} {
		i.native(op, 2, -1, func(args []Value) (Value, error) {
			// Validate all operands even if an early comparison is false.
			for _, arg := range args {
				if _, ok := compareNumbers(arg, int64(0)); !ok {
					return nil, failure(TypeError, "%s requires finite numbers", op)
				}
			}
			for index := 1; index < len(args); index++ {
				cmp, _ := compareNumbers(args[index-1], args[index])
				if (op == "<" && cmp >= 0) || (op == "<=" && cmp > 0) || (op == ">" && cmp <= 0) || (op == ">=" && cmp < 0) {
					return false, nil
				}
			}
			return true, nil
		})
	}
}

func arithmetic(op string, args []Value) (Value, error) {
	useFloat := op == "/"
	for _, arg := range args {
		switch n := arg.(type) {
		case int64:
		case float64:
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, failure(ArithmeticError, "non-finite Float")
			}
			useFloat = true
		default:
			return nil, failure(TypeError, "%s requires numbers, got %T", op, arg)
		}
	}
	if useFloat {
		values := make([]float64, len(args))
		for index, arg := range args {
			switch n := arg.(type) {
			case int64:
				values[index] = float64(n)
			case float64:
				values[index] = n
			}
		}
		result := float64(0)
		start := 0
		if op == "*" {
			result = 1
		}
		if op == "-" || op == "/" {
			result = values[0]
			start = 1
		}
		if op == "-" && len(values) == 1 {
			result = -result
		}
		for _, n := range values[start:] {
			switch op {
			case "+":
				result += n
			case "-":
				result -= n
			case "*":
				result *= n
			case "/":
				if n == 0 {
					return nil, failure(ArithmeticError, "division by zero")
				}
				result /= n
			}
			if math.IsInf(result, 0) || math.IsNaN(result) {
				return nil, failure(ArithmeticError, "non-finite Float result")
			}
		}
		return result, nil
	}
	result := big.NewInt(0)
	start := 0
	if op == "*" {
		result.SetInt64(1)
	}
	if op == "-" {
		result.SetInt64(args[0].(int64))
		start = 1
		if len(args) == 1 {
			result.Neg(result)
		}
	}
	for _, arg := range args[start:] {
		n := big.NewInt(arg.(int64))
		switch op {
		case "+":
			result.Add(result, n)
		case "-":
			result.Sub(result, n)
		case "*":
			result.Mul(result, n)
		}
		if !result.IsInt64() {
			return nil, failure(ArithmeticError, "Integer overflow")
		}
	}
	if !result.IsInt64() {
		return nil, failure(ArithmeticError, "Integer overflow")
	}
	return result.Int64(), nil
}
