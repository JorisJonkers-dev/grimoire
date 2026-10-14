package effects

import (
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
)

// Reveal strips Visibility Qualities from every creature and object in the Effect's area, for
// everyone, whatever their Senses (Faerie Fire outlining the invisible).
type Reveal struct {
	Qualities []string
}

func (Reveal) isPart() {}

// Kind is reveal.
func (Reveal) Kind() string { return "reveal" }

// Text is the rules sentence.
func (r Reveal) Text() string {
	names := make([]string, 0, len(r.Qualities))
	for _, q := range r.Qualities {
		names = append(names, title(q))
	}
	return "Every creature and object in the area loses " + strings.Join(names, " and ") + ", whatever anyone's senses."
}

// Asks for the qualities stripped across the area.
func (r Reveal) Asks() []Act {
	return []Act{StripQualities(r)}
}

// LandsOnBearer strips the qualities where the Effect lands.
func (r Reveal) LandsOnBearer() []Act {
	return []Act{StripQualities(r)}
}

func qualities() []string {
	out := make([]string, 0, len(vision.Qualities()))
	for _, q := range vision.Qualities() {
		out = append(out, string(q))
	}
	return out
}

type revealKind struct{}

func (revealKind) Describe() Description {
	return Description{
		Slug: "reveal", Label: "Reveal", Summary: "Strips Visibility Qualities from everything in the area, whatever anyone's senses.",
		Fields: []Field{
			{Name: "qualities", Label: "Visibility Qualities", Type: ChoicesField, Options: qualities(), Required: true, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k revealKind) Decode(v Values, _ Groups) (Part, error) {
	got, err := v.choices(k.Describe().field("qualities"))
	if err != nil {
		return nil, err
	}
	return Reveal{Qualities: got}, nil
}
