package effects

// Hinter is a kind that shapes its builder form beyond its fields.
type Hinter interface {
	Hints() Hints
}

// Hints shape a kind's builder form: the order its fields show in (a field left out follows, in
// definition order), sections of fields drawn together under a label, and a line of help per field.
type Hints struct {
	Order    []string
	Sections []Section
	Help     []Help
}

// Section is fields a builder form draws together under a label.
type Section struct {
	Label  string
	Fields []string
}

// Help is a line of help for one field.
type Help struct {
	Field string
	Text  string
}

// HintsOf is a kind's hints; none when it gives none.
func HintsOf(k Kind) Hints {
	if h, ok := k.(Hinter); ok {
		return h.Hints()
	}
	return Hints{Order: nil, Sections: nil, Help: nil}
}
