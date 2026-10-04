package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/guides"
)

// The guides are worked out of the SRD documents and of nothing else: spells by level, the attacks of
// monsters by Challenge Rating, and magic items by the tier of play their rarity first suits. What
// the compendium lists and opens for anyone is SRD too.
func TestTheGuidesAndThePublicCompendiumHoldOnlySRDContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO compendium.documents (key, title, ruleset_year, precedence, license, attribution, url) VALUES
		('srd-2014', 'SRD 5.1', 2014, 10, 'CC-BY-4.0', 'a', 'https://a'), ('srd-2024', 'SRD 5.2', 2024, 20, 'CC-BY-4.0', 'a', 'https://a'),
		('tome-of-secrets', 'Somebody else''s book', 2024, 99, 'All rights reserved', 'b', 'https://b')`)
	exec(`INSERT INTO compendium.magic_schools (slug, name) VALUES ('evocation', 'Evocation'), ('abjuration', 'Abjuration')`)
	spell := func(doc, slug, name string, level int, school string) {
		t.Helper()
		exec(`INSERT INTO compendium.spells (document_id, slug, name, level, school_id, casting_time, range_text, requires_verbal, requires_somatic, requires_material, ritual, concentration, duration, description, attack_roll)
			SELECT d.id, $2, $3, $4, s.id, '1 action', '150 feet', true, true, false, false, false, 'Instantaneous', '', false
			FROM compendium.documents d, compendium.magic_schools s WHERE d.key = $1 AND s.slug = $5`, doc, slug, name, level, school)
	}
	spell("srd-2024", "fire-bolt", "Fire Bolt", 0, "evocation")
	spell("srd-2024", "shield", "Shield", 1, "abjuration")
	spell("srd-2024", "alarm", "Alarm", 1, "abjuration")
	spell("srd-2014", "fireball", "Fireball (2014)", 3, "evocation")
	spell("srd-2024", "fireball", "Fireball", 3, "evocation")
	spell("srd-2014", "old-ward", "Old Ward", 9, "abjuration")
	spell("tome-of-secrets", "forbidden-word", "Forbidden Word", 1, "evocation")
	spell("tome-of-secrets", "fireball", "Fireball, Rewritten", 3, "evocation")
	monster := func(doc, slug string, cr float64, attacks ...guides.Attack) {
		t.Helper()
		exec(`INSERT INTO compendium.monsters (document_id, slug, name, size, creature_type, alignment, armor_class, hit_points, hit_dice, challenge_rating, xp,
			strength, dexterity, constitution, intelligence, wisdom, charisma, passive_perception)
			SELECT id, $2, $2, 'Medium', 'Beast', 'neutral', 13, 10, '2d8', $3, 100, 10, 10, 10, 10, 10, 10, 10 FROM compendium.documents WHERE key = $1`, doc, slug, cr)
		for i, a := range attacks {
			exec(`WITH act AS (INSERT INTO compendium.monster_actions (monster_id, ordering, name, description, action_type)
					SELECT m.id, $3, 'Strike', '', 'action' FROM compendium.monsters m JOIN compendium.documents d ON d.id = m.document_id WHERE d.key = $1 AND m.slug = $2 RETURNING id)
				INSERT INTO compendium.monster_attacks (action_id, ordering, name, kind, to_hit, reach_feet, range_feet, long_range_feet, damage_dice, damage_bonus, extra_dice)
				SELECT id, 0, 'Strike', 'weapon', $4, 5, 0, 0, NULLIF($5, ''), $6, NULLIF($7, '') FROM act`, doc, slug, i, a.ToHit, a.Dice, a.Bonus, a.ExtraDice)
		}
	}
	monster("srd-2024", "rat", 0)
	monster("srd-2024", "goblin", 0.25, guides.Attack{ToHit: 4, Dice: "1d6", Bonus: 2}, guides.Attack{ToHit: 4, Dice: "1d6", Bonus: 2})
	monster("srd-2024", "wolf", 0.25, guides.Attack{ToHit: 5, Dice: "2d4", Bonus: 3})
	monster("srd-2014", "goblin", 1, guides.Attack{ToHit: 9, Dice: "9d6"})
	monster("srd-2014", "ogre", 2, guides.Attack{ToHit: 6, Dice: "2d8", Bonus: 4}, guides.Attack{ToHit: 6, Bonus: 11, ExtraDice: "1d6"})
	monster("tome-of-secrets", "horror", 0.25, guides.Attack{ToHit: 19, Dice: "10d10"})
	monster("tome-of-secrets", "wolf", 5, guides.Attack{ToHit: 15, Dice: "8d8"})
	item := func(doc, slug, name string, magic bool, rarity *string) {
		t.Helper()
		exec(`INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, rarity, requires_attunement)
			SELECT id, $2, $3, '', 'wondrous', 1, 1, $4, $5, false FROM compendium.documents WHERE key = $1`, doc, slug, name, magic, rarity)
	}
	rarity := func(s string) *string { return &s }
	item("srd-2024", "potion-of-healing", "Potion of Healing", true, rarity("common"))
	item("srd-2024", "bag-of-holding", "Bag of Holding", true, rarity("uncommon"))
	item("srd-2024", "amulet-of-health", "Amulet of Health", true, rarity("rare"))
	item("srd-2014", "amulet-of-health", "Amulet of Health (2014)", true, rarity("uncommon"))
	item("srd-2024", "staff-of-power", "Staff of Power", true, rarity("very-rare"))
	item("srd-2024", "vorpal-sword", "Vorpal Sword", true, rarity("legendary"))
	item("srd-2014", "old-ring", "Old Ring", true, rarity("rare"))
	item("srd-2024", "orb-of-dragonkind", "Orb of Dragonkind", true, rarity("artifact"))
	item("srd-2024", "odd-trinket", "Odd Trinket", true, nil)
	item("srd-2024", "rope", "Rope", false, nil)
	item("srd-2024", "fine-rope", "Fine Rope", false, rarity("common"))
	item("tome-of-secrets", "ring-of-secrets", "Ring of Secrets", true, rarity("rare"))
	item("tome-of-secrets", "bag-of-holding", "Bag of Stolen Holding", true, rarity("uncommon"))

	s := pgstore.New(pool)
	g, err := s.Guides(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	names := func(level compendium.GuideLevel) []string {
		out := []string{}
		for _, sp := range level.Spells {
			out = append(out, sp.Name+" ("+sp.School+") "+sp.Slug)
		}
		return out
	}
	if len(g.Spells) != 10 {
		t.Fatalf("spell levels = %d", len(g.Spells))
	}
	for level, want := range map[int][]string{
		0: {"Fire Bolt (evocation) fire-bolt"}, 1: {"Alarm (abjuration) alarm", "Shield (abjuration) shield"}, 2: {}, 3: {"Fireball (evocation) fireball"},
		9: {"Old Ward (abjuration) old-ward"},
	} {
		if got := names(g.Spells[level]); g.Spells[level].Level != level || !slices.Equal(got, want) {
			t.Errorf("level %d spells = %v, want %v", level, got, want)
		}
	}
	// By Challenge Rating, from the lowest: the newest rules of each monster, and the attacks summed up.
	if want := []compendium.GuideChallenge{
		{Challenge: "0", Monsters: 1, Band: guides.Band{}},
		{Challenge: "1/4", Monsters: 2, Band: guides.Band{Attacks: 3, ToHitLow: 4, ToHit: 4, ToHitHigh: 5, Damage: 5}},
		{Challenge: "2", Monsters: 1, Band: guides.Band{Attacks: 2, ToHitLow: 6, ToHit: 6, ToHitHigh: 6, Damage: 13}},
	}; !slices.Equal(g.Challenges, want) {
		t.Errorf("challenges = %+v", g.Challenges)
	}
	if len(g.Tiers) != 4 || g.Tiers[1].From != 5 || !slices.Equal(g.Tiers[3].Rarities, []string{"common", "uncommon", "rare", "very-rare", "legendary"}) {
		t.Errorf("tiers = %+v", g.Tiers)
	}
	found := []string{}
	for _, r := range g.Rarities {
		line := r.Rarity + " from tier " + strconv.Itoa(r.FirstTier) + ":"
		for _, it := range r.Items {
			line += " " + it.Slug + "=" + it.Name
		}
		found = append(found, line)
	}
	if want := []string{
		"common from tier 1: potion-of-healing=Potion of Healing",
		"uncommon from tier 1: bag-of-holding=Bag of Holding",
		"rare from tier 2: amulet-of-health=Amulet of Health old-ring=Old Ring",
		"very-rare from tier 3: staff-of-power=Staff of Power",
		"legendary from tier 4: vorpal-sword=Vorpal Sword",
	}; !slices.Equal(found, want) {
		t.Errorf("loot = %v", found)
	}

	// One ruleset alone.
	old, err := s.Guides(ctx, "srd-2014")
	if err != nil {
		t.Fatal(err)
	}
	if got := names(old.Spells[3]); !slices.Equal(got, []string{"Fireball (2014) (evocation) fireball"}) || len(old.Spells[0].Spells) != 0 {
		t.Errorf("the 2014 spells = %v", got)
	}
	if want := []compendium.GuideChallenge{
		{Challenge: "1", Monsters: 1, Band: guides.Band{Attacks: 1, ToHitLow: 9, ToHit: 9, ToHitHigh: 9, Damage: 31}},
		{Challenge: "2", Monsters: 1, Band: guides.Band{Attacks: 2, ToHitLow: 6, ToHit: 6, ToHitHigh: 6, Damage: 13}},
	}; !slices.Equal(old.Challenges, want) {
		t.Errorf("the 2014 challenges = %+v", old.Challenges)
	}
	if len(old.Rarities[1].Items) != 1 || old.Rarities[1].Items[0].Name != "Amulet of Health (2014)" || len(old.Rarities[2].Items) != 1 {
		t.Errorf("the 2014 loot = %+v", old.Rarities)
	}
	// A ruleset that is not the SRD's has no guides, even asked for by name.
	if other, err := s.Guides(ctx, "tome-of-secrets"); err != nil || len(other.Challenges) != 0 || len(other.Spells[1].Spells)+len(other.Spells[3].Spells) != 0 || len(other.Rarities[2].Items) != 0 {
		t.Errorf("guides from another book = %+v %v", other, err)
	}

	// The compendium anyone reads lists and opens SRD entries only, whatever is asked for.
	for _, ruleset := range []string{"", "tome-of-secrets"} {
		spells, err := s.ListSpells(ctx, compendium.SpellFilter{Ruleset: ruleset, PageSize: 50})
		if err != nil {
			t.Fatal(err)
		}
		for _, sp := range spells {
			if sp.Ruleset == "tome-of-secrets" || sp.Slug == "forbidden-word" {
				t.Errorf("the spell list (ruleset %q) holds %+v", ruleset, sp)
			}
		}
		if ruleset != "" && len(spells) != 0 {
			t.Errorf("spells listed from another book: %+v", spells)
		}
		if _, err := s.GetSpell(ctx, "forbidden-word", ruleset); !errors.Is(err, compendium.ErrNotFound) {
			t.Errorf("a spell of another book opens (ruleset %q): %v", ruleset, err)
		}
		entries, err := s.ListEntries(ctx, compendium.EntryFilter{Kind: "magic-item", Ruleset: ruleset, PageSize: 50})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Ruleset == "tome-of-secrets" || e.Slug == "ring-of-secrets" {
				t.Errorf("the magic items (ruleset %q) hold %+v", ruleset, e)
			}
		}
		if _, err := s.GetEntry(ctx, "magic-item", "ring-of-secrets", ruleset); !errors.Is(err, compendium.ErrNotFound) {
			t.Errorf("an item of another book opens (ruleset %q): %v", ruleset, err)
		}
	}
	// The newest SRD rules still win where another book has the same entry at a higher precedence.
	if sp, err := s.GetSpell(ctx, "fireball", ""); err != nil || sp.Name != "Fireball" || sp.Ruleset != "srd-2024" {
		t.Errorf("Fireball = %+v %v", sp, err)
	}
	if e, err := s.GetEntry(ctx, "magic-item", "bag-of-holding", ""); err != nil || e.Name != "Bag of Holding" {
		t.Errorf("Bag of Holding = %+v %v", e, err)
	}
	sources, err := s.ListSources(ctx)
	if err != nil || len(sources) != 2 || sources[0].Key != "srd-2024" || sources[1].Key != "srd-2014" {
		t.Errorf("sources = %+v %v", sources, err)
	}

	// Every read reports a database fault.
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, err := pgstore.NewFaulty(pool, f).Guides(ctx, "")
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("guides: %v", err)
		}
		return err
	})
}
