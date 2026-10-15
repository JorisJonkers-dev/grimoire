package conditionbuild

import (
	"encoding/json"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// row is a part as the builder saved it before Parts were authored by kind. A design saved then still
// reads, each row as the Part it always built.
type row struct {
	Type    string `json:"type"`
	Ability string `json:"ability"`
	Feet    int    `json:"feet"`
	Text    string `json:"text"`
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
		if r.Type != "" {
			r.Authored = r.upgrade()
		}
		d.Parts = append(d.Parts, r.Authored)
	}
	return nil
}

// upgrade is the Part a row always built.
func (r row) upgrade() effects.Authored {
	edge := func(against, advantage bool) effects.Values {
		return effects.Values{"against": flag(against), "advantage": flag(advantage), "range": text("any")}
	}
	save := func(mode effects.SaveMode) effects.Values {
		return effects.Values{"ability": text(r.Ability), "mode": text(string(mode))}
	}
	v, ok := map[string]effects.Values{
		"attack_advantage":      edge(false, true),
		"attack_disadvantage":   edge(false, false),
		"attacked_advantage":    edge(true, true),
		"attacked_disadvantage": edge(true, false),
		"save_advantage":        save(effects.SaveAdvantage),
		"save_disadvantage":     save(effects.SaveDisadvantage),
		"save_fails":            save(effects.SaveFails),
		"speed_penalty":         {"ft": number(r.Feet)},
		"crit_within":           {"feet": number(r.Feet)},
		"manual":                {"instruction": text(strings.TrimSpace(r.Text))},
	}[r.Type]
	kind := map[string]string{
		"attack_advantage": "edge", "attack_disadvantage": "edge", "attacked_advantage": "edge", "attacked_disadvantage": "edge",
		"save_advantage": "save_edge", "save_disadvantage": "save_edge", "save_fails": "save_edge",
	}[r.Type]
	if kind == "" {
		kind = r.Type
	}
	if !ok {
		v = effects.Values{}
	}
	return effects.Authored{Kind: kind, Values: v, Groups: []effects.AuthoredGroup{}}
}

func text(s string) effects.Value { return effects.Value{Text: s, Number: 0, Flag: false, Texts: nil} }

func number(n int) effects.Value { return effects.Value{Text: "", Number: n, Flag: false, Texts: nil} }

func flag(b bool) effects.Value { return effects.Value{Text: "", Number: 0, Flag: b, Texts: nil} }
