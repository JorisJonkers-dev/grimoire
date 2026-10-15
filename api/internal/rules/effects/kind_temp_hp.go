package effects

import "strconv"

// TempHP gives the bearer temporary hit points when the Effect lands (False Life, Heroism).
type TempHP struct {
	Amount int
}

func (TempHP) isPart() {}

// Kind is temp_hp.
func (TempHP) Kind() string { return "temp_hp" }

// Text is the rules sentence.
func (t TempHP) Text() string {
	return "The target gains " + strconv.Itoa(t.Amount) + " Temporary Hit Points."
}

// LandsOnBearer gives the bearer the temporary hit points.
func (t TempHP) LandsOnBearer() []Act {
	return []Act{GainTempHP(t)}
}

// Grow grows the hit points.
func (t TempHP) Grow(field string, by func(Value) Value) Part {
	if field == "amount" {
		t.Amount = by(numberValue(t.Amount)).Number
	}
	return t
}

type tempHPKind struct{}

func (tempHPKind) Scales() []Scalable {
	return []Scalable{{Field: "amount", Formula: true}}
}

func (tempHPKind) Describe() Description {
	return Description{
		Slug: "temp_hp", Label: "Temporary Hit Points", Summary: "Gives the bearer temporary hit points as the Effect lands.",
		Fields: []Field{
			{Name: "amount", Label: "Hit points", Type: NumberField, Options: nil, Required: true, Min: 1, Max: 500},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k tempHPKind) Decode(v Values, _ Groups) (Part, error) {
	n, err := v.number(k.Describe().field("amount"))
	if err != nil {
		return nil, err
	}
	return TempHP{Amount: n}, nil
}
