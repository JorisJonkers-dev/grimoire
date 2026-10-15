package pgstore_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func seedOwner(d effects.Definition) effects.Owner {
	return effects.Owner{Kind: d.Owner, Slug: d.Slug}
}

// The migrations seed the SRD Effects the engine resolves: whole Effects, areas and the parts left to
// the DM all come back from rows.
func TestFreshDatabasesHoldTheSRDEffects(t *testing.T) {
	t.Parallel()
	got, err := pgstore.New(openPool(t)).Effects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"animate-dead", "bless", "burning-hands", "cone-of-cold", "counterspell", "dispel-magic", "dodging", "exhaustion", "faerie-fire", "false-life", "find-familiar", "fireball", "grease", "hellish-rebuke", "hunters-mark", "invisible", "lightning-bolt", "misty-step", "paralyzed", "polymorph", "prone", "restrained", "sapped", "shatter", "slowed", "spirit-guardians", "stunned", "thunderwave", "vexed", "wall-of-fire", "wild-shape"}
	if !reflect.DeepEqual(got.Automated(), want) {
		t.Fatalf("automated = %v", got.Automated())
	}
	partial := []string{"blinded", "charmed", "deafened", "frightened", "grappled", "incapacitated", "petrified", "poisoned", "unconscious"}
	if !reflect.DeepEqual(got.Partial(), partial) {
		t.Fatalf("partial = %v", got.Partial())
	}
	blessed := []effects.Active{{Slug: "bless", Source: "cleric"}}
	if p := got.ForAttack(blessed, []effects.Active{{Slug: "prone"}}, "cleric", true); !reflect.DeepEqual(p.AttackDice, []string{"1d4"}) || !reflect.DeepEqual(p.Advantages, []string{"Prone: advantage"}) {
		t.Fatalf("Bless against a prone target = %+v", p)
	}
	if fb, ok := got.AreaOf("fireball"); !ok || fb.Area != (effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150}) || fb.Damage.Dice != "8d6" || fb.Save != "dexterity" {
		t.Fatalf("fireball = %+v", fb)
	}
	paralyzed := []effects.Active{{Slug: "paralyzed"}}
	if !got.Incapacitated(paralyzed) || !got.Immobile(paralyzed) || !got.ForSave(paralyzed, "dexterity").Fails || !got.ForAttack(nil, paralyzed, "x", true).Crit {
		t.Fatalf("paralysed from rows = %+v", got["paralyzed"])
	}
	if got.SpeedPenaltyFt([]effects.Active{{Slug: "exhaustion", Level: 2}}) != 10 {
		t.Fatalf("exhaustion from rows = %+v", got["exhaustion"])
	}
	if g, _ := got.AreaOf("grease"); g.Condition != "prone" || g.Surface.Rounds != 10 {
		t.Fatalf("grease = %+v", g)
	}
	if tw, _ := got.AreaOf("thunderwave"); tw.Push != (effects.ForcedMove{Ft: 10, Toward: false}) || got["thunderwave"].Owner != effects.OwnedBySpell {
		t.Fatalf("thunderwave = %+v", got["thunderwave"])
	}
	if ft, ok := got.TeleportOf("misty-step"); !ok || ft != 30 {
		t.Fatalf("misty step = %d %v", ft, ok)
	}
	if l := got.LandingOf("false-life", ""); l.TempHP != 9 || !got.LandingOf("dispel-magic", "").Dispels {
		t.Fatalf("false life = %+v", l)
	}
	if ff, _ := got.AreaOf("faerie-fire"); !reflect.DeepEqual(ff.Reveals, []string{"invisible"}) {
		t.Fatalf("faerie fire reveals = %+v", ff)
	}
	if wall, _ := got.AreaOf("wall-of-fire"); wall.Area.Shape != hex.WallArea || !got["wall-of-fire"].Concentration {
		t.Fatalf("wall of fire = %+v", wall)
	}
	if !reflect.DeepEqual(got.Instructions("poisoned", "Poisoned", ""), []string{"Poisoned: ability checks are made with disadvantage."}) {
		t.Fatalf("poisoned = %v", got.Instructions("poisoned", "Poisoned", ""))
	}
}

// Every seeded Effect survives being saved again unchanged, so the editor can write back what it read.
func TestSeededEffectsRoundTripThroughTheDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	seeded, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for slug, d := range seeded {
		if err := s.SaveEffect(ctx, seedOwner(d), d); err != nil {
			t.Fatalf("save %s: %v", slug, err)
		}
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, seeded) {
		t.Fatalf("saved again differs:\n got %+v\nwant %+v", got, seeded)
	}
}

// Saving an Effect again replaces its Parts rather than adding to them.
func TestSavingAnEffectReplacesItsParts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	owner := effects.Owner{Kind: effects.OwnedBySpell, Slug: "frost-ring"}
	first := effects.Definition{Slug: "frost-ring", Name: "Frost Ring", Parts: []effects.Part{
		effects.Area{Shape: hex.SphereArea, SizeFt: 10, RangeFt: 30},
		effects.SaveDamage{Ability: "constitution", Dice: "2d6", Type: "cold", Half: true},
		effects.Manual{Instruction: "Frost Ring: the ground freezes."},
	}}
	if err := s.SaveEffect(ctx, owner, first); err != nil {
		t.Fatal(err)
	}
	second := effects.Definition{Slug: "frost-ring", Name: "Frost Ring", Owner: effects.OwnedBySpell, Concentration: true, Parts: []effects.Part{
		effects.MoveCost{Multiplier: 2},
		effects.TempHP{Amount: 5},
		effects.Teleport{RangeFt: 30},
		effects.ForcedMove{Ft: 10, Toward: true},
		effects.Dispel{},
		effects.Counter{RangeFt: 60},
		effects.GrantFeature{Name: "Darkvision"},
		effects.ResourceChange{Resource: "rage", Delta: -1},
		effects.Summon{Monster: "wolf", Count: 2, Shares: true, NeedsCommand: true},
		effects.Form{Monster: "owl", TempHP: 4},
		effects.Reveal{Qualities: []string{"invisible", "secret"}},
	}}
	if err := s.SaveEffect(ctx, owner, second); err != nil {
		t.Fatal(err)
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["frost-ring"], second) {
		t.Fatalf("frost-ring = %+v", got["frost-ring"])
	}
}

// Choices and branches nest through the database; durations and scaling come back as they went in.
func TestShapedEffectsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	d := effects.Definition{
		Slug: "frost-choice", Name: "Frost Choice", Owner: effects.OwnedBySpell, Concentration: true,
		Duration: effects.Duration{Kind: effects.Minutes, Amount: 1, RepeatSave: "wisdom"},
		Targeting: effects.Targeting{
			Kind: effects.TargetCreatures, Count: 2, RangeFt: 60, Side: effects.Enemies, Types: []string{"undead", "fiend"},
			Seen: true, Willing: false, SpreadFt: 30,
		},
		Save: effects.InitialSave{Ability: "wisdom", DC: 0},
		Scaling: []effects.Scaling{
			{Part: effects.OnActivation, Field: "concentration", Axis: effects.SlotLevel, Class: "", Rule: effects.Table{Steps: []effects.Step{{At: 4, Value: effects.Value{Flag: false}}, {At: 6, Value: effects.Value{Flag: true}}}}},
			{Part: effects.OnActivation, Field: "duration", Axis: effects.SlotLevel, Class: "", Rule: effects.Formula{Expr: "10 * (level - 2)"}},
			{Part: effects.OnActivation, Field: "duration_unit", Axis: effects.SlotLevel, Class: "", Rule: effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "hours"}}}}},
			{Part: 2, Field: "dice", Axis: effects.CharacterLevel, Class: "", Rule: effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "2d10"}}, {At: 11, Value: effects.Value{Text: "3d10"}}}}},
			{Part: 2, Field: "dice", Axis: effects.Charges, Class: "", Rule: effects.Linear{Base: 1, Per: effects.Value{Text: "1d6"}}},
			{Part: 6, Field: "n", Axis: effects.SlotLevel, Class: "", Rule: effects.Table{Steps: nil}},
			{Part: 9, Field: "amount", Axis: effects.ClassLevel, Class: "cleric", Rule: effects.Linear{Base: 2, Per: effects.Value{Number: 5}}},
			{Part: 9, Field: "amount", Axis: effects.TableColumn, Class: "rogue", Rule: effects.Column{Name: "Sneak Attack"}},
		},
		Parts: []effects.Part{
			effects.Manual{Instruction: "Frost forms."},
			effects.Choice{Modes: []effects.Group{
				{Name: "Bite", Parts: []effects.Part{effects.ExtraDamage{Dice: "1d6"}, effects.Branch{
					When: effects.Condition{Kind: effects.CreatureIs, N: 0, Type: "fiend"}, Then: []effects.Part{effects.Dispel{}},
				}}},
				{Name: "Numb", Parts: []effects.Part{effects.SpeedPenalty{Ft: 10}}},
			}},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5, Type: ""}, Then: []effects.Part{effects.Incapacitated{}, effects.MoveCost{Multiplier: 2}}},
			effects.TempHP{Amount: 3},
		},
	}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: d.Slug}, d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[d.Slug], d) {
		t.Fatalf("shaped effect:\n got %+v\nwant %+v", got[d.Slug], d)
	}
	slot := []effects.Scaling{{Part: 1, Field: "dice", Axis: effects.SlotLevel, Class: "", Rule: effects.Linear{Base: 3, Per: effects.Value{Text: "1d6"}}}}
	if fb := got["fireball"]; !reflect.DeepEqual(fb.Scaling, slot) || fb.Duration.Kind != effects.Instant {
		t.Fatalf("fireball scales by slot = %+v", fb)
	}
	if b := got["bless"]; b.Duration != (effects.Duration{Kind: effects.Minutes, Amount: 1}) {
		t.Fatalf("bless lasts a minute = %+v", b.Duration)
	}
	d.Scaling, d.Duration, d.Targeting, d.Save = nil, effects.Duration{}, effects.Targeting{Kind: effects.TargetSelf}, effects.InitialSave{}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: d.Slug}, d); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Effects(ctx); got[d.Slug].Scaling != nil || got[d.Slug].Duration != (effects.Duration{}) || !reflect.DeepEqual(got[d.Slug].Targeting, d.Targeting) || got[d.Slug].Save != (effects.InitialSave{}) {
		t.Fatalf("saving again clears the scaling = %+v", got[d.Slug])
	}
}

// The database refuses parts that cannot be right, so a bad Effect never reaches play.
func TestInvalidEffectsAreRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	owner := effects.Owner{Kind: effects.OwnedBySpell, Slug: "bad"}
	damage := []effects.Part{effects.SaveDamage{Ability: "dexterity", Dice: "2d6", Type: "fire", Half: true}}
	bad := []effects.Definition{
		{Slug: "Bad Slug", Name: "x"},
		{Slug: "bad-dice", Name: "x", Parts: []effects.Part{effects.ExtraDamage{Dice: "lots"}}},
		{Slug: "bad-shape", Name: "x", Parts: []effects.Part{effects.Area{Shape: "pyramid", SizeFt: 10}}},
		{Slug: "no-roll", Name: "x", Parts: []effects.Part{effects.BonusDie{Dice: "1d4"}}},
		{Slug: "bad-duration", Name: "x", Duration: effects.Duration{Kind: "fortnight"}},
		{Slug: "bad-scaling", Name: "x", Parts: damage, Scaling: []effects.Scaling{{Part: 0, Field: "dice", Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: effects.Value{Text: "lots"}}}}},
		{Slug: "bad-step", Name: "x", Parts: damage, Scaling: []effects.Scaling{{Part: 0, Field: "dice", Axis: effects.CharacterLevel, Rule: effects.Table{Steps: []effects.Step{{At: 30, Value: effects.Value{Text: "1d6"}}}}}}},
		{Slug: "bad-formula", Name: "x", Scaling: []effects.Scaling{{Part: effects.OnActivation, Field: "duration", Axis: effects.SlotLevel, Rule: effects.Formula{Expr: "level; drop"}}}},
		{Slug: "bad-axis", Name: "x", Scaling: []effects.Scaling{{Part: effects.OnActivation, Field: "duration", Axis: "phase_of_moon", Rule: effects.Linear{Base: 1, Per: effects.Value{Number: 1}}}}},
		{Slug: "bad-column", Name: "x", Parts: damage, Scaling: []effects.Scaling{{Part: 0, Field: "dice", Axis: effects.SlotLevel, Rule: effects.Column{Name: "Sneak Attack"}}}},
		{Slug: "bad-field", Name: "x", Parts: damage, Scaling: []effects.Scaling{{Part: 0, Field: "colour", Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: effects.Value{Text: "1d6"}}}}},
		{Slug: "bad-formula-field", Name: "x", Parts: damage, Scaling: []effects.Scaling{{Part: 0, Field: "colour", Axis: effects.SlotLevel, Rule: effects.Formula{Expr: "level"}}}},
		{Slug: "bad-part", Name: "x", Parts: damage, Scaling: []effects.Scaling{{Part: 4, Field: "dice", Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: effects.Value{Text: "1d6"}}}}},
		{Slug: "bad-branch", Name: "x", Parts: []effects.Part{effects.Branch{When: effects.Condition{Kind: "full_moon"}}}},
		{Slug: "bad-reveal", Name: "x", Parts: []effects.Part{effects.Reveal{Qualities: []string{"shiny"}}}},
		{Slug: "bad-mode", Name: "x", Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: ""}}}}},
		{Slug: "bad-nested", Name: "x", Parts: []effects.Part{effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn}, Then: []effects.Part{effects.ExtraDamage{Dice: "lots"}}}}},
	}
	for _, d := range bad {
		if err := s.SaveEffect(ctx, owner, d); err == nil {
			t.Errorf("%s saved", d.Slug)
		}
	}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: "spellbook", Slug: "x"}, effects.Definition{Slug: "odd-owner", Name: "x"}); err == nil {
		t.Error("unknown owner kind saved")
	}
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store.Pool()
}
