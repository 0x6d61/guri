package reader_test

import (
	"fmt"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func ExampleReadAll() {
	nodes, err := reader.ReadAll("example.scm", "(name . #u8(1 2)) #e1.25", nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(nodes[0].Car.Datum().(*scheme.Symbol).Name())
	fmt.Println(nodes[0].Cdr.Datum().(*scheme.Bytevector).Bytes())
	fmt.Println(nodes[1].Datum().(*scheme.Rational).String())
	// Output:
	// name
	// [1 2]
	// 5/4
}
