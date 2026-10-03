package effects

// AreaSave is the saving throw every creature in an area makes, when what a failure brings is in
// Branches rather than in a SaveDamage or SaveCondition of its own.
type AreaSave struct {
	Ability string
}

// Light sheds bright light out to BrightFt and dim light DimFt beyond it from where the Effect lands.
type Light struct {
	BrightFt int
	DimFt    int
}

func (AreaSave) isComponent() {}
func (Light) isComponent()    {}
