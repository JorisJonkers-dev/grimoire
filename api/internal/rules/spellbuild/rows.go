package spellbuild

import (
	"encoding/json"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// row is a part as the builder saved it before Parts were authored by kind. A design saved then still
// reads, each row as the Parts it always built.
type row struct {
	Type       string   `json:"type"`
	When       string   `json:"when"`
	Dice       string   `json:"dice"`
	DamageType string   `json:"damageType"`
	Half       bool     `json:"half"`
	Condition  string   `json:"condition"`
	OnlyTypes  []string `json:"onlyTypes"`
	BrightFt   int      `json:"brightFt"`
	DimFt      int      `json:"dimFt"`
	Qualities  []string `json:"qualities"`
	Surface    string   `json:"surface"`
	Rounds     int      `json:"rounds"`
	Text       string   `json:"text"`
}

// UnmarshalJSON reads a design whose Parts are authored by kind, or were saved earlier as rows.
func (d *Design) UnmarshalJSON(raw []byte) error {
	type plain Design
	var in struct {
		plain
		Parts []json.RawMessage `json:"parts"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	*d = Design(in.plain)
	d.Parts = make([]effects.Authored, 0, len(in.Parts))
	for _, p := range in.Parts {
		var r struct {
			row
			effects.Authored
		}
		if err := json.Unmarshal(p, &r); err != nil {
			return err
		}
		if r.Type == "" {
			d.Parts = append(d.Parts, r.Authored)
			continue
		}
		d.upgrade(r.row)
	}
	return nil
}

// upgrade adds the Parts a row always built; start-of-turn damage becomes the spell's TurnDamage.
func (d *Design) upgrade(r row) {
	switch r.Type {
	case "damage":
		if r.When == "start_of_turn" {
			d.TurnDamage = &TurnDamage{Dice: r.Dice, DamageType: r.DamageType}
			return
		}
		v := effects.Values{"dice": text(r.Dice), "damage_type": text(r.DamageType)}
		if r.Half {
			v["half"] = effects.Value{Text: "", Number: 0, Flag: true, Texts: nil}
		}
		d.Parts = append(d.Parts, authored("save_damage", v))
	case "condition":
		d.Parts = append(d.Parts, conditionRow(r)...)
	case "light":
		d.Parts = append(d.Parts, authored("light", effects.Values{"bright_ft": number(r.BrightFt), "dim_ft": number(r.DimFt)}))
	case "reveal":
		d.Parts = append(d.Parts, authored("reveal", effects.Values{"qualities": {Text: "", Number: 0, Flag: false, Texts: r.Qualities}}))
	case "surface":
		d.Parts = append(d.Parts, authored("create_surface", effects.Values{"surface": text(r.Surface), "rounds": number(r.Rounds)}))
	case "manual":
		d.Parts = append(d.Parts, authored("manual", effects.Values{"instruction": text(r.Text)}))
	default:
		d.Parts = append(d.Parts, authored(r.Type, effects.Values{}))
	}
}

// conditionRow is a condition on a failed save, in a branch for each creature type it is limited to.
func conditionRow(r row) []effects.Authored {
	failed := authored("branch", effects.Values{"when": text("fails_by"), "n": number(1)},
		effects.AuthoredGroup{Name: "then", Parts: []effects.Authored{authored("save_condition", effects.Values{"condition": text(r.Condition)})}})
	if len(r.OnlyTypes) == 0 {
		return []effects.Authored{failed}
	}
	out := make([]effects.Authored, 0, len(r.OnlyTypes))
	for _, t := range r.OnlyTypes {
		out = append(out, authored("branch", effects.Values{"when": text("creature_is"), "creature_type": text(t)},
			effects.AuthoredGroup{Name: "then", Parts: []effects.Authored{failed}}))
	}
	return out
}

func authored(kind string, v effects.Values, groups ...effects.AuthoredGroup) effects.Authored {
	return effects.Authored{Kind: kind, Values: v, Groups: groups, Scaling: nil}
}

func text(s string) effects.Value { return effects.Value{Text: s, Number: 0, Flag: false, Texts: nil} }

func number(n int) effects.Value { return effects.Value{Text: "", Number: n, Flag: false, Texts: nil} }
