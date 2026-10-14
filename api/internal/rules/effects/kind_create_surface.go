package effects

import (
	"regexp"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// CreateSurface leaves a Surface on the area for some rounds.
type CreateSurface struct {
	Surface surface.Kind
	Rounds  int
}

func (CreateSurface) isPart() {}

// Kind is create_surface.
func (CreateSurface) Kind() string { return "create_surface" }

// Text is the rules sentence.
func (c CreateSurface) Text() string {
	return "The area is covered in " + string(c.Surface) + " for " + plural(c.Rounds, "round") + "."
}

// Asks for the Surface laid on the area.
func (c CreateSurface) Asks() []Act {
	return []Act{LaySurface(c)}
}

var surfaceSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

type createSurfaceKind struct{}

func (createSurfaceKind) Describe() Description {
	return Description{
		Slug: "create_surface", Label: "Surface", Summary: "Covers the area in a Surface for some rounds.",
		Fields: []Field{
			{Name: "surface", Label: "Surface", Type: TextField, Options: nil, Required: true, Min: 0, Max: 40},
			{Name: "rounds", Label: "Rounds", Type: NumberField, Options: nil, Required: true, Min: 1, Max: 1000},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k createSurfaceKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	if !surfaceSlug.MatchString(v["surface"].Text) {
		return nil, refuse("surface", "name a Surface by its slug, like grease")
	}
	rounds, err := v.number(d.field("rounds"))
	if err != nil {
		return nil, err
	}
	return CreateSurface{Surface: surface.Kind(v["surface"].Text), Rounds: rounds}, nil
}
