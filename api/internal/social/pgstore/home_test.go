package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/pgstore"
)

// The Dashboard shows the Sessions under way in the caller's Campaigns and what needs them before the
// next one. The search finds what the caller may open, and nothing else: no Campaign they are not in,
// nothing a DM alone sees where they are a Player, nobody's private Library, nobody who is not a Friend.
func TestTheDashboardAndTheSearchShowOnlyWhatIsTheCallers(t *testing.T) {
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
	aria, bram, cara := account(t, pool, "aria"), account(t, pool, "bram"), account(t, pool, "cara")
	exec(`UPDATE identity.accounts SET nickname = 'Firebeard' WHERE id = $1`, bram)
	exec(`UPDATE identity.accounts SET nickname = 'Firecara' WHERE id = $1`, cara)
	campaign := func(name string, members map[string]string) (uuid.UUID, map[string]uuid.UUID) {
		t.Helper()
		id, ids := uuid.New(), map[string]uuid.UUID{}
		exec(`INSERT INTO campaign.campaigns (id, name, created_by) VALUES ($1, $2, 'x')`, id, name)
		for subject, role := range members {
			ids[subject] = uuid.New()
			exec(`INSERT INTO campaign.members (id, campaign_id, auth_subject, display_name, role) VALUES ($1, $2, $3, $3, $4)`, ids[subject], id, subject, role)
		}
		return id, ids
	}
	morvain, inMorvain := campaign("Morvain", map[string]string{"aria": "dm", "bram": "player"})
	salt, inSalt := campaign("Fire Reach", map[string]string{"bram": "dm", "aria": "player"})
	vale, inVale := campaign("Fireholm", map[string]string{"cara": "dm"})
	hero := func(campaign, owner uuid.UUID, name string, ready bool, days int) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
				SELECT $1, m.auth_subject, $4, 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
			INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
			ability_method, hp_max, hp_current, level, level_up_ready, downtime_days) VALUES ($1, $1, $2, $3, $4, 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 20, 20, 4, $5, $6)`,
			id, campaign, owner, name, ready, days)
		return id
	}
	kara := hero(salt, inSalt["aria"], "Kara", true, 3)
	hero(salt, inSalt["aria"], "Quiet Tam", false, 0)
	fira := hero(morvain, inMorvain["bram"], "Firefoot", true, 1)
	hero(vale, inVale["cara"], "Firetoe", true, 2)
	session := func(campaign uuid.UUID, number int, status string, parent *uuid.UUID) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`INSERT INTO play.sessions (id, campaign_id, number, status, ended_at, parent_session_id, group_name)
			VALUES ($1, $2, $3, $4, CASE WHEN $4 = 'ended' THEN now() END, $5, CASE WHEN $5::uuid IS NULL THEN '' ELSE 'Scouts' END)`, id, campaign, number, status, parent)
		return id
	}
	live := session(morvain, 3, "live", nil)
	session(morvain, 4, "live", &live)
	session(salt, 1, "ended", nil)
	session(vale, 1, "live", nil)
	proposal := func(campaign uuid.UUID, author, name, status string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`INSERT INTO library.proposals (id, campaign_id, author_subject, author_name, kind, name, fields, status, created_at, updated_at)
			VALUES ($1, $2, $3, $3, 'creature', $4, '{}', $5, now(), now())`, id, campaign, author, name, status)
		return id
	}
	proposal(morvain, "bram", "Ash Hound", "pending")
	proposal(morvain, "bram", "Cinder Imp", "pending")
	proposal(morvain, "bram", "Old Idea", "declined")
	owlbear := proposal(salt, "aria", "Owlbear", "changes_requested")
	proposal(salt, "aria", "Waiting", "pending")
	proposal(vale, "cara", "Elsewhere", "pending")
	roll := func(campaign, member uuid.UUID, subject, status string) {
		t.Helper()
		exec(`INSERT INTO play.roll_requests (campaign_id, purpose, notation, requested_by_name, roller_member_id, roller_subject, roller_name, status, total, resolved_at)
			VALUES ($1, 'Stealth', '1d20', 'DM', $2, $3, $3, $4, CASE WHEN $4 = 'resolved' THEN 12 END, CASE WHEN $4 = 'resolved' THEN now() END)`, campaign, member, subject, status)
	}
	roll(salt, inSalt["aria"], "aria", "pending")
	roll(salt, inSalt["aria"], "aria", "resolved")
	roll(salt, inSalt["bram"], "bram", "pending")
	roll(morvain, inMorvain["bram"], "bram", "pending")
	exec(`INSERT INTO social.friend_requests (id, from_account, to_account, status, created_at) VALUES (gen_random_uuid(), $1, $2, 'pending', now()), (gen_random_uuid(), $3, $2, 'pending', now())`, bram, aria, cara)
	exec(`INSERT INTO social.blocks (blocker, blocked, created_at) VALUES ($1, $2, now())`, aria, cara)
	// A request already answered asks for nothing more.
	exec(`INSERT INTO social.friend_requests (id, from_account, to_account, status, created_at, decided_at) VALUES (gen_random_uuid(), $1, $2, 'declined', now(), now())`, account(t, pool, "dane"), aria)

	home := &app.Home{Repo: pgstore.New(pool)}
	dash, err := home.Dashboard(ctx, "aria")
	if err != nil {
		t.Fatal(err)
	}
	if want := []domain.LiveSession{{Campaign: morvain, CampaignName: "Morvain", Session: live, Number: 3, DM: true}}; !slices.Equal(dash.Live, want) {
		t.Fatalf("Aria's Sessions under way = %+v", dash.Live)
	}
	needs := func(d domain.Dashboard) []string {
		out := []string{}
		for _, n := range d.Needs {
			out = append(out, n.Kind+" | "+n.Campaign+" | "+n.Title+" | "+n.Path)
		}
		return out
	}
	if got, want := needs(dash), []string{
		"level_up | Fire Reach | Kara can level up | /campaigns/" + salt.String() + "/characters/" + kara.String() + "/level-up",
		"proposals | Morvain | 2 Proposals to review | /campaigns/" + morvain.String() + "/proposals",
		"revise_proposal | Fire Reach | Your Proposal Owlbear needs changes | /campaigns/" + salt.String() + "/proposals/" + owlbear.String(),
		"rolls | Fire Reach | 1 roll waiting on you | /campaigns/" + salt.String() + "/dice",
		"downtime | Fire Reach | Kara has 3 downtime days to spend | /campaigns/" + salt.String() + "/downtime",
		"friend_requests |  | 1 Friend Request to answer | /friends",
	}; !slices.Equal(got, want) {
		t.Fatalf("what needs Aria =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Bram plays in Morvain and runs Fire Reach: the same Campaigns, seen from the other side.
	dash, err = home.Dashboard(ctx, "bram")
	if err != nil {
		t.Fatal(err)
	}
	if len(dash.Live) != 1 || dash.Live[0].DM || dash.Live[0].Session != live {
		t.Fatalf("Bram's Sessions under way = %+v", dash.Live)
	}
	if got, want := needs(dash), []string{
		"level_up | Morvain | Firefoot can level up | /campaigns/" + morvain.String() + "/characters/" + fira.String() + "/level-up",
		"proposals | Fire Reach | 1 Proposal to review | /campaigns/" + salt.String() + "/proposals",
		"rolls | Fire Reach | 1 roll waiting on you | /campaigns/" + salt.String() + "/dice",
		"rolls | Morvain | 1 roll waiting on you | /campaigns/" + morvain.String() + "/dice",
		"downtime | Morvain | Firefoot has 1 downtime day to spend | /campaigns/" + morvain.String() + "/downtime",
	}; !slices.Equal(got, want) {
		t.Fatalf("what needs Bram =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Somebody in no Campaign, with no Account, has an empty Dashboard, not a missing one.
	if dash, err := home.Dashboard(ctx, "nobody"); err != nil || dash.Live == nil || len(dash.Live) != 0 || dash.Needs == nil || len(dash.Needs) != 0 {
		t.Fatalf("nobody's Dashboard = %+v %v", dash, err)
	}

	// What there is to find.
	exec(`INSERT INTO compendium.documents (key, title, ruleset_year, precedence, license, attribution, url) VALUES
		('home-old', 'Old', 2014, 1, 'CC-BY-4.0', 'a', 'https://a'), ('home-new', 'New', 2024, 2, 'CC-BY-4.0', 'a', 'https://a')`)
	exec(`INSERT INTO compendium.magic_schools (slug, name) VALUES ('home-evocation', 'Evocation')`)
	spell := func(doc, slug, name string, level int) {
		t.Helper()
		exec(`INSERT INTO compendium.spells (document_id, slug, name, level, school_id, casting_time, range_text, requires_verbal, requires_somatic, requires_material, ritual, concentration, duration, description, attack_roll)
			SELECT d.id, $2, $3, $4, s.id, '1 action', '150 feet', true, true, false, false, false, 'Instantaneous', '', false
			FROM compendium.documents d, compendium.magic_schools s WHERE d.key = $1 AND s.slug = 'home-evocation'`, doc, slug, name, level)
	}
	spell("home-old", "home-fireball", "Fireball (old)", 3)
	spell("home-new", "home-fireball", "Fireball", 3)
	spell("home-new", "home-fire-bolt", "Fire Bolt", 0)
	for i := range 6 {
		spell("home-new", "home-zz-"+string(rune('a'+i)), "Zzyzx "+string(rune('A'+i)), 1)
	}
	monster := func(slug, name, kind string, cr float64) {
		t.Helper()
		exec(`INSERT INTO compendium.monsters (document_id, slug, name, size, creature_type, alignment, armor_class, hit_points, hit_dice, challenge_rating, xp,
			strength, dexterity, constitution, intelligence, wisdom, charisma, passive_perception)
			SELECT id, $1, $2, 'Large', $3, 'neutral', 13, 100, '12d10', $4, 100, 10, 10, 10, 10, 10, 10, 10 FROM compendium.documents WHERE key = 'home-new'`, slug, name, kind, cr)
	}
	monster("home-fire-elemental", "Fire Elemental", "Elemental", 5)
	monster("home-firenewt", "Firenewt", "Humanoid", 0.5)
	monster("home-fire-snake", "Fire Snake", "Elemental", 0.25)
	monster("home-fire-mote", "Fire Mote", "Elemental", 0.125)
	item := func(slug, name, category string, magic bool, rarity *string) {
		t.Helper()
		exec(`INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, rarity, requires_attunement)
			SELECT id, $1, $2, '', $3, 1, 1, $4, $5, false FROM compendium.documents WHERE key = 'home-new'`, slug, name, category, magic, rarity)
	}
	rare := "rare"
	item("home-fire-opal", "Fire Opal", "wondrous", true, &rare)
	item("home-fire-charm", "Fire Charm", "wondrous", true, nil)
	item("home-firewood", "Firewood", "gear", false, nil)
	entry := func(owner, kind, name string, shared bool) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`INSERT INTO library.entries (id, owner_subject, kind, name, created_at, updated_at, shared) VALUES ($1, $2, $3, $4, now(), now(), $5)`, id, owner, kind, name, shared)
		return id
	}
	drake := entry("aria", "creature", "Fire Drake", false)
	entry("bram", "creature", "Firebrand", false)
	entry("bram", "item", "Firefly Lantern", true)
	entry("bram", "creature", "Fire Ant", true)
	lord := uuid.New()
	exec(`INSERT INTO campaign.npcs (id, campaign_id, name) VALUES ($1, $2, 'Firelord'), (gen_random_uuid(), $3, 'Firemage'), (gen_random_uuid(), $4, 'Firewarden')`, lord, morvain, salt, vale)
	exec(`INSERT INTO social.friendships (a, b, since) VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid), now())`, aria, bram)

	found := func(subject, query string) []string {
		t.Helper()
		hits, err := home.Search(ctx, subject, query)
		if err != nil {
			t.Fatalf("searching %q: %v", query, err)
		}
		out := []string{}
		for _, h := range hits {
			out = append(out, h.Group+" | "+h.Kind+" | "+h.Title+" | "+h.Preview+" | "+h.Path)
		}
		return out
	}
	if got, want := found("aria", "  FIRE "), []string{
		"compendium | spell | Fireball | Level 3 evocation spell | /compendium/spells/home-fireball",
		"compendium | spell | Fire Bolt | Cantrip evocation spell | /compendium/spells/home-fire-bolt",
		"compendium | monster | Firenewt | Large humanoid · CR 1/2 | /compendium/monster/home-firenewt",
		"compendium | monster | Fire Mote | Large elemental · CR 1/8 | /compendium/monster/home-fire-mote",
		"compendium | monster | Fire Snake | Large elemental · CR 1/4 | /compendium/monster/home-fire-snake",
		"compendium | monster | Fire Elemental | Large elemental · CR 5 | /compendium/monster/home-fire-elemental",
		"compendium | item | Firewood | Equipment · gear | /compendium/item/home-firewood",
		"compendium | magic-item | Fire Opal | Magic item · rare | /compendium/magic-item/home-fire-opal",
		"compendium | magic-item | Fire Charm | Magic item | /compendium/magic-item/home-fire-charm",
		"library | creature | Fire Drake | Your Library · creature | /library/" + drake.String(),
		"library | creature | Fire Ant | Shared Library · creature | /shared-library",
		"library | item | Firefly Lantern | Shared Library · item | /shared-library",
		"campaigns | campaign | Fire Reach | You play in this Campaign | /campaigns/" + salt.String(),
		"campaigns | character | Firefoot | Level 4 fighter in Morvain | /campaigns/" + morvain.String() + "/characters/" + fira.String(),
		"campaigns | npc | Firelord | NPC in Morvain | /campaigns/" + morvain.String() + "/npcs/" + lord.String(),
		"people | friend | Firebeard | Friend · @bram | /friends",
	}; !slices.Equal(got, want) {
		t.Fatalf("Aria searches for fire =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// What Aria must not find, each by its own name: Cara's Campaign, a Character and an NPC in it, an
	// NPC where Aria only plays, Bram's private entry, and Cara, who is no Friend of hers.
	for _, hidden := range []string{"Fireholm", "Firetoe", "Firewarden", "Firemage", "Firebrand", "Firecara", "cara"} {
		if got := found("aria", hidden); len(got) != 0 {
			t.Errorf("Aria finds %q: %v", hidden, got)
		}
	}
	// Each of them is there for whoever may see it.
	for query, who := range map[string]string{"Fireholm": "cara", "Firetoe": "cara", "Firewarden": "cara", "Firemage": "bram", "Firebrand": "bram"} {
		if got := found(who, query); len(got) != 1 {
			t.Errorf("%s searching %q finds %v", who, query, got)
		}
	}
	if got := found("bram", "morvain"); len(got) != 1 || !strings.Contains(got[0], "You play in this Campaign") {
		t.Fatalf("Bram finds the Campaign he plays in = %v", got)
	}
	if got := found("aria", "morvain"); len(got) != 1 || !strings.Contains(got[0], "You are the DM of this Campaign") {
		t.Fatalf("Aria finds the Campaign she runs = %v", got)
	}
	// A Friend is found by username too; somebody without an Account has no Friends to find.
	if got := found("aria", "bram"); len(got) != 1 || !strings.HasPrefix(got[0], "people | friend | Firebeard") {
		t.Fatalf("a Friend by username = %v", got)
	}
	if got := found("nobody", "bram"); len(got) != 0 {
		t.Fatalf("nobody's Friends = %v", got)
	}
	// A name that starts with what was typed comes before one that only holds it, and a short name
	// before a long one: the thing itself before its variants.
	spell("home-new", "home-delayed-fireball", "Delayed Blast Fireball", 7)
	spell("home-new", "home-fireball-storm", "Fireball Storm", 9)
	if got := found("aria", "fireball"); len(got) != 3 || !strings.Contains(got[0], "| Fireball |") || !strings.Contains(got[1], "| Fireball Storm |") || !strings.Contains(got[2], "| Delayed Blast Fireball |") {
		t.Fatalf("the order of what was found = %v", got)
	}
	// Five of a kind at most, by name.
	if got := found("aria", "zzyzx"); len(got) != app.SearchPerKind || !strings.Contains(got[0], "Zzyzx A") || !strings.Contains(got[4], "Zzyzx E") {
		t.Fatalf("six spells of one name = %v", got)
	}
	// What is typed is looked for as it stands: a percent sign and an underscore are no wildcards.
	entry("aria", "item", "100% Proof", false)
	entry("aria", "item", "100 Proof", false)
	entry("aria", "item", "Scroll_Case", false)
	entry("aria", "item", "ScrollXCase", false)
	entry("aria", "item", `Back\slash`, false)
	for query, want := range map[string]string{"100%": "100% Proof", "0% p": "100% Proof", "l_c": "Scroll_Case", `k\s`: `Back\slash`} {
		if got := found("aria", query); len(got) != 1 || !strings.Contains(got[0], "| "+want+" |") {
			t.Errorf("searching %q finds %v, want only %s", query, got, want)
		}
	}
	if got := found("aria", "%%"); len(got) != 0 {
		t.Errorf("two percent signs find %v", got)
	}
	if got := found("aria", "__"); len(got) != 0 {
		t.Errorf("two underscores find %v", got)
	}

	// A search is 2 to 80 characters long once trimmed.
	for _, query := range []string{"", "f", " f ", strings.Repeat("é", app.MaxSearch+1)} {
		if _, err := home.Search(ctx, "aria", query); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("searching %q: %v", query, err)
		}
	}
	if _, err := home.Search(ctx, "aria", strings.Repeat("é", app.MaxSearch)); err != nil {
		t.Errorf("the longest search: %v", err)
	}
	if got := found("aria", "fi"); len(got) == 0 {
		t.Errorf("the shortest search finds nothing")
	}

	// Every read reports a database fault.
	for name, op := range map[string]func(h *app.Home) error{
		"dashboard": func(h *app.Home) error { _, err := h.Dashboard(ctx, "aria"); return err },
		"search":    func(h *app.Home) error { _, err := h.Search(ctx, "aria", "fire"); return err },
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(&app.Home{Repo: pgstore.NewFaulty(pool, f)})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
