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
		Attacks: []live.AttackView{{Name: "Longsword", ToHit: 5, ReachFt: 5, Damage: "1d8", DamageBonus: 3, DamageType: "slashing", Mastery: "sap"}}, Shield: true,
		Reactions: []live.ReactionSettingView{{Kind: "shield", Mode: "always"}}, Dying: &live.DyingView{Successes: 1, Failures: 2, RollID: "0190c7a8-0000-7000-8000-000000000038"},
	}, live.TokenView{ID: "0190c7a8-0000-7000-8000-000000000014", Label: "Orc", Kind: "enemy", Health: "bloodied"})
	seventeen, zero := 17, 0
	view.Combat = &live.CombatView{Status: "active", Round: 2, Combatants: []live.CombatantView{{
		ID: "0190c7a8-0000-7000-8000-000000000010", TokenID: token.ID, Label: "Goblin", Kind: "enemy", RollID: "0190c7a8-0000-7000-8000-000000000011",
		Initiative: &seventeen, Rank: 1, Acting: true, Action: true, Reaction: true, MovementFt: 20, SpeedFt: 30, Surprised: true, Disengaged: true, Readied: true, AttacksLeft: 1, OffHand: true, Interaction: true, Cleave: true,
		Tactics: "auto", Suggestion: &live.SuggestionView{AttackNo: &zero, TargetID: "0190c7a8-0000-7000-8000-000000000013", Reason: "Simple: Aria is the nearest enemy, 5 ft away."},
	}}, Attack: &live.PendingAttackView{
		AttackerID: token.ID, TargetID: "0190c7a8-0000-7000-8000-000000000013", Name: "Scimitar", Stage: "damage",
		RollID: "0190c7a8-0000-7000-8000-000000000015", Critical: true,
	}, Prompt: &live.PromptView{
		ID: "0190c7a8-0000-7000-8000-000000000016", Kind: "shield", ReactorID: "0190c7a8-0000-7000-8000-000000000013", TriggerID: token.ID,
		Effect: "Shield: AC 15 → 20, so the attack (18) would miss.", SecondsLeft: 9,
	}}
	view.Tokens[0].Effects = []live.EffectView{{ID: "0190c7a8-0000-7000-8000-000000000017", Slug: "bless", Name: "Bless", SourceID: token.ID, Concentration: true, RoundsLeft: 9}, {ID: "0190c7a8-0000-7000-8000-000000000037", Slug: "exhaustion", Name: "Exhaustion", Level: 2}}
	view.Resolving = true
	view.Saves = []live.SaveView{{RollID: "0190c7a8-0000-7000-8000-000000000019", TokenID: token.ID, Effect: "Hold Person", DC: 13}}
	view.Surfaces = []live.SurfaceView{{Q: 1, R: 1, Kind: "grease", RoundsLeft: 9}}
	view.Elevation = []live.ElevationView{{Q: 1, R: 1, ElevationFt: 10}}
	view.Area = &live.AreaView{
		CasterID: token.ID, Name: "Fireball", Hexes: []live.Hex{{Q: 3, R: 0}}, DamageRollID: "0190c7a8-0000-7000-8000-000000000020",
		Saves: []live.AreaSave{{TokenID: token.ID, RollID: "0190c7a8-0000-7000-8000-000000000021"}},
	}
	view.Table = &live.TableView{Camera: "free", Q: 2, R: -1, ZoomPct: 150, Scene: "world", WorldMap: view.Map}
	view.World = &live.WorldView{
		Map: *view.Map, Revealed: []live.Hex{{Q: 0, R: 0}},
		Nodes: []live.NodeView{{ID: "0190c7a8-0000-7000-8000-000000000022", Name: "Oakford", Q: 0, R: 0}, {ID: "0190c7a8-0000-7000-8000-000000000023", Name: "Mill", Q: 5, R: 0}},
		Routes: []live.RouteView{{
			ID: "0190c7a8-0000-7000-8000-000000000024", FromNodeID: "0190c7a8-0000-7000-8000-000000000022", ToNodeID: "0190c7a8-0000-7000-8000-000000000023", DistanceMi: 12,
			Plans: []live.PlanView{{Pace: "slow", Minutes: 360, Days: 1}, {Pace: "normal", Minutes: 240, Days: 1}, {Pace: "fast", Minutes: 180, Days: 1}},
		}},
		PartyNodeID: "0190c7a8-0000-7000-8000-000000000022",
		Legs:        []live.LegView{{From: "Mill", To: "Oakford", Pace: "normal", DistanceMi: 12, Minutes: 240, Days: 1}},
	}
	view.Perception = []live.PerceptionView{{RollID: "0190c7a8-0000-7000-8000-000000000026", TokenID: token.ID}}
	three := 3
	view.Inventory = []live.ContainerView{
		{
			ID: "0190c7a8-0000-7000-8000-000000000030", Kind: "character", Label: "Aria", CharacterID: "0190c7a8-0000-7000-8000-000000000012", OwnerID: "0190c7a8-0000-7000-8000-00000000000f",
			Items: []live.ItemView{{Slug: "rope", Name: "Rope", Count: 2, WeightLb: 10}}, Coins: []live.CoinView{{Coin: "gp", Count: 50}}, WeightLb: 11, CapacityLb: 120,
			Instances: []live.InstanceView{{ID: "0190c7a8-0000-7000-8000-000000000034", Slug: "rope", Name: "Climbing Line", Count: 1, Charges: &three, Identified: true, Attuned: true, Slot: "neck", WeightLb: 5}},
		},
		{
			ID: "0190c7a8-0000-7000-8000-000000000031", Kind: "loot_drop", Label: "Loot: Hoard", Items: []live.ItemView{}, Instances: []live.InstanceView{}, Coins: []live.CoinView{}, Encumbered: false,
			Claims: []live.ClaimView{{CharacterID: "0190c7a8-0000-7000-8000-000000000012", Name: "Aria", Item: "rope", Choice: "need", Roll: 14}},
		},
		{ID: "0190c7a8-0000-7000-8000-000000000035", Kind: "bag", Label: "Backpack", ParentID: "0190c7a8-0000-7000-8000-000000000030", Items: []live.ItemView{}, Instances: []live.InstanceView{}, Coins: []live.CoinView{}},
	}
	view.Rest = &live.RestView{
		Kind: live.RestShort, Status: "resting", ProposedBy: "0190c7a8-0000-7000-8000-00000000000f", Agreed: []string{"0190c7a8-0000-7000-8000-00000000000f"},
		Waiting: []string{}, WaitingOnDM: false, Resters: []live.ResterView{{
			CharacterID: "0190c7a8-0000-7000-8000-000000000012", TokenID: token.ID, Name: "Aria", HitDie: "d10", HitDiceLeft: 2,
			RollID: "0190c7a8-0000-7000-8000-000000000036",
		}},
	}
	view.Checks = []live.CheckView{{ID: "0190c7a8-0000-7000-8000-000000000027", Trigger: "long_rest", Visibility: "open", Status: "resolved", Outcome: "encounter", ChancePct: 25, ChanceRoll: 12}}
	off := -10
	view.Shop = &live.ShopView{
		ID: "0190c7a8-0000-7000-8000-000000000033", Name: "Store", Kind: "general", Settlement: "Oakford", Owner: "Tamsin",
		Stock:  []live.StockView{{Slug: "rope", Name: "Rope", Count: 3, PriceCP: 150, WeightLb: 5}},
		Offers: []live.OfferView{{CharacterID: "0190c7a8-0000-7000-8000-000000000012", Slug: "silver-ingot", PriceCP: 275, Junk: true}},
		Haggles: []live.HaggleView{
			{CharacterID: "0190c7a8-0000-7000-8000-000000000012", AdjustPct: &off},
			{CharacterID: "0190c7a8-0000-7000-8000-000000000034", RollID: "0190c7a8-0000-7000-8000-000000000035"},
		},
	}
	view.GameDay = 3
	dmView := *view
	noticed, target := true, "0190c7a8-0000-7000-8000-000000000013"
	dmView.Checks = []live.CheckView{{
		ID: "0190c7a8-0000-7000-8000-000000000027", Trigger: "long_rest", Visibility: "open", Status: "resolved", Outcome: "encounter", ChancePct: 25, ChanceRoll: 12,
		RollID: "0190c7a8-0000-7000-8000-000000000028", TableName: "Road", Mode: "normal", Seed: "42", EntryLabel: "Ambush",
		Monsters: []live.CheckMonsterView{{Slug: "goblin", Count: 3}},
	}}
	dmView.Zones = []live.ZoneView{{
		ID: "0190c7a8-0000-7000-8000-000000000025", Name: "Ambush", Q: 3, R: 0, RadiusHexes: 2, Status: "spotting", DC: 16, Creatures: 2,
		Checks: []live.ZoneCheckView{{TokenID: token.ID}, {TokenID: target, Noticed: &noticed}},
	}}
	dmView.Walls, dmView.Ambient = []live.Hex{{Q: 2, R: 0}}, "dark"
	dmView.Resolving, dmView.Manual = false, []live.ManualView{{ID: "0190c7a8-0000-7000-8000-000000000018", Text: "Goblin: Resolve Hold Person by hand."}}
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
			{Nonce: "n25", Kind: live.CmdReact, Use: true},
			{Nonce: "n26", Kind: live.CmdPlace, Label: "Mage", TokenKind: "party", Shield: true},
			{Nonce: "n27", Kind: live.CmdApplyEffect, TargetID: token.ID, Effect: "hold-person", EffectName: "Hold Person", SourceID: token.ID, Rounds: 10, SaveAbility: "wisdom", SaveDC: 13},
			{Nonce: "n28", Kind: live.CmdEndEffect, EffectID: "0190c7a8-0000-7000-8000-000000000017"},
			{Nonce: "n29", Kind: live.CmdResolveManual, ManualID: "0190c7a8-0000-7000-8000-000000000018"},
			{Nonce: "n30", Kind: live.CmdPreviewArea, TokenID: token.ID, Effect: "fireball", Q: 3, R: 0},
			{Nonce: "n31", Kind: live.CmdCastArea, TokenID: token.ID, Effect: "fireball", Q: 3, R: 0},
			{Nonce: "n32", Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 1, R: 1}}, Surface: "grease", Rounds: 10},
			{Nonce: "n33", Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 1, R: 1}}, ElevationFt: 10},
			{Nonce: "n34", Kind: live.CmdTableCamera, Camera: "free", Q: 2, R: -1, ZoomPct: 150},
			{Nonce: "n35", Kind: live.CmdTableScene, Scene: "world", MapID: id, Title: "Greyfen", Body: "Mists."},
			{Nonce: "n36", Kind: live.CmdTableBlackout, On: true},
			{Nonce: "n37", Kind: live.CmdPing, Q: 1, R: 0},
			{Nonce: "n38", Kind: live.CmdSetWorld, MapID: id},
			{Nonce: "n39", Kind: live.CmdAddNode, Label: "Oakford", Q: 0, R: 0},
			{Nonce: "n40", Kind: live.CmdAddRoute, NodeID: "0190c7a8-0000-7000-8000-000000000022", ToNodeID: "0190c7a8-0000-7000-8000-000000000023", DistanceMi: 12},
			{Nonce: "n41", Kind: live.CmdRemoveNode, NodeID: "0190c7a8-0000-7000-8000-000000000023"},
			{Nonce: "n42", Kind: live.CmdRemoveRoute, RouteID: "0190c7a8-0000-7000-8000-000000000024"},
			{Nonce: "n43", Kind: live.CmdPlaceParty, NodeID: "0190c7a8-0000-7000-8000-000000000022"},
			{Nonce: "n44", Kind: live.CmdTravel, RouteID: "0190c7a8-0000-7000-8000-000000000024", Pace: "fast"},
			{Nonce: "n45", Kind: live.CmdAddZone, Label: "Ambush", Q: 3, R: 0, RadiusHexes: 2, DMOnly: true},
			{Nonce: "n46", Kind: live.CmdHoldZone, ZoneID: "0190c7a8-0000-7000-8000-000000000025", On: true},
			{Nonce: "n47", Kind: live.CmdSpringZone, ZoneID: "0190c7a8-0000-7000-8000-000000000025"},
			{Nonce: "n48", Kind: live.CmdRemoveZone, ZoneID: "0190c7a8-0000-7000-8000-000000000025"},
			{Nonce: "n49", Kind: live.CmdRest, Rest: "long"},
			{Nonce: "n50", Kind: live.CmdEncounterCheck, TableID: "0190c7a8-0000-7000-8000-000000000029", Mode: "pick", Entry: 2},
			{Nonce: "n51", Kind: live.CmdScheduleCheck, TableID: "0190c7a8-0000-7000-8000-000000000029", Due: "next_travel"},
			{Nonce: "n52", Kind: live.CmdRollLoot, LootTableID: "0190c7a8-0000-7000-8000-000000000032"},
			{Nonce: "n53", Kind: live.CmdMoveItem, FromID: "0190c7a8-0000-7000-8000-000000000031", ToID: "0190c7a8-0000-7000-8000-000000000030", ItemSlug: "rope", Count: 2},
			{Nonce: "n54b", Kind: live.CmdClaimLoot, FromID: "0190c7a8-0000-7000-8000-000000000031", CharacterID: "0190c7a8-0000-7000-8000-000000000012", ItemSlug: "rope", Option: "need"},
			{
				Nonce: "n54d", Kind: live.CmdTrade, FromID: "0190c7a8-0000-7000-8000-000000000030",
				Sells: []live.TradeLine{{ItemSlug: "silver-ingot", Count: 2}}, Buys: []live.TradeLine{{ItemSlug: "rope", Count: 1}},
			},
			{Nonce: "n54c", Kind: live.CmdSettleLoot, FromID: "0190c7a8-0000-7000-8000-000000000031"},
			{Nonce: "n54", Kind: live.CmdMoveCoins, FromID: "0190c7a8-0000-7000-8000-000000000031", ToID: "0190c7a8-0000-7000-8000-000000000030", Coin: "gp", Count: 50},
			{Nonce: "n55", Kind: live.CmdEndCombat, LootTableID: "0190c7a8-0000-7000-8000-000000000032"},
			{Nonce: "n56", Kind: live.CmdOpenShop, ShopID: "0190c7a8-0000-7000-8000-000000000033"},
			{Nonce: "n57", Kind: live.CmdBuy, FromID: "0190c7a8-0000-7000-8000-000000000030", ItemSlug: "rope", Count: 1},
			{Nonce: "n58", Kind: live.CmdSell, FromID: "0190c7a8-0000-7000-8000-000000000030", ItemSlug: "rope", Count: 1},
			{Nonce: "n59", Kind: live.CmdHaggle, FromID: "0190c7a8-0000-7000-8000-000000000030"},
			{Nonce: "n60", Kind: live.CmdCloseShop},
			{Nonce: "n61", Kind: live.CmdSpawnEncounter, Q: 2, R: 0, Hidden: true, Monsters: []live.SpawnMonster{{Slug: "goblin", Count: 3}}},
			{Nonce: "n62", Kind: live.CmdAdjustHP, TokenID: token.ID, HPDelta: -4},
			{Nonce: "n63", Kind: live.CmdUndo, Seq: 42},
			{Nonce: "n64", Kind: live.CmdProposeRest, Rest: live.RestLong},
			{Nonce: "n65", Kind: live.CmdAgreeRest},
			{Nonce: "n66", Kind: live.CmdSpendHitDie, TokenID: token.ID},
			{Nonce: "n67", Kind: live.CmdFinishRest},
			{Nonce: "n68", Kind: live.CmdInterruptRest},
			{Nonce: "n69", Kind: live.CmdTakeAction, TokenID: token.ID, Action: "ready", Trigger: "enters_reach", AttackNo: 0},
			{Nonce: "n70", Kind: live.CmdTakeAction, TokenID: token.ID, Action: "utilize", Detail: "pulls the lever"},
			{Nonce: "n71", Kind: live.CmdUnarmed, TokenID: token.ID, TargetID: token.ID, Option: "shove_push"},
			{Nonce: "n72", Kind: live.CmdAttack, TokenID: token.ID, AttackNo: 1, TargetID: token.ID, OffHand: true},
			{Nonce: "n73", Kind: live.CmdInteract, TokenID: token.ID, Detail: "draws a dagger"},
			{Nonce: "n73b", Kind: live.CmdSwapWeapons, TokenID: token.ID},
			{Nonce: "n74", Kind: live.CmdAttack, TokenID: token.ID, AttackNo: 0, TargetID: token.ID, Cleave: true},
			{Nonce: "n76", Kind: live.CmdStabilise, TokenID: token.ID, TargetID: token.ID, Option: "medicine"},
			{Nonce: "n77", Kind: live.CmdRevive, TargetID: token.ID, Option: "revivify"},
			{Nonce: "n75", Kind: live.CmdSetReaction, TokenID: token.ID, ReactionKind: "opportunity_attack", ReactionMode: "always", Condition: "target_bloodied"},
		},
		Updates: []live.Update{
			{
				Kind: live.UpdSnapshot, Seq: 4, View: view,
				Session: &live.SessionView{ID: "0190c7a8-0000-7000-8000-00000000000b", Number: 3, GridRadius: 10, Audience: live.AudienceParty},
			},
			{Kind: live.UpdView, Seq: 5, Nonce: "n3", ActionSeq: 42, View: &dmView},
			{Kind: live.UpdView, Seq: 6, View: &live.View{Tokens: []live.TokenView{}, Visible: []live.Hex{}, Remembered: []live.Hex{}}},
			{Kind: live.UpdRejected, Seq: 6, Nonce: "n9", Reason: "Only the DM can change the table."},
			{Kind: live.UpdPath, Seq: 6, Nonce: "n13", Path: &live.PathView{TokenID: token.ID, Hexes: []live.Hex{{Q: 2, R: -1}, {Q: 3, R: -1}, {Q: 3, R: 0}}, CostFt: 10}},
			{Kind: live.UpdView, Seq: 7, Nonce: "n14", View: view, Steps: []live.View{*view}},
			{Kind: live.UpdAttackPreview, Seq: 7, Nonce: "n21", Preview: &live.AttackPreview{
				TokenID: token.ID, TargetID: "0190c7a8-0000-7000-8000-000000000013", AttackNo: 1, Name: "Shortbow", HitChance: 30, Mode: "disadvantage",
				DamageMin: 3, DamageMax: 8, CritMax: 14, Reasons: []string{"Shortbow: +4 to hit", "Disadvantage: long range"},
			}},
			{Kind: live.UpdAreaPreview, Seq: 7, Nonce: "n30", Area: &live.AreaPreview{
				TokenID: token.ID, Effect: "fireball", Name: "Fireball", DC: 13, Hexes: []live.Hex{{Q: 3, R: 0}},
				Targets: []live.AreaTarget{{TokenID: token.ID, Ally: true}}, Allies: 1,
			}},
			{Kind: live.UpdPing, Seq: 7, Ping: &live.Hex{Q: 1, R: 0}},
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
