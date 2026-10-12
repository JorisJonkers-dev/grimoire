#!/usr/bin/env python3
"""Census of the SRD 5.2 items in the Compendium seed: which Item Properties and Effect component
kinds each one needs for Automation Level full.

Run from the repository root:

    python3 docs/research/census-items.py

It reads api/db/seeds/compendium.json.gz, groups templated rows, checks that every one of the seed's
SRD 5.2 items lands in exactly one template or one exclusion, and writes census-items.md's generated
sections (between the BEGIN/END markers) and census-items.csv next to this file.

The classification itself is hand-authored below (TEMPLATES and EXCLUDED). The script's own first-pass
guess is kept only as a cross-check: it reports where its guess and the hand classification disagree.
"""

from __future__ import annotations

import csv
import gzip
import json
import re
import sys
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SEED = ROOT / "api/db/seeds/compendium.json.gz"
CHARGES_MIGRATION = ROOT / "api/db/migrations/00117_item_charges.sql"
OUT_MD = Path(__file__).with_name("census-items.md")
OUT_CSV = Path(__file__).with_name("census-items.csv")
DOCUMENT = "srd-2024"

# Effect component kinds the engine has today: the kinds in the newest effect_components_kind_check
# (api/db/migrations/00077_visibility.sql; "mode" is a row of a choice), plus area_save and light,
# which exist in the Go union (api/internal/rules/effects/light.go) but only inside a spell Design.
HAVE = {
    "bonus_die", "edge", "extra_damage", "move_cost", "manual", "area", "save_damage", "save_condition",
    "create_surface", "incapacitated", "immobile", "save_edge", "crit_within", "exhausting", "speed_penalty",
    "reacts", "temp_hp", "teleport", "forced_move", "dispel", "counter", "grant_feature", "resource_change",
    "choice", "branch", "summon", "form", "reveal", "area_save", "light",
}

# Item Property kinds and Design fields of the Homebrew item builder (api/internal/rules/itembuild).
PROPS = {
    "enchantment", "weapon", "skill_boost", "bonus", "resistance", "extra_damage", "sense", "speed", "cantrip",
    "spell", "light", "consumable", "curse", "sentient", "growth", "set_bonus", "container", "firearm", "trigger",
    "manual",
}

# Effect component kinds that do not exist today, with the definition printed below the table.
MISSING = {
    "modifier": "A flat bonus or penalty while the Effect is active: to attack rolls, damage rolls, Armor Class, saving throws, ability checks, spell attack rolls, spell save DC, Initiative, Hit Point maximum or Proficiency Bonus. It can be scoped: attacks with this item only, ranged attacks only, one creature type, only while unarmored. ARCHITECTURE.md §9.3 names it `Modifier`; today the engine has bonus dice (`bonus_die`) and a penalty from exhaustion, but no flat bonus.",
    "resistance": "Resistance, Immunity or Vulnerability to one or more damage types, or ignoring a target's Resistance. The species and monster builders record resistances as data; no Effect grants one, and nothing read for this census applies one to damage in play.",
    "damage": "Damage dice or a flat amount of one type dealt to one target outside an area save: an on-hit rider, the result of an attack roll, or automatic damage. `save_damage` only works on an Area behind a save, and `extra_damage` is dice the Effect's source adds against the bearer (Hunter's Mark), with no damage type.",
    "trigger": "A typed event that fires components: a hit with this item, a natural 20 on the attack roll, taking damage, failing a save, entering or starting or ending a turn somewhere, dawn, or a fixed interval. `reacts` exists but is a reaction the DM resolves from free text. ARCHITECTURE.md §9.3 names it `Trigger`.",
    "save_gate": "A saving throw with its own DC source: a fixed number, 8 + an ability modifier + Proficiency Bonus, or the wielder's spell save DC, for one target or an Area. Today `area_save`, `save_damage` and `save_condition` always take the caster token's spell save DC (13 when the stat block has none) and need an Area. ARCHITECTURE.md §9.3 names it `SaveGate`.",
    "attack_gate": "An attack roll the Effect makes itself, with a fixed bonus or the wielder's ability modifier and Proficiency Bonus, gating the components behind it. ARCHITECTURE.md §9.3 names it `AttackGate`.",
    "heal": "Restores Hit Points: dice and a bonus, once or on a schedule. `temp_hp` exists; healing does not. Play heals for `potion-of-healing` through a rule keyed on the slug, not through an Effect. ARCHITECTURE.md §9.3 names it `Heal`.",
    "apply_condition": "Puts a condition on a target for a duration without an area save: on the user (Invisible), on a hit, or behind a save_gate or attack_gate. `save_condition` only works on an Area behind the caster's save DC. ARCHITECTURE.md §9.3 names it `ApplyCondition`.",
    "sense": "Grants a Sense with a range: Darkvision, Truesight, Blindsight. Tokens carry Senses and the item builder has a `sense` row, but no Effect grants one. ARCHITECTURE.md §9.3 names it `Sense`.",
    "speed": "Grants or changes a Speed: a Fly, Swim or Climb Speed of a number of feet or equal to the bearer's Speed, a doubled Speed, a floor, or added feet. `speed_penalty` only subtracts.",
    "ability_score": "Sets an ability score to a floor (19, 21 to 29) or raises it with a cap, while worn or permanently.",
    "check_edge": "Advantage or Disadvantage on ability checks of one skill or ability, or on Initiative, including Disadvantage imposed on other creatures' checks against the bearer. `edge` covers attack rolls and `save_edge` saving throws; nothing covers ability checks. The builder's Skill Boost row has no Effect behind it.",
    "edge_scope": "A filter on `edge`, `save_edge` and `bonus_die`: only against one creature type, only against spells and other magical effects, only saves to avoid or end one condition, only ranged, spell or opportunity attacks, only with other weapons. Today an edge applies to every attack and a save_edge to every save of one ability.",
    "proficiency": "Grants proficiency or Expertise with a weapon, an armor category, a skill or a tool, or a language the bearer knows.",
    "immunity": "Immunity to a condition, or to a named hazard or class of effect: Poisoned, magical Paralysis or Restraint, falling damage, Difficult Terrain, suffocating under water, extreme cold, Divination targeting, thought reading.",
    "outcome_change": "Changes the result of a d20 roll after it is known: a failed save becomes a success, a failed D20 Test is rerolled, a Death Saving Throw of 9 or lower counts as 10, a Critical Hit becomes a normal hit.",
    "damage_reduction": "Reduces damage the bearer is about to take by dice or an amount, usually as a Reaction or against one damage type.",
    "create_object": "Places an object, structure, barrier or vehicle with its own statistics (AC, Hit Points, size, what it blocks) on the Map or in an Inventory. ARCHITECTURE.md §9.3 names it `CreateObject`.",
    "banish": "Removes a creature from the Map into another plane or an extradimensional space until it is released, returns or escapes.",
    "compel": "Forces behaviour on a creature: it must flee, must attack the nearest creature, obeys the user's commands, or the user decides its move and action.",
    "slay": "The target dies or is destroyed outright, usually behind a save or below a Hit Point threshold.",
    "escape_check": "The Effect ends when the target, or a creature helping it, takes an action and succeeds on an ability check against a set DC, or when the binding object (AC, Hit Points) is destroyed. A Duration today can only repeat a save.",
    "spell_store": "Stores the levels of a spell cast into the item, to be cast later by the bearer or turned into spell slots.",
    "reflect_spell": "Absorbs a spell that targets only the bearer, or turns it back on its caster. `counter` only cancels a spell cast within a range.",
    "stat_override": "Overrides a base statistic of the item or its bearer: a weapon's damage type or properties (Thrown, Finesse, returns to the hand), the Unarmed Strike die, an armor's Stealth Disadvantage or Strength requirement, the base Armor Class formula.",
    "roll_table": "Rolls a die and resolves the matching row's components, including a flat percentage chance. Roll Table results can already own Effects (`roll_table_result`), but no component rolls on a table from inside an Effect.",
    "stabilize": "Makes a creature at 0 Hit Points Stable without a check.",
    "note": "A described outcome with nothing to compute, shown to the table and written to the Action Log without waiting on the DM. A `manual` component blocks on a DM prompt and makes the entry less than full, so descriptive outcomes need a kind of their own, or a ruling that they are not Manual Effects.",
}

# Item mechanics that are not Effect components but that the entry cannot be authored without.
GAPS = {
    "spell-options": "A granted spell with options the builder's `spell` row lacks: a fixed save DC or attack bonus, a fixed spell level, extra Charges to cast at a higher level, a restriction on targets or modes (self only, Beasts only), its own duration or no Concentration, or a different casting time. Play only checks the grant and spends the Charges; the spell then resolves with the caster's own numbers.",
    "use-limits": "Several independent limited uses on one item, or a recharge schedule other than dawn, a Long Rest or a Short Rest: after 1 hour, after a number of days, at dusk, a rolled number of hours, or a running total of minutes. An item holds one Charges pool today.",
    "depletion": "What happens when the last charge or use is spent: roll 1d20 and destroy the item on a 1, the item becomes nonmagical, a lifetime total, or a rolled number of starting Charges.",
    "variant": "A choice fixed when the Item Instance is made: its damage type, creature type, plane, colour, size or spell. Several seed rows bundle all their variants in one description.",
    "curse-lock": "A curse the rules enforce: the item cannot be unattuned or removed until Remove Curse, and the curse stays hidden until the item is identified or attuned. The builder's `curse` row is text with a cannot-drop flag that play does not read.",
    "utilize-check": "An item action that calls for an ability check against a set DC and reports the outcome, as tools do with Utilize.",
    "charge-scaling": "An Effect that scales with the number of Charges spent on one use (1 to 3 Charges for more damage). Scaling today follows a spell slot, a level or a class table column.",
    "focus": "The item counts as a Spellcasting Focus for some classes, or supplies free Material Components.",
}

PATTERNS = {
    "flat-bonus": "A passive numeric bonus and nothing else.",
    "weapon-rider": "A magic weapon with a property beyond a flat bonus: extra damage, a rider on a hit or on a natural 20, a toggle.",
    "passive-ward": "A passive defence: Resistance, Immunity, Advantage on saves, a changed outcome.",
    "passive-trait": "A passive trait: a Sense, a Speed, an ability score, a skill or a proficiency.",
    "grants-spell": "Only lets the bearer cast spells, at will, for Charges or once per day.",
    "spell-plus": "Grants spells and has other properties of its own.",
    "activated-effect": "An action, often for Charges or once per day, that resolves an Effect of the item's own.",
    "consumable-effect": "Used up as it resolves an Effect: potions, oils, dusts, thrown vials, special ammunition.",
    "summon": "Brings a creature in under the user's control or beside them.",
    "restraint": "Binds a creature until it escapes with a check.",
    "hazard-area": "Leaves a hazard on the ground that creatures entering it must deal with.",
    "light-source": "Sheds light.",
    "container": "Storage, mostly extradimensional.",
    "object-or-vehicle": "Becomes or creates a vehicle, a structure or another object with its own statistics.",
    "utility-object": "A magic tool whose outcome is mostly a described fact.",
    "planar-utility": "Moves creatures between planes or traps them outside the Map.",
    "spell-resource": "Stores spells or restores spell slots.",
    "random-table": "The outcome is rolled on the item's own table.",
    "permanent-boost": "Changes a Character permanently after downtime.",
    "situational-check": "Advantage or a bonus on an ability check when the item applies.",
    "tool-check": "A tool or piece of gear whose use is an ability check against a set DC.",
    "spellcasting-focus": "A Spellcasting Focus or a Component Pouch.",
}


@dataclass
class Template:
    key: str
    name: str
    match: object
    use: str
    props: str
    have: str
    miss: str
    gaps: str
    pattern: str
    spells: tuple = ()
    only_spell: bool = False
    flags: str = ""
    exception: str = ""
    oneoff: str = ""
    note: str = ""
    cat: str = ""
    monsters: tuple = ()
    rows: list = field(default_factory=list)

    def words(self, attr):
        return [w for w in getattr(self, attr).split() if w]

    def clean(self):
        return not self.miss and not self.gaps and not self.oneoff


def rx(pattern):
    return re.compile(pattern)


def slugs(text):
    return set(text.split())


TEMPLATES: list[Template] = []


def T(key, name, match, use, props, have, miss, gaps, pattern, **kw):
    TEMPLATES.append(Template(key, name, match, use, props, have, miss, gaps, pattern, **kw))


# --- Magic weapons -------------------------------------------------------------------------------
T("weapon-plus", "Weapon, +1, +2 or +3", rx(r"^(?!ammunition-).+-plus-[123]$"), "worn", "enchantment", "", "modifier", "", "flat-bonus", cat="weapon")
T("ammunition-plus", "Ammunition, +1, +2 or +3", rx(r"^ammunition-plus-"), "consumed", "enchantment consumable", "", "modifier", "", "flat-bonus", flags="Co", note="Nonmagical once it hits.")
T("defender", "Defender", rx(r"^defender(-|$)"), "worn", "enchantment", "choice", "modifier", "", "weapon-rider", note="Each turn the bonus is split between attack and damage rolls and Armor Class.")
T("giant-slayer", "Giant Slayer", rx(r"^giant-slayer-"), "worn", "enchantment extra_damage", "branch", "modifier trigger damage save_gate apply_condition", "", "weapon-rider")
T("holy-avenger", "Holy Avenger", rx(r"^holy-avenger(-|$)"), "worn", "enchantment extra_damage", "branch area save_edge", "modifier trigger damage edge_scope", "", "weapon-rider", note="The Emanation grows at Paladin level 17: Scaling by class level exists.")
T("nine-lives-stealer", "Nine Lives Stealer", rx(r"^nine-lives-stealer-"), "worn", "enchantment", "branch", "modifier trigger save_gate slay", "depletion", "weapon-rider", flags="Ch", note="1d8 + 1 Charges that never return.")
T("vicious", "Vicious Weapon", rx(r"^vicious-"), "worn", "extra_damage", "", "trigger damage", "", "weapon-rider")
T("weapon-of-warning", "Weapon of Warning", rx(r"^weapon-of-warning-"), "worn", "", "area", "check_edge note", "", "passive-trait", note="Waking sleepers at the start of combat is a described outcome.")
T("flame-tongue", "Flame Tongue", rx(r"^flame-tongue-"), "worn action", "extra_damage light", "light choice", "trigger damage", "", "weapon-rider")
T("ammunition-of-slaying", "Ammunition of Slaying", rx(r"^ammunition-of-.+-slaying$"), "consumed", "consumable", "branch", "trigger damage save_gate", "", "consumable-effect", flags="Co", note="The seed has one row per creature type, so no variant is needed.")
T("luck-blade", "Luck Blade", rx(r"^luck-blade-"), "worn action", "enchantment bonus spell", "", "modifier outcome_change", "use-limits depletion", "weapon-rider", spells=("Wish",), flags="Ch", note="1d3 Charges that never return; Luck and Wish each once per dawn.")
T("frost-brand", "Frost Brand", rx(r"^frost-brand-"), "worn", "extra_damage resistance light", "light", "trigger damage resistance note", "use-limits", "weapon-rider", note="Extinguishing flames, once per hour, is a described outcome.")
T("life-stealing", "Sword of Life Stealing", rx(r"-of-life-stealing$"), "worn", "", "temp_hp branch", "trigger damage", "", "weapon-rider")
T("wounding", "Sword of Wounding", rx(r"-of-wounding$"), "worn", "extra_damage", "", "trigger damage save_gate", "", "weapon-rider", oneoff="the target cannot regain Hit Points")
T("dancing", "Dancing Sword", rx(r"^dancing-"), "action", "", "grant_feature", "", "", "weapon-rider", oneoff="a hovering weapon that attacks by itself with the wielder's attack roll, four times, then returns")
T("sharpness", "Sword of Sharpness", rx(r"-of-sharpness$"), "worn", "", "exhausting", "trigger damage apply_condition stat_override", "", "weapon-rider", cat="weapon", note="Maximised damage dice against objects; an Exhaustion level on a natural 20.")
T("vorpal", "Vorpal Sword", rx(r"^vorpal-"), "worn", "enchantment", "branch", "modifier resistance trigger slay damage", "", "weapon-rider", note="The DM confirms whether the creature has a head it needs and is small enough; a yes or no on the monster, not open judgment.")
T("berserker", "Berserker Axe", rx(r"^berserker-"), "worn", "enchantment curse", "", "modifier edge_scope trigger save_gate compel", "curse-lock", "weapon-rider", flags="Cu", note="Hit Point maximum rises by 1 per level.")
T("hammer-of-thunderbolts", "Hammer of Thunderbolts", rx(r"^hammer-of-thunderbolts-"), "worn action", "enchantment set_bonus", "area save_condition branch", "modifier stat_override trigger save_gate slay ability_score", "", "weapon-rider", flags="Ch", note="Set bonus with a Belt of Giant Strength or Gauntlets of Ogre Power.")
T("oathbow", "Oathbow", rx(r"^oathbow-"), "worn action", "", "edge extra_damage", "edge_scope", "use-limits", "weapon-rider", note="The sworn enemy is a mark like Hunter's Mark; it lasts until the target dies or dawn seven days later.")
T("dagger-of-venom", "Dagger of Venom", slugs("dagger-of-venom"), "worn action", "enchantment consumable", "", "modifier trigger save_gate damage apply_condition", "", "weapon-rider", flags="Ch*")
T("dragon-slayer", "Dragon Slayer", slugs("dragon-slayer"), "worn", "enchantment extra_damage", "branch", "modifier trigger damage", "", "weapon-rider")
T("dwarven-thrower", "Dwarven Thrower", slugs("dwarven-thrower"), "worn", "enchantment extra_damage", "branch", "modifier stat_override trigger damage", "", "weapon-rider")
T("javelin-of-lightning", "Javelin of Lightning", slugs("javelin-of-lightning"), "worn action", "", "area save_damage", "stat_override save_gate", "", "weapon-rider", flags="Ch*")
T("mace-of-disruption", "Mace of Disruption", slugs("mace-of-disruption"), "worn", "extra_damage light", "light branch", "trigger damage save_gate slay apply_condition", "", "weapon-rider")
T("mace-of-smiting", "Mace of Smiting", slugs("mace-of-smiting"), "worn", "enchantment", "branch", "modifier trigger damage slay", "", "weapon-rider")
T("mace-of-terror", "Mace of Terror", slugs("mace-of-terror"), "action", "", "area save_condition", "save_gate compel", "", "activated-effect", flags="Ch")
T("scimitar-of-speed", "Scimitar of Speed", slugs("scimitar-of-speed"), "worn", "enchantment", "grant_feature", "modifier", "", "weapon-rider", note="The Bonus Action attack is a Feature the Effect grants; that Feature needs automating too.")
T("sun-blade", "Sun Blade", slugs("sun-blade"), "worn action", "enchantment extra_damage light", "light branch choice", "modifier stat_override trigger damage", "", "weapon-rider", oneoff="the light is sunlight and its radius can be changed in 5-foot steps")
T("trident-of-fish-command", "Trident of Fish Command", slugs("trident-of-fish-command"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Dominate Beast",), only_spell=True, flags="Ch")

# --- Magic armor and shields ---------------------------------------------------------------------
T("armor-plus", "Armor, +1, +2 or +3", rx(r"^(?!shield-).+-plus-[123]$"), "worn", "enchantment", "", "modifier", "", "flat-bonus", cat="armor")
T("shield-plus", "Shield, +1, +2 or +3", rx(r"^shield-plus-[123]$"), "worn", "enchantment", "", "modifier", "", "flat-bonus")
T("armor-of-resistance", "Armor of Resistance", rx(r"^armor-of-resistance-"), "worn", "resistance", "", "resistance", "variant", "passive-ward")
T("armor-of-vulnerability", "Armor of Vulnerability", rx(r"^armor-of-vulnerability-"), "worn", "resistance curse", "", "resistance", "variant curse-lock", "passive-ward", flags="Cu")
T("demon-armor", "Demon Armor", rx(r"^demon-"), "worn", "bonus curse", "", "modifier proficiency stat_override edge_scope", "curse-lock", "passive-ward", flags="Cu")
T("adamantine-armor", "Adamantine Armor", rx(r"^adamantine-armor-"), "worn", "", "", "outcome_change", "", "passive-ward")
T("mithral-armor", "Mithral Armor", rx(r"^mithral-armor-"), "worn", "", "", "stat_override", "", "passive-trait")
T("dwarven-plate", "Dwarven Plate", slugs("dwarven-half-plate dwarven-plate"), "worn", "bonus", "", "modifier", "", "passive-ward", oneoff="a Reaction that shortens forced movement by 10 feet")
T("armor-of-etherealness", "Plate Armor of Etherealness", slugs("half-plate-armor-of-etherealness plate-armor-of-etherealness"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Etherealness",), only_spell=True, flags="Ch*")
T("animated-shield", "Animated Shield", slugs("animated-shield"), "action", "", "", "stat_override", "", "activated-effect", note="For 1 minute the Shield's bonus applies without a hand holding it.")
T("armor-of-invulnerability", "Armor of Invulnerability", slugs("armor-of-invulnerability"), "worn action", "resistance", "", "resistance", "", "passive-ward", flags="Ch*")
T("arrow-catching-shield", "Arrow-Catching Shield", slugs("arrow-catching-shield"), "worn", "bonus", "", "modifier", "", "passive-ward", oneoff="a Reaction that makes the bearer the target of a ranged attack instead")
T("dragon-scale-mail", "Dragon Scale Mail", slugs("dragon-scale-mail"), "worn action", "bonus resistance", "save_edge", "modifier resistance edge_scope note", "variant", "passive-ward", flags="Ch*", note="Sensing the nearest dragon of its kind is a described outcome.")
T("elven-chain", "Elven Chain", slugs("elven-chain-mail elven-chain-shirt"), "worn", "bonus", "", "modifier proficiency", "", "flat-bonus")
T("glamoured-studded-leather", "Glamoured Studded Leather", slugs("glamoured-studded-leather"), "worn action", "bonus", "", "modifier note", "", "flat-bonus", note="The changed appearance is a described outcome.")
T("shield-of-missile-attraction", "Shield of Missile Attraction", slugs("shield-of-missile-attraction"), "worn", "resistance curse", "", "resistance", "curse-lock", "passive-ward", flags="Cu", oneoff="ranged attacks on creatures within 10 feet are redirected to the bearer")
T("spellguard-shield", "Spellguard Shield", slugs("spellguard-shield"), "worn", "", "save_edge edge", "edge_scope", "", "passive-ward")

# --- Rings ---------------------------------------------------------------------------------------
T("ring-of-animal-influence", "Ring of Animal Influence", slugs("ring-of-animal-influence"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Animal Friendship", "Fear", "Speak with Animals"), only_spell=True, flags="Ch")
T("ring-of-djinni-summoning", "Ring of Djinni Summoning", slugs("ring-of-djinni-summoning"), "action", "", "summon", "", "use-limits depletion", "summon", monsters=("Djinni",))
T("ring-of-elemental-command", "Ring of Elemental Command", slugs("ring-of-elemental-command"), "worn action", "spell resistance speed", "edge save_condition", "edge_scope save_gate compel proficiency resistance speed immunity", "variant spell-options", "spell-plus", flags="Ch", oneoff="moving through solid earth or rock",
  spells=("Chain Lightning", "Feather Fall", "Gust of Wind", "Wind Wall", "Earthquake", "Stone Shape", "Stoneskin", "Wall of Stone", "Burning Hands", "Fireball", "Fire Storm", "Wall of Fire", "Create or Destroy Water", "Ice Storm", "Tsunami", "Wall of Ice", "Water Walk"))
T("ring-of-evasion", "Ring of Evasion", slugs("ring-of-evasion"), "worn", "", "", "outcome_change", "", "passive-ward", flags="Ch")
T("ring-of-feather-falling", "Ring of Feather Falling", slugs("ring-of-feather-falling"), "worn", "", "", "immunity", "", "passive-ward")
T("ring-of-free-action", "Ring of Free Action", slugs("ring-of-free-action"), "worn", "", "", "immunity", "", "passive-ward")
T("ring-of-invisibility", "Ring of Invisibility", slugs("ring-of-invisibility"), "action", "", "", "apply_condition", "", "activated-effect")
T("ring-of-jumping", "Ring of Jumping", slugs("ring-of-jumping"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Jump",), only_spell=True)
T("ring-of-mind-shielding", "Ring of Mind Shielding", slugs("ring-of-mind-shielding"), "worn", "", "", "immunity note", "", "passive-ward", note="The soul entering the ring on death is a described outcome.")
T("ring-of-protection", "Ring or Cloak of Protection", slugs("ring-of-protection cloak-of-protection"), "worn", "bonus", "", "modifier", "", "flat-bonus")
T("ring-of-regeneration", "Ring of Regeneration", slugs("ring-of-regeneration"), "worn", "", "", "heal trigger note", "", "passive-trait", note="Regrowing a body part is a described outcome.")
T("ring-of-resistance", "Ring of Resistance", slugs("ring-of-resistance"), "worn", "resistance", "", "resistance", "variant", "passive-ward")
T("ring-of-shooting-stars", "Ring of Shooting Stars", slugs("ring-of-shooting-stars"), "action", "cantrip spell", "area save_damage light choice", "save_gate", "charge-scaling", "spell-plus", spells=("Dancing Lights", "Light", "Faerie Fire"), flags="Ch", oneoff="lightning spheres that move each turn and discharge at the first creature they come near")
T("ring-of-spell-storing", "Ring of Spell Storing", slugs("ring-of-spell-storing"), "action", "", "", "spell_store", "", "spell-resource")
T("ring-of-spell-turning", "Ring of Spell Turning", slugs("ring-of-spell-turning"), "worn", "", "save_edge", "edge_scope reflect_spell", "", "passive-ward")
T("ring-of-swimming", "Ring of Swimming", slugs("ring-of-swimming"), "worn", "speed", "", "speed", "", "passive-trait")
T("ring-of-telekinesis", "Ring of Telekinesis", slugs("ring-of-telekinesis"), "action", "spell", "", "", "", "grants-spell", spells=("Telekinesis",), only_spell=True)
T("ring-of-the-ram", "Ring of the Ram", slugs("ring-of-the-ram"), "action", "", "forced_move choice", "attack_gate damage", "charge-scaling utilize-check", "activated-effect", flags="Ch")
T("ring-of-three-wishes", "Ring of Three Wishes", slugs("ring-of-three-wishes"), "action", "spell", "", "", "", "grants-spell", spells=("Wish",), only_spell=True, flags="Ch", note="Three Charges that never return fit the Charges model (no dice, no bonus).")
T("ring-of-warmth", "Ring of Warmth", slugs("ring-of-warmth"), "worn", "", "", "damage_reduction immunity", "", "passive-ward")
T("ring-of-water-walking", "Ring of Water Walking", slugs("ring-of-water-walking"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Water Walk",), only_spell=True)
T("ring-of-x-ray-vision", "Ring of X-ray Vision", slugs("ring-of-x-ray-vision"), "action", "sense", "exhausting", "sense save_gate apply_condition", "use-limits", "activated-effect")

# --- Potions and oils ----------------------------------------------------------------------------
T("oil-of-etherealness", "Oil of Etherealness", slugs("oil-of-etherealness"), "consumed", "consumable spell", "", "", "spell-options", "grants-spell", spells=("Etherealness",), only_spell=True, flags="Co")
T("oil-of-sharpness", "Oil of Sharpness", slugs("oil-of-sharpness"), "consumed", "consumable", "", "modifier", "", "consumable-effect", flags="Co", oneoff="the Effect lands on another Item Instance, not on a creature")
T("oil-of-slipperiness", "Oil of Slipperiness", slugs("oil-of-slipperiness"), "consumed", "consumable spell", "", "", "spell-options", "grants-spell", spells=("Freedom of Movement", "Grease"), only_spell=True, flags="Co")
T("philter-of-love", "Philter of Love", slugs("philter-of-love"), "consumed", "consumable", "", "apply_condition trigger", "", "consumable-effect", flags="Co")
T("potion-spell", "Potion that gives a spell's effect", slugs("potion-of-animal-friendship potion-of-clairvoyance potion-of-diminution potion-of-gaseous-form potion-of-growth potion-of-mind-reading potion-of-speed"), "consumed", "consumable spell", "", "", "spell-options", "grants-spell",
  spells=("Animal Friendship", "Clairvoyance", "Enlarge/Reduce", "Gaseous Form", "Detect Thoughts", "Haste"), only_spell=True, flags="Co")
T("potion-of-climbing", "Potion of Climbing", slugs("potion-of-climbing"), "consumed", "consumable speed skill_boost", "", "speed check_edge", "", "consumable-effect", flags="Co")
T("potion-of-flying", "Potion of Flying", slugs("potion-of-flying"), "consumed", "consumable speed", "", "speed", "", "consumable-effect", flags="Co")
T("potion-of-heroism", "Potion of Heroism", slugs("potion-of-heroism"), "consumed", "consumable spell", "temp_hp bonus_die", "", "spell-options", "consumable-effect", spells=("Bless",), flags="Co", note="Temporary Hit Points and the seeded Bless Effect, without Concentration, for 1 hour.")
T("potion-of-invisibility", "Potion of Invisibility", slugs("potion-of-invisibility"), "consumed", "consumable", "", "apply_condition trigger", "", "consumable-effect", flags="Co")
T("potion-of-poison", "Potion of Poison", slugs("potion-of-poison"), "consumed", "consumable", "", "damage save_gate apply_condition", "", "consumable-effect", flags="Co", note="Looks like a healing potion until identified; the builder's hidden flag covers that.")
T("potion-of-resistance", "Potion of Resistance", slugs("potion-of-resistance"), "consumed", "consumable resistance", "", "resistance", "variant", "consumable-effect", flags="Co")
T("potion-of-water-breathing", "Potion of Water Breathing", slugs("potion-of-water-breathing"), "consumed", "consumable", "", "immunity", "", "consumable-effect", flags="Co")
T("potion-of-healing", "Potion of Healing", slugs("potion-of-healing potions-of-healing"), "consumed", "consumable", "", "heal", "variant", "consumable-effect", flags="Co", note="Play already heals 2d4 + 2 for the slug `potion-of-healing`; the greater, superior and supreme potions exist only as a table in the `potions-of-healing` row.")
T("potion-of-giant-strength", "Potion of Giant Strength", slugs("potion-of-giant-strength"), "consumed", "consumable", "", "ability_score", "variant", "consumable-effect", flags="Co")
T("antitoxin", "Antitoxin", slugs("antitoxin"), "consumed", "consumable", "save_edge", "edge_scope", "", "consumable-effect", flags="Co")

# --- Wands ---------------------------------------------------------------------------------------
T("wand-spell-dc", "Wand that casts spells at a set DC", slugs("wand-of-binding wand-of-fear wand-of-fireballs wand-of-lightning-bolts wand-of-polymorph wand-of-web"), "action", "spell", "", "", "spell-options depletion", "grants-spell",
  spells=("Hold Monster", "Hold Person", "Command", "Fear", "Fireball", "Lightning Bolt", "Polymorph", "Web"), only_spell=True, flags="Ch", note="The seed's Wand of Web row carries the Wand of Fear text; it is classified here by its name, as a wand that casts Web.")
T("wand-of-magic-missiles", "Wand of Magic Missiles", slugs("wand-of-magic-missiles"), "action", "spell", "", "", "spell-options depletion", "grants-spell", spells=("Magic Missile",), only_spell=True, flags="Ch")
T("wand-of-magic-detection", "Wand of Magic Detection", slugs("wand-of-magic-detection"), "action", "spell", "", "", "", "grants-spell", spells=("Detect Magic",), only_spell=True, flags="Ch")
T("wand-of-enemy-detection", "Wand of Enemy Detection", slugs("wand-of-enemy-detection"), "action", "", "", "", "depletion", "utility-object", flags="Ch", oneoff="tells the user the direction of the nearest Hostile creature, Invisible or hidden ones included")
T("wand-of-paralysis", "Wand of Paralysis", slugs("wand-of-paralysis"), "action", "", "save_condition", "save_gate", "depletion", "activated-effect", flags="Ch")
T("wand-of-secrets", "Wand of Secrets", slugs("wand-of-secrets"), "action", "", "reveal area", "", "", "utility-object", flags="Ch", oneoff="points only at the nearest secret door or trap")
T("wand-of-the-war-mage", "Wand of the War Mage, +1, +2 or +3", rx(r"^wand-of-the-war-mage-plus-[123]$"), "worn", "bonus", "", "modifier", "", "flat-bonus", note="Ignoring Half Cover on spell attacks is a scoped modifier.")
T("wand-of-wonder", "Wand of Wonder", slugs("wand-of-wonder"), "action", "spell", "area save_condition create_surface summon form", "roll_table save_gate damage apply_condition note", "depletion spell-options", "random-table", flags="Ch",
  spells=("Darkness", "Faerie Fire", "Fireball", "Slow", "Stinking Cloud", "Gust of Wind", "Lightning Bolt", "Enlarge/Reduce", "Invisibility", "Polymorph"), monsters=("Rhinoceros", "Elephant", "Rat", "Black Bear", "Giant Wasp", "Frog"),
  note="Every row is a spell, a condition, a summon or a described oddity; none is open DM judgment.")

# --- Staffs --------------------------------------------------------------------------------------
T("staff-of-charming", "Staff of Charming", slugs("staff-of-charming"), "action", "spell", "", "reflect_spell outcome_change", "use-limits depletion", "spell-plus", spells=("Charm Person", "Command", "Comprehend Languages"), flags="Ch")
T("staff-of-fire-frost", "Staff of Fire, Staff of Frost", slugs("staff-of-fire staff-of-frost"), "worn action", "spell resistance", "", "resistance", "depletion", "spell-plus", spells=("Burning Hands", "Fireball", "Wall of Fire", "Cone of Cold", "Fog Cloud", "Ice Storm", "Wall of Ice"), flags="Ch")
T("staff-of-healing", "Staff of Healing", slugs("staff-of-healing"), "action", "spell", "", "", "spell-options depletion", "grants-spell", spells=("Cure Wounds", "Lesser Restoration", "Mass Cure Wounds"), only_spell=True, flags="Ch")
T("staff-of-power", "Staff of Power", slugs("staff-of-power"), "worn action", "enchantment bonus spell", "area save_damage", "modifier save_gate damage roll_table banish", "spell-options depletion charge-scaling", "spell-plus", flags="Ch",
  spells=("Cone of Cold", "Fireball", "Globe of Invulnerability", "Hold Monster", "Levitate", "Lightning Bolt", "Magic Missile", "Ray of Enfeeblement", "Wall of Force"))
T("staff-of-striking", "Staff of Striking", slugs("staff-of-striking"), "worn", "enchantment extra_damage", "", "modifier trigger damage", "charge-scaling depletion", "weapon-rider", flags="Ch")
T("staff-of-swarming-insects", "Staff of Swarming Insects", slugs("staff-of-swarming-insects"), "action", "spell", "area create_surface", "", "depletion", "spell-plus", spells=("Giant Insect", "Insect Plague"), flags="Ch", oneoff="a cloud that obscures for everyone but the user")
T("staff-of-the-magi", "Staff of the Magi", slugs("staff-of-the-magi"), "worn action", "enchantment bonus spell cantrip", "save_edge area save_damage", "modifier edge_scope reflect_spell save_gate damage roll_table banish", "spell-options depletion charge-scaling", "spell-plus", flags="Ch",
  spells=("Arcane Lock", "Conjure Elemental", "Detect Magic", "Dispel Magic", "Enlarge/Reduce", "Fireball", "Flaming Sphere", "Ice Storm", "Invisibility", "Knock", "Light", "Lightning Bolt", "Mage Hand", "Passwall", "Plane Shift", "Protection from Evil and Good", "Telekinesis", "Wall of Fire", "Web"))
T("staff-of-the-python", "Staff of the Python", slugs("staff-of-the-python"), "action", "", "summon", "", "use-limits depletion", "summon", monsters=("Giant Constrictor Snake",))
T("staff-of-the-woodlands", "Staff of the Woodlands", slugs("staff-of-the-woodlands"), "worn action", "enchantment bonus spell", "", "modifier create_object", "depletion", "spell-plus", flags="Ch",
  spells=("Animal Friendship", "Awaken", "Barkskin", "Locate Animals or Plants", "Pass without Trace", "Speak with Animals", "Speak with Plants", "Wall of Thorns"))
T("staff-of-thunder-and-lightning", "Staff of Thunder and Lightning", slugs("staff-of-thunder-and-lightning"), "worn action", "enchantment", "area save_damage save_condition", "modifier trigger damage save_gate apply_condition", "use-limits", "weapon-rider", note="Five properties, each once per dawn.")
T("staff-of-withering", "Staff of Withering", slugs("staff-of-withering"), "worn", "extra_damage", "save_edge", "trigger damage save_gate check_edge", "", "weapon-rider", flags="Ch")

# --- Rods ----------------------------------------------------------------------------------------
T("immovable-rod", "Immovable Rod", slugs("immovable-rod"), "action", "", "", "create_object", "utilize-check", "utility-object")
T("rod-of-absorption", "Rod of Absorption", slugs("rod-of-absorption"), "action", "", "", "reflect_spell spell_store", "depletion", "spell-resource")
T("rod-of-alertness", "Rod of Alertness", slugs("rod-of-alertness"), "worn action", "skill_boost spell light", "light area reveal", "check_edge modifier", "", "spell-plus", spells=("Detect Evil and Good", "Detect Magic", "Detect Poison and Disease", "See Invisibility"), flags="Ch*")
T("rod-of-lordly-might", "Rod of Lordly Might", slugs("rod-of-lordly-might"), "worn action", "enchantment light extra_damage", "choice light area save_condition", "modifier stat_override trigger damage heal save_gate apply_condition create_object note", "use-limits", "weapon-rider", note="Six button forms and three properties, each once per dawn.")
T("rod-of-rulership", "Rod of Rulership", slugs("rod-of-rulership"), "action", "", "area save_condition", "save_gate trigger", "", "activated-effect", flags="Ch*")
T("rod-of-security", "Rod of Security", slugs("rod-of-security"), "action", "", "", "banish heal note", "use-limits", "planar-utility")

# --- Wondrous items ------------------------------------------------------------------------------
T("ability-19", "Item that sets an ability score to 19", slugs("amulet-of-health gauntlets-of-ogre-power headband-of-intellect"), "worn", "", "", "ability_score", "", "passive-trait")
T("belt-of-giant-strength", "Belt of Giant Strength", rx(r"^belt-of-giant-strength-"), "worn", "", "", "ability_score", "", "passive-trait")
T("amulet-of-proof", "Amulet of Proof against Detection and Location", slugs("amulet-of-proof-against-detection-and-location"), "worn", "", "", "immunity", "", "passive-ward")
T("amulet-of-the-planes", "Amulet of the Planes", slugs("amulet-of-the-planes"), "action", "spell", "", "roll_table banish", "utilize-check", "planar-utility", spells=("Plane Shift",))
T("apparatus-of-the-crab", "Apparatus of the Crab", slugs("apparatus-of-the-crab"), "action", "light", "light choice", "create_object attack_gate damage apply_condition escape_check", "", "object-or-vehicle")
T("bag-of-beans", "Bag of Beans", slugs("bag-of-beans"), "consumed", "consumable", "area save_damage", "save_gate roll_table", "depletion", "random-table", flags="Co", exception="Yes, for the planted-bean table only: most rows end in \"GM's choice\" (what the geyser spouts, which monster the toads become, what waits in the pyramid, where the beanstalk leads). The explosion is ordinary area damage and automates.")
T("bag-of-devouring", "Bag of Devouring", slugs("bag-of-devouring"), "action", "container", "", "roll_table escape_check slay banish note", "", "container", note="Not labelled a curse in the text, but built to pass as a Bag of Holding.")
T("extradimensional-container", "Bag of Holding, Handy Haversack", slugs("bag-of-holding handy-haversack"), "worn", "container", "", "", "", "container", oneoff="put inside another extradimensional space, both are destroyed and nearby creatures are pulled to the Astral Plane")
T("efficient-quiver", "Efficient Quiver", slugs("efficient-quiver"), "worn", "container", "", "", "", "container", oneoff="three compartments limited by the number and kind of items, where the container row has one capacity in pounds")
T("portable-hole", "Portable Hole", slugs("portable-hole"), "action", "container", "", "create_object escape_check", "", "container", oneoff="put inside another extradimensional space, both are destroyed and nearby creatures are pulled to the Astral Plane")
T("bag-of-tricks", "Bag of Tricks", slugs("bag-of-tricks"), "action", "", "summon", "roll_table", "variant", "summon", flags="Ch*",
  monsters=("Weasel", "Giant Rat", "Badger", "Boar", "Panther", "Giant Badger", "Dire Wolf", "Giant Elk", "Rat", "Owl", "Mastiff", "Goat", "Giant Goat", "Giant Boar", "Lion", "Brown Bear", "Jackal", "Ape", "Baboon", "Axe Beak", "Black Bear", "Giant Weasel", "Giant Hyena", "Tiger"))
T("bead-of-force", "Bead of Force", slugs("bead-of-force"), "consumed", "consumable", "area save_damage forced_move", "save_gate create_object", "", "consumable-effect", flags="Co")
T("belt-of-dwarvenkind", "Belt of Dwarvenkind", slugs("belt-of-dwarvenkind"), "worn", "skill_boost sense resistance", "save_edge", "proficiency check_edge ability_score sense resistance edge_scope note", "", "passive-trait", note="Some benefits apply only to wearers who are not dwarves; the beard is a described outcome.")
T("boots-of-elvenkind", "Boots of Elvenkind", slugs("boots-of-elvenkind"), "worn", "skill_boost", "", "check_edge", "", "passive-trait")
T("boots-of-levitation", "Boots of Levitation", slugs("boots-of-levitation"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Levitate",), only_spell=True)
T("boots-of-speed", "Boots of Speed", slugs("boots-of-speed"), "action", "speed", "edge", "speed edge_scope", "use-limits", "activated-effect")
T("boots-of-striding", "Boots of Striding and Springing", slugs("boots-of-striding-and-springing"), "worn", "speed", "", "speed stat_override", "", "passive-trait")
T("boots-of-the-winterlands", "Boots of the Winterlands", slugs("boots-of-the-winterlands"), "worn", "resistance", "", "resistance immunity", "", "passive-ward")
T("elemental-summoner", "Bowl, Brazier, Censer or Stone that summons an elemental", slugs("bowl-of-commanding-water-elementals brazier-of-commanding-fire-elementals censer-of-controlling-air-elementals stone-of-controlling-earth-elementals"), "action", "", "summon", "", "", "summon", flags="Ch*",
  monsters=("Water Elemental", "Fire Elemental", "Air Elemental", "Earth Elemental"))
T("bracers-of-archery", "Bracers of Archery", slugs("bracers-of-archery"), "worn", "bonus", "", "proficiency modifier", "", "passive-trait")
T("bracers-of-defense", "Bracers of Defense", slugs("bracers-of-defense"), "worn", "bonus", "", "modifier", "", "flat-bonus", note="Only while wearing no armor and using no Shield: a scoped modifier.")
T("brooch-of-shielding", "Brooch of Shielding", slugs("brooch-of-shielding"), "worn", "resistance", "", "resistance", "", "passive-ward", oneoff="Immunity to damage from one named spell (Magic Missile)")
T("broom-of-flying", "Broom of Flying", slugs("broom-of-flying"), "action", "speed", "", "speed note", "", "object-or-vehicle", note="A ridden object with its own Fly Speed that drops with the load carried.")
T("candle-of-invocation", "Candle of Invocation", slugs("candle-of-invocation"), "action", "consumable light spell", "light area edge save_edge", "check_edge", "variant use-limits", "spell-plus", spells=("Gate",), flags="Co", oneoff="Clerics and Druids in its light cast level 1 spells without spending slots")
T("cape-of-the-mountebank", "Cape of the Mountebank", slugs("cape-of-the-mountebank"), "action", "spell", "", "note", "", "spell-plus", spells=("Dimension Door",), flags="Ch*", note="The smoke leaves the space Lightly Obscured; Surfaces only model Heavily Obscured and darkness.")
T("carpet-of-flying", "Carpet of Flying", slugs("carpet-of-flying"), "action", "speed", "", "create_object speed", "variant", "object-or-vehicle")
T("chime-of-opening", "Chime of Opening", slugs("chime-of-opening"), "action", "spell", "", "", "", "grants-spell", spells=("Knock",), only_spell=True, flags="Ch*", note="Ten uses that never return fit the Charges model.")
T("circlet-of-blasting", "Circlet of Blasting", slugs("circlet-of-blasting"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Scorching Ray",), only_spell=True, flags="Ch*")
T("cloak-of-arachnida", "Cloak of Arachnida", slugs("cloak-of-arachnida"), "worn action", "resistance speed spell", "", "resistance speed immunity", "spell-options", "spell-plus", spells=("Web",), flags="Ch*")
T("cloak-of-displacement", "Cloak of Displacement", slugs("cloak-of-displacement"), "worn", "", "edge", "trigger", "", "passive-ward", note="The Disadvantage switches off when the bearer takes damage and while their Speed is 0.")
T("cloak-of-elvenkind", "Cloak of Elvenkind", slugs("cloak-of-elvenkind"), "worn", "skill_boost", "", "check_edge", "", "passive-trait")
T("cloak-of-the-bat", "Cloak of the Bat", slugs("cloak-of-the-bat"), "worn action", "skill_boost speed spell", "form", "check_edge speed", "spell-options", "spell-plus", spells=("Polymorph",), flags="Ch*", oneoff="the Fly Speed holds only in Dim Light or Darkness", monsters=("Bat",))
T("cloak-of-the-manta-ray", "Cloak of the Manta Ray", slugs("cloak-of-the-manta-ray"), "worn", "speed", "", "speed immunity", "", "passive-trait")
T("crystal-ball", "Crystal Ball", slugs("crystal-ball crystal-ball-of-mind-reading crystal-ball-of-telepathy"), "action", "spell", "", "", "spell-options use-limits", "grants-spell", spells=("Scrying", "Detect Thoughts", "Suggestion"), only_spell=True, note="The telepathy variant also lets the user speak with creatures near the sensor, a described outcome.")
T("crystal-ball-of-true-seeing", "Crystal Ball of True Seeing", slugs("crystal-ball-of-true-seeing"), "action", "spell sense", "", "sense", "spell-options", "spell-plus", spells=("Scrying",))
T("cube-of-force", "Cube of Force", slugs("cube-of-force"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Mage Armor", "Shield", "Tiny Hut", "Private Sanctum", "Resilient Sphere", "Wall of Force"), only_spell=True, flags="Ch")
T("cubic-gate", "Cubic Gate", slugs("cubic-gate"), "action", "spell", "", "", "variant", "grants-spell", spells=("Gate", "Plane Shift"), only_spell=True, flags="Ch")
T("decanter-of-endless-water", "Decanter of Endless Water", slugs("decanter-of-endless-water"), "action", "", "area save_damage save_condition choice", "save_gate note", "", "activated-effect")
T("deck-of-illusions", "Deck of Illusions", slugs("deck-of-illusions"), "consumed", "consumable", "", "roll_table create_object", "utilize-check depletion", "random-table", flags="Co", note="The illusion is a harmless token with the illusory Visibility Quality.")
T("dimensional-shackles", "Dimensional Shackles", slugs("dimensional-shackles"), "action", "", "", "escape_check", "", "restraint", oneoff="the bound creature cannot teleport or change plane")
T("dragon-orb", "Dragon Orb", slugs("dragon-orb"), "action", "spell cantrip sentient", "", "save_gate apply_condition compel note", "spell-options use-limits curse-lock", "spell-plus", spells=("Cure Wounds", "Daylight", "Death Ward", "Detect Magic", "Scrying", "Suggestion"), flags="Ch",
  exception="Yes, for two parts: what the orb's dragon essence wants (it casts Suggestion on a Charmed user toward ends \"the GM decides\") and Call Dragons, whose result is which dragons arrive and how they feel about it. The saving throw, the Charmed condition, the Charges and the spells automate.")
T("dust-of-disappearance", "Dust of Disappearance", slugs("dust-of-disappearance"), "consumed", "consumable", "area", "apply_condition trigger", "", "consumable-effect", flags="Co")
T("dust-of-dryness", "Dust of Dryness", slugs("dust-of-dryness"), "consumed", "consumable", "branch save_damage", "save_gate note", "depletion", "consumable-effect", flags="Co", note="Absorbing a cube of water into a pellet is a described outcome.")
T("dust-of-sneezing", "Dust of Sneezing and Choking", slugs("dust-of-sneezing-and-choking"), "consumed", "consumable", "area save_condition branch", "save_gate", "", "consumable-effect", flags="Co", oneoff="the failed save also leaves the creature suffocating", note="Identifies as Dust of Disappearance; the builder's hidden flag covers that.")
T("efreeti-bottle", "Efreeti Bottle", slugs("efreeti-bottle"), "action", "", "summon", "roll_table", "use-limits depletion", "summon", spells=("Wish",), monsters=("Efreeti",))
T("elemental-gem", "Elemental Gem", slugs("elemental-gem"), "consumed", "consumable", "summon", "", "variant", "summon", flags="Co", monsters=("Air Elemental", "Water Elemental", "Fire Elemental", "Earth Elemental"))
T("eversmoking-bottle", "Eversmoking Bottle", slugs("eversmoking-bottle"), "action", "", "area create_surface", "", "", "activated-effect", oneoff="the cloud grows by 10 feet each minute the bottle stays open")
T("eyes-of-charming", "Eyes of Charming", slugs("eyes-of-charming"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Charm Person",), only_spell=True, flags="Ch")
T("eyes-of-minute-seeing", "Eyes of Minute Seeing", slugs("eyes-of-minute-seeing"), "worn", "sense skill_boost", "", "sense check_edge", "", "passive-trait")
T("eyes-of-the-eagle", "Eyes of the Eagle", slugs("eyes-of-the-eagle"), "worn", "skill_boost", "", "check_edge", "", "passive-trait")
T("feather-token-anchor", "Feather Token (Anchor)", slugs("feather-token-anchor"), "consumed", "consumable", "", "note", "", "utility-object", flags="Co", note="A vessel cannot be moved for 24 hours.")
T("feather-token-bird", "Feather Token (Bird)", slugs("feather-token-bird"), "consumed", "consumable", "summon", "note", "", "summon", flags="Co", monsters=("Roc",), note="A Roc that cannot attack and carries the party; the travel pace is a described outcome.")
T("feather-token-fan", "Feather Token (Fan)", slugs("feather-token-fan"), "consumed", "consumable", "", "note", "", "utility-object", flags="Co", note="A ship gains 5 miles per hour for 8 hours.")
T("feather-token-swan-boat", "Feather Token (Swan Boat)", slugs("feather-token-swan-boat"), "consumed", "consumable", "", "create_object", "", "object-or-vehicle", flags="Co")
T("feather-token-tree", "Feather Token (Tree)", slugs("feather-token-tree"), "consumed", "consumable", "", "create_object", "", "object-or-vehicle", flags="Co")
T("feather-token-whip", "Feather Token (Whip)", slugs("feather-token-whip"), "consumed", "consumable", "", "create_object attack_gate damage", "", "consumable-effect", flags="Co")
T("figurine", "Figurine of Wondrous Power (plain summon)", slugs("figurine-of-wondrous-power-bronze-griffon figurine-of-wondrous-power-ebony-fly figurine-of-wondrous-power-golden-lions figurine-of-wondrous-power-marble-elephant figurine-of-wondrous-power-onyx-dog figurine-of-wondrous-power-serpentine-owl"), "action", "", "summon", "", "use-limits", "summon",
  monsters=("Griffon", "Giant Fly", "Lion", "Elephant", "Mastiff", "Giant Owl"), note="The Onyx Dog's Mastiff has Intelligence 8 and Blindsight; the owl's telepathy is a described outcome.")
T("figurine-obsidian-steed", "Figurine of Wondrous Power (Obsidian Steed)", slugs("figurine-of-wondrous-power-obsidian-stead"), "action", "", "summon", "roll_table banish", "use-limits", "summon", monsters=("Nightmare",))
T("figurine-silver-raven", "Figurine of Wondrous Power (Silver Raven)", slugs("figurine-of-wondrous-power-silver-raven"), "action", "spell", "summon", "", "use-limits spell-options", "summon", spells=("Animal Messenger",), monsters=("Raven",))
T("figurine-ivory-goats", "Figurine of Wondrous Power (Ivory Goats)", slugs("figurine-of-wondrous-power-ivory-goats"), "action", "", "summon area save_condition", "save_gate create_object", "use-limits variant", "summon", flags="Ch", monsters=("Giant Goat", "Riding Horse"), note="Three goats in one row; the Goat of Terror's horns become a +1 Lance and a +2 Longsword.")
T("folding-boat", "Folding Boat", slugs("folding-boat"), "action", "container", "choice", "create_object", "", "object-or-vehicle")
T("gem-of-brightness", "Gem of Brightness", slugs("gem-of-brightness"), "action", "light", "light choice area save_condition", "save_gate", "", "activated-effect", flags="Ch")
T("gem-of-seeing", "Gem of Seeing", slugs("gem-of-seeing"), "action", "sense", "", "sense", "", "activated-effect", flags="Ch")
T("gloves-of-missile-snaring", "Gloves of Missile Snaring", slugs("gloves-of-missile-snaring"), "worn", "", "", "damage_reduction", "", "passive-ward")
T("gloves-of-swimming-and-climbing", "Gloves of Swimming and Climbing", slugs("gloves-of-swimming-and-climbing"), "worn", "speed skill_boost", "", "speed modifier", "", "passive-trait")
T("goggles-of-night", "Goggles of Night", slugs("goggles-of-night"), "worn", "sense", "", "sense", "", "passive-trait")
T("hat-of-disguise", "Hat of Disguise", slugs("hat-of-disguise"), "action", "spell", "", "", "", "grants-spell", spells=("Disguise Self",), only_spell=True)
T("helm-of-brilliance", "Helm of Brilliance", slugs("helm-of-brilliance"), "worn action", "resistance spell light extra_damage", "area light branch", "resistance trigger damage roll_table save_gate", "spell-options use-limits depletion", "spell-plus", spells=("Daylight", "Fireball", "Prismatic Spray", "Wall of Fire"), note="Four gem pools, one per spell.")
T("helm-of-comprehending-languages", "Helm of Comprehending Languages", slugs("helm-of-comprehending-languages"), "action", "spell", "", "", "", "grants-spell", spells=("Comprehend Languages",), only_spell=True)
T("helm-of-telepathy", "Helm of Telepathy", slugs("helm-of-telepathy"), "worn action", "spell", "", "note", "spell-options use-limits", "spell-plus", spells=("Detect Thoughts", "Suggestion"), note="Telepathy out to 30 feet is a described outcome.")
T("helm-of-teleportation", "Helm of Teleportation", slugs("helm-of-teleportation"), "action", "spell", "", "", "", "grants-spell", spells=("Teleport",), only_spell=True, flags="Ch")
T("horn-of-blasting", "Horn of Blasting", slugs("horn-of-blasting"), "action", "", "area save_damage save_condition", "save_gate roll_table damage", "", "activated-effect")
T("horn-of-valhalla", "Horn of Valhalla", slugs("horn-of-valhalla"), "action", "", "summon", "", "variant use-limits", "summon", monsters=("Berserker",), oneoff="the spirits turn on a user who lacks the horn's proficiency requirement")
T("horseshoes-of-a-zephyr", "Horseshoes of a Zephyr", slugs("horseshoes-of-a-zephyr"), "worn", "", "", "immunity note", "", "passive-trait", note="Worn by a mount.")
T("horseshoes-of-speed", "Horseshoes of Speed", slugs("horseshoes-of-speed"), "worn", "speed", "", "speed", "", "passive-trait", note="Worn by a mount.")
T("instant-fortress", "Instant Fortress", slugs("instant-fortress"), "action", "", "forced_move", "create_object", "", "object-or-vehicle")
T("iron-bands", "Iron Bands", slugs("iron-bands"), "action", "", "", "attack_gate apply_condition escape_check", "", "restraint", flags="Ch*")
T("iron-flask", "Iron Flask", slugs("iron-flask"), "action", "", "", "save_gate banish compel", "", "planar-utility")
T("lantern-of-revealing", "Lantern of Revealing", slugs("lantern-of-revealing"), "action", "light", "light reveal choice", "", "", "light-source")
T("mantle-of-spell-resistance", "Mantle of Spell Resistance", slugs("mantle-of-spell-resistance"), "worn", "", "save_edge", "edge_scope", "", "passive-ward")
T("manual-tome", "Manual or Tome that raises an ability score", slugs("manual-of-bodily-health manual-of-gainful-exercise manual-of-quickness-of-action tome-of-clear-thought tome-of-leadership-and-influence tome-of-understanding"), "consumed", "consumable", "", "ability_score", "", "permanent-boost", flags="Co", note="48 hours of study over 6 days: Downtime Days exist.")
T("manual-of-golems", "Manual of Golems", slugs("manual-of-golems"), "consumed", "consumable", "summon", "damage", "variant", "summon", flags="Co", monsters=("Clay Golem", "Flesh Golem", "Iron Golem", "Stone Golem"), note="30 to 120 days of work and a cost in gold: downtime, not an encounter Effect.")
T("marvelous-pigments", "Marvelous Pigments", slugs("marvelous-pigments"), "consumed", "consumable", "", "create_object", "depletion", "utility-object", flags="Co", exception="Yes: the user paints \"any number of three-dimensional objects and terrain features\" into being. What was painted, what it is worth and what it does is open by nature; the engine can spend the pot and nothing more.")
T("medallion-of-thoughts", "Medallion of Thoughts", slugs("medallion-of-thoughts"), "action", "spell", "", "", "spell-options", "grants-spell", spells=("Detect Thoughts",), only_spell=True, flags="Ch")
T("mirror-of-life-trapping", "Mirror of Life Trapping", slugs("mirror-of-life-trapping"), "action", "", "", "save_gate banish create_object trigger", "", "planar-utility")
T("mysterious-deck", "Mysterious Deck", slugs("mysterious-deck"), "consumed", "", "summon temp_hp edge save_edge", "roll_table modifier ability_score check_edge proficiency", "depletion", "random-table", spells=("Wish",), monsters=("Knight",),
  exception="Yes, for seven cards: Fates, Flames, Rogue, Ruin, Sage, Void and the keep that Throne grants change the world or the story in ways only the DM can rule. The draw itself and the cards with numbers (Balance, Comet, Euryale, Fool, Gem, Jester, Knight, Moon, Puzzle, Skull, Star, Talons, the rest of Throne) automate; Key and Sun need the DM to pick an item, which is a choice and not a ruling.",
  note="The SRD 5.2 name of the Deck of Many Things. The Avatar of Death is an inline stat block, not a seeded monster, and the seed text lists Donjon in its table without describing it.")
T("necklace-of-adaptation", "Necklace of Adaptation", slugs("necklace-of-adaptation"), "worn", "", "save_edge", "immunity edge_scope", "", "passive-ward")
T("necklace-of-fireballs", "Necklace of Fireballs", slugs("necklace-of-fireballs"), "consumed", "consumable spell", "", "", "spell-options charge-scaling depletion", "grants-spell", spells=("Fireball",), only_spell=True, flags="Co")
T("necklace-of-prayer-beads", "Necklace of Prayer Beads", slugs("necklace-of-prayer-beads"), "action", "spell", "", "", "spell-options use-limits variant", "grants-spell", spells=("Bless", "Cure Wounds", "Greater Restoration", "Shining Smite", "Guardian of Faith", "Wind Walk"), only_spell=True)
T("pearl-of-power", "Pearl of Power", slugs("pearl-of-power"), "action", "", "resource_change", "", "", "spell-resource", flags="Ch*", oneoff="restores a spell slot of level 3 or lower; resource_change addresses a named Resource, and spell slots are not one today")
T("periapt-of-health", "Periapt of Health", slugs("periapt-of-health"), "worn action", "", "save_edge", "heal edge_scope", "", "passive-ward", flags="Ch*")
T("periapt-of-proof-against-poison", "Periapt of Proof against Poison", slugs("periapt-of-proof-against-poison"), "worn", "resistance", "", "immunity resistance", "", "passive-ward")
T("periapt-of-wound-closure", "Periapt of Wound Closure", slugs("periapt-of-wound-closure"), "worn", "", "", "outcome_change", "", "passive-ward", oneoff="Hit Point Dice restore double")
T("pipes-of-haunting", "Pipes of Haunting", slugs("pipes-of-haunting"), "action", "", "area save_condition", "save_gate", "", "activated-effect", flags="Ch")
T("pipes-of-the-sewers", "Pipes of the Sewers", slugs("pipes-of-the-sewers"), "action", "", "summon", "save_gate compel", "charge-scaling", "summon", flags="Ch", monsters=("Swarm of Rats",))
T("robe-of-eyes", "Robe of Eyes", slugs("robe-of-eyes"), "worn", "sense skill_boost", "", "sense check_edge trigger save_gate apply_condition", "", "passive-trait")
T("robe-of-scintillating-colors", "Robe of Scintillating Colors", slugs("robe-of-scintillating-colors"), "action", "light", "light edge area save_condition", "save_gate", "", "activated-effect", flags="Ch")
T("robe-of-stars", "Robe of Stars", slugs("robe-of-stars"), "worn action", "bonus spell", "", "modifier banish", "spell-options use-limits", "spell-plus", spells=("Magic Missile",), flags="Ch*", note="Six stars that return 1d6 at dusk.")
T("robe-of-the-archmagi", "Robe of the Archmagi", slugs("robe-of-the-archmagi"), "worn", "bonus", "save_edge", "stat_override edge_scope modifier", "", "passive-ward")
T("robe-of-useful-items", "Robe of Useful Items", slugs("robe-of-useful-items"), "consumed", "consumable", "summon", "create_object roll_table", "depletion", "utility-object", flags="Co", monsters=("Riding Horse", "Mastiff"))
T("rope-of-climbing", "Rope of Climbing", slugs("rope-of-climbing"), "action", "skill_boost", "", "create_object check_edge note", "", "utility-object")
T("rope-of-entanglement", "Rope of Entanglement", slugs("rope-of-entanglement"), "action", "", "save_condition", "save_gate escape_check create_object", "", "restraint")
T("scarab-of-protection", "Scarab of Protection", slugs("scarab-of-protection"), "worn", "bonus", "save_edge", "modifier edge_scope outcome_change", "depletion", "passive-ward", flags="Ch")
T("slippers-of-spider-climbing", "Slippers of Spider Climbing", slugs("slippers-of-spider-climbing"), "worn", "speed", "", "speed", "", "passive-trait")
T("glue-and-solvent", "Sovereign Glue, Universal Solvent", slugs("sovereign-glue universal-solvent"), "consumed", "consumable", "", "note", "depletion", "utility-object", flags="Co")
T("sphere-of-annihilation", "Sphere of Annihilation", slugs("sphere-of-annihilation"), "action", "", "", "create_object save_gate damage slay roll_table banish", "utilize-check", "object-or-vehicle")
T("stone-of-good-luck", "Stone of Good Luck (Luckstone)", slugs("stone-of-good-luck-luckstone"), "worn", "bonus", "", "modifier", "", "flat-bonus")
T("talisman", "Talisman of Pure Good, Talisman of Ultimate Evil", slugs("talisman-of-pure-good talisman-of-ultimate-evil"), "worn action", "bonus", "branch save_edge", "modifier trigger damage save_gate slay", "depletion focus", "activated-effect", flags="Ch")
T("talisman-of-the-sphere", "Talisman of the Sphere", slugs("talisman-of-the-sphere"), "worn", "skill_boost", "", "check_edge note", "", "situational-check")
T("well-of-many-worlds", "Well of Many Worlds", slugs("well-of-many-worlds"), "action", "", "", "create_object banish", "use-limits", "planar-utility", note="The DM chooses where the portal leads; that is choosing a destination, as with Plane Shift, not resolving an outcome.")
T("wind-fan", "Wind Fan", slugs("wind-fan"), "action", "spell", "", "roll_table", "spell-options", "spell-plus", spells=("Gust of Wind",), note="A cumulative 20 percent chance to tear on each further use before dawn.")
T("winged-boots", "Winged Boots", slugs("winged-boots"), "action", "speed", "", "speed", "", "activated-effect", flags="Ch")
T("wings-of-flying", "Wings of Flying", slugs("wings-of-flying"), "action", "speed", "", "speed", "use-limits", "activated-effect")

# --- Items the seed does not flag as magic -------------------------------------------------------
T("ioun-stone", "Ioun Stone", slugs("ioun-stone"), "worn", "bonus", "counter", "modifier ability_score check_edge heal trigger spell_store immunity", "variant depletion", "passive-trait", note="Fourteen stones in one seed row. The two absorption stones cancel spells up to a level and burn out after 20 levels.")
T("spell-scroll", "Spell Scroll", slugs("spell-scroll"), "consumed", "consumable spell", "", "", "spell-options variant", "grants-spell", only_spell=True, flags="Co", note="Any spell; the seed row covers the cantrip and level 1 scrolls (save DC 13, +5 to hit).")
T("thrown-vial", "Acid, Holy Water", slugs("acid holy-water"), "consumed", "consumable", "branch", "save_gate damage", "", "consumable-effect", flags="Co")
T("alchemists-fire", "Alchemist's Fire", slugs("alchemists-fire"), "consumed", "consumable", "", "save_gate damage trigger", "", "consumable-effect", flags="Co", note="A failed save leaves the target Burning: fire damage at the start of each turn until an action puts it out.")
T("oil", "Oil", slugs("oil"), "consumed", "consumable", "choice create_surface area", "save_gate trigger damage", "", "hazard-area", flags="Co")
T("net", "Net", slugs("net"), "action", "", "", "save_gate apply_condition escape_check", "", "restraint")
T("torch", "Torch", slugs("torch"), "consumed", "consumable light", "light", "damage", "", "light-source", flags="Co")
T("plain-light", "Candle, Lamp, Hooded Lantern", slugs("candle lamp lantern-hooded"), "action", "light", "light choice", "", "", "light-source", note="The Candle burns down in an hour; Lamps and Lanterns burn Oil.")
T("lantern-bullseye", "Bullseye Lantern", slugs("lantern-bullseye"), "action", "light", "light", "", "", "light-source", oneoff="light shed in a Cone, not a radius")
T("ball-bearings", "Ball Bearings", slugs("ball-bearings"), "consumed", "", "area create_surface save_condition branch", "save_gate", "", "hazard-area")
T("caltrops", "Caltrops", slugs("caltrops"), "consumed", "", "area create_surface save_damage immobile branch", "save_gate", "", "hazard-area")
T("hunting-trap", "Hunting Trap", slugs("hunting-trap"), "action", "", "immobile", "save_gate damage trigger escape_check", "", "hazard-area")
T("binding-gear", "Chain, Manacles, Rope", slugs("chain manacles rope"), "action", "", "edge", "apply_condition escape_check", "utilize-check", "restraint")
T("cold-weather-bedding", "Bedroll, Blanket", slugs("bedroll blanket"), "worn", "", "save_edge", "edge_scope", "", "passive-ward", note="Against extreme cold only; the Bedroll makes the save succeed outright.")
T("situational-gear", "Gear that gives Advantage or a bonus on a fitting check", slugs("book map costume crowbar magnifying-glass perfume pole ram-portable saddle-military"), "worn", "skill_boost", "", "check_edge modifier", "", "situational-check", note="Whether the item applies (the Book is about the topic, the Crowbar has leverage) is the player's call to switch on, as with any situational Advantage.")
T("poison-basic", "Poison, Basic", slugs("poison-basic"), "consumed", "consumable", "", "trigger damage", "", "consumable-effect", flags="Co")
T("healers-kit", "Healer's Kit", slugs("healers-kit"), "action", "", "", "stabilize", "", "consumable-effect", flags="Ch*", note="Ten uses that never return fit the Charges model.")
T("climbers-kit", "Climber's Kit", slugs("climbers-kit"), "action", "", "", "note", "", "utility-object", note="Anchored, the user cannot fall or move more than 25 feet from the anchor.")
T("set-dc-gear", "Grappling Hook, Lock", slugs("grappling-hook lock"), "action", "", "", "", "utilize-check", "tool-check")
T("tools", "Artisan's tools, kits, gaming sets and musical instruments", "TOOLS", "action", "", "", "", "utilize-check", "tool-check", note="Each row is an ability, one or two Utilize checks with a DC, and a Craft list; crafting belongs to Recipes.")
T("spellcasting-focus", "Component Pouch, Holy Symbol, Druidic Focus", slugs("component-pouch holy-symbol-amulet holy-symbol-emblem holy-symbol-reliquary druidic-focus-sprig-of-mistletoe druidic-focus-wooden-staff druidic-focus-yew-wand"), "worn", "", "", "", "focus", "spellcasting-focus")

# Tools that are their own entry and so not part of the generic tools template.
TOOLS_OWN = {"climbers-kit", "healers-kit"}

# --- Items with no rules behaviour ---------------------------------------------------------------
EXCLUDED = [
    ("base weapon", "A plain weapon. Its damage, properties and Weapon Mastery live in the weapons table and are resolved by the attack rules.",
     lambda i: i["category"] == "weapon" and not i["magic"] and re.fullmatch(r"An? [a-z ]+\.", i["description"]) is not None),
    ("base armor", "Plain armor or a Shield. Its Armor Class, Stealth Disadvantage and Strength requirement live in the armor table.",
     lambda i: i["category"] == "armor" and not i["magic"]),
    ("ammunition", "Plain ammunition, spent by a weapon's Ammunition property.", lambda i: i["category"] == "ammunition"),
    ("mount", "A priced creature. Its rules are its stat block and the Mount model.", lambda i: i["category"] == "mount"),
    ("vehicle", "A priced vehicle. Its rules are the Vehicle model.", lambda i: i["category"] in ("land-vehicle", "waterborne-vehicle") or i["slug"] == "airship"),
    ("equipment pack", "A bundle of other items, or the Backpack that holds them.", lambda i: i["category"] == "equipment-pack"),
    ("container", "A mundane container; the text is a capacity.",
     lambda i: i["slug"] in slugs("barrel basket bottle-glass bucket case-crossbow-bolt case-map-or-scroll chest flask jug pot-iron pouch quiver sack vial waterskin")),
    ("descriptive gear", "Mundane gear whose text describes what it is or what it is for, with no roll, modifier, state or resource for the engine to resolve.",
     lambda i: i["slug"] in slugs("bell block-and-tackle clothes-fine clothes-travelers feed-per-day ink ink-pen ladder mirror paper parchment rations robe saddle-exotic saddle-riding shovel signal-whistle spikes-iron spyglass string tent tinderbox")),
]


# --- First pass ----------------------------------------------------------------------------------
FIRST_PASS = [
    ("tool-check", r"Utilize: "),
    ("flat-bonus", r"bonus to (attack rolls and damage rolls|Armor Class|spell attack rolls)"),
    ("grants-spell", r"\bcast\b|effect of the .+ spell"),
    ("summon", r"summon|becomes a living creature|appears in an unoccupied space"),
    ("passive-ward", r"Resistance to|Immunity to|Advantage on saving throws"),
    ("passive-trait", r"(Strength|Dexterity|Constitution|Intelligence|Wisdom|Charisma) (is \d+|increases by|changes)|Darkvision|Truesight|(Fly|Swim|Climb) Speed"),
    ("light-source", r"Bright Light"),
    ("activated-effect", r"saving throw"),
    ("situational-check", r"Advantage on .*checks?|bonus to .*checks"),
]


def first_pass(item):
    """A guess at the pattern from the description alone, and whether the guess is confident: one
    rule fires and the text is short enough to hold one behaviour."""
    hits = [name for name, pattern in FIRST_PASS if re.search(pattern, item["description"])]
    if not hits:
        return "", False
    return hits[0], len(hits) == 1 and len(item["description"]) <= 400


def auto_flags(item):
    d = item["description"]
    out = set()
    if re.search(r"\bcharges?\b", d):
        out.add("Ch")
    if re.search(r"\b[Cc]urse", d):
        out.add("Cu")
    if item["category"] == "potion" or re.search(r"enough of it for one use|is no longer magical|becomes nonmagical|ceases to be magical|token disappears|is consumed|scroll disintegrates", d):
        out.add("Co")
    return out


# --- Assembly ------------------------------------------------------------------------------------
def load():
    data = json.load(gzip.open(SEED))
    items = [i for i in data["items"] if i.get("document") == DOCUMENT]
    spells = {s["name"] for s in data["spells"] if s.get("document") == DOCUMENT}
    monsters = {m["name"] for m in data["monsters"] if m.get("document") == DOCUMENT}
    return items, spells, monsters


def matches(t, item):
    if t.cat and item["category"] != t.cat:
        return False
    if t.match == "TOOLS":
        return item["category"] == "tools" and item["slug"] not in TOOLS_OWN
    if isinstance(t.match, set):
        return item["slug"] in t.match
    return t.match.search(item["slug"]) is not None


def assign(items):
    excluded = defaultdict(list)
    problems = []
    for item in items:
        mine = [t for t in TEMPLATES if matches(t, item)]
        out = [e for e in EXCLUDED if e[2](item)]
        if len(mine) + len(out) != 1:
            problems.append((item["slug"], [t.key for t in mine], [e[0] for e in out]))
            continue
        if mine:
            mine[0].rows.append(item)
        else:
            excluded[out[0][0]].append(item)
    for t in TEMPLATES:
        if isinstance(t.match, set) and len(t.rows) != len(t.match):
            problems.append((t.key, "slug list does not match the seed", sorted(t.match - {i["slug"] for i in t.rows})))
        if not t.rows:
            problems.append((t.key, "no rows", []))
    return excluded, problems


def validate(spells, monsters):
    problems = []
    for t in TEMPLATES:
        for w in t.words("have"):
            if w not in HAVE:
                problems.append((t.key, "unknown existing kind", w))
        for w in t.words("miss"):
            if w not in MISSING:
                problems.append((t.key, "unknown missing kind", w))
        for w in t.words("gaps"):
            if w not in GAPS:
                problems.append((t.key, "unknown gap", w))
        for w in t.words("props"):
            if w not in PROPS:
                problems.append((t.key, "unknown Item Property", w))
        if t.pattern not in PATTERNS:
            problems.append((t.key, "unknown pattern", t.pattern))
        text = " ".join(i["description"] for i in t.rows).replace("/ ", "/")
        for s in t.spells:
            if s not in spells:
                problems.append((t.key, "spell not in the seed", s))
            if s not in text and t.key != "wand-spell-dc":
                problems.append((t.key, "spell not named in the description", s))
        for m in t.monsters:
            if m not in monsters:
                problems.append((t.key, "monster not in the seed", m))
        if t.only_spell and (t.miss or t.have or t.oneoff):
            problems.append((t.key, "marked spell-only but needs Effect kinds", t.miss or t.have or t.oneoff))
    return problems


def attuned(t):
    n = sum(1 for i in t.rows if i["requiresAttunement"])
    return n * 2 > len(t.rows)


def flag_text(t):
    out = []
    if "Ch*" in t.flags.split():
        out.append("Ch*")
    elif "Ch" in t.flags.split():
        out.append("Ch")
    if attuned(t):
        out.append("At")
    if "Co" in t.flags.split():
        out.append("Co")
    if "Cu" in t.flags.split():
        out.append("Cu")
    return " ".join(out) or "–"


def code(words):
    return ", ".join(f"`{w}`" for w in words) or "–"


def category(t):
    return Counter(i["category"] for i in t.rows).most_common(1)[0][0]


def table(templates):
    head = "| Entry | Category | Rows | Use | Item Properties | Effect kinds needed | Missing today | Flags | Only grants a spell | Pattern | Manual exception |"
    rule = "|---|---|---:|---|---|---|---|---|---|---|---|"
    lines = [head, rule]
    for t in templates:
        missing = []
        if t.miss:
            missing.append(code(t.words("miss")))
        if t.gaps:
            missing.append("item: " + ", ".join(t.words("gaps")))
        if t.oneoff:
            missing.append("one-off: " + t.oneoff)
        if not missing:
            missing.append("nothing")
        only = "–"
        if t.only_spell:
            only = "Yes: " + (", ".join(t.spells) if t.spells else "any spell")
        elif t.spells:
            only = "No (also casts " + ", ".join(t.spells) + ")"
        entry = f"**{t.name}**"
        if t.note:
            entry += f"<br>{t.note}"
        exception = t.exception or "No"
        lines.append("| " + " | ".join([
            entry, category(t), str(len(t.rows)), t.use.replace(" ", ", "), code(t.words("props")),
            code(t.words("have") + t.words("miss")), "; ".join(missing), flag_text(t), only, t.pattern, exception,
        ]) + " |")
    return "\n".join(lines)


def blockers(t):
    return set(t.words("miss")) | {"item: " + g for g in t.words("gaps")} | ({"one-off"} if t.oneoff else set())


def ranking(templates):
    need = defaultdict(list)
    for t in templates:
        for b in blockers(t):
            need[b].append(t)
    rows = []
    for b, ts in need.items():
        sole = [t for t in ts if blockers(t) == {b}]
        rows.append((b, len(ts), sum(len(t.rows) for t in ts), len(sole), sum(len(t.rows) for t in sole)))
    rows.sort(key=lambda r: (-r[1], -r[2], r[0]))
    return rows


def charges_in_migration():
    text = CHARGES_MIGRATION.read_text()
    return set(re.findall(r"^\s+\('([a-z0-9-]+)',", text, flags=re.M))


def generated(items, excluded, spells):
    templates = TEMPLATES
    classified = sum(len(t.rows) for t in templates)
    n_excluded = sum(len(v) for v in excluded.values())
    out = {}

    out["TABLE"] = table(templates)

    out["MISSING"] = "\n".join(f"- `{k}`: {v}" for k, v in MISSING.items())
    out["GAPS"] = "\n".join(f"- **{k}**: {v}" for k, v in GAPS.items())

    by_pattern = defaultdict(list)
    for t in templates:
        by_pattern[t.pattern].append(t)
    lines = ["| Pattern | What it is | Entries | Seed rows |", "|---|---|---:|---:|"]
    for p, ts in sorted(by_pattern.items(), key=lambda kv: (-len(kv[1]), kv[0])):
        lines.append(f"| {p} | {PATTERNS[p]} | {len(ts)} | {sum(len(t.rows) for t in ts)} |")
    lines.append(f"| **Total** | | **{len(templates)}** | **{classified}** |")
    out["PATTERNS"] = "\n".join(lines)

    lines = ["| Missing | Kind | Entries that need it | Seed rows | Entries it alone completes | Seed rows |", "|---|---|---:|---:|---:|---:|"]
    for b, n, r, sole, sole_rows in ranking(templates):
        kind = "item mechanic" if b.startswith("item: ") else ("one-off" if b == "one-off" else "Effect component")
        label = b if kind != "Effect component" else f"`{b}`"
        lines.append(f"| {label} | {kind} | {n} | {r} | {sole} | {sole_rows} |")
    out["RANKING"] = "\n".join(lines)

    clean = [t for t in templates if t.clean()]
    clean_rows = sum(len(t.rows) for t in clean)
    lines = ["| Entry | Rows | Pattern | Item Properties | Effect kinds | Flags |", "|---|---:|---|---|---|---|"]
    for t in clean:
        lines.append(f"| {t.name} | {len(t.rows)} | {t.pattern} | {code(t.words('props'))} | {code(t.words('have'))} | {flag_text(t)} |")
    out["TODAY"] = "\n".join(lines)

    lines = ["| Reason | Count | Why | Items |", "|---|---:|---|---|"]
    for name, why, _ in EXCLUDED:
        rows = excluded[name]
        lines.append(f"| {name} | {len(rows)} | {why} | {'; '.join(sorted(i['name'] for i in rows))} |")
    lines.append(f"| **Total** | **{n_excluded}** | | |")
    out["EXCLUDED"] = "\n".join(lines)

    lines = []
    for t in templates:
        if t.exception:
            text = t.exception.removeprefix("Yes, ").removeprefix("Yes: ")
            lines.append(f"- **{t.name}.** {text[0].upper()}{text[1:]}")
    out["EXCEPTIONS"] = "\n".join(lines)

    spell_only = [t for t in templates if t.only_spell]
    spell_some = [t for t in templates if t.spells and not t.only_spell]
    named = sorted({s for t in templates for s in t.spells})
    with_flag = lambda f: [t for t in templates if f in flag_text(t).split()]
    in_migration = charges_in_migration()
    charged = {i["slug"] for t in templates if "Ch" in t.flags.split() for i in t.rows}
    magic = sum(1 for i in items if i["magic"])
    unflagged = sorted(i["name"] for t in templates for i in t.rows if not i["magic"] and t.key in ("ioun-stone", "spell-scroll", "potion-of-healing", "potion-of-giant-strength"))

    agree = confident = confident_agree = guessed = 0
    for t in templates:
        for i in t.rows:
            guess, sure = first_pass(i)
            guessed += bool(guess)
            agree += guess == t.pattern
            confident += sure
            confident_agree += sure and guess == t.pattern

    facts = {
        "items": len(items), "magic": magic, "classified": classified, "excluded": n_excluded,
        "entries": len(templates), "templated_entries": sum(1 for t in templates if len(t.rows) > 1),
        "templated_rows": sum(len(t.rows) for t in templates if len(t.rows) > 1),
        "clean": len(clean), "clean_rows": clean_rows,
        "spell_only": len(spell_only), "spell_only_rows": sum(len(t.rows) for t in spell_only),
        "spell_some": len(spell_some), "spells_named": len(named),
        "ch": len(with_flag("Ch")), "ch_rows": sum(len(t.rows) for t in with_flag("Ch")),
        "chstar": len(with_flag("Ch*")), "at": len(with_flag("At")), "at_rows": sum(len(t.rows) for t in with_flag("At")),
        "co": len(with_flag("Co")), "co_rows": sum(len(t.rows) for t in with_flag("Co")),
        "cu": len(with_flag("Cu")), "cu_rows": sum(len(t.rows) for t in with_flag("Cu")),
        "exceptions": sum(1 for t in templates if t.exception),
        "missing_kinds": len({w for t in templates for w in t.words("miss")}),
        "gaps": len({w for t in templates for w in t.words("gaps")}),
        "oneoffs": sum(1 for t in templates if t.oneoff),
        "charges_migration": len(in_migration), "charges_census": len(charged),
        "charges_not_in_migration": ", ".join(
            (f"{t.name} ({len(missing)} rows)" if len(missing) > 1 else t.name) if len(missing) == len(t.rows) else ", ".join(i["name"] for i in missing)
            for t in templates
            for missing in [[i for i in t.rows if "Ch" in t.flags.split() and i["slug"] not in in_migration]] if missing),
        "charges_only_in_migration": ", ".join(sorted(in_migration - charged)) or "none",
        "unflagged_magic": ", ".join(unflagged),
        "first_guessed": guessed, "first_agree": agree, "first_confident": confident, "first_confident_agree": confident_agree,
        "seed_attuned": sum(1 for i in items if i["requiresAttunement"]),
        "spell_list": ", ".join(named),
    }
    f = facts
    out["HEADLINE"] = "\n".join([
        f"- The seed holds **{f['items']}** SRD 5.2 items, {f['magic']} of them flagged magic.",
        f"- **{f['classified']}** have rules behaviour. Grouped, they are **{f['entries']} entries**: {f['templated_entries']} templates that cover {f['templated_rows']} seed rows, and {f['entries'] - f['templated_entries']} single items.",
        f"- **{f['excluded']}** have no rules behaviour of their own and are excluded.",
        f"- **{f['clean']} entries ({f['clean_rows']} seed rows)** can be authored with today's Effect component kinds and item mechanics alone.",
        f"- The other {f['entries'] - f['clean']} entries wait on {f['missing_kinds']} missing Effect component kinds, {f['gaps']} missing item mechanics and {f['oneoffs']} one-off mechanics.",
        f"- **{f['spell_only']} entries ({f['spell_only_rows']} seed rows)** only grant spells; {f['spell_some']} more grant spells beside other behaviour. Between them they name {f['spells_named']} distinct spells, all present in the seed.",
        f"- Charges: {f['ch']} entries ({f['ch_rows']} seed rows), plus {f['chstar']} entries with a single limited use that fits the Charges model. Attunement: {f['at']} entries ({f['at_rows']} seed rows). Consumable use: {f['co']} entries ({f['co_rows']} seed rows). A curse: {f['cu']} entries ({f['cu_rows']} seed rows).",
        f"- **{f['exceptions']} entries** are proposed for the Manual exception list, each for part of its behaviour only.",
    ])
    out["CHARGES"] = (
        f"`compendium.item_charges` (migration 00117) lists {f['charges_migration']} slugs. The census finds Charges on {f['charges_census']} seed rows. "
        f"Every slug in the migration is in the census. The census rows with no row in the migration are: {f['charges_not_in_migration']}."
    )
    out["SPELLS"] = f["spell_list"] + "."
    lines = ["| Built together | Entries completed | Seed rows |", "|---|---:|---:|"]
    sets = [
        ("modifier",),
        ("modifier", "trigger", "damage"),
        ("modifier", "trigger", "damage", "save_gate"),
        ("modifier", "trigger", "damage", "save_gate", "apply_condition", "slay"),
        ("modifier", "resistance", "edge_scope", "check_edge", "speed", "sense", "ability_score", "immunity", "proficiency"),
        ("item: spell-options", "item: depletion", "item: use-limits", "item: variant"),
    ]
    for group in sets:
        ts = [t for t in templates if blockers(t) and blockers(t) <= set(group)]
        names = ", ".join(g if g.startswith("item: ") else f"`{g}`" for g in group)
        lines.append(f"| {names} | {len(ts)} | {sum(len(t.rows) for t in ts)} |")
    out["CLUSTERS"] = "\n".join(lines)
    out["FIRSTPASS"] = (
        f"The first pass guessed a pattern for {f['first_guessed']} of the {f['classified']} classified rows and was confident (one rule fired, text of 400 characters or fewer) for {f['first_confident']}. "
        f"Its confident guesses match the final pattern for {f['first_confident_agree']} rows; over all rows it matches for {f['first_agree']}. "
        "Most misses are magic weapons whose text opens with a flat bonus and goes on to a rider."
    )
    return out, facts


def write_csv(excluded):
    with OUT_CSV.open("w", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        w.writerow(["slug", "name", "category", "magic", "requires_attunement", "template", "template_name", "pattern"])
        rows = []
        for t in TEMPLATES:
            for i in t.rows:
                rows.append([i["slug"], i["name"], i["category"], str(i["magic"]).lower(), str(i["requiresAttunement"]).lower(), t.key, t.name, t.pattern])
        for name, items in excluded.items():
            for i in items:
                rows.append([i["slug"], i["name"], i["category"], str(i["magic"]).lower(), str(i["requiresAttunement"]).lower(), "excluded", "excluded: " + name, ""])
        rows.sort()
        w.writerows(rows)
    return len(rows)


def splice(sections):
    text = OUT_MD.read_text()
    for name, body in sections.items():
        begin, end = f"<!-- BEGIN {name} -->", f"<!-- END {name} -->"
        if begin not in text:
            continue
        head, rest = text.split(begin, 1)
        _, tail = rest.split(end, 1)
        text = head + begin + "\n" + body + "\n" + end + tail
    OUT_MD.write_text(text)


def main():
    items, spells, monsters = load()
    excluded, problems = assign(items)
    problems += validate(spells, monsters)
    if problems:
        for p in problems:
            print("PROBLEM", p, file=sys.stderr)
        return 1
    sections, facts = generated(items, excluded, spells)
    n = write_csv(excluded)
    assert n == len(items) == facts["classified"] + facts["excluded"], (n, len(items))

    # Cross-check the hand-set flags against what the text says; differences are printed for review.
    for t in TEMPLATES:
        auto = set().union(*(auto_flags(i) for i in t.rows))
        hand = {f for f in t.flags.split() if f != "Ch*"}
        if auto != hand:
            print(f"review flags {t.key}: text suggests {sorted(auto)}, hand says {sorted(hand)}", file=sys.stderr)

    if OUT_MD.exists():
        splice(sections)
    for k, v in facts.items():
        print(f"{k}: {v}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
