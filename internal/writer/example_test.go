package writer_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/writer"
)

func ExampleWrite() {
	nodes, err := reader.ReadAll("data.scm", `(#0=(a b) #0#)`, nil)
	if err != nil {
		panic(err)
	}
	for _, mode := range []writer.Mode{writer.Normal, writer.Shared, writer.Simple} {
		var output strings.Builder
		if err := writer.Write(context.Background(), &output, nodes[0].Datum(), mode); err != nil {
			panic(err)
		}
		fmt.Println(output.String())
	}
	// Output:
	// ((a b) (a b))
	// (#0=(a b) #0#)
	// ((a b) (a b))
}
