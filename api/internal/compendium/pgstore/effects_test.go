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
	want := []string{"bless", "burning-hands", "cone-of-cold", "counterspell", "dispel-magic", "dodging", "exhaustion", "faerie-fire", "false-life", "fireball", "grease", "hellish-rebuke", "hunters-mark", "invisible", "lightning-bolt", "misty-step", "paralyzed", "prone", "restrained", "sapped", "shatter", "slowed", "spirit-guardians", "stunned", "thunderwave", "vexed", "wall-of-fire"}
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

// Saving an Effect again replaces its components rather than adding to them.
func TestSavingAnEffectReplacesItsComponents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	owner := effects.Owner{Kind: effects.OwnedBySpell, Slug: "frost-ring"}
	first := effects.Definition{Slug: "frost-ring", Name: "Frost Ring", Components: []effects.Component{
		effects.Area{Shape: hex.SphereArea, SizeFt: 10, RangeFt: 30},
		effects.SaveDamage{Ability: "constitution", Dice: "2d6", Type: "cold", Half: true},
		effects.Manual{Instruction: "Frost Ring: the ground freezes."},
	}}
	if err := s.SaveEffect(ctx, owner, first); err != nil {
		t.Fatal(err)
	}
	second := effects.Definition{Slug: "frost-ring", Name: "Frost Ring", Owner: effects.OwnedBySpell, Concentration: true, Components: []effects.Component{
		effects.MoveCost{Multiplier: 2},
		effects.TempHP{Amount: 5},
		effects.Teleport{RangeFt: 30},
		effects.ForcedMove{Ft: 10, Toward: true},
		effects.Dispel{},
		effects.Counter{RangeFt: 60},
		effects.GrantFeature{Name: "Darkvision"},
		effects.ResourceChange{Resource: "rage", Delta: -1},
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
		Scaling:  &effects.Scaling{Axis: effects.CharacterLevel, Class: "", Column: "", Base: 0, Dice: "", Steps: []effects.Step{{At: 5, Dice: "2d10"}, {At: 11, Dice: "3d10"}}},
		Components: []effects.Component{
			effects.Manual{Instruction: "Frost forms."},
			effects.Choice{Modes: []effects.Mode{
				{Name: "Bite", Components: []effects.Component{effects.ExtraDamage{Dice: "1d6"}, effects.Branch{
					When: effects.Condition{Kind: effects.CreatureIs, N: 0, Type: "fiend"}, Then: []effects.Component{effects.Dispel{}},
				}}},
				{Name: "Numb", Components: []effects.Component{effects.SpeedPenalty{Ft: 10}}},
			}},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5, Type: ""}, Then: []effects.Component{effects.Incapacitated{}, effects.MoveCost{Multiplier: 2}}},
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
	if fb := got["fireball"]; !reflect.DeepEqual(fb.Scaling, &effects.Scaling{Axis: effects.SlotLevel, Base: 3, Dice: "1d6"}) || fb.Duration.Kind != effects.Instant {
		t.Fatalf("fireball scales by slot = %+v", fb)
	}
	if b := got["bless"]; b.Duration != (effects.Duration{Kind: effects.Minutes, Amount: 1}) {
		t.Fatalf("bless lasts a minute = %+v", b.Duration)
	}
	d.Scaling, d.Duration = nil, effects.Duration{}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: d.Slug}, d); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Effects(ctx); got[d.Slug].Scaling != nil || got[d.Slug].Duration != (effects.Duration{}) {
		t.Fatalf("saving again clears the scaling = %+v", got[d.Slug])
	}
}

// The database refuses parts that cannot be right, so a bad Effect never reaches play.
func TestInvalidEffectsAreRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	owner := effects.Owner{Kind: effects.OwnedBySpell, Slug: "bad"}
	bad := []effects.Definition{
		{Slug: "Bad Slug", Name: "x"},
		{Slug: "bad-dice", Name: "x", Components: []effects.Component{effects.ExtraDamage{Dice: "lots"}}},
		{Slug: "bad-shape", Name: "x", Components: []effects.Component{effects.Area{Shape: "pyramid", SizeFt: 10}}},
		{Slug: "no-roll", Name: "x", Components: []effects.Component{effects.BonusDie{Dice: "1d4"}}},
		{Slug: "bad-duration", Name: "x", Duration: effects.Duration{Kind: "fortnight"}},
		{Slug: "bad-scaling", Name: "x", Scaling: &effects.Scaling{Axis: effects.SlotLevel, Base: 1, Dice: "lots"}},
		{Slug: "bad-step", Name: "x", Scaling: &effects.Scaling{Axis: effects.CharacterLevel, Steps: []effects.Step{{At: 30, Dice: "1d6"}}}},
		{Slug: "bad-branch", Name: "x", Components: []effects.Component{effects.Branch{When: effects.Condition{Kind: "full_moon"}}}},
		{Slug: "bad-mode", Name: "x", Components: []effects.Component{effects.Choice{Modes: []effects.Mode{{Name: ""}}}}},
		{Slug: "bad-nested", Name: "x", Components: []effects.Component{effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn}, Then: []effects.Component{effects.ExtraDamage{Dice: "lots"}}}}},
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
