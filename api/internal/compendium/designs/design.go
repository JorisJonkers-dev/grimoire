package designs

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// design is one Design file: the entry it automates, how fully, and its Effects.
type design struct {
	Entry      string   `yaml:"entry"`
	Name       string   `yaml:"name"`
	Automation string   `yaml:"automation"`
	Exception  []string `yaml:"exception"`
	Pattern    string   `yaml:"pattern"`
	Effects    []effect `yaml:"effects"`
}

// effect is one Effect of an entry: its label when the entry has several, its Activation, its Parts and
// the Scaling of the Activation's own values.
type effect struct {
	Label      string      `yaml:"label"`
	Activation activation  `yaml:"activation"`
	Parts      []yaml.Node `yaml:"parts"`
	Scaling    []yaml.Node `yaml:"scaling"`
}

type activation struct {
	Concentration bool      `yaml:"concentration"`
	Duration      duration  `yaml:"duration"`
	Targeting     targeting `yaml:"targeting"`
	Save          save      `yaml:"save"`
}

// save is the saving throw each target makes as the Effect is activated; no dc is the caster's spell
// save DC.
type save struct {
	Ability string `yaml:"ability"`
	DC      int    `yaml:"dc"`
}

// targeting is what the Activation may pick; none states no Targeting.
type targeting struct {
	Kind     string   `yaml:"kind"`
	Count    int      `yaml:"count"`
	RangeFt  int      `yaml:"range_ft"`
	Side     string   `yaml:"side"`
	Types    []string `yaml:"creature_types"`
	Seen     bool     `yaml:"seen"`
	Willing  bool     `yaml:"willing"`
	SpreadFt int      `yaml:"spread_ft"`
}

type duration struct {
	Unit       string `yaml:"unit"`
	Amount     int    `yaml:"amount"`
	RepeatSave string `yaml:"repeat_save"`
}

// design reads one Design file and, when every check passes, keeps its Effects.
func (r *reader) design(file string, body []byte) {
	var d design
	if err := strict(body, &d); err != nil {
		r.bad(file, "%v", err)
		return
	}
	entry := strings.TrimSuffix(file, ".yaml")
	if problem := r.envelope(entry, d); problem != "" {
		r.bad(file, "%s", problem)
		return
	}
	folder, slugOf, _ := strings.Cut(entry, "/")
	loaded := make([]Loaded, 0, len(d.Effects))
	for i, e := range d.Effects {
		def, err := e.definition(d.Name, slugOf, len(d.Effects) > 1)
		if err != nil {
			r.bad(file, "effect %d: %v", i+1, err)
			return
		}
		def.Owner = owner(folder)
		loaded = append(loaded, Loaded{Owner: effects.Owner{Kind: def.Owner, Slug: slugOf}, Definition: def})
	}
	if problem := manualIn(d, loaded); problem != "" {
		r.bad(file, "%s", problem)
		return
	}
	for _, l := range loaded {
		if other := r.owners[l.Definition.Slug]; other != "" {
			r.bad(file, "an Effect named %s is already loaded from %s", l.Definition.Slug, other)
			return
		}
		r.owners[l.Definition.Slug] = file
	}
	r.effects = append(r.effects, loaded...)
}

// envelope checks what a file says about itself against its path, the seed and the exception list.
func (r *reader) envelope(entry string, d design) string {
	switch {
	case d.Entry != entry:
		return "entry is " + d.Entry + ", not the file's own path"
	case r.seed[entry] == "":
		return "no such entry in the SRD 2024 seed"
	case r.seed[entry] != d.Name:
		return "name is " + d.Name + "; the seed calls it " + r.seed[entry]
	case d.Automation != "full" && d.Automation != "exception":
		return "automation is full or exception, not " + d.Automation
	case len(d.Effects) == 0:
		return "no effects"
	}
	return r.exception(entry, d)
}

// exception checks an entry against the approved list, both ways.
func (r *reader) exception(entry string, d design) string {
	approved, listed := r.exceptions[entry]
	switch {
	case d.Automation == "full" && len(d.Exception) > 0:
		return "a full entry states no exception"
	case d.Automation == "full" && listed:
		return "the approved list marks this entry an exception"
	case d.Automation == "full":
		return labels(d.Effects)
	case !listed:
		return "an exception must be on the approved list"
	case len(d.Exception) == 0:
		return "an exception states the parts left to the DM"
	case !slices.Equal(d.Exception, approved.Parts):
		return "the approved list leaves " + strings.Join(approved.Parts, ", ") + " to the DM, not " + strings.Join(d.Exception, ", ")
	}
	return labels(d.Effects)
}

// labels checks that an entry with several Effects names each once.
func labels(es []effect) string {
	if len(es) < 2 {
		return ""
	}
	seen := map[string]bool{}
	for _, e := range es {
		if e.Label == "" {
			return "name every Effect with a label when the entry has several"
		}
		if seen[slug(e.Label)] {
			return "label each Effect once"
		}
		seen[slug(e.Label)] = true
	}
	return ""
}

// manualIn refuses a Manual Part in an entry the file says is full.
func manualIn(d design, loaded []Loaded) string {
	if d.Automation != "full" {
		return ""
	}
	for _, l := range loaded {
		if hasManual(l.Definition.Parts) {
			return "a full entry holds no Manual Part"
		}
	}
	return ""
}

func hasManual(ps []effects.Part) bool {
	for _, p := range ps {
		if p.Kind() == "manual" {
			return true
		}
		if g, ok := p.(effects.Grouper); ok {
			for _, group := range g.Groups() {
				if hasManual(group.Parts) {
					return true
				}
			}
		}
	}
	return false
}

// definition makes one Effect: its Parts and their Scaling through the kinds, its Activation's values
// and their Scaling.
func (e effect) definition(name, slugOf string, several bool) (effects.Definition, error) {
	d := effects.Definition{Slug: slugOf, Name: name, Concentration: e.Activation.Concentration}
	if several {
		d.Slug, d.Name = slugOf+"-"+slug(e.Label), name+": "+e.Label
	}
	var err error
	if d.Duration, err = e.Activation.Duration.read(); err != nil {
		return d, fmt.Errorf("duration: %w", err)
	}
	if d.Targeting, err = e.Activation.Targeting.read(); err != nil {
		return d, fmt.Errorf("targeting: %w", err)
	}
	d.Save = effects.InitialSave{Ability: e.Activation.Save.Ability, DC: e.Activation.Save.DC}
	if err := d.Save.Check(); err != nil {
		return d, fmt.Errorf("save: %w", err)
	}
	authored, err := partsOf(pointers(e.Parts))
	if err != nil {
		return d, err
	}
	if d.Parts, err = effects.Decode(authored); err != nil {
		return d, err
	}
	own, err := scalingOf(pointers(e.Scaling), effects.ActivationFields())
	if err != nil {
		return d, err
	}
	if d.Scaling, err = effects.DecodeActivationScaling(own); err != nil {
		return d, err
	}
	parts, err := effects.DecodeScaling(authored, 0)
	d.Scaling = append(d.Scaling, parts...)
	return d, err
}

func (t targeting) read() (effects.Targeting, error) {
	out := effects.Targeting{
		Kind: effects.TargetKind(t.Kind), Count: t.Count, RangeFt: t.RangeFt, Side: effects.Side(t.Side), Types: t.Types,
		Seen: t.Seen, Willing: t.Willing, SpreadFt: t.SpreadFt,
	}
	if reflect.ValueOf(t).IsZero() {
		return out, nil
	}
	return out, out.Check()
}

func (u duration) read() (effects.Duration, error) {
	out := effects.Duration{Kind: effects.DurationKind(u.Unit), Amount: u.Amount, RepeatSave: u.RepeatSave}
	units := effects.ActivationFields()[1].Options
	abilities := effects.Abilities()
	switch {
	case u == (duration{}):
	case !slices.Contains(units, u.Unit):
		return out, fmt.Errorf("unit: choose one of %s", strings.Join(units, ", "))
	case u.Amount < 0 || u.Amount > 1000:
		return out, fmt.Errorf("amount: runs from 0 to 1000")
	case u.RepeatSave != "" && !slices.Contains(abilities, u.RepeatSave):
		return out, fmt.Errorf("repeat_save: choose one of %s", strings.Join(abilities, ", "))
	}
	return out, nil
}

// partsOf reads Parts as the kinds name their fields, each value typed as its field, and the Parts beneath
// in the groups named by the remaining keys.
func partsOf(nodes []*yaml.Node) ([]effects.Authored, error) {
	out := make([]effects.Authored, 0, len(nodes))
	for i, n := range nodes {
		a, err := partOf(n)
		if err != nil {
			return nil, effects.PartError{At: "part " + strconv.Itoa(i+1), Err: err}
		}
		out = append(out, a)
	}
	return out, nil
}

func pointers(nodes []yaml.Node) []*yaml.Node {
	out := make([]*yaml.Node, len(nodes))
	for i := range nodes {
		out[i] = &nodes[i]
	}
	return out
}
