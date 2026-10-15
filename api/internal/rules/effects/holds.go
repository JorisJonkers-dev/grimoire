package effects

// Holds is what a builder takes as Parts: the kinds at the top and beneath a Part, the fields it fills in
// itself when a Part leaves them out, and the text fields it takes only from a list.
type Holds struct {
	Kinds   []string
	Beneath []string
	Fills   []string
	Narrows []Narrow
}

// Narrow is a text field a builder takes only from a list, such as the conditions play knows.
type Narrow struct {
	Kind    string
	Field   string
	Options []string
}
