package scheme

// Symbol names are immutable and interned within an interpreter's symbol table.
type Symbol struct{ name string }

func (*Symbol) Kind() Kind          { return KindSymbol }
func (v *Symbol) schemeValue() bool { return v != nil }
func (s *Symbol) Name() string      { return s.name }

type SymbolTable struct{ symbols map[string]*Symbol }

func NewSymbolTable() *SymbolTable { return &SymbolTable{symbols: make(map[string]*Symbol)} }
func (t *SymbolTable) Intern(name string) *Symbol {
	if s := t.symbols[name]; s != nil {
		return s
	}
	if t.symbols == nil {
		t.symbols = make(map[string]*Symbol)
	}
	s := &Symbol{name: name}
	t.symbols[name] = s
	return s
}

// Nonzero-sized tokens provide distinct identities without global counters.
// Scope sets preserve lexical context; resolving them is the expander's job.
type Scope struct{ identity byte }

func NewScope() *Scope { return &Scope{} }

type Identifier struct {
	symbol *Symbol
	scopes []*Scope
}

func NewIdentifier(symbol *Symbol) Identifier {
	if symbol == nil {
		panic("scheme: identifier needs a symbol")
	}
	return Identifier{symbol: symbol}
}
func (id Identifier) Symbol() *Symbol  { return id.symbol }
func (id Identifier) Scopes() []*Scope { return append([]*Scope(nil), id.scopes...) }
func (id Identifier) WithScope(scope *Scope) Identifier {
	if scope == nil {
		panic("scheme: nil scope")
	}
	for _, s := range id.scopes {
		if s == scope {
			return id
		}
	}
	return Identifier{symbol: id.symbol, scopes: append(id.Scopes(), scope)}
}
func (id Identifier) SameContext(other Identifier) bool {
	if id.symbol != other.symbol || len(id.scopes) != len(other.scopes) {
		return false
	}
	for _, s := range id.scopes {
		found := false
		for _, o := range other.scopes {
			if s == o {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Cell is a binding location, never a Scheme value. Closures retain its pointer.
type Cell struct {
	value       Value
	initialized bool
}

func NewCell(value Value) *Cell   { requireValue(value); return &Cell{value: value, initialized: true} }
func NewUninitializedCell() *Cell { return &Cell{} }
func (c *Cell) Get() (Value, error) {
	if !c.initialized {
		return nil, ErrUninitialized
	}
	return c.value, nil
}
func (c *Cell) Set(value Value) { requireValue(value); c.value = value; c.initialized = true }

type BindingID struct{ identity byte }
type Binding struct {
	id         *BindingID
	identifier Identifier
	cell       *Cell
}

func NewBinding(id Identifier, cell *Cell) *Binding {
	if id.symbol == nil || cell == nil {
		panic("scheme: binding needs an identifier and a cell")
	}
	return &Binding{id: &BindingID{}, identifier: id, cell: cell}
}
func (b *Binding) ID() *BindingID         { return b.id }
func (b *Binding) Identifier() Identifier { return b.identifier }
func (b *Binding) Cell() *Cell            { return b.cell }

// Source positions use one-based lines and Unicode scalar columns.
type Source struct {
	Filename     string
	Line, Column int
}

// Syntax associates a datum with source and lexical context. It is metadata,
// not a Scheme value. Container traversal/source trees belong to the reader.
type Syntax struct {
	datum  Value
	source Source
	scopes []*Scope
}

func NewSyntax(datum Value, source Source, scopes ...*Scope) Syntax {
	requireValue(datum)
	var unique []*Scope
	for _, scope := range scopes {
		if scope == nil {
			panic("scheme: nil scope")
		}
		found := false
		for _, previous := range unique {
			if previous == scope {
				found = true
				break
			}
		}
		if !found {
			unique = append(unique, scope)
		}
	}
	return Syntax{datum: datum, source: source, scopes: unique}
}
func (s Syntax) Datum() Value   { return s.datum }
func (s Syntax) Source() Source { return s.source }
func (s Syntax) Identifier() (Identifier, bool) {
	symbol, ok := s.datum.(*Symbol)
	if !ok {
		return Identifier{}, false
	}
	return Identifier{symbol: symbol, scopes: append([]*Scope(nil), s.scopes...)}, true
}
