package live_test

import (
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

var update = flag.Bool("update", false, "rewrite the shared live message fixture")

const fixture = "../../../../fixtures/live-messages.json"

type contract struct {
	Commands []live.Command `json:"commands"`
	Updates  []live.Update  `json:"updates"`
}

// samples covers every command and update kind; the web client parses each with its generated schemas.
func samples() contract {
	token := live.TokenView{ID: "0190c7a8-0000-7000-8000-00000000000a", Label: "Goblin", Kind: "enemy", Q: 2, R: -1, Hidden: false, DarkvisionFt: 0, ControllerID: "0190c7a8-0000-7000-8000-00000000000f"}
	id := "0190c7a8-0000-7000-8000-00000000000c"
	view := &live.View{
		Tokens: []live.TokenView{token}, Fog: true, Visible: []live.Hex{{Q: 0, R: 0}}, Remembered: []live.Hex{{Q: 1, R: 0}},
		Map: &live.MapView{
			ID: id, Name: "Crypt", ImageURL: "/api/v1/campaigns/0190c7a8-0000-7000-8000-00000000000d/maps/" + id + "/image?v=2",
			Width: 400, Height: 300, HexSizePx: 40, OriginX: 34.64, OriginY: 40, ImageVersion: 2,
		},
	}
	ac, hp, most := 15, 4, 7
	view.Tokens = append(view.Tokens, live.TokenView{
		ID: "0190c7a8-0000-7000-8000-000000000013", Label: "Aria", Kind: "party", AC: &ac, HP: &hp, HPMax: &most,
		Attacks: []live.AttackView{{Name: "Longsword", ToHit: 5, ReachFt: 5, Damage: "1d8", DamageBonus: 3, DamageType: "slashing"}},
	}, live.TokenView{ID: "0190c7a8-0000-7000-8000-000000000014", Label: "Orc", Kind: "enemy", Health: "bloodied"})
	seventeen, zero := 17, 0
	view.Combat = &live.CombatView{Status: "active", Round: 2, Combatants: []live.CombatantView{{
		ID: "0190c7a8-0000-7000-8000-000000000010", TokenID: token.ID, Label: "Goblin", Kind: "enemy", RollID: "0190c7a8-0000-7000-8000-000000000011",
		Initiative: &seventeen, Rank: 1, Acting: true, Action: true, Reaction: true, MovementFt: 20, SpeedFt: 30,
		Tactics: "auto", Suggestion: &live.SuggestionView{AttackNo: &zero, TargetID: "0190c7a8-0000-7000-8000-000000000013", Reason: "Simple: Aria is the nearest enemy, 5 ft away."},
	}}, Attack: &live.PendingAttackView{
		AttackerID: token.ID, TargetID: "0190c7a8-0000-7000-8000-000000000013", Name: "Scimitar", Stage: "damage",
		RollID: "0190c7a8-0000-7000-8000-000000000015", Critical: true,
	}}
	dmView := *view
	dmView.Walls, dmView.Ambient = []live.Hex{{Q: 2, R: 0}}, "dark"
	dmView.Lights = []live.LightView{{ID: "0190c7a8-0000-7000-8000-00000000000e", Q: 4, R: 0, BrightFt: 20, DimFt: 40}}
	return contract{
		Commands: []live.Command{
			{Nonce: "n1", Kind: live.CmdResync},
			{Nonce: "n2", Kind: live.CmdPlace, Label: "Goblin", TokenKind: "enemy", Q: 2, R: -1, Hidden: true, DarkvisionFt: 60},
			{Nonce: "n3", Kind: live.CmdMove, TokenID: token.ID, Q: 3, R: -1},
			{Nonce: "n4", Kind: live.CmdSetHidden, TokenID: token.ID, Hidden: false},
			{Nonce: "n5", Kind: live.CmdRemove, TokenID: token.ID},
			{Nonce: "n6", Kind: live.CmdSetMap, MapID: id},
			{Nonce: "n7", Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 1, R: 1}}, On: true},
			{Nonce: "n8", Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 2, R: 0}}, On: true},
			{Nonce: "n9", Kind: live.CmdPlaceLight, Q: 4, R: 0, BrightFt: 20, DimFt: 40},
			{Nonce: "n10", Kind: live.CmdRemoveLight, LightID: "0190c7a8-0000-7000-8000-00000000000e"},
			{Nonce: "n11", Kind: live.CmdSetAmbient, Ambient: "dark"},
			{Nonce: "n12", Kind: live.CmdPlace, Label: "Aria", TokenKind: "party", ControllerID: "0190c7a8-0000-7000-8000-00000000000f"},
			{Nonce: "n13", Kind: live.CmdPlanWalk, TokenID: token.ID, Q: 3, R: 0},
			{Nonce: "n14", Kind: live.CmdWalk, TokenID: token.ID, Q: 3, R: 0},
			{Nonce: "n15", Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: token.ID, InitiativeBonus: 2, SpeedFt: 30}}},
			{Nonce: "n16", Kind: live.CmdSpend, CombatantID: "0190c7a8-0000-7000-8000-000000000010", Resource: live.ResourceBonusAction},
			{Nonce: "n17", Kind: live.CmdEndTurn, CombatantID: "0190c7a8-0000-7000-8000-000000000010"},
			{Nonce: "n18", Kind: live.CmdEndCombat},
			{Nonce: "n19", Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: "enemy"},
			{Nonce: "n20", Kind: live.CmdPlace, CharacterID: "0190c7a8-0000-7000-8000-000000000012"},
			{Nonce: "n21", Kind: live.CmdPreviewAttack, TokenID: token.ID, AttackNo: 1, TargetID: "0190c7a8-0000-7000-8000-000000000013"},
			{Nonce: "n22", Kind: live.CmdAttack, TokenID: token.ID, TargetID: "0190c7a8-0000-7000-8000-000000000013"},
			{Nonce: "n23", Kind: live.CmdUndoDamage},
			{Nonce: "n24", Kind: live.CmdSetTactics, TokenID: token.ID, Tactics: "cunning"},
		},
		Updates: []live.Update{
			{
				Kind: live.UpdSnapshot, Seq: 4, View: view,
				Session: &live.SessionView{ID: "0190c7a8-0000-7000-8000-00000000000b", Number: 3, GridRadius: 10, Audience: live.AudienceParty},
			},
			{Kind: live.UpdView, Seq: 5, Nonce: "n3", View: &dmView},
			{Kind: live.UpdView, Seq: 6, View: &live.View{Tokens: []live.TokenView{}, Visible: []live.Hex{}, Remembered: []live.Hex{}}},
			{Kind: live.UpdRejected, Seq: 6, Nonce: "n9", Reason: "Only the DM can change the table."},
			{Kind: live.UpdPath, Seq: 6, Nonce: "n13", Path: &live.PathView{TokenID: token.ID, Hexes: []live.Hex{{Q: 2, R: -1}, {Q: 3, R: -1}, {Q: 3, R: 0}}, CostFt: 10}},
			{Kind: live.UpdView, Seq: 7, Nonce: "n14", View: view, Steps: []live.View{*view}},
			{Kind: live.UpdAttackPreview, Seq: 7, Nonce: "n21", Preview: &live.AttackPreview{
				TokenID: token.ID, TargetID: "0190c7a8-0000-7000-8000-000000000013", AttackNo: 1, Name: "Shortbow", HitChance: 30, Mode: "disadvantage",
				DamageMin: 3, DamageMax: 8, CritMax: 14, Reasons: []string{"Shortbow: +4 to hit", "Disadvantage: long range"},
			}},
			{Kind: live.UpdEnded, Seq: 7},
		},
	}
}

// TestContractFixture pins the Go wire format to the fixture the web client validates.
func TestContractFixture(t *testing.T) {
	got := samples()
	if *update {
		raw, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixture, append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var want contract
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("live messages changed; rerun with -update and check the web client still parses them")
	}
}
