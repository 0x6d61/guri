package guri

// Kind identifies the syntax of a reader expression, before evaluation.
type Kind string

const (
	Nil     Kind = "Nil"
	Boolean Kind = "Boolean"
	Integer Kind = "Integer"
	Float   Kind = "Float"
	String  Kind = "String"
	Symbol  Kind = "Symbol"
	Keyword Kind = "Keyword"
	List    Kind = "List"
	Vector  Kind = "Vector"
	Map     Kind = "Map"
)

// Position uses one-based lines and Unicode code point columns.
type Position struct {
	Filename string
	Line     int
	Column   int
}

// Expr is reader syntax, not a runtime collection. Map elements alternate keys
// and values in source order so evaluation can preserve side effects.
// Value is nil, bool, int64, float64, or string according to Kind.
// Symbol and Keyword names use string; Keyword names omit the leading colon.
type Expr struct {
	Kind     Kind
	Value    any
	Elements []Expr
	Position Position
}
