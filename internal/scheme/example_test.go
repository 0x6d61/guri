package scheme_test

import (
	"fmt"
	"math/big"

	"github.com/0x6d61/guri/internal/scheme"
)

func ExampleNewRational() {
	value, err := scheme.NewRational(big.NewInt(2), big.NewInt(6))
	if err != nil {
		panic(err)
	}
	fmt.Println(value.(*scheme.Rational).String(), value.Exact())
	// Output: 1/3 true
}

func ExampleNewString() {
	value, err := scheme.NewString("A日本")
	if err != nil {
		panic(err)
	}
	ch, err := scheme.NewCharacter('😀')
	if err != nil {
		panic(err)
	}
	if err := value.Set(1, ch); err != nil {
		panic(err)
	}
	fmt.Println(value.Text(), value.Len())
	// Output: A😀本 3
}
