package spellbuild

import (
	"maps"
	"slices"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// kinds are the kinds of Part a spell holds: what an area spell plays.
func kinds() []string {
	return []string{"branch", "create_surface", "light", "manual", "reveal", "save_condition", "save_damage"}
}

// beneathBranch are the kinds a branch in a spell holds: what lands on each creature.
func beneathBranch() []string {
	return []string{"branch", "manual", "save_condition"}
}

// parts checks the spell's Parts and decodes each through its kind, with the spell's save filled in
// wherever a Part rolls one.
func (d Design) parts(surfaces []string) ([]effects.Part, error) {
	if err := held(d.Parts, kinds(), "a spell holds"); err != nil {
		return nil, err
	}
	saved, err := d.saved(d.Parts)
	if err != nil {
		return nil, err
	}
	if err := d.once(); err != nil {
		return nil, err
	}
	if err := walk(saved, func(a effects.Authored) error { return catalogued(a, surfaces) }); err != nil {
		return nil, err
	}
	parts, err := effects.Decode(saved)
	if err != nil {
		return nil, DesignError(err.Error())
	}
	return parts, nil
}

// held refuses a Part of a kind a spell does not hold where it sits.
func held(in []effects.Authored, allowed []string, where string) error {
	for _, a := range in {
		if !slices.Contains(allowed, a.Kind) {
			return DesignError(where + " Parts of the kinds " + strings.Join(allowed, ", "))
		}
		for _, g := range a.Groups {
			if err := held(g.Parts, beneathBranch(), "beneath a branch, a spell holds"); err != nil {
				return err
			}
		}
	}
	return nil
}

// saved is the Parts with the spell's save filled in wherever a Part rolls one and leaves it out.
func (d Design) saved(in []effects.Authored) ([]effects.Authored, error) {
	out := make([]effects.Authored, 0, len(in))
	for _, a := range in {
		values, err := d.save(a)
		if err != nil {
			return nil, err
		}
		groups := make([]effects.AuthoredGroup, 0, len(a.Groups))
		for _, g := range a.Groups {
			parts, err := d.saved(g.Parts)
			if err != nil {
				return nil, err
			}
			groups = append(groups, effects.AuthoredGroup{Name: g.Name, Parts: parts})
		}
		out = append(out, effects.Authored{Kind: a.Kind, Values: values, Groups: groups})
	}
	return out, nil
}

// save is a Part's values with the spell's save as its ability, when its kind rolls one.
func (d Design) save(a effects.Authored) (effects.Values, error) {
	k, _ := effects.KindOf(a.Kind) // held has refused a kind the rules do not know
	if !slices.ContainsFunc(k.Describe().Fields, func(f effects.Field) bool { return f.Name == "ability" }) {
		return a.Values, nil
	}
	if d.Save == "" {
		return nil, DesignError("a Part that rolls a save needs the save the targets make")
	}
	if got := a.Values["ability"].Text; got != "" && got != d.Save {
		return nil, DesignError("each Part that rolls a save rolls the spell's save, " + d.Save)
	}
	out := effects.Values{}
	maps.Copy(out, a.Values)
	out["ability"] = effects.Value{Text: d.Save, Number: 0, Flag: false, Texts: nil}
	return out, nil
}

// once refuses a second damage on cast, a second light, and a second ground.
func (d Design) once() error {
	if count(d.Parts, "save_damage") > 1 {
		return DesignError("a spell deals its damage on cast once")
	}
	if count(d.Parts, "light") > 1 {
		return DesignError("a spell sheds one light")
	}
	ground := count(d.Parts, "create_surface")
	if d.TurnDamage != nil {
		ground++
	}
	if ground > 1 {
		return DesignError("a spell leaves one Surface or one start-of-turn damage, not both")
	}
	return nil
}

// catalogued refuses a Surface or a condition play does not know.
func catalogued(a effects.Authored, surfaces []string) error {
	if s := a.Values["surface"].Text; a.Kind == "create_surface" && !slices.Contains(surfaces, s) {
		return DesignError("choose a Surface: " + strings.Join(surfaces, ", "))
	}
	if c := a.Values["condition"].Text; a.Kind == "save_condition" && !slices.Contains(Conditions(), c) {
		return DesignError("choose a condition: " + strings.Join(Conditions(), ", "))
	}
	return nil
}

// walk visits every Part, those beneath others too.
func walk(in []effects.Authored, visit func(effects.Authored) error) error {
	for _, a := range in {
		if err := visit(a); err != nil {
			return err
		}
		for _, g := range a.Groups {
			if err := walk(g.Parts, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// count is how many of the spell's own Parts are of a kind.
func count(in []effects.Authored, kind string) int {
	n := 0
	for _, a := range in {
		if a.Kind == kind {
			n++
		}
	}
	return n
}
