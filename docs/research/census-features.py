#!/usr/bin/env python3
"""Census of the Effect component kinds SRD 5.2 Features, feats and species traits need.

Reads api/db/seeds/compendium.json.gz (document srd-2024) and writes census-features.md next to
this file. Entries the rules below recognise with confidence are classified by rule; every other
entry is classified by hand in HAND, after reading its text. The script fails when an entry has no
classification, or a hand row names an entry the seed does not hold, so the table always has
exactly one row per entry.

Run from the repository root:  python3 docs/research/census-features.py
"""

from __future__ import annotations

import gzip
import json
import re
import sys
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SEED = ROOT / "api" / "db" / "seeds" / "compendium.json.gz"
OUT = Path(__file__).with_suffix(".md")
DOCUMENT = "srd-2024"

# Component kinds the engine has today: the sealed union in api/internal/rules/effects
# (effects.go, shape.go, light.go). `mode` is the row kind of a Choice's options and is counted
# with `choice`. `area_save` and `light` exist in the union but have no catalogue row kind.
EXISTING = {
    "bonus_die": "adds a die to the bearer's attack rolls or saving throws",
    "edge": "advantage or disadvantage on the bearer's attack rolls, or on attack rolls against it",
    "extra_damage": "extra dice whenever the Effect's source hits the bearer",
    "move_cost": "multiplies what each foot of the bearer's movement costs",
    "manual": "a part the DM resolves from a prompt (a Manual Effect)",
    "area": "an area template: shape, size, range",
    "save_damage": "every creature in the area saves or takes fixed dice of damage, halved or not",
    "save_condition": "every creature in the area that fails the save gets a condition",
    "create_surface": "leaves a Surface on the area for some rounds",
    "incapacitated": "takes away actions, Bonus Actions and Reactions",
    "immobile": "holds Speed at 0",
    "save_edge": "advantage, disadvantage or sure failure on saves of one ability",
    "crit_within": "hits from within a distance are Critical Hits",
    "exhausting": "a stacking level that taxes d20 tests and Speed",
    "speed_penalty": "takes a fixed number of feet off Speed",
    "reacts": "a Reaction when the bearer is damaged, resolved by the DM from an instruction",
    "temp_hp": "a fixed number of Temporary Hit Points when the Effect lands",
    "teleport": "the caster teleports up to a fixed range",
    "forced_move": "creatures that fail the area's save are pushed or pulled a fixed distance",
    "dispel": "ends every spell on the bearer",
    "counter": "counters a spell cast within range, as a Reaction",
    "grant_feature": "names a feature the bearer gains (lands as a note for the DM today)",
    "resource_change": "spends or gives back a fixed number of uses of a Resource",
    "choice": "modes; whoever applies the Effect picks one",
    "branch": "extra components when a test holds: failed by N, HP at most N, first time this turn, creature type",
    "summon": "brings creatures in under the caster's control",
    "form": "overlays the bearer's stat block with a creature's (a Form)",
    "reveal": "strips Visibility Qualities in the area",
    "area_save": "the save every creature in an area makes when the outcome sits in branches",
    "light": "sheds bright and dim light where the Effect lands",
}

# Component kinds that do not exist yet. The names follow ARCHITECTURE.md section 9.3 where it
# already names one (Damage, Heal, SaveGate, Modifier, Sense, Trigger).
NEW = {
    "modifier": "A numeric change to a derived number, passive or while an Effect lasts: Armor Class (a bonus or a base formula), Speed, Hit Point maximum, an ability score and its cap, Initiative, an attack or damage roll, a saving throw or ability check bonus, spell save DC, the Critical Hit range, cover, attunement slots; or the ability a check or a jump uses. Carries a value source (fixed, ability modifier, Proficiency Bonus, class level, a Scale) and a scope (unarmoured, with Ranged weapons).",
    "check_edge": "Advantage or disadvantage on ability checks (by ability or skill), Initiative rolls and Death Saving Throws. `edge` covers attack rolls only and `save_edge` covers saves only.",
    "resistance": "Resistance, Immunity or Vulnerability to damage types, or damage that ignores Resistance.",
    "condition_immunity": "Immunity to a condition, or a condition that has no effect while its bearer is somewhere.",
    "sense": "Gives a Sense with a range: darkvision, blindsight, tremorsense, truesight, or sight in magical Darkness.",
    "move_mode": "Gives a Fly, Climb or Swim Speed, movement across liquids or up walls, or movement through larger creatures' spaces.",
    "damage": "Damage to one target that is not an area save: dice or a formula, of a fixed type, a type chosen on use, or the type of the attack that carried it.",
    "heal": "Restores Hit Points: dice, a formula, points spent from a pool, or setting Hit Points to a value; with a cap such as half the Hit Point maximum.",
    "save_gate": "A saving throw by one chosen target instead of everything in an area, gating the components after it, with a half-or-nothing outcome.",
    "apply_condition": "Puts a condition on a target for a duration with no save and no area.",
    "remove_condition": "Ends listed conditions on a target, or lowers its Exhaustion level.",
    "trigger": "Fires components when something happens: a hit, a Critical Hit or natural 20, a miss, a kill, taking damage, dropping to 0 Hit Points, the start or end of a turn, rolling Initiative, finishing a Rest, casting a spell; with a once-per-turn limit. `reacts` covers one trigger (damaged) and hands the outcome to the DM.",
    "roll_adjust": "Changes a roll after it is made or how its dice are read: reroll, add a die after a failure, subtract a die from another creature's roll, treat the d20 as a 20, treat 9 or lower as 10, a minimum face on damage dice, roll damage twice and keep either, use a score in place of a total, use the highest number on each die, turn a miss into a hit.",
    "grant_action": "Changes the action economy: a standard action as a Bonus Action, an extra action, an extra attack, an attack as a Reaction, replacing one attack of the Attack action, or lifting a precondition of a standard action.",
    "move": "Movement an Effect gives or forces without a save: up to a distance or half Speed without provoking Opportunity Attacks, pushing one target a fixed distance, carrying a willing creature along.",
    "grant_spell": "Spells from a Feature: always prepared by level, a known cantrip, casts without a spell slot per Rest or at will, casting without components, or more spell lists to pick from.",
    "spell_change": "Alters a spell as it is cast or resolved: range, duration, number of targets, damage type, casting time, components, chosen creatures that save automatically, half damage on a miss, bonus or maximised damage or healing, Concentration rules, a summoned creature's statistics.",
    "slot_change": "Spends, recovers, creates or converts spell slots, including recovery within a budget of slot levels.",
    "weapon_change": "Alters the bearer's weapon or Unarmed Strike profile: the ability used, the damage die, the damage type, the mastery property, proficiency with one weapon.",
    "damage_reduction": "Lowers damage the bearer is about to take: by a rolled amount, by half, or by shifting a save's outcome one step (Evasion).",
    "grant_proficiency": "Proficiency that does not come from a Choice: saving throws, weapons, armor training, tools, languages.",
    "aura": "An area that stays centred on its bearer, gives components to chosen creatures while they are inside, and switches off while the bearer is Incapacitated.",
    "restrict": "Forbids something: casting spells, Concentration, Opportunity Attacks, or all but one of move, action and Bonus Action on a turn.",
    "detect": "Shows the player information the table would otherwise not have: creature types and positions in range, a target's Resistances, another creature's senses.",
    "grant_inspiration": "Gives Heroic Inspiration.",
    "turn_order": "Changes Initiative order: swapping Initiative with an ally, a second turn in the first round.",
    "size_change": "Changes the bearer's size.",
    "create_item": "Conjures a bound object: a pact weapon, a Book of Shadows, a clockwork device.",
    "rest_rule": "Changes what a Rest takes or allows: a four-hour Long Rest, swapping a prepared spell on a Short Rest.",
    "banish": "Takes a creature off the Map and returns it later.",
}

# Existing kinds or data that would do the job if they carried more.
WIDEN = {
    "value_source": "Amounts read from the bearer on kinds that only take a fixed number today (`temp_hp`, `resource_change`, `speed_penalty`, `form`): an ability modifier, the Proficiency Bonus, a class level, a Scale, all of a Resource, up to a floor, or an amount chosen on use.",
    "dc_source": "A save DC other than the caster's spell save DC: 8 plus an ability modifier and the Proficiency Bonus, or a fixed DC that climbs with each use. Live play reads every DC from the caster's spell DC today.",
    "branch_condition": "More tests for `branch`: the attack had Advantage, an ally stands next to the target, the target is missing Hit Points or Bloodied, the target's size, the bearer's own state (raging, unarmoured, not Incapacitated, in Dim Light), the target is marked by a spell, a creature type that must not match.",
    "edge_filter": "`edge` limited to some attacks: Strength-based, spell attacks, Opportunity Attacks, the next attack only, attacks against one marked or grappled target; or cancelling Advantage against the bearer.",
    "save_edge_filter": "`save_edge` limited by what the save is against: avoiding or ending one condition, a save forced by a creature type, Concentration, one spell.",
    "duration_rule": "Durations the engine lacks: until the start of the bearer's next turn, until the end of the current turn, a number of hours or days read from a level, ending early on an event (taking damage, acting, the bearer Incapacitated, donning Heavy armor), or needing upkeep every turn.",
    "mode_from_choice": "An Effect's mode fixed by a Feature Choice made at level-up or on a Rest, instead of picked each time the Effect is applied.",
}

GAPS = {**NEW, **WIDEN}

PATTERNS = {
    "table-column": "A column of a class table stored as a Feature row. Nothing to resolve; the value lives in the class profile, a Resource or a Scale.",
    "core-traits": "The class's core traits table: Hit Die, saves, skills, weapon and armor training.",
    "spell-list": "A class spell list, kept as a reference table.",
    "spellcasting-progression": "Spellcasting or Pact Magic: cantrips, prepared spells and slots by level.",
    "subclass-choice": "Picks a subclass at a level.",
    "feat-choice": "Picks a feat from a category at a level.",
    "ability-increase": "Raises ability scores at level-up.",
    "extra-attack": "More attacks in the Attack action.",
    "weapon-mastery": "Mastering a number of weapon kinds.",
    "base-stat": "A species's size or walking Speed.",
    "proficiency-grant": "Proficiency, Expertise or languages.",
    "passive-modifier": "An always-on change to a number on the sheet.",
    "passive-edge": "Always-on advantage or disadvantage.",
    "passive-defence": "Always-on Resistance, Immunity or damage avoidance.",
    "passive-sense": "A Sense the character always has.",
    "passive-movement": "A movement mode the character always has.",
    "activated-state": "Entered on purpose, usually for a Resource, and lasting a while: a bundle of modifiers on the bearer.",
    "on-hit-rider": "Added to a hit: extra damage, a save or a condition, usually once per turn.",
    "reaction": "A Reaction to a trigger.",
    "roll-rescue": "Spending a use to fix a roll after seeing it.",
    "roll-rule": "An always-on change to how dice are read.",
    "resource-refill": "Getting Resource uses or spell slots back, or converting between them.",
    "granted-spell": "Spells always prepared, or cast without a slot.",
    "spell-knowledge": "More spells to pick from, or more freedom in preparing them.",
    "spell-alteration": "Changes spells the character casts.",
    "action-economy": "Changes what an action, Bonus Action or turn can hold.",
    "attack-profile": "Changes how the character's own attacks are built.",
    "save-action": "An action that makes creatures save against damage or a condition.",
    "heal-action": "An action that heals or gives Temporary Hit Points.",
    "aura": "An emanation around the character that helps or harms those inside.",
    "option-pick": "A lasting pick among options, each with its own Effect.",
    "event-benefit": "Something the character gains when an event happens.",
    "zero-hp-ward": "Staying up when dropped to 0 Hit Points.",
    "information": "Tells the player something about creatures nearby.",
    "item-rule": "Changes how magic items are used.",
    "rest-rule": "Changes how the character rests.",
}

# Class table columns: where each value already lives.
COLUMNS = {
    "proficiency-bonus": ("sheet", "-", "The sheet derives it from level."),
    "cantrips": ("sheet", "-", "Cantrips known by level are in the class profile."),
    "prepared-spells": ("sheet", "-", "Prepared spells by level are in the class profile."),
    "slot-level": ("sheet", "-", "Pact slot level is in the class profile."),
    "spell-slots": ("sheet", "-", "Pact slots are in the class profile."),
    "rages": ("sheet", "seeded", "Resource `rage` holds the maxima."),
    "channel-divinity-uses": ("sheet", "seeded", "Resource `channel-divinity` (Cleric) or `paladin-channel-divinity` holds the maxima."),
    "wild-shape-uses": ("sheet", "seeded", "Resource `wild-shape` holds the maxima."),
    "second-wind-uses": ("sheet", "seeded", "Resource `second-wind` holds the maxima."),
    "focus-points": ("sheet", "seeded", "Resource `focus-points` is the Monk level."),
    "favored-enemy-uses": ("sheet", "seeded", "Resource `favored-enemy` holds the maxima."),
    "sorcery-points": ("sheet", "seeded", "Resource `sorcery-points` is the Sorcerer level."),
    "bardic-die": ("sheet", "seeded", "Resource `bardic-inspiration` holds the die by level."),
    "weapon-mastery-count": ("sheet", "-", "Scale `<class>-weapon-mastery` is seeded and read."),
    "rage-damage": ("-", "-", "Scale `rage-damage` is seeded; nothing reads it yet."),
    "martial-arts-dice": ("-", "-", "Scale `martial-arts` is seeded; nothing reads it yet."),
    "sneak-attack-column-data": ("-", "-", "Scale `sneak-attack` is seeded; nothing reads it yet."),
}

SPELL_TABLES = {
    "draconic-sorcery_draconic-spells",
    "fiend-patron_fiend-spells",
    "life-domain_life-domain-spells",
    "oath-of-devotion_spells",
}

# Hand classifications, one line per entry:
# key | pattern | kinds needed | Choice | Resource | Prerequisite | passive | exception | note
# Choice: seeded, row (pool exists, row not seeded), pool (needs a pool the schema lacks).
# Resource: seeded, row (schema fits, row not seeded), rule (needs a rule the schema lacks).
# Prerequisite: seeded, kind (needs a prerequisite kind the schema lacks).
# passive: sheet (a passive value the sheet already computes), passive (passive, not computed), -.
HAND = r"""
barbarian/rage | activated-state | resource_change resistance modifier save_edge check_edge restrict duration_rule | - | seeded | - | - | - | Scale `rage-damage` is seeded for the damage bonus. Needs upkeep each turn.
barbarian/unarmored-defense | passive-modifier | modifier | - | - | - | passive | - | AC 10 + Dex + Con without armor; the sheet's AC knows armor, Dexterity and a shield only.
barbarian/danger-sense | passive-edge | save_edge branch_condition | - | - | - | passive | - | `save_edge` gives the Advantage; the gap is switching it off while Incapacitated.
barbarian/reckless-attack | activated-state | edge edge_filter duration_rule | - | - | - | - | - | Both edges exist; the own Advantage is for Strength attacks only and lasts to the start of the next turn.
barbarian/primal-knowledge | proficiency-grant | modifier branch_condition | row | - | - | - | - | One more skill from the Barbarian list; while raging, five skills may use Strength.
barbarian/fast-movement | passive-modifier | modifier | - | - | - | passive | - | Speed +10 feet without Heavy armor.
barbarian/feral-instinct | passive-edge | check_edge | - | - | - | passive | - | Advantage on Initiative.
barbarian/instinctive-pounce | action-economy | move | - | - | - | - | - | Half Speed as part of entering Rage.
barbarian/brutal-strike | on-hit-rider | trigger damage choice move speed_penalty duration_rule | - | - | - | - | - | Scale `brutal-strike` is seeded. Forgoes Advantage from Reckless Attack; the push has no save.
barbarian/relentless-rage | zero-hp-ward | trigger save_gate heal dc_source branch_condition | - | rule | - | - | - | DC 10, +5 each use until a Rest: a counter no Resource holds.
barbarian/improved-brutal-strike | on-hit-rider | trigger choice save_edge restrict modifier duration_rule | - | - | - | - | - | Adds two Brutal Strike options.
barbarian/persistent-rage | resource-refill | trigger resource_change value_source duration_rule | - | row | - | - | - | All Rage uses back on Initiative, once per Long Rest; Rage no longer needs upkeep.
barbarian/improved-brutal-strike-enhanced | on-hit-rider | trigger damage choice | - | - | - | - | - | 2d10 from the seeded Scale; two options on one strike, where `choice` picks one.
barbarian/indomitable-might | roll-rule | roll_adjust | - | - | - | passive | - | Strength score in place of a lower total.
barbarian/primal-champion | passive-modifier | modifier | - | - | - | passive | - | Strength and Constitution +4, cap 25.
bard/bardic-inspiration | roll-rescue | resource_change roll_adjust | - | seeded | - | - | - | The ally holds one die for an hour and adds it after failing a D20 Test; `bonus_die` adds to every roll instead.
bard/expertise | proficiency-grant | - | seeded | - | - | sheet | - | Choices at levels 2 and 9 are seeded.
bard/jack-of-all-trades | passive-modifier | modifier | - | - | - | passive | - | Half Proficiency Bonus on checks without proficiency.
bard/font-of-inspiration | resource-refill | slot_change resource_change | - | seeded | - | - | - | The Short Rest recharge from level 5 is already a seeded recharge row; the gap is trading a spell slot for a use.
bard/countercharm | reaction | trigger roll_adjust | - | - | - | - | - | Reroll with Advantage a failed save against Charmed or Frightened within 30 feet.
bard/magical-secrets | spell-knowledge | grant_spell | pool | - | - | - | - | Prepared spells may come from four class lists.
bard/superior-inspiration | resource-refill | trigger resource_change value_source | - | seeded | - | - | - | Back up to two uses on Initiative.
bard/words-of-creation | granted-spell | grant_spell spell_change | - | - | - | - | - | Two spells always prepared; a second target within 10 feet.
champion/champion_improved-critical | passive-modifier | modifier | - | - | - | passive | - | Critical Hit on 19-20.
champion/champion_remarkable-athlete | passive-edge | check_edge | - | - | - | passive | - | Advantage on Initiative and Athletics.
champion/champion_additional-fighting-style | feat-choice | - | row | - | - | - | - | A second Fighting Style feat; the pool exists, the subclass row does not.
champion/champion_heroic-warrior | event-benefit | trigger grant_inspiration | - | - | - | - | - | Heroic Inspiration at the start of a turn in Combat.
champion/champion_superior-critical | passive-modifier | modifier | - | - | - | passive | - | Critical Hit on 18-20.
champion/champion_survivor | event-benefit | check_edge roll_adjust trigger heal branch_condition | - | - | - | - | - | Death save Advantage, 18-20 counts as 20, and healing each turn while Bloodied.
circle-of-the-land/circle-of-the-land_lands-aid | save-action | resource_change area save_damage heal | - | seeded | - | - | - | Area, save, class-level dice and the Wild Shape cost exist; healing one creature does not.
circle-of-the-land/circle-of-the-land_spell-list | granted-spell | grant_spell mode_from_choice | row | - | - | - | - | Land type picked after each Long Rest decides the prepared spells.
circle-of-the-land/circle-of-the-land_natural-recovery | resource-refill | grant_spell slot_change | - | row | - | - | - | One free Circle spell per Long Rest; slots back on a Short Rest within a budget.
circle-of-the-land/circle-of-the-land_natures-ward | passive-defence | condition_immunity resistance mode_from_choice | - | - | - | passive | - | Classified from the SRD rule the slug names; the seed text under this slug is Nature's Sanctuary's.
circle-of-the-land/circle-of-the-land_natures-sanctuary | aura | resource_change area aura modifier resistance grant_action | - | seeded | - | - | - | Classified from the SRD rule the slug names; the seed text under this slug repeats Natural Recovery's.
cleric/divine-order | option-pick | grant_proficiency grant_spell modifier mode_from_choice | pool | - | - | - | - | Protector or Thaumaturge; Thaumaturge picks an extra cantrip.
cleric/channel-divinity | save-action | resource_change choice heal save_gate damage area save_condition incapacitated branch duration_rule | - | seeded | - | - | - | Divine Spark and Turn Undead. A turned creature's flight is the DM's movement of a monster, as always.
cleric/sear-undead | save-action | area branch damage | - | - | - | - | - | Adds Radiant damage to Turn Undead for those that fail.
cleric/blessed-strikes | option-pick | trigger damage spell_change mode_from_choice | row | - | - | - | - | Divine Strike or Potent Spellcasting.
cleric/divine-intervention | granted-spell | grant_spell | - | row | - | - | - | Any Cleric spell of level 5 or lower, no slot, once per Long Rest.
cleric/improved-blessed-strikes | option-pick | trigger damage spell_change temp_hp value_source mode_from_choice | - | - | - | - | - | Upgrades whichever Blessed Strikes option was picked.
cleric/greater-divine-intervention | granted-spell | grant_spell | - | rule | - | - | - | Wish, then no Divine Intervention for 2d4 Long Rests: a lockout no Resource holds.
college-of-lore/bonus-proficiencies | proficiency-grant | - | row | - | - | sheet | - | Three skills; the skill pool exists, the subclass row does not.
college-of-lore/cutting-words | reaction | trigger resource_change roll_adjust | - | seeded | - | - | - | Subtracts a Bardic Inspiration die from another creature's roll.
college-of-lore/magical-discoveries | spell-knowledge | grant_spell | pool | - | - | - | - | Two spells from three other lists, always prepared.
college-of-lore/peerless-skill | roll-rescue | trigger resource_change roll_adjust | - | seeded | - | - | - | A die added to a failed check or attack; kept if it still fails.
draconic-sorcery/draconic-sorcery_draconic-resilience | passive-modifier | modifier | - | - | - | passive | - | Hit Point maximum and unarmoured AC.
draconic-sorcery/draconic-sorcery_elemental-affinity | option-pick | resistance spell_change mode_from_choice | row | - | - | - | - | A damage type: Resistance to it and Charisma on one damage roll of matching spells.
draconic-sorcery/draconic-sorcery_dragon-wings | activated-state | move_mode resource_change | - | row | - | - | - | Fly Speed 60 for an hour; the use comes back for 3 Sorcery Points.
draconic-sorcery/draconic-sorcery_dragon-companion | granted-spell | grant_spell spell_change | - | row | - | - | - | Summon Dragon without a slot once per Long Rest, optionally without Concentration.
druid/druidic | granted-spell | grant_spell grant_proficiency | - | - | - | - | yes | Speak with Animals always prepared and the Druidic language. Leaving and spotting hidden messages is narrative play.
druid/primal-order | option-pick | grant_spell modifier grant_proficiency mode_from_choice | pool | - | - | - | - | Magician or Warden; Magician picks an extra cantrip.
druid/wild-companion | granted-spell | grant_spell slot_change resource_change spell_change | - | seeded | - | - | - | Find Familiar for a spell slot or a Wild Shape use; the familiar is Fey.
druid/wild-shape | activated-state | form resource_change temp_hp value_source restrict duration_rule | pool | seeded | - | - | yes | The `form` Effect is seeded. Known Beast forms by Challenge Rating are a Choice with no pool. The Objects rule asks the DM which equipment the form can wear.
druid/wild-resurgence | resource-refill | slot_change resource_change branch_condition | - | row | - | - | - | A spell slot for a Wild Shape use, and once per Long Rest a use for a level 1 slot.
druid/elemental-fury | option-pick | spell_change trigger damage mode_from_choice | row | - | - | - | - | Potent Spellcasting or Primal Strike.
druid/improved-elemental-fury | option-pick | spell_change trigger damage mode_from_choice | - | - | - | - | - | Upgrades whichever Elemental Fury option was picked.
druid/beast-spells | activated-state | restrict | - | - | - | - | - | Lifts Wild Shape's ban on casting.
druid/archdruid | resource-refill | trigger resource_change branch_condition slot_change | - | row | - | - | - | A Wild Shape use on Initiative when none are left; uses into a spell slot once per Long Rest. Longevity has no game outcome.
evoker/evoker_evocation-savant | spell-knowledge | grant_spell | pool | - | - | - | - | Free Evocation spells in the spellbook.
evoker/evoker_potent-cantrip | spell-alteration | spell_change | - | - | - | - | - | Half damage on a miss or a successful save.
evoker/evoker_sculpt-spells | spell-alteration | spell_change | - | - | - | - | - | Chosen creatures save automatically and take no damage.
evoker/evoker_empowered-evocation | spell-alteration | spell_change | - | - | - | - | - | Intelligence modifier on one damage roll.
evoker/evoker_overchannel | spell-alteration | spell_change damage | - | rule | - | - | - | Maximum damage; self-damage that grows with each use before a Long Rest.
fiend-patron/fiend-patron_dark-ones-blessing | event-benefit | trigger temp_hp value_source | - | - | - | - | - | Temporary Hit Points on a kill: Charisma modifier plus Warlock level.
fiend-patron/fiend-patron_dark-ones-own-luck | roll-rescue | trigger resource_change roll_adjust | - | row | - | - | - | 1d10 on a check or save, Charisma modifier times per Long Rest.
fiend-patron/fiend-patron_fiendish-resilience | passive-defence | resistance mode_from_choice | row | - | - | - | - | A damage type picked after each Rest.
fiend-patron/fiend-patron_hurl-through-hell | on-hit-rider | trigger save_gate damage incapacitated branch branch_condition banish slot_change | - | row | - | - | - | Once per Long Rest, or again for a Pact Magic slot.
fighter/fighting-style | feat-choice | - | seeded | - | - | - | - | The Choice is seeded; the feats are their own entries.
fighter/second-wind | heal-action | resource_change heal | - | seeded | - | - | - | 1d10 plus Fighter level.
fighter/action-surge | action-economy | resource_change grant_action | - | seeded | - | - | - | One more action, not the Magic action.
fighter/tactical-mind | roll-rescue | trigger resource_change roll_adjust | - | seeded | - | - | - | 1d10 on a failed check for a Second Wind use, kept if it still fails.
fighter/tactical-shift | action-economy | move | - | - | - | - | - | Half Speed without Opportunity Attacks with Second Wind.
fighter/indomitable | roll-rescue | trigger resource_change roll_adjust | - | seeded | - | - | - | Reroll a failed save with a bonus of the Fighter level.
fighter/tactical-master | attack-profile | weapon_change | - | - | - | - | - | Swap a weapon's mastery for Push, Sap or Slow on one attack.
fighter/studied-attacks | event-benefit | trigger edge | - | - | - | - | - | After a miss, Advantage on the next attack against that creature; the edge exists (Vex), the trigger does not.
hunter/hunter_hunters-lore | information | detect branch_condition | - | - | - | - | - | The marked creature's Immunities, Resistances and Vulnerabilities.
hunter/hunter_hunters-prey | option-pick | trigger damage branch_condition grant_action mode_from_choice | row | - | - | - | - | Colossus Slayer or Horde Breaker, picked again after any Rest.
hunter/hunter_defensive-tactics | option-pick | edge edge_filter trigger mode_from_choice | row | - | - | - | - | Escape the Horde or Multiattack Defense.
hunter/hunter_superior-hunters-prey | on-hit-rider | trigger damage branch_condition | - | - | - | - | - | Hunter's Mark damage to a second creature.
hunter/hunter_superior-hunters-defense | reaction | trigger resistance duration_rule | - | - | - | - | - | Resistance to the damage type until the end of the turn.
life-domain/life-domain_disciple-of-life | spell-alteration | spell_change | - | - | - | - | - | Healing spells restore 2 + slot level more.
life-domain/life-domain_preserve-life | heal-action | resource_change heal branch_condition | - | seeded | - | - | - | A pool of five times Cleric level split among Bloodied creatures, up to half their maximum.
life-domain/life-domain_blessed-healer | spell-alteration | trigger heal | - | - | - | - | - | Healing others with a slot heals the Cleric.
life-domain/life-domain_supreme-healing | spell-alteration | roll_adjust | - | - | - | - | - | Healing dice count as their highest face.
monk/deflect-attacks | reaction | trigger damage_reduction resource_change save_gate damage dc_source | - | seeded | - | - | - | The seed gives no level for this entry.
monk/martial-arts | attack-profile | weapon_change grant_action branch_condition | - | - | - | passive | - | Scale `martial-arts` is seeded. Dexterity for attacks, the Martial Arts die, a Bonus Action Unarmed Strike.
monk/unarmored-defense | passive-modifier | modifier | - | - | - | passive | - | AC 10 + Dex + Wis without armor or a Shield.
monk/monks-focus | action-economy | resource_change grant_action modifier dc_source | - | seeded | - | - | - | Flurry of Blows, Patient Defense, Step of the Wind. Dodge has a seeded Effect.
monk/unarmored-movement | passive-modifier | modifier | - | - | - | passive | - | Scale `unarmored-movement` is seeded; nothing reads it yet.
monk/uncanny-metabolism | resource-refill | trigger resource_change value_source heal | - | row | - | - | - | All Focus Points and some Hit Points on Initiative, once per Long Rest.
monk/stunning-strike | on-hit-rider | trigger resource_change save_gate apply_condition modifier edge edge_filter dc_source duration_rule | - | seeded | - | - | - | Stunned on a failure; halved Speed and one attack with Advantage on a success.
monk/slow-fall | reaction | trigger damage_reduction | - | - | - | - | - | Falling damage less five times the Monk level.
monk/empowered-strikes | attack-profile | weapon_change | - | - | - | passive | - | Unarmed Strikes may deal Force damage.
monk/evasion | passive-defence | damage_reduction branch_condition | - | - | - | passive | - | No damage on a successful Dexterity save, half on a failure.
monk/acrobatic-movement | passive-movement | move_mode branch_condition | - | - | - | passive | - | Up walls and across liquids while unarmoured.
monk/heightened-focus | action-economy | resource_change grant_action temp_hp value_source move | - | seeded | - | - | - | Upgrades the three Monk's Focus options.
monk/self-restoration | event-benefit | trigger remove_condition | - | - | - | - | - | Ends Charmed, Frightened or Poisoned at the end of each turn. Going without food has no modelled outcome.
monk/deflect-energy | reaction | trigger damage_reduction | - | - | - | - | - | Deflect Attacks for every damage type.
monk/disciplined-survivor | roll-rescue | grant_proficiency trigger resource_change roll_adjust | - | seeded | - | - | - | Proficiency in every save, and a reroll for a Focus Point.
monk/perfect-focus | resource-refill | trigger resource_change value_source branch_condition | - | seeded | - | - | - | Back up to four Focus Points on Initiative.
monk/superior-defense | activated-state | resource_change resistance duration_rule | - | seeded | - | - | - | Resistance to all but Force for a minute, for 3 Focus Points.
monk/body-and-mind | passive-modifier | modifier | - | - | - | passive | - | Dexterity and Wisdom +4, cap 25.
oath-of-devotion/oath-of-devotion_sacred-weapon | activated-state | resource_change modifier weapon_change light duration_rule | - | seeded | - | - | - | Charisma on attack rolls, Radiant damage, light from the weapon.
oath-of-devotion/oath-of-devotion_aura-of-devotion | aura | aura condition_immunity | - | - | - | - | - | Charmed has no effect inside the Aura of Protection.
oath-of-devotion/oath-of-devotion_smite-of-protection | aura | aura trigger modifier duration_rule | - | - | - | - | - | Half Cover in the aura after casting Divine Smite.
oath-of-devotion/oath-of-devotion_holy-nimbus | aura | aura save_edge save_edge_filter trigger damage light resource_change slot_change | - | row | - | - | - | Once per Long Rest, or again for a level 5 slot. `light` has no sunlight flag.
paladin/lay-on-hands | heal-action | resource_change value_source heal remove_condition | - | seeded | - | - | - | The pool is seeded; the amount spent is chosen on use.
paladin/fighting-style | feat-choice | grant_spell | pool | - | - | - | - | The feat Choice is seeded; Blessed Warrior picks two Cleric cantrips instead.
paladin/paladins-smite | granted-spell | grant_spell | - | row | - | - | - | Divine Smite always prepared, once per Long Rest without a slot.
paladin/channel-divinity | information | resource_change detect duration_rule | - | seeded | - | - | - | Divine Sense: Celestials, Fiends and Undead within 60 feet.
paladin/faithful-steed | granted-spell | grant_spell | - | row | - | - | - | Find Steed always prepared, once per Long Rest without a slot.
paladin/aura-of-protection | aura | aura modifier | - | - | - | - | - | Charisma modifier on saves for the Paladin and allies within 10 feet.
paladin/abjure-foes | save-action | resource_change save_gate apply_condition restrict duration_rule | - | seeded | - | - | - | Up to Charisma modifier chosen creatures; Frightened until damaged.
paladin/aura-of-courage | aura | aura condition_immunity | - | - | - | - | - | Frightened has no effect inside the Aura of Protection.
paladin/radiant-strikes | on-hit-rider | trigger damage | - | - | - | - | - | 1d8 Radiant on every melee hit.
paladin/restoring-touch | heal-action | resource_change remove_condition | - | seeded | - | - | - | Lay On Hands also ends six conditions for 5 points each.
paladin/aura-expansion | aura | aura | - | - | - | - | - | The Aura of Protection becomes 30 feet.
path-of-the-berserker/frenzy | on-hit-rider | trigger damage branch_condition | - | - | - | - | - | d6s equal to the Rage Damage bonus on the first Reckless hit.
path-of-the-berserker/mindless-rage | passive-defence | condition_immunity remove_condition branch_condition | - | - | - | - | - | Immune to Charmed and Frightened while raging.
path-of-the-berserker/retaliation | reaction | trigger grant_action | - | - | - | - | - | A melee attack as a Reaction when damaged from within 5 feet.
path-of-the-berserker/intimidating-presence | save-action | area save_condition dc_source resource_change | - | row | - | - | - | Area, condition, repeat save and the Rage cost to restore it exist; the DC of 8 + Str + Proficiency Bonus does not.
ranger/favored-enemy | granted-spell | grant_spell | - | seeded | - | - | - | Hunter's Mark always prepared, with seeded free casts.
ranger/deft-explorer | proficiency-grant | grant_proficiency | pool | - | - | - | - | The Expertise Choice is seeded; two languages have no pool.
ranger/fighting-style | feat-choice | grant_spell | pool | - | - | - | - | The feat Choice is seeded; Druidic Warrior picks two Druid cantrips instead.
ranger/roving | passive-modifier | modifier move_mode | - | - | - | passive | - | Speed +10 feet, Climb and Swim Speeds.
ranger/expertise | proficiency-grant | - | row | - | - | sheet | - | Two more Expertise skills at level 9; only the level 2 row is seeded.
ranger/tireless | heal-action | resource_change temp_hp value_source trigger remove_condition | - | row | - | - | - | Temporary Hit Points as a Magic action; one Exhaustion level less after a Short Rest.
ranger/relentless-hunter | spell-alteration | spell_change | - | - | - | - | - | Damage cannot break Concentration on Hunter's Mark.
ranger/natures-veil | activated-state | resource_change apply_condition | - | row | - | - | - | Invisible until the end of the next turn.
ranger/precise-hunter | passive-edge | edge edge_filter | - | - | - | - | - | Advantage against the creature marked by Hunter's Mark.
ranger/feral-senses | passive-sense | sense | - | - | - | passive | - | Blindsight 30 feet.
ranger/foe-slayer | spell-alteration | spell_change | - | - | - | - | - | Hunter's Mark deals d10s.
rogue/expertise | proficiency-grant | - | seeded | - | - | sheet | - | Choices at levels 1 and 6 are seeded.
rogue/sneak-attack | on-hit-rider | trigger damage branch_condition | - | - | - | - | - | Scale `sneak-attack` is seeded. Needs Advantage or an ally next to the target, and a Finesse or Ranged weapon.
rogue/thieves-cant | proficiency-grant | grant_proficiency | pool | - | - | - | - | Thieves' Cant and one more language.
rogue/cunning-action | action-economy | grant_action | - | - | - | - | - | Dash, Disengage or Hide as a Bonus Action.
rogue/steady-aim | activated-state | edge edge_filter immobile duration_rule branch_condition | - | - | - | - | - | Advantage on the next attack this turn; Speed 0 until the turn ends.
rogue/cunning-strike | on-hit-rider | trigger choice save_gate apply_condition move dc_source branch_condition | - | - | - | - | - | Poison, Trip or Withdraw, paid for in Sneak Attack dice.
rogue/uncanny-dodge | reaction | trigger damage_reduction | - | - | - | - | - | Halves one attack's damage.
rogue/evasion | passive-defence | damage_reduction branch_condition | - | - | - | passive | - | No damage on a successful Dexterity save, half on a failure.
rogue/reliable-talent | roll-rule | roll_adjust | - | - | - | passive | - | A d20 of 9 or lower counts as 10 on proficient checks.
rogue/improved-cunning-strike | on-hit-rider | trigger choice | - | - | - | - | - | Two Cunning Strike options on one hit, where `choice` picks one.
rogue/devious-strikes | on-hit-rider | trigger choice save_gate apply_condition restrict dc_source duration_rule | - | - | - | - | - | Daze, Knock Out and Obscure.
rogue/slippery-mind | proficiency-grant | grant_proficiency | - | - | - | passive | - | Wisdom and Charisma save proficiency; the sheet takes save proficiencies from the class only.
rogue/elusive | passive-edge | edge_filter branch_condition | - | - | - | passive | - | No attack has Advantage against the Rogue.
rogue/stroke-of-luck | roll-rescue | trigger roll_adjust resource_change | - | row | - | - | - | A failed D20 Test becomes a 20, once per Short Rest.
sorcerer/metamagic-options | option-pick | resource_change spell_change roll_adjust save_edge save_edge_filter | - | seeded | - | - | - | Ten options in one entry, each a change to one casting for Sorcery Points.
sorcerer/innate-sorcery | activated-state | resource_change modifier edge edge_filter | - | seeded | - | - | - | Spell save DC +1 and Advantage on spell attacks for a minute.
sorcerer/font-of-magic | resource-refill | resource_change value_source slot_change | - | seeded | - | - | - | Spell slots into Sorcery Points and back.
sorcerer/metamagic | option-pick | spell_change mode_from_choice | row | - | - | - | - | Two options at levels 2, 10 and 17; the seed lists levels 2 and 10 only.
sorcerer/sorcerous-restoration | resource-refill | trigger resource_change value_source | - | row | - | - | - | Half the Sorcerer level in points after a Short Rest, once per Long Rest.
sorcerer/sorcery-incarnate | activated-state | resource_change spell_change branch_condition | - | seeded | - | - | - | Innate Sorcery for 2 Sorcery Points; two Metamagic options per spell while it lasts.
sorcerer/arcane-apotheosis | spell-alteration | spell_change branch_condition | - | - | - | - | - | One free Metamagic option per turn during Innate Sorcery.
thief/thief_fast-hands | action-economy | grant_action | - | - | - | - | - | Sleight of Hand or Utilize as a Bonus Action.
thief/thief_second-story-work | passive-movement | move_mode modifier | - | - | - | passive | - | Climb Speed; jump distance from Dexterity.
thief/thief_supreme-sneak | on-hit-rider | trigger choice duration_rule branch_condition | - | - | - | - | - | A Cunning Strike option that keeps Hide's Invisible condition.
thief/thief_use-magic-device | item-rule | modifier trigger roll_adjust spell_change | - | - | - | - | - | Four attuned items, a chance to keep Charges, any Spell Scroll.
thief/thief_thiefs-reflexes | action-economy | turn_order | - | - | - | - | - | A second turn in the first round at Initiative minus 10.
warlock/eldritch-invocation-count | table-column | - | row | - | - | - | - | The number of invocations by level is not seeded as a Scale or a Choice count.
warlock/eldritch-invocation-options | option-pick | grant_spell spell_change sense save_edge save_edge_filter trigger damage heal slot_change apply_condition branch_condition move_mode detect create_item weapon_change grant_action move resistance mode_from_choice | pool | row | kind | - | - | 28 invocations in one entry. Fourteen only grant a spell. Prerequisites name a Warlock level, another invocation or a damaging cantrip.
warlock/eldritch-invocations | option-pick | mode_from_choice | pool | - | kind | - | - | Picks invocations by level, each with its own prerequisites, and swaps one per level.
warlock/magical-cunning | resource-refill | resource_change slot_change | - | seeded | - | - | - | Half the Pact Magic slots back after a one-minute rite.
warlock/contact-patron | granted-spell | grant_spell spell_change | - | row | - | - | - | Contact Other Plane without a slot once per Long Rest, saving automatically.
warlock/mystic-arcanum | granted-spell | grant_spell | pool | row | - | - | - | One spell each of levels 6 to 9, each once per Long Rest.
warlock/eldritch-master | resource-refill | slot_change | - | - | - | - | - | Magical Cunning returns every Pact Magic slot.
warrior-of-the-open-hand/warrior-of-the-open-hand_open-hand-technique | on-hit-rider | trigger choice restrict save_gate move apply_condition dc_source | - | - | - | - | - | Addle, Push or Topple on a Flurry of Blows hit.
warrior-of-the-open-hand/warrior-of-the-open-hand_wholeness-of-body | heal-action | resource_change heal | - | row | - | - | - | A Martial Arts die plus Wisdom modifier, Wisdom modifier times per Long Rest.
warrior-of-the-open-hand/warrior-of-the-open-hand_fleet-step | action-economy | trigger grant_action | - | - | - | - | - | Step of the Wind after any other Bonus Action.
warrior-of-the-open-hand/warrior-of-the-open-hand_quivering-palm | on-hit-rider | trigger resource_change save_gate damage dc_source duration_rule grant_action | - | seeded | - | - | - | Set on a hit for 4 Focus Points, ended later for 10d12 Force.
wizard/arcane-recovery | resource-refill | trigger slot_change | - | seeded | - | - | - | Slots back on a Short Rest within a budget of half the Wizard level.
wizard/ritual-adept | spell-knowledge | grant_spell | - | - | - | - | - | Ritual spells cast from the spellbook without preparing them.
wizard/scholar | proficiency-grant | - | seeded | - | - | sheet | - | The Expertise Choice is seeded; it does not limit the pick to the six listed skills.
wizard/memorize-spell | spell-knowledge | rest_rule | - | - | - | - | - | Swap one prepared spell after a Short Rest.
wizard/spell-mastery | granted-spell | grant_spell | pool | - | - | - | - | One level 1 and one level 2 spell at will.
wizard/signature-spells | granted-spell | grant_spell | pool | row | - | - | - | Two level 3 spells, each once per Short Rest without a slot.
feat/ability-score-improvement | ability-increase | - | - | - | seeded | sheet | - | Level-up applies the increases and their cap.
feat/alert | passive-modifier | modifier turn_order | - | - | - | passive | - | Proficiency Bonus on Initiative; swap Initiative with an ally.
feat/archery | passive-modifier | modifier | - | - | seeded | passive | - | +2 on attack rolls with Ranged weapons.
feat/boon-of-combat-prowess | roll-rescue | modifier trigger roll_adjust | row | - | seeded | - | - | One ability +1 to a cap of 30; a miss becomes a hit once per turn.
feat/boon-of-dimensional-travel | event-benefit | modifier trigger teleport | row | - | seeded | - | - | `teleport` exists; it has to fire after the Attack or Magic action.
feat/boon-of-fate | roll-rescue | modifier trigger roll_adjust resource_change | row | row | seeded | - | - | 2d4 on anyone's D20 Test within 60 feet; back on Initiative or a Rest.
feat/boon-of-irresistible-offense | on-hit-rider | modifier resistance trigger damage | row | - | seeded | - | - | Ignores Resistance to weapon damage types; extra damage on a natural 20.
feat/boon-of-spell-recall | resource-refill | modifier trigger slot_change | row | - | seeded | - | - | A 1d4 roll that keeps the slot.
feat/boon-of-the-night-spirit | activated-state | modifier apply_condition resistance branch_condition duration_rule | row | - | seeded | - | - | Invisible and Resistance in Dim Light or Darkness.
feat/boon-of-truesight | passive-sense | modifier sense | row | - | seeded | passive | - | Truesight 60 feet.
feat/defense | passive-modifier | modifier | - | - | seeded | passive | - | +1 AC in armor.
feat/grappler | attack-profile | modifier trigger grant_action edge edge_filter | row | - | seeded | - | - | Damage and Grapple on one Unarmed Strike; Advantage against a creature the character grapples.
feat/great-weapon-fighting | roll-rule | roll_adjust | - | - | seeded | passive | - | 1s and 2s on damage dice count as 3.
feat/magic-initiate | granted-spell | grant_spell | pool | row | - | - | - | The spell list Choice is seeded; the cantrips, the spell and the casting ability are not.
feat/savage-attacker | roll-rule | trigger roll_adjust | - | - | - | - | - | Roll weapon damage twice, once per turn.
feat/skilled | proficiency-grant | - | seeded | - | - | sheet | - | The Choice is seeded.
feat/two-weapon-fighting | passive-modifier | modifier | - | - | seeded | passive | - | Ability modifier on the Light weapon's extra attack.
species:dragonborn/Draconic Ancestry | option-pick | mode_from_choice | seeded | - | - | - | - | The pick sets the damage type of Breath Weapon and Damage Resistance.
species:dragonborn/Breath Weapon | save-action | resource_change area save_damage choice dc_source grant_action mode_from_choice | - | seeded | - | - | - | Cone or line, character-level dice, replaces one attack; DC 8 + Con + Proficiency Bonus.
species:dragonborn/Damage Resistance | passive-defence | resistance mode_from_choice | - | - | - | passive | - | Resistance to the ancestry's damage type.
species:dragonborn/Draconic Flight | activated-state | resource_change move_mode duration_rule | - | row | - | - | - | Fly Speed for 10 minutes, once per Long Rest.
species:dwarf/Dwarven Resilience | passive-defence | resistance save_edge save_edge_filter | - | - | - | passive | - | Poison Resistance; Advantage on saves against Poisoned.
species:dwarf/Dwarven Toughness | passive-modifier | modifier | - | - | - | passive | - | Hit Point maximum +1 per level.
species:dwarf/Stonecunning | activated-state | resource_change sense branch_condition | - | seeded | - | - | - | Tremorsense 60 feet for 10 minutes on stone.
species:elf/Elven Lineage | option-pick | grant_spell sense modifier mode_from_choice | pool | row | - | - | - | The lineage Choice is seeded; the casting ability and the High Elf's cantrip swap are not.
species:elf/Fey Ancestry | passive-edge | save_edge save_edge_filter | - | - | - | passive | - | Advantage on saves against Charmed.
species:elf/Keen Senses | proficiency-grant | - | row | - | - | sheet | - | One of three skills.
species:elf/Trance | rest-rule | rest_rule condition_immunity | - | - | - | passive | - | A four-hour Long Rest; magic cannot put the Elf to sleep.
species:gnome/Gnomish Cunning | passive-edge | save_edge | - | - | - | passive | - | Three `save_edge` components cover it.
species:gnome/Gnomish Lineage | option-pick | grant_spell create_item mode_from_choice | pool | row | - | - | yes | The lineage Choice is seeded. The Rock Gnome's clockwork device produces a free-form Prestidigitation effect.
species:goliath/Giant Ancestry | option-pick | resource_change teleport trigger damage speed_penalty apply_condition branch_condition damage_reduction duration_rule mode_from_choice | seeded | row | - | - | - | Six boons, Proficiency Bonus times per Long Rest. `teleport` covers Cloud's Jaunt.
species:goliath/Large Form | activated-state | resource_change size_change check_edge modifier | - | seeded | - | - | - | Large for 10 minutes: Advantage on Strength checks, Speed +10.
species:goliath/Powerful Build | passive-edge | check_edge modifier | - | - | - | passive | - | Advantage on checks to end Grappled; carrying capacity.
species:halfling/Brave | passive-edge | save_edge save_edge_filter | - | - | - | passive | - | Advantage on saves against Frightened.
species:halfling/Halfling Nimbleness | passive-movement | move_mode | - | - | - | passive | - | Through the space of larger creatures.
species:halfling/Luck | roll-rule | trigger roll_adjust | - | - | - | passive | - | Reroll a natural 1.
species:halfling/Naturally Stealthy | action-economy | grant_action | - | - | - | passive | - | Hide behind a larger creature.
species:human/Resourceful | event-benefit | trigger grant_inspiration | - | - | - | - | - | Heroic Inspiration after a Long Rest.
species:human/Skillful | proficiency-grant | - | seeded | - | - | sheet | - | The Choice is seeded.
species:human/Versatile | feat-choice | - | seeded | - | - | - | - | The Choice is seeded.
species:orc/Adrenaline Rush | action-economy | resource_change grant_action temp_hp value_source | - | seeded | - | - | - | Dash as a Bonus Action with Temporary Hit Points equal to the Proficiency Bonus.
species:orc/Relentless Endurance | zero-hp-ward | trigger resource_change heal | - | seeded | - | - | - | 1 Hit Point instead of 0, once per Long Rest.
species:tiefling/Fiendish Legacy | option-pick | resistance grant_spell mode_from_choice | pool | row | - | - | - | The legacy Choice is seeded; the casting ability is not.
species:tiefling/Otherworldly Presence | granted-spell | grant_spell | - | - | - | - | - | The Thaumaturgy cantrip.
"""

# Why each proposed exception is DM judgment by nature, and what stays automated.
EXCEPTIONS = {
    "druid/druidic": "Leaving a hidden message in Druidic and deciding who comes across it is narrative play with no state for the engine to hold. Speak with Animals always prepared and the language are automated.",
    "druid/wild-shape": "The Objects rule says the GM decides whether a piece of equipment is practical for the new form to wear. Everything else in Wild Shape is automated.",
    "species:gnome/Gnomish Lineage": "The Rock Gnome's clockwork device produces one Prestidigitation effect of the player's invention, a free-form sensory effect. The lineage's cantrips and spell are automated.",
}

# Seed rows that do not say what the SRD says. Reported, not fixed here.
ANOMALIES = [
    "`circle-of-the-land_natures-ward` carries the text of Nature's Sanctuary, and `circle-of-the-land_natures-sanctuary` repeats the text of Natural Recovery. The SRD text of Nature's Ward is not in the seed.",
    "`cleric/core-traits` is named Cleric Subclasses, and `druid/wild-shape-uses` is named Cantrips Known.",
    "`monk/deflect-attacks` and `bard/bard-subclass` have no level. `monk/stunning-strike` lists levels 3 and 5, and `monk/unarmored-movement` lists level 2 twice.",
    "`sorcerer/metamagic` lists levels 2 and 10; its text also grants options at level 17.",
    "The cantrip table in `druid/druid-spell-list` has lost its Spell column.",
    "`champion_remarkable-athlete` stops after the Advantage sentence. From memory of the SRD it also grants movement after a Critical Hit; that sentence is not in the seed and was not classified.",
    "Names carry typos: Studdied Attacks, Unarmoed Movement, Magic Cunning.",
]

# Keyword checks on hand rows: text that usually means a kind is needed. A warning is printed for
# every match the row does not cover, unless it is listed in REVIEWED with the reason it is left out.
LINT = [
    (r"\bResistance to\b", {"resistance"}),
    (r"Temporary Hit Points", {"temp_hp"}),
    (r"regains? (a number of |additional )?Hit Points|restore[s]? (a number of |a total number of )?Hit Points", {"heal", "spell_change"}),
    (r"Darkvision|Blindsight|Tremorsense|Truesight", {"sense"}),
    (r"\bteleport\b", {"teleport"}),
    (r"take a Reaction", {"trigger"}),
    (r"always have .* prepared|without expending a spell slot", {"grant_spell"}),
    (r"Fly Speed|Climb Speed|Swim Speed", {"move_mode"}),
    (r"Advantage on [A-Za-z, ]*saving throws", {"save_edge"}),
    (r"Advantage on (Initiative|[A-Za-z() ]*checks)", {"check_edge"}),
    (r"Speed increases|bonus to (Armor Class|attack rolls)|Hit Point maximum increases|scores? increase", {"modifier"}),
    (r"extra \d+d\d+ [A-Za-z, ]*damage|takes? \d+d\d+ [A-Za-z]+ damage", {"damage", "save_damage"}),
    (r"must (succeed on|make) an? [A-Za-z]+ saving throw", {"save_gate", "save_damage", "save_condition"}),
    (r"expend (one|a|1|3|4) (use|Focus Point)|spend \d Sorcery Point", {"resource_change"}),
    (r"Advantage on attack rolls|attack rolls? [a-z ]*ha(s|ve) (Advantage|Disadvantage)", {"edge", "edge_filter"}),
    (r"proficiency (in|with)", {"grant_proficiency"}),
    (r"Immunity to the", {"condition_immunity"}),
]


REVIEWED = {
    ("barbarian/primal-knowledge", "grant_proficiency"): "the skill comes from a Choice",
    ("bard/expertise", "grant_proficiency"): "Expertise comes from a Choice",
    ("rogue/expertise", "grant_proficiency"): "Expertise comes from a Choice",
    ("college-of-lore/bonus-proficiencies", "grant_proficiency"): "the skills come from a Choice",
    ("feat/skilled", "grant_proficiency"): "the skills come from a Choice",
    ("species:elf/Keen Senses", "grant_proficiency"): "the skill comes from a Choice",
    ("species:human/Skillful", "grant_proficiency"): "the skill comes from a Choice",
    ("warlock/eldritch-invocation-options", "grant_proficiency"): "Pact of the Blade's proficiency is weapon_change",
    ("warlock/eldritch-invocation-options", "temp_hp"): "Fiendish Vigor casts False Life, a spell",
    ("circle-of-the-land/circle-of-the-land_natures-sanctuary", "grant_spell"): "the seed text is another feature's",
    ("circle-of-the-land/circle-of-the-land_natures-ward", "resource_change"): "the seed text is another feature's",
    ("druid/wild-shape", "move_mode"): "Fly Speed limits which forms are eligible",
    ("life-domain/life-domain_supreme-healing", "heal"): "it changes how healing dice are read",
    ("paladin/restoring-touch", "heal"): "the text says these points do not heal",
    ("rogue/sneak-attack", "edge"): "Advantage is a condition of Sneak Attack, not something it gives",
}


def load():
    with gzip.open(SEED) as f:
        data = json.load(f)
    entries = []
    for c in data["classes"]:
        if c["document"] != DOCUMENT:
            continue
        parent = c.get("parent")
        owner = f"subclass {c['slug']} ({parent})" if parent else f"class {c['slug']}"
        for f_ in sorted(c["features"], key=lambda x: (min(x["levels"], default=0), x["slug"])):
            levels = sorted(set(f_["levels"]))
            entries.append({
                "key": f"{c['slug']}/{f_['slug']}", "owner": owner, "group": "subclass" if parent else "class",
                "slug": f_["slug"], "name": f_["name"], "text": f_["description"],
                "level": ", ".join(map(str, levels)) if levels else "-",
            })
    for f_ in data["feats"]:
        if f_["document"] != DOCUMENT:
            continue
        m = re.search(r"Level (\d+)\+", f_.get("prerequisite") or "")
        entries.append({
            "key": f"feat/{f_['slug']}", "owner": f"feat ({f_['type']})", "group": "feat", "slug": f_["slug"],
            "name": f_["name"], "text": " ".join(f_["benefits"]), "level": m.group(1) if m else "-",
        })
    for s in data["species"]:
        if s["document"] != DOCUMENT:
            continue
        for t in s["traits"]:
            late = re.search(r"character level 5, you can", t["description"])
            entries.append({
                "key": f"species:{s['slug']}/{t['name']}", "owner": f"species {s['slug']}", "group": "species",
                "slug": t["name"], "name": t["name"], "text": t["description"], "level": "5" if late else "1",
            })
    return entries


def row(pattern, kinds="", choice="-", resource="-", prereq="-", passive="-", exc="-", note=""):
    return {"pattern": pattern, "kinds": kinds.split(), "choice": choice, "resource": resource, "prereq": prereq,
            "passive": passive, "exc": exc, "note": note}


def by_rule(e):
    """Classifies the entries whose shape leaves no doubt; None for the rest."""
    slug, text, group = e["slug"], e["text"], e["group"]
    if text.strip() == "[Column data]":
        if slug.startswith("slots-"):
            return row("table-column", passive="sheet", note="Spell slots by level are in the rules' slot table.")
        if slug in COLUMNS:
            passive, resource, note = COLUMNS[slug]
            return row("table-column", passive=passive, resource=resource, note=note)
        return None
    if group == "class" and slug == "core-traits":
        kinds = "grant_proficiency" if "Tool Proficiencies" in text else ""
        note = "The class profile holds saves, skill count and weapon and armor training; the skills Choice is seeded."
        if kinds:
            note += " Tool proficiency has no home."
        return row("core-traits", kinds, choice="seeded", passive="sheet", note=note)
    if group == "class" and slug.endswith("spell-list"):
        return row("spell-list", note="Reference table; nothing to resolve.")
    if group == "class" and re.search(r"-subclass(es)?$", slug):
        return row("subclass-choice", choice="seeded", note="The subclass Choice at level 3 is seeded.")
    if group == "class" and slug in ("ability-score-improvement", "epic-boon"):
        return row("feat-choice", choice="seeded", note="The feat Choice is seeded for each level.")
    if group == "class" and slug in ("spellcasting", "pact-magic"):
        return row("spellcasting-progression", passive="sheet",
                   note="Cantrips, prepared spells and slots come from the class profile; preparation is checked by the rules.")
    if group == "class" and slug in ("extra-attack", "two-extra-attacks", "three-extra-attacks"):
        return row("extra-attack", passive="sheet", note="Scale `<class>-extra-attack` is seeded and read by live play.")
    if group == "class" and slug == "weapon-mastery":
        return row("weapon-mastery", choice="seeded", passive="sheet",
                   note="Choice and Scale are seeded; live play applies masteries to carried weapons.")
    if slug in SPELL_TABLES:
        return row("granted-spell", "grant_spell", note="Spells always prepared from a class level.")
    if group == "species" and slug == "Size":
        choice = "row" if "chosen when you select" in text else "-"
        return row("base-stat", choice=choice, note="Size; a pick between Medium and Small where the species offers one.")
    if group == "species" and slug == "Speed":
        return row("base-stat", passive="sheet", note="The sheet reads walking Speed from the species.")
    if group == "species" and slug == "Darkvision":
        return row("passive-sense", "sense", passive="passive",
                   note="A token's darkvision is entered when it is placed; nothing reads it from the species.")
    return None


def parse_hand():
    out = {}
    for line in HAND.strip().splitlines():
        parts = [p.strip() for p in line.split("|")]
        if len(parts) != 9:
            sys.exit(f"hand row has {len(parts)} fields, wants 9: {line[:60]}")
        key, pattern, kinds, choice, resource, prereq, passive, exc, note = parts
        if key in out:
            sys.exit(f"hand row twice: {key}")
        out[key] = row(pattern, "" if kinds == "-" else kinds, choice, resource, prereq, passive, exc, note)
    return out


def classify(entries):
    hand = parse_hand()
    keys = Counter(e["key"] for e in entries)
    doubles = [k for k, n in keys.items() if n > 1]
    if doubles:
        sys.exit(f"entries share a key: {doubles}")
    stray = sorted(set(hand) - set(keys))
    if stray:
        sys.exit(f"hand rows for entries the seed does not hold: {stray}")
    rows, missing = [], []
    for e in entries:
        if e["key"] in hand:
            r, src = hand[e["key"]], "hand"
        else:
            r, src = by_rule(e), "rule"
        if r is None:
            missing.append(e["key"])
            continue
        unknown = [k for k in r["kinds"] if k not in EXISTING and k not in GAPS]
        if unknown or r["pattern"] not in PATTERNS:
            sys.exit(f"{e['key']}: unknown kind or pattern {unknown} {r['pattern']}")
        allowed = {"choice": "- seeded row pool", "resource": "- seeded row rule", "prereq": "- seeded kind",
                   "passive": "- sheet passive", "exc": "- yes"}
        bad = [f for f, values in allowed.items() if r[f] not in values.split()]
        if bad:
            sys.exit(f"{e['key']}: unknown value in {bad}")
        r = dict(r, **e, src=src, gaps=[k for k in r["kinds"] if k in GAPS])
        rows.append(r)
    if missing:
        sys.exit("no classification for: " + ", ".join(missing))
    return rows


def lint(rows):
    for r in rows:
        if r["src"] != "hand":
            continue
        for pattern, kinds in LINT:
            if re.search(pattern, r["text"]) and not kinds & set(r["kinds"]):
                if any((r["key"], k) in REVIEWED for k in kinds):
                    continue
                print(f"lint: {r['key']}: text matches /{pattern}/ but no {sorted(kinds)}", file=sys.stderr)


def data_gap(r):
    return r["choice"] == "pool" or r["resource"] == "rule" or r["prereq"] == "kind"


def greedy(rows):
    """Orders the gaps so each step completes the most entries; ties go to the gap more entries need."""
    need = Counter(g for r in rows for g in r["gaps"])
    open_rows = [set(r["gaps"]) for r in rows if r["gaps"]]
    built, steps, done = set(), [], 0
    while len(built) < len(need):
        best = max((g for g in need if g not in built),
                   key=lambda g: (sum(1 for s in open_rows if s and s <= built | {g}), need[g], g))
        built.add(best)
        now = sum(1 for s in open_rows if s <= built)
        steps.append((best, now - done, now))
        done = now
    return steps


def cell(s):
    return str(s).replace("|", "/")


def render(rows):
    total = len(rows)
    groups = Counter(r["group"] for r in rows)
    need = Counter(g for r in rows for g in r["gaps"])
    sole = Counter(r["gaps"][0] for r in rows if len(r["gaps"]) == 1)
    used = Counter(k for r in rows for k in r["kinds"] if k in EXISTING)
    patterns = Counter(r["pattern"] for r in rows)
    clear = [r for r in rows if not r["gaps"]]
    no_effect = [r for r in clear if not r["kinds"]]
    with_effect = [r for r in clear if r["kinds"]]
    clear_data = [r for r in clear if data_gap(r)]
    clear_rows = [r for r in clear if not data_gap(r) and "row" in (r["choice"], r["resource"])]
    exceptions = [r for r in rows if r["exc"] == "yes"]
    blocked = [r for r in rows if r["gaps"]]

    out = []
    w = out.append
    w("# Census: Effect component kinds for SRD 5.2 Features, feats and species traits")
    w("")
    w("Research for [issue 238](https://github.com/JorisJonkers-dev/grimoire/issues/238), part of the map in "
      "[issue 236](https://github.com/JorisJonkers-dev/grimoire/issues/236). Generated by "
      "[`census-features.py`](census-features.py) from the Compendium seed; rerun the script after changing a "
      "classification instead of editing this file.")
    w("")
    w("## Answer")
    w("")
    w(f"The seed holds {total} entries for SRD 5.2: {groups['class']} class features, {groups['subclass']} subclass "
      f"features, {groups['feat']} feats and {groups['species']} species traits.")
    w("")
    w(f"- **{len(clear)} need no component kind the engine lacks.** {len(no_effect)} of them need no Effect at all: "
      f"table columns, spell lists, and Choices and Resources that are already data. {len(with_effect)} "
      f"{'needs' if len(with_effect) == 1 else 'need'} an Effect that today's kinds can express "
      f"({', '.join('`' + r['key'] + '`' for r in with_effect)}).")
    w(f"- **{len(blocked)} need at least one kind that is missing**, {sum(1 for r in blocked if len(r['gaps']) == 1)} "
      "of them exactly one.")
    w(f"- **{len(NEW)} new component kinds and {len(WIDEN)} widenings** cover every entry. `trigger` is needed most "
      f"({need['trigger']} entries); `{sole.most_common(1)[0][0]}` is the sole blocker most often "
      f"({sole.most_common(1)[0][1]} entries).")
    w(f"- **{len(exceptions)} entries are proposed for the Manual exception list**, each for one clause; the rest of "
      "each entry is automated.")
    w("")
    w("## How the engine stands today")
    w("")
    w("- **Component kinds.** The sealed union in `api/internal/rules/effects` (`effects.go`, `shape.go`, `light.go`) "
      f"has {len(EXISTING)} types. The newest `effect_components_kind_check`, in "
      "`api/db/migrations/00077_visibility.sql`, lists 29 row kinds: 28 of those types plus `mode`, the row kind of a "
      "`choice`'s options. `area_save` and `light` are in the union but have no catalogue row; "
      "`api/internal/compendium/pgstore/effects.go` leaves them to a spell's design.")
    w("- **Feature data.** `api/internal/rules/features` resolves Resources (maximum by table, class level, ability "
      "or Proficiency Bonus; a die by level; recharge on a Short Rest, Long Rest, dawn, Initiative or a roll), Scales, "
      "Choices (pools: feat category, subclass, skill, expertise, weapon, listed) and Prerequisites (level, ability, "
      "spellcasting, feat, feature). `00051_seed_srd_feature_mechanics.sql` seeds 20 Resources, 10 Scales, the class "
      "Choices and every feat Prerequisite; `00059_attacks_left.sql` adds the five Extra Attack Scales.")
    w("- **What reads it.** Level-up reads Choices and Prerequisites; Rests read Resources; live play reads the Extra "
      "Attack and Weapon Mastery Scales (`api/internal/play/pgstore/statblocks.go`). The Scales `sneak-attack`, "
      "`martial-arts`, `rage-damage`, `unarmored-movement` and `brutal-strike` are seeded and nothing reads them.")
    w("- **The sheet.** `api/internal/rules/sheet.go` derives Proficiency Bonus, Armor Class from armor, Dexterity "
      "and a shield, Initiative from Dexterity, Speed from the species, saves from the class, skills with Expertise, "
      "Hit Dice and spell slots. It has no input for a Feature.")
    w("- **Effects owned by Features.** Two are seeded: `dodging` (`00057_actions.sql`) and `wild-shape` "
      "(`00075_forms.sql`). The Dodge action puts `dodging` on by its slug (`api/internal/play/live/actions.go`), and "
      "`wild-shape` is applied by a command that names the slug. Nothing links a Feature to the Effect it applies, "
      "to how it is activated, or to the Resource it costs.")
    w("- **Limits that shape the gaps.** Live play takes every save DC from the caster's spell save DC "
      "(`api/internal/play/live/area.go`). A landed `grant_feature` becomes a note for the DM "
      "(`api/internal/play/live/effects.go`). Nothing under `api/internal/play` applies damage Resistance, Immunity "
      "or Vulnerability when damage is dealt; the only match for those words there is Legendary Resistance.")
    w("")
    w("## Method")
    w("")
    by_rule_n = sum(1 for r in rows if r["src"] == "rule")
    w(f"The script classifies {by_rule_n} entries by rule: table columns, core traits, spell lists, the subclass and "
      "feat Choices, Spellcasting, Extra Attack, Weapon Mastery, always-prepared spell tables, and species Size, Speed "
      f"and Darkvision. The other {total - by_rule_n} were read in full and classified by hand; those rows are in the "
      "script's `HAND` table, and the `Src` column says which is which. The script stops if any entry has no row.")
    w("")
    w("Rules used for every row:")
    w("")
    w("- **Kinds needed** lists what full automation of the entry's text takes, existing kinds and missing ones alike. "
      "**Missing today** is the part of that list the engine lacks.")
    w("- An entry that upgrades another Feature lists the kinds its upgraded behaviour runs on, so it is not counted "
      "as ready while its base is blocked.")
    w("- A spell a Feature grants is counted as `grant_spell` only. What the spell does belongs to the spells census.")
    w("- An entry that holds many options (Metamagic Options, Eldritch Invocation Options, Giant Ancestry) is one row "
      "with the union of its options' kinds.")
    w("- **Choice**: `seeded` (the row exists), `row` (the pool exists, the row does not), `pool` (it picks from "
      "something no pool covers: spells, cantrips, languages, Beast forms, options with prerequisites).")
    w("- **Resource**: `seeded`, `row` (the schema fits, the row does not exist), `rule` (it needs a counter or "
      "lockout the schema cannot hold).")
    w("- **Prereq**: `seeded`, or `kind` (it needs a prerequisite kind the schema lacks).")
    w("- **Passive**: `sheet` for a passive value the sheet or live play already computes, `passive` for one it does "
      "not, `-` for anything that is not a passive modifier.")
    w("- **Exception** is yes only where the outcome is the DM's judgment by nature. An entry that is merely hard is "
      "a gap.")
    w("")
    w("## Table")
    w("")
    w("| Owner | Entry | Level | Pattern | Kinds needed | Missing today | Choice | Resource | Prereq | Passive | Exception | Notes | Src |")
    w("|---|---|---|---|---|---|---|---|---|---|---|---|---|")
    for r in rows:
        entry = f"`{r['slug']}`" if r["group"] != "species" else r["name"]
        if r["group"] != "species" and r["name"]:
            entry += f" {r['name']}"
        kinds = " ".join(r["kinds"]) or "-"
        gaps = " ".join(r["gaps"]) or "-"
        w("| " + " | ".join(cell(x) for x in (r["owner"], entry, r["level"], r["pattern"], kinds, gaps, r["choice"],
                                              r["resource"], r["prereq"], r["passive"], r["exc"], r["note"],
                                              r["src"])) + " |")
    w("")
    w("### Missing kinds")
    w("")
    w("New component kinds:")
    w("")
    for k, text in NEW.items():
        w(f"- **`{k}`**: {text}")
    w("")
    w("Widenings of kinds and data the engine already has:")
    w("")
    for k, text in WIDEN.items():
        w(f"- **`{k}`**: {text}")
    w("")
    w("### Patterns")
    w("")
    for k, text in PATTERNS.items():
        w(f"- **{k}**: {text}")
    w("")
    w("## Summary")
    w("")
    w("### Count per pattern")
    w("")
    w("| Pattern | Entries | Blocked by a missing kind |")
    w("|---|---|---|")
    for p, n in sorted(patterns.items(), key=lambda x: (-x[1], x[0])):
        w(f"| {p} | {n} | {sum(1 for r in rows if r['pattern'] == p and r['gaps'])} |")
    w(f"| **Total** | **{total}** | **{len(blocked)}** |")
    w("")
    w("### Missing kinds ranked by entries unlocked")
    w("")
    w("*Needed by* counts the entries that list the kind. *Sole blocker* counts the entries for which it is the "
      "only missing kind. *Data ready* is the part of those whose Choice, Resource and Prerequisite data fits today's "
      "schema, so the kind alone finishes them. Ranked by needed-by.")
    w("")
    w("| Rank | Kind | New or widening | Needed by | Sole blocker | Data ready |")
    w("|---|---|---|---|---|---|")
    ready = Counter(r["gaps"][0] for r in rows if len(r["gaps"]) == 1 and not data_gap(r))
    for i, (k, n) in enumerate(sorted(need.items(), key=lambda x: (-x[1], -sole[x[0]], x[0])), 1):
        w(f"| {i} | `{k}` | {'new' if k in NEW else 'widening'} | {n} | {sole[k]} | {ready[k]} |")
    unused = [k for k in GAPS if k not in need]
    if unused:
        sys.exit(f"gaps no row uses: {unused}")
    w("")
    w("A build order that finishes the most entries at each step, breaking ties by needed-by. It counts kinds only; "
      f"{sum(1 for r in blocked if data_gap(r))} of the {len(blocked)} blocked entries also wait on Feature data the "
      "schema cannot hold yet.")
    w("")
    w("| Step | Kind | Entries finished by this step | Running total |")
    w("|---|---|---|---|")
    for i, (k, gained, now) in enumerate(greedy(rows), 1):
        w(f"| {i} | `{k}` | {gained} | {now} of {len(blocked)} |")
    w("")
    w("### Automatable with today's kinds")
    w("")
    w(f"{len(clear)} of {total} entries need no missing kind.")
    w("")
    w(f"- {len(no_effect)} need no Effect: {sum(1 for r in no_effect if r['pattern'] == 'table-column')} table columns, "
      f"{sum(1 for r in no_effect if r['pattern'] == 'spell-list')} spell lists, and "
      f"{sum(1 for r in no_effect if r['pattern'] not in ('table-column', 'spell-list'))} entries that are Choices, "
      "Scales or sheet values.")
    w(f"- {len(with_effect)} {'needs' if len(with_effect) == 1 else 'need'} an Effect built from existing kinds: "
      + ", ".join(f"`{r['key']}` ({' '.join(r['kinds'])})" for r in with_effect) + ".")
    w(f"- {len(clear_rows)} of the {len(clear)} still need a Choice or Resource row that fits today's schema: "
      + ", ".join(f"`{r['key']}`" for r in clear_rows) + ".")
    if clear_data:
        w(f"- {len(clear_data)} of the {len(clear)} need Feature data the schema cannot hold yet: "
          + ", ".join(f"`{r['key']}`" for r in clear_data) + ".")
    w("")
    w("Existing kinds the blocked and ready entries would use, by entries:")
    w("")
    w(", ".join(f"`{k}` {n}" for k, n in sorted(used.items(), key=lambda x: (-x[1], x[0]))) + ".")
    w("")
    never = sorted(k for k in EXISTING if k not in used)
    w("Existing kinds no entry here needs: " + ", ".join(f"`{k}`" for k in never) + ".")
    w("")
    w("### Feature data")
    w("")
    for field, label in (("choice", "Choice"), ("resource", "Resource"), ("prereq", "Prerequisite")):
        c = Counter(r[field] for r in rows if r[field] != "-")
        w(f"- **{label}**: " + ", ".join(f"{n} {k}" for k, n in sorted(c.items(), key=lambda x: -x[1]))
          + f" ({sum(c.values())} entries).")
    c = Counter(r["passive"] for r in rows if r["passive"] != "-")
    w(f"- **Passive values**: {c['sheet']} entries are values the sheet, level-up or live play already computes, most "
      f"of them table columns. {c['passive']} are passive modifiers nothing computes yet.")
    w("")
    w("### Proposed Manual exceptions")
    w("")
    w("| Entry | Clause left to the DM | Why it is judgment by nature |")
    w("|---|---|---|")
    clause = {"druid/druidic": "Hidden messages", "druid/wild-shape": "Objects", "species:gnome/Gnomish Lineage": "Rock Gnome's clockwork device"}
    for r in exceptions:
        w(f"| `{r['key']}` | {clause[r['key']]} | {EXCEPTIONS[r['key']]} |")
    if {r["key"] for r in exceptions} != set(EXCEPTIONS):
        sys.exit("exception reasons and rows differ")
    w("")
    w("Considered and left as gaps or as text with no outcome:")
    w("")
    w("- **Turn Undead** (`cleric/channel-divinity`). A turned creature moving away is the DM moving a monster, as on "
      "any turn; the Effect can mark it.")
    w("- **Divine Sense** (`paladin/channel-divinity`). The engine knows creature types and positions; consecrated "
      "ground is knowable once Hallow is an Effect on the Map.")
    w("- **Hunter's Lore**. The engine holds the marked creature's Resistances.")
    w("- **Divine Intervention, Greater Divine Intervention, Contact Patron**. Each casts a spell; whether Wish or "
      "Contact Other Plane is an exception is for the spells census.")
    w("- **Archdruid's Longevity, Self-Restoration's food and drink, Thieves' Cant**. Text with nothing to resolve in "
      "play; no Manual Effect is needed.")
    w("- **Naturally Stealthy, Supreme Sneak, Fast Hands**. Rules for Hide and Sleight of Hand; the engine has both.")
    w("")
    w("## Findings beyond the table")
    w("")
    w("- **A Feature has no link to its Effect.** Passive Features need their Effect on the Character without anyone "
      "applying it, and activated ones need an activation (action, Bonus Action, Reaction, Magic action, no action) "
      "and a Resource cost. None of that is data today, so every row with a kind in it waits on that link as well as "
      "on its kinds. This is the open question the map lists for Design files.")
    top = [k for k, _ in need.most_common(3)]
    w(f"- **Three kinds carry most of the weight.** `{top[0]}`, `{top[1]}` and `{top[2]}` are the top three by "
      f"needed-by. `trigger` finishes only {sole['trigger']} entries alone, because what it fires is usually missing "
      "too.")
    cheap = Counter(r["gaps"][0] for r in rows if r["passive"] == "passive" and len(r["gaps"]) == 1)
    w("- **Passive entries are the cheap wins.** They need no activation and no Resource. Sole blockers among them: "
      + ", ".join(f"`{k}` {n}" for k, n in cheap.most_common()) + ".")
    w("- **Four existing kinds come close and do not fit.** `reacts` fires on being damaged only and hands the outcome "
      "to the DM; `extra_damage` sits on the target and counts for one source; `bonus_die` adds to every roll while "
      "it lasts; `forced_move` needs an area save. Features need the general forms: `trigger`, `damage`, "
      "`roll_adjust` and `move`.")
    w("- **Spells from Features are one kind.** `grant_spell` covers the always-prepared tables, the free casts and "
      "most Eldritch Invocations; the Choice pool for spells is the matching data gap.")
    w("- **Choices need more pools than kinds.** The entries marked `pool` pick spells, cantrips, languages, Beast "
      "forms or options with their own prerequisites.")
    w("- **Seeded Scales have no reader.** Sneak Attack, Martial Arts, Rage Damage, Unarmored Movement and Brutal "
      "Strike dice are data already; the kinds that would read them (`damage`, `modifier`, `weapon_change`) are missing.")
    w("")
    w("## Seed anomalies")
    w("")
    w("Found while reading; not fixed here.")
    w("")
    for a in ANOMALIES:
        w(f"- {a}")
    w("")
    w("## Not verified")
    w("")
    w("- Classifications are from the seed text and the engine source, not from running the app.")
    w("- The two Circle of the Land rows whose seed text is wrong were classified from the SRD rule their slug names, "
      "from memory of the SRD rather than from a file in this repository.")
    w("- Whether Ritual casting from a spellbook (`wizard/ritual-adept`) is already handled by the spellcasting rules "
      "was not traced; the row assumes it is not.")
    w("- Spellcasting rows: replacing a cantrip at level-up and the Spellcasting Focus were not traced. Weapon "
      "Mastery rows: changing a mastered weapon after a Long Rest was not traced. Core traits rows: starting "
      "equipment was not traced.")
    w("- How a Choice from the `skill` pool limited to a few skills (Keen Senses, Primal Knowledge, Scholar) is offered "
      "was not traced; the rows assume a `listed` or `skill` row is enough.")
    w("- The gap names are proposals. Two gaps may turn out to be one component, or one gap two, when they are designed.")
    w("")
    w("## Sources")
    w("")
    w("- `api/db/seeds/compendium.json.gz`, keys `classes`, `feats`, `species`, document `srd-2024`.")
    w("- `api/internal/rules/effects/effects.go`, `shape.go`, `light.go`, `text.go`: the component union.")
    w("- `api/internal/rules/features/features.go`: Resources, Scales, Choices, Prerequisites.")
    w("- `api/internal/rules/class.go`, `sheet.go`: the class profile and the sheet.")
    w("- `api/internal/rules/featbuild`, `speciesbuild`, `subclassbuild`, `classbuild`: what Homebrew builders hold.")
    w("- `api/db/migrations/00044_effect_definitions.sql`, `00050_feature_mechanics.sql`, "
      "`00051_seed_srd_feature_mechanics.sql`, `00057_actions.sql`, `00059_attacks_left.sql`, `00075_forms.sql`, "
      "`00077_visibility.sql`.")
    w("- `api/internal/play/live/effects.go`, `area.go`, `actions.go`, `rest.go`; "
      "`api/internal/play/pgstore/statblocks.go`; `api/internal/campaign/app/levelup.go`; "
      "`api/internal/compendium/pgstore/effects.go`.")
    w("- `ARCHITECTURE.md` section 9; `CONTEXT.md`.")
    w("")
    return "\n".join(out)


def main():
    rows = classify(load())
    lint(rows)
    OUT.write_text(render(rows), encoding="utf-8")
    print(f"{len(rows)} rows -> {OUT.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
