#!/usr/bin/env python3
"""Census of the Effect component kinds every SRD 5.2 spell needs.

Reads the Compendium seed, makes a first-pass classification from the structured fields and the
description text, lays the hand classification over it, and writes census-spells.md next to this file.

    python3 docs/research/census-spells.py            # rewrite census-spells.md
    python3 docs/research/census-spells.py --diff     # also list where the first pass and the hand rows differ

The hand rows in HAND are the census; the first pass is the cross-check. To change a row, edit HAND
and run the script again. Never edit census-spells.md by hand.
"""

import collections
import gzip
import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent.parent
SEED = ROOT / "api" / "db" / "seeds" / "compendium.json.gz"
OUT = HERE / "census-spells.md"
DOCUMENT = "srd-2024"

# Component kinds the engine has today: the Go union in api/internal/rules/effects.
EXISTING = {
    "area": "the template: shape, size, range",
    "area_save": "the save every creature in an area makes (builder only, no catalogue table)",
    "save_damage": "damage to every creature in the area, halved or avoided on a save",
    "save_condition": "a condition on every creature in the area that fails the save",
    "create_surface": "a Surface on the area for some rounds",
    "forced_move": "push or pull creatures that fail, straight away from or toward the origin",
    "reveal": "strip Visibility Qualities in the area",
    "light": "bright and dim light where the Effect lands (builder only, no catalogue table)",
    "bonus_die": "a die added to attack rolls, saving throws, or both",
    "edge": "advantage or disadvantage on the bearer's attacks or on attacks against it",
    "save_edge": "advantage, disadvantage or sure failure on saves of one ability",
    "extra_damage": "extra dice when the Effect's source hits the bearer",
    "move_cost": "a multiplier on what each foot of movement costs",
    "speed_penalty": "feet taken off the bearer's Speed",
    "crit_within": "hits from within some feet are Critical Hits",
    "incapacitated": "no actions, Bonus Actions or Reactions",
    "immobile": "Speed held at 0",
    "exhausting": "a stacking level with a penalty per level",
    "reacts": "a Reaction on a trigger, resolved by the DM from an instruction",
    "temp_hp": "a fixed number of Temporary Hit Points",
    "teleport": "the caster teleports to an open hex within range",
    "dispel": "every spell on the bearer ends",
    "counter": "a Reaction that makes a spell cast within range fail",
    "grant_feature": "a named feature, noted to the DM",
    "resource_change": "uses of a Resource given back or taken",
    "summon": "a fixed count of one monster under the caster's control",
    "form": "the bearer takes a creature's stat block",
    "choice": "modes; whoever applies the Effect picks one",
    "branch": "extra parts when a save fails by N, Hit Points are at most N, on the first touch in a turn, or for a creature type",
    "manual": "a part the DM resolves from a prompt",
}

# Kinds no Effect can state today. Names follow ARCHITECTURE.md section 9.3 where it has one.
MISSING = {
    "save_gate": "A saving throw made by chosen creatures rather than by everyone in an `area`: the cast names its targets, rolls each save against the spell save DC, and fires the parts under it on a failure or a success. Today a save is rolled only by creatures inside an `area`. (`SaveGate` in ARCHITECTURE.md 9.3.)",
    "attack_gate": "A melee or ranged spell attack roll against a chosen target, with parts that fire on a hit and optionally on a miss. (`AttackGate` in 9.3.)",
    "check_gate": "An ability check that decides an outcome: the action a Restrained creature takes to break free, the caster's check in Dispel Magic, the Investigation check out of Maze.",
    "damage": "Damage that is not behind an area save: on a hit, on a miss, with no roll to avoid it (Magic Missile), or death outright under a Hit Point threshold. Covers flat modifiers and the spellcasting ability modifier. (`Damage` in 9.3.)",
    "heal": "Hit Points restored: dice plus the spellcasting ability modifier, a fixed amount, all of them, or a pool the caster divides. Also a change to healing the bearer receives. (`Heal` in 9.3.)",
    "revive": "A dead creature returns to life with stated Hit Points, within a time limit; also making a dying creature Stable.",
    "end_effect": "Ends or suppresses named conditions, curses or Effects on the target (Lesser Restoration), as opposed to `dispel`, which ends every spell.",
    "apply_condition": "A condition put on the bearer with no area save: outright (Invisibility), under a `save_gate` or `attack_gate`, or under a Hit Point threshold. (`ApplyCondition` in 9.3.)",
    "timing": "When a part fires after the cast: on entering the area, at the start or end of a turn (the creature's or the caster's), once or every turn, with its own save, and counting successes and failures where the spell asks for three of a kind. Today the only later timing is save-less damage from a Surface.",
    "trigger": "Parts that fire on an event rather than a turn boundary: the bearer hits, is hit, takes damage, drops to 0 Hit Points, is targeted, or the Effect ends. Typed parts, unlike `reacts`, whose outcome is a sentence for the DM. (`Trigger` in 9.3.)",
    "ends_when": "An early end other than a repeat save or lost concentration: the target takes damage, attacks or casts, moves out of range, dons armor, or a counter runs out.",
    "grant_action": "While the Effect lasts, the caster or bearer gains an action, Bonus Action or Magic action that fires typed parts again, moves the Effect, or switches its mode (Call Lightning, Spiritual Weapon, Moonbeam). `grant_feature` only notes a name to the DM.",
    "target_filter": "Targeting that picks: only creatures the caster chooses inside an area, allies only, enemies only, or a cap on how many. Today an `area` touches every creature in it.",
    "ac_modifier": "A bonus or penalty to Armor Class, or a new base or floor for it (Shield, Mage Armor, Barkskin).",
    "resistance": "Resistance, Immunity or Vulnerability to damage types, or damage taken reduced by a die.",
    "condition_immunity": "The bearer cannot gain named conditions, from anyone or from stated creature types.",
    "speed_modifier": "A Speed bonus or multiplier, a granted Fly, Climb or Swim Speed, hovering, jumping, or ignoring Difficult Terrain. `speed_penalty` only subtracts feet.",
    "roll_modifier": "Modifiers on D20 Tests that `bonus_die`, `edge` and `save_edge` cannot state: ability checks and Initiative, a subtracted die, a flat number, a replaced roll, and an edge or automatic result that depends on who rolls or attacks (creature type, size, a Sense) or on the circumstance.",
    "attack_modifier": "A change to the bearer's own attacks: a bonus to hit and damage, extra or fewer damage dice, a new damage die or type, the spellcasting ability in place of Strength. `extra_damage` only covers hits by the Effect's source against the bearer.",
    "action_economy": "Actions, Bonus Actions, Reactions or whole turns granted or taken away: Haste's extra action, Slow's limit, Dash as a Bonus Action, no Opportunity Attacks, Time Stop's extra turns.",
    "restriction": "Something the bearer cannot do that is not a condition: regain Hit Points, cast spells, speak or lie, make ranged attacks, be raised as Undead.",
    "compel": "The spell dictates what the target does on its turn: flee, approach, halt, drop what it holds, dance in place, attack at random, or obey the caster outright.",
    "banish": "Takes a creature off the Map for a time and puts it back where it left, or sends it to another plane for good.",
    "create_object": "Places a thing on the Map that is not a creature: a wall, dome, cage, sensor, floating weapon or hand, with a position and where it applies Armor Class, Hit Points, cover and what it blocks. (`CreateObject` in 9.3.)",
    "sense": "Grants a Sense with a range (Darkvision, Truesight, seeing the Invisible) or sight and hearing through a remote sensor. (`Sense` in 9.3.)",
    "detect": "Tells the caster which things of a named class are within range: magic, poison, traps, creature types, a known creature or object, an item's properties. `reveal` strips Visibility Qualities for everyone; this informs one creature.",
    "grant_quality": "Gives the bearer a Visibility Quality other than Invisible: Disguised with a shown identity, Ethereal, or Hidden.",
    "create_item": "Creates, conjures, stores or recalls items: berries, food and water, a chest kept on another plane.",
    "travel": "Moves creatures off the current Map: to a known place, a teleportation circle, another plane, or an extradimensional room.",
    "hp_max": "Raises or lowers the Hit Point maximum.",
    "ward": "An area or bearer that blocks spells of some level, teleportation, planar travel, Divination, sound, or the passage of creatures of stated types.",
    "alter_map": "Changes Map Objects, terrain or weather: locks and unlocks, opens a passage, reshapes earth or water, destroys an object.",
    "message": "Opens a private channel between creatures regardless of distance.",
    "roll_table": "Rolls a die on a table and applies the row's parts (Confusion, Prismatic Spray, Reincarnate, Teleport).",
    "create_trap": "Places a Trap or stored spell that fires its Effect when a trigger area is entered or touched.",
    "stat_modifier": "Sets or changes a statistic that is not Speed or Armor Class: size, an ability score, creature type, languages understood, breathing water.",
}

# Kinds the engine has, but too narrow for the spells marked with them.
WIDEN = {
    "area+": "An area that moves after the cast (with the caster's action, on its own each turn, or with the object it is cast on), or several areas in one cast (Meteor Swarm's four spheres, Fire Storm's ten cubes).",
    "save_damage+": "More than one damage roll or type under one save (Flame Strike), or a flat number added to or in place of the dice (Disintegrate's 10d6 + 40, Guardian of Faith's 20).",
    "create_surface+": "A lightly obscured Surface.",
    "forced_move+": "Movement in a direction the caster picks, upward, or by a distance worked out from an ability modifier. Today it is straight away from or toward the origin.",
    "light+": "Light that travels with the creature or object that bears it, dim light alone, and sunlight.",
    "temp_hp+": "Temporary Hit Points from dice or from an ability modifier, and granted again each turn.",
    "teleport+": "Teleporting someone or something other than the caster, bringing a creature along, a destination the caster cannot see, and repeated use while the Effect lasts.",
    "dispel+": "Ending only spells up to a level, with a check for higher ones, or only spells of one kind (light, darkness).",
    "resource_change+": "Everything a Short Rest gives back, not one named Resource.",
    "summon+": "A stat block chosen from a list at cast, a count or statistics that grow with the slot, a creature made from a corpse or object, a control period that must be renewed.",
    "form+": "Limits on the form (creature type, Challenge Rating against the target's level), what the bearer keeps, and forms that are objects.",
}

PATTERNS = {
    "AH": "save against damage in an area",
    "AC": "save or suffer a condition in an area",
    "AR": "area save with damage and a rider",
    "LZ": "lasting area that saves on entry or by turn",
    "LT": "lasting area of terrain, obscurement, light or silence",
    "AA": "aura around the caster",
    "SA": "spell attack for damage",
    "SAR": "spell attack for damage and a rider",
    "SH": "chosen creature saves against damage",
    "SHR": "chosen creature saves against damage and a rider",
    "SC": "chosen creatures save or suffer a condition",
    "SB": "lasting penalty on chosen creatures",
    "SK": "save or the caster dictates behaviour",
    "AU": "automatic damage or an outcome set by Hit Points",
    "HE": "healing",
    "RD": "raising the dead",
    "RC": "removing conditions or curses",
    "DS": "dispelling or countering magic",
    "BU": "lasting buff on chosen creatures or the caster",
    "WE": "weapon or attack enhancement",
    "RX": "reaction spell",
    "SU": "summoned or created creatures",
    "CF": "lasting spell re-fired or moved with later actions",
    "CO": "conjured object or helper without an attack",
    "TR": "transformation",
    "TP": "teleport on the Map",
    "TV": "travel off the Map",
    "BN": "banishing or leaving the plane",
    "FM": "lifting or moving by force",
    "WL": "wall or barrier",
    "WD": "ward against magic or creatures",
    "IV": "invisibility, disguise, hiding or decoys",
    "SD": "senses and detection",
    "DV": "knowledge the DM supplies",
    "IL": "free-form illusion",
    "FF": "free-form creation, shaping or minor magic",
    "SG": "free-form instruction to a creature",
    "IT": "items created, moved or changed",
    "TS": "trap, alarm or stored spell",
    "MP": "change to the Map, terrain, objects or weather",
    "CM": "communication and language",
}

# Spells with a seeded Effect today (api/db/migrations 00045, 00063, 00069, 00073, 00075).
SEEDED = [
    "animate-dead", "bless", "burning-hands", "cone-of-cold", "counterspell", "dispel-magic", "faerie-fire", "false-life",
    "find-familiar", "fireball", "grease", "hellish-rebuke", "hunters-mark", "lightning-bolt", "misty-step", "polymorph",
    "shatter", "spirit-guardians", "thunderwave", "wall-of-fire",
]

# The census. One row per spell: slug | pattern | kinds needed | Scaling | reason, when proposed as an exception.
# Scaling is "-" (the spell does not grow), "yes" (dice by slot level or by character-level steps), or "no: what grows".
HAND = """
acid-arrow | SAR | attack_gate damage timing | yes
acid-splash | AH | area save_damage | yes
aid | BU | hp_max | no: flat amount
alarm | TS | area create_trap | -
alter-self | TR | choice speed_modifier grant_quality attack_modifier grant_action | -
animal-friendship | SC | save_gate save_condition branch ends_when | no: targets
animal-messenger | CM | save_gate manual | no: duration | a described recipient at a distant place and a free-form message; nothing on the Map to resolve
animal-shapes | TR | form+ grant_action | -
animate-dead | SU | summon+ choice | no: count
animate-objects | SU | summon+ | no: summon stat block
antilife-shell | WD | area ward | -
antimagic-field | WD | area ward | -
antipathysympathy | LZ | area choice timing save_condition compel ends_when | -
arcane-eye | SD | sense create_object grant_action | -
arcane-hand | CF | create_object grant_action choice attack_gate damage save_gate forced_move+ apply_condition check_gate | no: two dice steps
arcane-lock | MP | alter_map | -
arcane-sword | CF | create_object attack_gate damage grant_action | -
arcanists-magic-aura | WD | choice stat_modifier ward | -
astral-projection | TV | travel apply_condition | -
augury | DV | manual | - | the DM chooses the omen from what will happen in the story
aura-of-life | AA | area target_filter resistance timing heal | -
awaken | TR | stat_modifier apply_condition ends_when | -
bane | SB | save_gate roll_modifier | no: targets
banishment | BN | save_gate banish incapacitated branch | no: targets
barkskin | BU | ac_modifier | -
beacon-of-hope | BU | save_edge roll_modifier heal | -
befuddlement | SHR | save_gate save_damage restriction | -
bestow-curse | SB | save_gate choice save_edge roll_modifier edge timing action_economy extra_damage | no: duration and concentration
black-tentacles | LZ | area create_surface save_damage save_condition timing check_gate | -
blade-barrier | LZ | area create_surface save_damage timing create_object | -
bless | BU | bonus_die | no: targets
blight | SH | save_gate save_damage branch roll_modifier | yes
blindnessdeafness | SC | save_gate choice save_condition | no: targets
blink | BN | timing roll_table banish teleport | -
blur | BU | edge roll_modifier | -
burning-hands | AH | area save_damage | yes
call-lightning | CF | area save_damage grant_action | yes
calm-emotions | RC | area area_save branch choice condition_immunity end_effect ends_when | -
chain-lightning | SH | save_gate save_damage | -
charm-monster | SC | save_gate save_condition ends_when roll_modifier | no: targets
charm-person | SC | save_gate save_condition branch ends_when roll_modifier | no: targets
chill-touch | SAR | attack_gate damage restriction | yes
chromatic-orb | SAR | attack_gate damage choice trigger | no: leaps
circle-of-death | AH | area save_damage | yes
clairvoyance | SD | sense grant_action | -
clone | RD | trigger revive | -
cloudkill | LZ | area+ create_surface save_damage timing | yes
color-spray | AC | area save_condition | -
command | SK | save_gate choice compel save_condition | no: targets
commune | DV | manual | - | the DM answers the questions as the deity
commune-with-nature | DV | manual | - | the DM supplies facts about the surrounding region
comprehend-languages | CM | stat_modifier | -
compulsion | SK | save_gate save_condition grant_action compel timing | -
cone-of-cold | AH | area save_damage | yes
confusion | SK | area area_save action_economy timing roll_table compel | no: area
conjure-animals | LZ | area+ create_object target_filter timing save_damage save_edge | yes
conjure-celestial | LZ | area+ light target_filter choice timing heal save_damage | yes
conjure-elemental | LZ | create_object choice timing save_gate save_damage save_condition | yes
conjure-fey | CF | create_object attack_gate damage apply_condition grant_action teleport+ | yes
conjure-minor-elementals | AA | area attack_modifier create_surface target_filter | yes
conjure-woodland-beings | LZ | area+ target_filter timing save_damage action_economy | yes
contact-other-plane | DV | manual save_gate damage incapacitated | - | the DM answers the questions as the contacted entity
contagion | SHR | save_gate save_damage save_condition choice save_edge timing | -
contingency | TS | create_trap trigger | -
continual-flame | LT | light+ | -
control-water | MP | area choice alter_map grant_action timing save_damage forced_move check_gate | -
control-weather | MP | alter_map | -
counterspell | DS | counter save_gate | -
create-food-and-water | IT | create_item | -
create-or-destroy-water | IT | choice create_item alter_map | no: amount or area
create-undead | SU | summon+ | no: count and stat block
creation | FF | manual | no: area | the object is whatever the caster describes, within a size limit
cure-wounds | HE | heal | yes
dancing-lights | LT | light+ create_object grant_action | -
darkness | LT | area+ create_surface dispel+ | -
darkvision | SD | sense | -
daylight | LT | area+ light+ dispel+ | -
death-ward | BU | trigger | -
delayed-blast-fireball | AH | area save_damage timing trigger | yes
demiplane | TV | travel create_object | -
detect-evil-and-good | SD | detect | -
detect-magic | SD | detect grant_action | -
detect-poison-and-disease | SD | detect | -
detect-thoughts | DV | manual detect save_gate check_gate grant_action | - | the DM says what the target is thinking
dimension-door | TP | teleport+ damage | -
disguise-self | IV | grant_quality | -
disintegrate | SH | save_gate save_damage+ alter_map | yes
dispel-evil-and-good | WD | edge roll_modifier grant_action end_effect save_gate banish | -
dispel-magic | DS | dispel+ check_gate | no: level threshold
dissonant-whispers | SHR | save_gate save_damage compel | -
divination | DV | manual | - | the DM gives the reply
divine-favor | WE | attack_modifier | -
divine-smite | WE | trigger damage branch | yes
divine-word | AU | save_gate target_filter branch save_condition damage banish | -
dominate-beast | SK | save_gate save_condition branch compel trigger roll_modifier | no: duration
dominate-monster | SK | save_gate save_condition compel trigger roll_modifier | no: duration
dominate-person | SK | save_gate save_condition branch compel trigger roll_modifier | no: duration
dragons-breath | CF | choice grant_action area save_damage | yes
dream | CM | manual save_gate damage | - | a conversation and a dreamscape narrated at the table
druidcraft | FF | manual | - | no rules outcome: harmless sensory effects narrated at the table
earthquake | LZ | area create_surface area_save save_condition timing alter_map save_damage check_gate | -
eldritch-blast | SA | attack_gate damage | no: beams
elementalism | FF | manual | - | no rules outcome: harmless elemental effects narrated at the table
enhance-ability | BU | choice roll_modifier | no: targets
enlargereduce | TR | choice save_gate stat_modifier roll_modifier save_edge attack_modifier | -
ensnaring-strike | WE | trigger save_gate save_condition timing damage check_gate roll_modifier | yes
entangle | AC | area create_surface save_condition check_gate | -
enthrall | SB | save_gate target_filter roll_modifier | -
etherealness | BN | grant_quality restriction | no: targets
expeditious-retreat | BU | action_economy | -
eyebite | SC | save_gate choice save_condition ends_when compel grant_action | -
fabricate | FF | manual | - | the product is whatever the caster designs from the raw materials
faerie-fire | AC | area area_save save_condition edge reveal light+ | -
faithful-hound | CF | create_object sense timing save_gate save_damage grant_action | -
false-life | BU | temp_hp+ | no: flat amount
fear | SK | area save_condition compel timing | -
feather-fall | RX | reacts trigger resistance | -
find-familiar | SU | summon+ sense grant_action | -
find-steed | SU | summon+ | no: summon stat block
find-the-path | DV | manual | - | the route through the world is the DM's to describe
find-traps | SD | detect | -
finger-of-death | SH | save_gate save_damage+ trigger summon | -
fire-bolt | SA | attack_gate damage | yes
fire-shield | BU | choice resistance trigger damage light+ | -
fire-storm | AH | area+ save_damage | -
fireball | AH | area save_damage | yes
flame-blade | CF | grant_action attack_gate damage light+ | yes
flame-strike | AH | area save_damage+ | yes
flaming-sphere | CF | create_object timing save_damage grant_action light+ | yes
flesh-to-stone | SC | save_gate save_condition immobile branch timing roll_modifier | -
floating-disk | CO | create_object | -
fly | BU | speed_modifier | no: targets
fog-cloud | LT | area create_surface | no: area
forbiddance | WD | area ward choice timing damage branch | -
forcecage | WL | create_object choice ward save_gate | -
foresight | BU | roll_modifier edge | -
freedom-of-movement | BU | condition_immunity speed_modifier | no: targets
freezing-sphere | AH | area save_damage alter_map create_item | yes
gaseous-form | TR | speed_modifier resistance condition_immunity save_edge restriction ends_when | no: targets
gate | TV | travel create_object | -
geas | SG | manual save_gate save_condition damage | no: duration | a free-form command; whether the creature acts against it is the DM's call
gentle-repose | RD | restriction | -
giant-insect | SU | summon+ | no: summon stat block
glibness | BU | roll_modifier ward | -
globe-of-invulnerability | WD | area ward | no: level threshold
glyph-of-warding | TS | create_trap choice area save_damage | no: stored spell level
goodberry | IT | create_item heal | -
grease | LZ | area create_surface save_condition timing | -
greater-invisibility | IV | apply_condition | -
greater-restoration | RC | choice end_effect | -
guardian-of-faith | LZ | create_object area target_filter timing save_damage+ ends_when | -
guards-and-wards | WD | area ward alter_map create_surface roll_table create_trap | -
guidance | BU | roll_modifier | -
guiding-bolt | SAR | attack_gate damage edge ends_when | yes
gust-of-wind | LZ | area area_save forced_move timing move_cost grant_action | -
hallow | WD | area ward choice branch condition_immunity resistance create_surface light | -
hallucinatory-terrain | IL | manual | - | the look of the false terrain and whether it fools anyone are narrated
harm | SHR | save_gate save_damage hp_max | -
haste | BU | speed_modifier ac_modifier save_edge action_economy trigger incapacitated immobile | -
heal | HE | heal end_effect | no: flat amount
healing-word | HE | heal | yes
heat-metal | CF | damage grant_action save_gate edge roll_modifier | yes
hellish-rebuke | RX | reacts save_gate save_damage | yes
heroes-feast | BU | resistance condition_immunity hp_max | -
heroism | BU | condition_immunity temp_hp+ timing | no: targets
hex | SB | extra_damage choice roll_modifier grant_action | no: duration
hideous-laughter | SC | save_gate save_condition incapacitated trigger roll_modifier | no: targets
hold-monster | SC | save_gate save_condition | no: targets
hold-person | SC | save_gate save_condition branch | no: targets
holy-aura | AA | area target_filter save_edge edge trigger save_gate save_condition branch | -
hunters-mark | SB | extra_damage roll_modifier grant_action | no: duration
hypnotic-pattern | AC | area save_condition incapacitated immobile ends_when | -
ice-knife | SAR | attack_gate damage area save_damage | yes
ice-storm | AH | area save_damage+ create_surface | yes
identify | SD | detect | -
illusory-script | IL | manual | - | no rules outcome: hidden or altered writing narrated at the table
imprisonment | BN | save_gate choice banish save_condition ward | -
incendiary-cloud | LZ | area+ create_surface save_damage timing | -
inflict-wounds | SH | save_gate save_damage | yes
insect-plague | LZ | area create_surface+ save_damage timing | yes
instant-summons | IT | create_item grant_action | -
invisibility | IV | apply_condition ends_when | no: targets
irresistible-dance | SK | save_gate save_condition compel save_edge edge grant_action | -
jump | BU | speed_modifier | no: targets
knock | MP | alter_map | -
legend-lore | DV | manual | - | the DM supplies the lore
lesser-restoration | RC | choice end_effect | -
levitate | FM | save_gate forced_move+ immobile grant_action | -
light | LT | light+ | -
lightning-bolt | AH | area save_damage | yes
locate-animals-or-plants | DV | manual | - | whether and where such a creature or plant exists within 5 miles is the DM's to say
locate-creature | SD | detect | -
locate-object | SD | detect | -
longstrider | BU | speed_modifier | no: targets
mage-armor | BU | ac_modifier ends_when | -
mage-hand | CO | create_object grant_action | -
magic-circle | WD | area ward choice branch edge roll_modifier condition_immunity save_gate | no: duration
magic-jar | SK | save_gate compel form+ incapacitated trigger | -
magic-missile | AU | damage | no: darts
magic-mouth | TS | manual | - | a free-form trigger and a spoken message; no rules outcome
magic-weapon | WE | attack_modifier | no: flat bonus by slot tier
magnificent-mansion | TV | travel create_object | -
major-image | IL | manual | no: duration | the image is whatever the caster invents and whether it fools anyone is narrated
mass-cure-wounds | HE | area target_filter heal | yes
mass-heal | HE | heal target_filter end_effect | -
mass-healing-word | HE | heal | yes
mass-suggestion | SG | manual save_gate save_condition ends_when | no: duration | a free-form suggestion; how each creature pursues it is narrated
maze | BN | banish check_gate | -
meld-into-stone | IV | grant_quality immobile trigger damage | -
mending | FF | manual | - | no rules outcome: a small repair narrated at the table
message | CM | message | -
meteor-swarm | AH | area+ save_damage+ | -
mind-blank | BU | resistance condition_immunity ward | -
mind-spike | SHR | save_gate save_damage sense | yes
minor-illusion | IL | manual | - | the sound or image is whatever the caster invents and whether it fools anyone is narrated
mirage-arcane | IL | manual | - | the false terrain is whatever the caster invents
mirror-image | IV | trigger roll_table | -
mislead | IV | apply_condition ends_when create_object grant_action sense | -
misty-step | TP | teleport | -
modify-memory | SG | manual save_gate save_condition incapacitated ends_when roll_modifier | no: time window | the new memory and what it changes are narrated; the DM may rule it has no effect
moonbeam | LZ | area+ save_damage timing end_effect grant_action | yes
move-earth | MP | alter_map | -
nondetection | WD | ward | -
pass-without-trace | AA | area target_filter roll_modifier | -
passwall | MP | alter_map | -
phantasmal-force | IL | manual save_gate check_gate timing damage | - | the phantasm is whatever the caster invents and how the target reacts to it is narrated
phantasmal-killer | SHR | save_gate save_damage edge roll_modifier timing | yes
phantom-steed | SU | summon+ ends_when | -
planar-ally | SU | manual | - | which creature answers, whether it serves and at what price are the DM's to decide
planar-binding | SK | save_gate compel | no: duration
plane-shift | TV | travel | -
plant-growth | LT | choice area create_surface | -
poison-spray | SA | attack_gate damage | yes
polymorph | TR | save_gate form+ | -
power-word-heal | HE | heal end_effect | -
power-word-kill | AU | branch damage | -
power-word-stun | AU | branch apply_condition immobile | -
prayer-of-healing | HE | heal resource_change+ | yes
prestidigitation | FF | manual | - | no rules outcome: minor tricks narrated at the table
prismatic-spray | AR | area area_save roll_table save_damage save_condition timing banish | -
prismatic-wall | WL | create_object area light target_filter timing save_condition save_damage ward banish | -
private-sanctum | WD | area ward choice | no: area
produce-flame | CF | light+ grant_action attack_gate damage | yes
programmed-illusion | IL | manual | - | the scripted illusion and its trigger are whatever the caster describes
project-image | IL | manual sense | - | what the copy says and does at a distant place, and who it fools, are narrated
protection-from-energy | BU | choice resistance | -
protection-from-evil-and-good | BU | edge roll_modifier condition_immunity | -
protection-from-poison | BU | end_effect roll_modifier resistance | -
purify-food-and-drink | FF | manual | - | no rules outcome: food and drink made safe, narrated at the table
raise-dead | RD | revive end_effect exhausting | -
ray-of-enfeeblement | SB | save_gate edge ends_when roll_modifier save_edge attack_modifier | -
ray-of-frost | SAR | attack_gate damage speed_penalty | yes
ray-of-sickness | SAR | attack_gate damage apply_condition | yes
regenerate | HE | heal timing | -
reincarnate | RD | revive roll_table stat_modifier | -
remove-curse | RC | end_effect | -
resilient-sphere | WL | save_gate create_object ward | -
resistance | BU | choice trigger resistance | -
resurrection | RD | revive exhausting restriction | -
reverse-gravity | FM | area area_save forced_move+ damage | -
revivify | RD | revive | -
rope-trick | TV | travel ward | -
sacred-flame | SH | save_gate save_damage roll_modifier | yes
sanctuary | BU | trigger save_gate ends_when | -
scorching-ray | SA | attack_gate damage | no: rays
scrying | DV | manual save_gate sense | - | what the sensor shows of a distant creature or place is the DM's to describe
searing-smite | WE | trigger damage timing save_gate | yes
secret-chest | IT | create_item grant_action | -
see-invisibility | SD | sense | -
seeming | IV | save_gate grant_quality | -
sending | CM | message | -
sequester | WD | apply_condition ward ends_when | -
shapechange | TR | form+ grant_action | -
shatter | AH | area save_damage roll_modifier | yes
shield | RX | reacts ac_modifier ward | -
shield-of-faith | BU | ac_modifier | -
shillelagh | WE | attack_modifier ends_when | yes
shining-smite | WE | trigger damage light+ edge reveal | yes
shocking-grasp | SAR | attack_gate damage action_economy | yes
silence | LT | area create_surface resistance restriction | -
silent-image | IL | manual | - | the image is whatever the caster invents and whether it fools anyone is narrated
simulacrum | SU | summon+ | -
sleep | AC | area target_filter area_save incapacitated timing save_condition ends_when roll_modifier | -
sleet-storm | LZ | area create_surface timing save_condition | -
slow | SB | area target_filter area_save speed_modifier ac_modifier roll_modifier action_economy | -
sorcerous-burst | SA | attack_gate damage choice | yes
spare-the-dying | HE | revive | no: range
speak-with-animals | DV | manual | - | what the Beast knows and says is the DM's to voice
speak-with-dead | DV | manual | - | what the corpse knows and says is the DM's to voice
speak-with-plants | DV | manual create_surface | - | what the plants report is the DM's to voice
spider-climb | BU | speed_modifier | no: targets
spike-growth | LT | area create_surface check_gate | -
spirit-guardians | LZ | area target_filter speed_modifier timing save_damage | yes
spiritual-weapon | CF | create_object attack_gate damage grant_action | yes
starry-wisp | SAR | attack_gate damage light+ reveal | yes
stinking-cloud | LZ | area create_surface timing save_condition action_economy | -
stone-shape | FF | manual | - | the shape is whatever the caster designs, and what it is good for is narrated
stoneskin | BU | resistance | -
storm-of-vengeance | LZ | area area_save save_damage save_condition timing damage create_surface restriction | -
suggestion | SG | manual save_gate save_condition ends_when | - | a free-form suggestion; how the creature pursues it is narrated
summon-dragon | SU | summon+ | no: summon stat block
sunbeam | CF | area save_damage save_condition grant_action light+ | -
sunburst | AR | area save_damage save_condition dispel+ | -
symbol | TS | create_trap choice area timing save_damage save_condition compel light+ | -
telekinesis | FM | grant_action save_gate forced_move+ save_condition alter_map | -
telepathic-bond | CM | message | -
teleport | TV | travel roll_table damage | -
teleportation-circle | TV | travel | -
thaumaturgy | FF | manual | - | no rules outcome beyond one skill edge: minor wonders narrated at the table
thunderwave | AR | area save_damage forced_move | yes
time-stop | BU | action_economy ends_when | -
tiny-hut | WL | area create_object ward | -
tongues | CM | stat_modifier | -
transport-via-plants | TV | travel | -
tree-stride | TP | teleport+ grant_action | -
true-polymorph | TR | save_gate choice form+ summon+ | -
true-resurrection | RD | revive end_effect | -
true-seeing | SD | sense | -
true-strike | WE | attack_modifier | yes
tsunami | LZ | area+ save_damage timing forced_move+ check_gate | -
unseen-servant | CO | create_object grant_action | -
vampiric-touch | CF | attack_gate damage heal grant_action | yes
vicious-mockery | SHR | save_gate save_damage edge ends_when | yes
vitriolic-sphere | AR | area save_damage timing | yes
wall-of-fire | LZ | area save_damage timing damage | yes
wall-of-force | WL | create_object forced_move+ ward | -
wall-of-ice | WL | create_object area save_damage forced_move+ timing | no: two dice steps
wall-of-stone | WL | create_object forced_move+ save_gate | -
wall-of-thorns | LZ | area save_damage create_surface timing | yes
warding-bond | BU | ac_modifier roll_modifier resistance trigger ends_when | -
water-breathing | BU | stat_modifier | -
water-walk | BU | speed_modifier | -
web | LZ | area create_surface+ timing save_condition check_gate | -
weird | AR | area target_filter save_damage save_condition timing | -
wind-walk | TR | speed_modifier condition_immunity resistance action_economy grant_action apply_condition | -
wind-wall | WL | area save_damage create_object | -
wish | FF | manual | - | reshaping reality is ruled by the DM; the spell can also stand in for any other spell
word-of-recall | TV | travel | -
zone-of-truth | LZ | area area_save timing restriction | -
"""

SOURCE_COMMIT = "92a4a7a"
CONDITIONS = ["Blinded", "Charmed", "Deafened", "Frightened", "Grappled", "Incapacitated", "Invisible", "Paralyzed", "Petrified", "Poisoned", "Prone", "Restrained", "Stunned", "Unconscious"]
SHAPE = re.compile(r"\b(Sphere|Cone|Cube|Cylinder|Line|Emanation)\b|-foot square")
DAMAGE = re.compile(r"\d+d\d+(?: \+ \d+)? \w+ damage")
# Sentences this census does not count: what happens to unattended objects, how far a noise carries, and pure description.
IGNORED = re.compile(r"[^.]*(aren't being worn or carried|isn't being worn or carried|audible within|frozen statue)[^.]*\.")
REPEAT = re.compile(r"[^.]*end of each of its turns[^.]*repeats the save[^.]*\.")
# Words that mean the spell does more than the plain pattern, so the first pass must not trust itself.
RIDERS = [
    "Speed", "pushed", "pulled", "Advantage", "Disadvantage", "can't", "Temporary Hit Points", "end of", "start of", "Magic action", "Bonus Action",
    "Reaction", "teleport", "until", "ends its turn", "enters", "Hit Point maximum", "(your choice", "roll ", "instead", "Difficult Terrain",
    "Obscured", "Bright Light", "Dim Light", "automatically", "killed", "dies", "leap", "each of your", "additional", "Cubes", "different points", "damaged you", " ends", "Cover",
    "Beast", "Humanoid", "Plant", "Construct", "Undead", "and each creature",
]


# What grows, grouped for the Scaling summary.
GROWS = {
    "targets": "number of targets",
    "duration": "duration",
    "duration and concentration": "duration",
    "summon stat block": "summoned creatures: count or stat block",
    "count": "summoned creatures: count or stat block",
    "count and stat block": "summoned creatures: count or stat block",
    "area": "area or amount",
    "amount or area": "area or amount",
    "flat amount": "a flat number",
    "flat bonus by slot tier": "a flat number",
    "level threshold": "a spell-level threshold",
    "stored spell level": "a spell-level threshold",
    "two dice steps": "two rolls by different dice",
    "beams": "number of beams, darts, rays or leaps",
    "darts": "number of beams, darts, rays or leaps",
    "rays": "number of beams, darts, rays or leaps",
    "leaps": "number of beams, darts, rays or leaps",
    "range": "range",
    "time window": "duration",
}


def load():
    with gzip.open(SEED) as f:
        data = json.load(f)
    return sorted((s for s in data["spells"] if s.get("document") == DOCUMENT), key=lambda s: s["slug"])


def first_pass(s):
    """Classify one spell from its fields and text. Returns (pattern, kinds), or (None, None) when unsure."""
    text = REPEAT.sub("", IGNORED.sub("", s["description"]))
    area = bool(SHAPE.search(text))
    attack = "spell attack" in text
    save = "saving throw" in text
    damage = bool(DAMAGE.search(text))
    heal = bool(re.search(r"regains?[^.]*Hit Points", text))
    conditions = [c for c in CONDITIONS if c + " condition" in text]
    instant = s["duration"] == "instantaneous"
    plain = not [w for w in RIDERS if w in text] and len(DAMAGE.findall(text)) <= 1
    repeat = bool(REPEAT.search(s["description"]))
    if area and save and damage and instant and plain and not attack and not conditions:
        return "AH", ["area", "save_damage"]
    if attack and damage and instant and plain and not save and not conditions and not area:
        return "SA", ["attack_gate", "damage"]
    if save and damage and instant and plain and not area and not attack and not conditions:
        return "SH", ["save_gate", "save_damage"]
    if heal and instant and plain and not save and not attack and not damage and not conditions and not area:
        return "HE", ["heal"]
    if save and len(conditions) == 1 and plain and not area and not attack and not damage and (repeat or not instant):
        return "SC", ["save_gate", "save_condition"]
    return None, None


def first_pass_scaling(s):
    """Scaling from the higher-level text: '-', 'yes', 'no', or None when unsure."""
    hl = s.get("higherLevel") or ""
    if not hl:
        return "-"
    sentences = [x for x in re.split(r"(?<=\.)\s+", hl.strip()) if x]
    dice = re.compile(r"(damage|healing)[^.]*increases? by \d+d\d+ (for each|for every|when you reach)")
    if len(sentences) == 1 and dice.search(hl) and " and the damage" not in hl:
        return "yes"
    if re.search(r"one additional (creature|Beast|Humanoid)|additional (creature|target)", hl):
        return "no"
    return None


def parse_hand():
    rows = {}
    for line in HAND.strip().splitlines():
        parts = [p.strip() for p in line.split("|")]
        slug, pattern, kinds, scaling = parts[:4]
        reason = parts[4] if len(parts) > 4 else ""
        kinds = kinds.split()
        for k in kinds:
            if k not in EXISTING and k not in MISSING and k not in WIDEN:
                sys.exit(f"{slug}: unknown kind {k}")
        if pattern not in PATTERNS:
            sys.exit(f"{slug}: unknown pattern {pattern}")
        if scaling != "-" and scaling != "yes" and not scaling.startswith("no: "):
            sys.exit(f"{slug}: bad scaling {scaling}")
        if (reason != "") != ("manual" in kinds):
            sys.exit(f"{slug}: an exception row carries manual and a reason, no other row does")
        if slug in rows:
            sys.exit(f"{slug}: listed twice")
        rows[slug] = {"pattern": pattern, "kinds": kinds, "scaling": scaling, "reason": reason, "missing": [k for k in kinds if k in MISSING or k in WIDEN]}
    return rows


def ranked(counter):
    return sorted(counter.items(), key=lambda kv: (-kv[1], kv[0]))


def build_order(gaps):
    """Order the missing kinds so that spells are finished early.

    Each step looks two kinds ahead: it takes the kind that starts the pair finishing the most spells, so
    kinds that only pay off together (attack_gate with damage) are not pushed to the back. Ties go to the
    kind that finishes more on its own, then to the one more spells still need.
    """
    names = sorted({k for missing in gaps.values() for k in missing})
    bit = {k: 1 << i for i, k in enumerate(names)}
    left = {slug: sum(bit[k] for k in missing) for slug, missing in gaps.items()}
    built, done, steps = 0, 0, []
    while left:
        open_kinds = [k for k in names if any(m & bit[k] for m in left.values())]

        def finished(have):
            return sum(1 for m in left.values() if m & ~have == 0)

        def score(k):
            alone = finished(built | bit[k])
            paired = max((finished(built | bit[k] | bit[other]) for other in open_kinds if other != k), default=alone)
            return (paired, alone, sum(1 for m in left.values() if m & bit[k]), k)

        kind = max(open_kinds, key=score)
        built |= bit[kind]
        finished_now = sorted(slug for slug, m in left.items() if m & ~built == 0)
        for slug in finished_now:
            del left[slug]
        done += len(finished_now)
        steps.append((kind, len(finished_now), done, finished_now))
    return steps


def code(kinds):
    return " ".join(f"`{k}`" for k in kinds) if kinds else "none"


def main():
    spells = load()
    rows = parse_hand()
    slugs = [s["slug"] for s in spells]
    if set(slugs) != set(rows):
        sys.exit(f"rows and seed differ: {sorted(set(slugs) ^ set(rows))}")
    by_slug = {s["slug"]: s for s in spells}
    total = len(spells)

    exceptions = {k: v for k, v in rows.items() if v["reason"]}
    today = sorted(k for k, v in rows.items() if not v["reason"] and not v["missing"])
    gaps = {k: v["missing"] for k, v in rows.items() if not v["reason"] and v["missing"]}
    today_scaled = [k for k in today if not rows[k]["scaling"].startswith("no")]

    # First pass against the hand rows.
    confident, agree, differ = 0, 0, []
    for s in spells:
        pattern, kinds = first_pass(s)
        if pattern is None:
            continue
        confident += 1
        row = rows[s["slug"]]
        if pattern == row["pattern"] and sorted(kinds) == sorted(row["kinds"]):
            agree += 1
        else:
            differ.append((s["slug"], pattern, kinds, row["pattern"], row["kinds"]))
    sc_sure, sc_agree, sc_differ = 0, 0, []
    for s in spells:
        guess = first_pass_scaling(s)
        if guess is None:
            continue
        sc_sure += 1
        got = rows[s["slug"]]["scaling"]
        if got == guess or (guess == "no" and got.startswith("no")):
            sc_agree += 1
        else:
            sc_differ.append((s["slug"], guess, got))
    if "--diff" in sys.argv:
        for d in differ:
            print("kinds", d)
        for d in sc_differ:
            print("scaling", d)

    needed = collections.Counter()
    sole = collections.Counter()
    for missing in gaps.values():
        for k in missing:
            needed[k] += 1
        if len(missing) == 1:
            sole[missing[0]] += 1
    patterns = collections.Counter(v["pattern"] for v in rows.values())
    pattern_today = collections.Counter(rows[k]["pattern"] for k in today)
    pattern_exc = collections.Counter(v["pattern"] for v in exceptions.values())
    steps = build_order(gaps)
    scaling = collections.Counter()
    for k, v in rows.items():
        if v["reason"]:
            continue
        scaling[v["scaling"] if not v["scaling"].startswith("no") else "no"] += 1
    grows = collections.Counter(GROWS[v["scaling"][4:]] for v in rows.values() if not v["reason"] and v["scaling"].startswith("no"))
    chosen = sum(1 for k, v in rows.items() if not v["reason"] and "area" not in v["kinds"] and "area+" not in v["kinds"] and by_slug[k]["rangeText"] != "Self")

    # Defects in the seed that a reader mapping fields to parts should know about.
    empty = [s["slug"] for s in spells if not s["description"].strip()]
    false_attack = [s["slug"] for s in spells if s["attackRoll"] and "spell attack" not in s["description"]]
    false_save = [s["slug"] for s in spells if s.get("saveAbility") and "saving throw" not in s["description"]]
    unscaled = [s["slug"] for s in spells if s.get("higherLevel") and not s["scaling"]]

    o = []
    w = o.append
    w("# Census: the Effect component kinds every SRD 5.2 spell needs\n")
    w(f"Research for [#237](https://github.com/JorisJonkers-dev/grimoire/issues/237), part of the map in [#236](https://github.com/JorisJonkers-dev/grimoire/issues/236). "
      f"This file is written by [`census-spells.py`](census-spells.py) from the Compendium seed at commit `{SOURCE_COMMIT}`. Change the script and run it again; do not edit this file by hand.\n")

    w("## The answer in short\n")
    w(f"- The seed holds **{total}** SRD 5.2 spells. Every one has a row in [the table](#the-table).")
    w(f"- **{len(today)}** can be stated in full with the kinds the engine has today. For {len(today_scaled)} of them the Scaling is expressible too.")
    w(f"- **{len(exceptions)}** are proposed for the Manual exception list, because what they produce is something only a person at the table can supply.")
    w(f"- **{len(gaps)}** are gaps: they need at least one of **{len([k for k in needed if k in MISSING])}** kinds the engine lacks, or one of **{len([k for k in needed if k in WIDEN])}** kinds it has in too narrow a form.")
    top = ranked(needed)[:6]
    w("- The kinds most spells wait on are " + ", ".join(f"`{k}` ({n})" for k, n in top) + ".")
    w(f"- Built in the order suggested below, the first five kinds finish {steps[4][2]} of the gap spells, the first ten finish {steps[9][2]} and the first twenty finish {steps[19][2]}. All {len(steps)} are needed to finish every one.")
    w(f"- Scaling: of the {total - len(exceptions)} spells outside the exception list, {scaling['-']} do not grow, {scaling['yes']} grow by dice in a way the existing axes express, and {scaling['no']} grow something the Scaling record cannot hold.\n")

    w("## Sources\n")
    w("All primary, all in this repository at the commit above.\n")
    w("| What | Where |")
    w("|---|---|")
    w("| The spells | `api/db/seeds/compendium.json.gz`, key `spells`, `document == \"srd-2024\"` |")
    w("| The component kinds as Go types | [`api/internal/rules/effects/effects.go`](../../api/internal/rules/effects/effects.go), [`shape.go`](../../api/internal/rules/effects/shape.go), [`light.go`](../../api/internal/rules/effects/light.go) |")
    w("| The component kinds as stored | `effect_components_kind_check` in [`api/db/migrations/00077_visibility.sql`](../../api/db/migrations/00077_visibility.sql), the newest version of that constraint |")
    w("| What each kind does in play | [`api/internal/play/live/area.go`](../../api/internal/play/live/area.go), [`effects.go`](../../api/internal/play/live/effects.go), [`magic.go`](../../api/internal/play/live/magic.go) |")
    w("| Surfaces | [`api/internal/rules/surface/surface.go`](../../api/internal/rules/surface/surface.go) |")
    w("| The authoring Design | [`api/internal/rules/spellbuild/spellbuild.go`](../../api/internal/rules/spellbuild/spellbuild.go) |")
    w("| The seeded Effects | migrations `00045`, `00063`, `00069`, `00071`, `00073`, `00075`, `00077` |")
    w("| The target vocabulary | [`ARCHITECTURE.md`](../../ARCHITECTURE.md) section 9.3, and Effect, Targeting, Scaling, Manual Effect and Automation Level in [`CONTEXT.md`](../../CONTEXT.md) |\n")

    w("## What the engine has today\n")
    w("The stored constraint lists 29 names. One of them, `mode`, is not a component kind: it is the row a `choice` keeps for each of its modes. "
      "The Go union has 30 types: the other 28 stored kinds, plus `area_save` and `light`. Those two are built by the Homebrew spell builder and run in play, "
      "but have no catalogue table, so a seeded SRD Effect cannot hold them yet. This census counts all 30 as existing and marks the two.\n")
    w("| Kind | What it states today |")
    w("|---|---|")
    for k, v in EXISTING.items():
        w(f"| `{k}` | {v} |")
    w("")
    w("Limits of these kinds that decided rows below, each read from the code:\n")
    w("- A saving throw is rolled only by creatures inside an `area`. An Effect put on a chosen creature lands with no save; only its repeat save at the end of a turn is rolled (`planApply` and `saves` in `play/live/effects.go`).")
    w("- No kind rolls a spell attack, restores Hit Points, or changes Armor Class.")
    w("- An `area` touches every creature in it. Nothing spares the caster's allies or caps the number of targets.")
    w("- Later timings exist only through a Surface: save-less damage or an Effect on a creature that enters it or starts its turn there (`rules/surface/surface.go`).")
    w("- `reacts` and `grant_feature` both end in a sentence for the DM (`ReactionsTo`, and `land` in `play/live/effects.go`), so neither computes an outcome.")
    w("- Scaling grows one dice expression. In play it is applied only on the `slot_level` axis and only to an area's `save_damage` (`upcast` in `play/live/area.go`); `character_level` steps are stored and written as rules text but no cast applies them.")
    w("- The Design in `spellbuild.go` always has an area, takes one save, and offers six part kinds: damage, condition, light, reveal, surface, manual.\n")

    w("## How the rows were judged\n")
    w(f"The script makes a first pass from the structured fields and the description, and trusts itself only when a spell matches a plain pattern with no rider words. "
      f"It was sure of {confident} spells. Then all {total} descriptions were read and classified by hand; the hand rows are the census and sit in the script as data. "
      f"Where the first pass was sure, it agrees with the hand row for {agree} of {confident} spells; "
      f"its Scaling guess agrees for {sc_agree} of the {sc_sure} spells it was sure of. Run the script with `--diff` to list the rest.\n")
    w("The bar for a row:\n")
    w("- **Needed** is every kind an Effect must hold so that the rules text generated from it says what the SRD description says. A kind is listed once however often the spell uses it.")
    w("- **Missing** is the subset the engine cannot state today: a kind it lacks, or one it has in too narrow a form, written with a `+`.")
    w("- A rule that changes what happens to a creature counts, however small: a creature type that saves with Disadvantage, a spell that ends when its target takes damage.")
    w("- Four things do not count: what happens to unattended objects (Fireball lighting what is flammable), how far a noise carries, wind dispersing a cloud, and a sentence that only describes (Cone of Cold's frozen statue). The seeded Fireball, Thunderwave and Cone of Cold set that precedent.")
    w("- Putting an Effect on a chosen, willing creature counts as possible today, because that is how the seeded Bless works: it is applied to each token. Range, sight and the number of targets are not checked. "
      f"{chosen} of the spells outside the exception list are cast on chosen creatures, objects or points rather than on the caster or an area, so this is the largest thing the census takes on trust; see [Not verified](#not-verified).")
    w("- **Scaling** is `-` when the spell does not grow, `yes` when it grows by dice per slot level or by dice steps at character levels, and `no` with what grows when it is something else.")
    w("- **Exception** is yes only when the spell's outcome is content or a ruling that a person at the table has to supply. A spell that is hard to model is a gap.\n")

    w("## Summary\n")
    w("### Spells per pattern\n")
    w("A pattern is a way of resolving. Spells that share one can share a scenario test.\n")
    w("| Pattern | Spells | Automatable today | Proposed exceptions |")
    w("|---|---:|---:|---:|")
    for code_, n in sorted(patterns.items(), key=lambda kv: (-kv[1], PATTERNS[kv[0]])):
        w(f"| {PATTERNS[code_]} | {n} | {pattern_today[code_] or ''} | {pattern_exc[code_] or ''} |")
    w(f"| **Total** | **{total}** | **{len(today)}** | **{len(exceptions)}** |\n")

    w("### Missing kinds, ranked\n")
    w(f"Counted over the {len(gaps)} gap spells; the proposed exceptions are left out. *Needed by* is how many spells list the kind. "
      "*Only blocker* is how many list nothing else, so building that kind alone finishes them.\n")
    w("| Rank | Kind | Needed by | Only blocker | New or widened |")
    w("|---:|---|---:|---:|---|")
    for i, (k, n) in enumerate(ranked(needed), 1):
        w(f"| {i} | `{k}` | {n} | {sole[k] or ''} | {'widened' if k in WIDEN else 'new'} |")
    w("")

    w("### A build order\n")
    w("Most gap spells wait on several kinds at once, so the ranking above overstates what one kind buys. This order is greedy with a look two kinds ahead: each step takes the kind that starts the pair finishing the most spells, "
      "given the kinds before it, so that kinds which only pay off together (`attack_gate` with `damage`) are not pushed to the back. It counts spells and nothing else: not how often a spell is cast at a table, and not what a kind costs to build. "
      "It is one input to the gap-ordering decision, not that decision.\n")
    waits = collections.Counter(len(m) for m in gaps.values())
    w("| Missing kinds a gap spell waits on | 1 | 2 | 3 | 4 | 5 or more |")
    w("|---|---:|---:|---:|---:|---:|")
    w(f"| Spells | {waits[1]} | {waits[2]} | {waits[3]} | {waits[4]} | {sum(n for k, n in waits.items() if k >= 5)} |\n")
    w(f"The curve is close to a straight line, about {round(len(gaps) / len(steps))} spells a kind from the first step to the last. No small set of kinds finishes most of the list.\n")
    w("| Step | Kind | Spells it finishes | Finished so far |")
    w("|---:|---|---:|---:|")
    for i, (k, n, done, _) in enumerate(steps, 1):
        w(f"| {i} | `{k}` | {n} | {done} |")
    w("")

    w("### Automatable with today's kinds\n")
    w(f"{len(today)} spells need nothing new:\n")
    for k in today:
        r = rows[k]
        note = "" if not r["scaling"].startswith("no") else f"; Scaling not expressible ({r['scaling'][4:]})"
        seeded = ", seeded" if k in SEEDED else ""
        w(f"- `{k}`: {code(r['kinds'])}{seeded}{note}")
    w("")
    w("Plant Growth qualifies for its Overgrowth mode; the Enrichment mode has no outcome on a Map. A Surface lasts at most 1000 rounds, which is shorter than that spell's open-ended overgrowth.\n")

    w("### The 20 seeded spells against this bar\n")
    full = [k for k in SEEDED if k in today]
    w(f"{len(full)} of the 20 spells with a seeded Effect state their whole SRD 5.2 description. The other {20 - len(full)} are counted as fully automated by the engine today, "
      "since none has a `manual` part, yet each leaves something out:\n")
    w("| Seeded spell | Missing for the full text |")
    w("|---|---|")
    for k in SEEDED:
        if k not in today:
            w(f"| `{k}` | {code(rows[k]['missing'])} |")
    w("")
    w("The column lists kinds only. Some seeded Effects also leave out parts the engine already has: Faerie Fire is seeded without its `area` and save, and Grease as a 5-foot cylinder where the text gives a 10-foot square.\n")

    w("### Scaling\n")
    w(f"{scaling['yes']} spells grow by dice on the `slot_level` axis or by dice steps on `character_level`, which the Scaling record holds. "
      f"{scaling['no']} grow something else. The axes are not what is missing: every one of these grows with the slot or the caster's level. What is missing is the quantity. The record holds dice and nothing but dice.\n")
    w("| What grows | Spells |")
    w("|---|---:|")
    for k, n in ranked(grows):
        w(f"| {k} | {n} |")
    w("")
    w("Three notes on the `yes` rows. A cast applies Scaling only to an area's `save_damage` today, so healing dice and attack damage dice grow in the data but not in play until `heal` and `damage` exist. "
      "The thirteen cantrips that step at levels 5, 11 and 17 use the `character_level` axis, which no cast applies yet. "
      "And where one step grows two damage rolls by the same die (Acid Arrow, Flame Strike) the row says `yes`, on the reading that one Scaling can be applied to both.\n")

    w("### Proposed Manual exceptions\n")
    w(f"{len(exceptions)} spells, in five groups. The first four are DM judgment by nature. The fifth is a different case and should be decided as a group.\n")
    groups = [
        ("The DM supplies the knowledge", ["DV"]),
        ("The illusion is whatever the caster invents", ["IL"]),
        ("Free words: an instruction, a bargain or a message", ["SG", "SU", "CM", "TS"]),
        ("Open-ended creation or wish", None),
        ("No rules outcome to compute", None),
    ]
    used = set()
    for title, pats in groups:
        if pats is not None:
            members = sorted(k for k, v in exceptions.items() if v["pattern"] in pats and not v["reason"].startswith("no rules outcome"))
        elif title.startswith("Open"):
            members = sorted(k for k, v in exceptions.items() if v["pattern"] == "FF" and not v["reason"].startswith("no rules outcome"))
        else:
            members = sorted(k for k, v in exceptions.items() if v["reason"].startswith("no rules outcome"))
        used.update(members)
        w(f"**{title}** ({len(members)})\n")
        for k in members:
            w(f"- `{k}`: {rows[k]['reason']}")
        w("")
    if used != set(exceptions):
        sys.exit(f"exceptions not grouped: {sorted(set(exceptions) - used)}")
    w("The last group holds spells whose text has nothing for an engine to work out: a flower blooms, a shirt is cleaned. They are not hard and nobody rules on them. "
      "They are listed here because an Effect that says nothing would pass as fully automated while proving nothing against the SRD text. "
      "The alternative to an exception is to let such an Effect count as full with its description as its only content. That choice belongs to the maintainer.\n")

    w("### Borderline calls\n")
    w("Rows a reasonable reader could put on the other side of the line.\n")
    w("Proposed as exceptions, though part of the spell is computable:\n")
    w("- `detect-thoughts`, `scrying`, `phantasmal-force`, `dream`, `contact-other-plane`, `geas`: each has a save, and some deal damage, around a core the DM narrates.")
    w("- `find-the-path`, `locate-animals-or-plants`: the answer is a fact about the world. If the place or creature is on the world map the engine could give it.")
    w("- `stone-shape`, `magic-mouth`, `animal-messenger`, `project-image`: a bounded effect whose content the caster makes up.\n")
    w("Counted as gaps, though they lean on the DM:\n")
    w("- `contingency`, `glyph-of-warding`, `symbol`, `imprisonment`, `sequester`: the caster words a trigger. The census assumes a list of typed triggers and leaves the unusual ones to the DM.")
    w("- `locate-creature`, `locate-object`, `identify`, `detect-magic`: computable for what is on the Map or in an inventory, a question for the DM otherwise.")
    w("- `control-weather`, `control-water`, `move-earth`, `guards-and-wards`, `magnificent-mansion`, `gate`, `awaken`, `planar-binding`, `magic-jar`, `clone`: determinate, but each needs a part of the world the engine may never model.")
    w("- `comprehend-languages`, `tongues`, `water-breathing`, `zone-of-truth`, `message`, `sending`, `telepathic-bond`: an outcome with no number in it. They are gaps only if languages, breathing and private channels are things the engine tracks.\n")

    w("## Missing kinds defined\n")
    for k, n in ranked(needed):
        if k in MISSING:
            w(f"- **`{k}`** ({n}). {MISSING[k]}")
    for k in MISSING:
        if k not in needed:
            w(f"- **`{k}`** (0). {MISSING[k]}")
    w("")
    w("## Existing kinds that need widening\n")
    for k, n in ranked(needed):
        if k in WIDEN:
            w(f"- **`{k}`** ({n}). {WIDEN[k]}")
    w("")

    w("## The table\n")
    w("Kinds in the *Missing today* column are defined above. *Scaling* answers whether the existing axes and the dice-only Scaling record express how the spell grows.\n")
    w("| Spell | Level | Pattern | Kinds needed | Missing today | Scaling | Manual exception |")
    w("|---|---:|---|---|---|---|---|")
    for s in spells:
        r = rows[s["slug"]]
        exc = "yes: " + r["reason"] if r["reason"] else "no"
        w(f"| `{s['slug']}` | {s['level']} | {PATTERNS[r['pattern']]} | {code(r['kinds'])} | {code(r['missing'])} | {r['scaling']} | {exc} |")
    w("")

    w("## Seed defects noticed\n")
    w("Found while reading; none is fixed here.\n")
    w(f"- `description` is empty for {', '.join(f'`{k}`' for k in empty)}. Its row is classified from the SRD 5.2 rule that the target has the Invisible condition for the duration.")
    w("- `moonbeam` starts mid-spell, without the sentence that sets the Cylinder; `conjure-celestial` names two lights and describes neither; `telekinesis` stops mid-sentence; `tsunami` ends with a stray section heading.")
    w(f"- `attackRoll` is true for {len(false_attack)} spells whose text makes no spell attack (Bless, Blur, Foresight and others); each of them mentions attack rolls for another reason. "
      f"`saveAbility` is set for {', '.join(f'`{k}`' for k in false_save)}, whose text has no saving throw. `damageRoll` holds dice that are not damage for several more (Bless 1d4, Blink 1d6, Teleport 1d100). "
      "A loader that maps these fields to parts would be wrong for those spells.")
    w(f"- `scaling` has rows for damage dice only. {len(unscaled)} spells have `higherLevel` text and no `scaling` rows.")
    w("- `chain-lightning` and `dissonant-whispers` have no `higherLevel` text, and may be missing it; see below.\n")

    w("## Not verified\n")
    w("- The seed was not compared with the SRD 5.2 document itself. Rows follow the seed's text, so a wrong or cut-off description gives a wrong row. The five spells named above are the known cases.")
    w("- Whether `chain-lightning` and `dissonant-whispers` grow with the slot in SRD 5.2. Their rows say `-`, as the seed does. The SRD 5.1 Chain Lightning in the same seed adds a bolt per slot level, which suggests the 5.2 entry lost its text. "
      "The seed's upstream API could not be reached from where this was written, so neither was checked.")
    w("- That applying an Effect to a chosen token is an acceptable stand-in for Targeting of chosen creatures. If it is not, a kind for it comes ahead of everything in the ranking, since it is needed by every spell counted in the trust note above.")
    w("- Nothing was run. The limits listed under \"What the engine has today\" come from reading the code, not from casting a spell in a Session.")
    w("- The missing kinds are named and scoped by this census. Whether `timing` and `trigger`, or `save_gate` and `apply_condition`, should be one kind or two is for the engine-gap ticket to settle; merging two rows of the ranking changes the order.\n")

    OUT.write_text("\n".join(o))
    print(f"spells {total}  today {len(today)}  exceptions {len(exceptions)}  gaps {len(gaps)}")
    print(f"first pass sure {confident}, agrees {agree}; scaling sure {sc_sure}, agrees {sc_agree}")
    print("patterns", ranked(patterns))
    print("missing", ranked(needed))


if __name__ == "__main__":
    main()
