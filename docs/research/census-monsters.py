#!/usr/bin/env python3
"""Census of the Effect component kinds SRD 5.2 monster actions and traits need.

Reads the Compendium seed and the newest effect_components_kind_check, puts every action and trait of
the srd-2024 monsters in a group, and writes census-monsters.csv and the generated blocks of
census-monsters.md. Nothing is sampled: an entry no rule or hand row accounts for stops the run.

    python3 docs/research/census-monsters.py
"""

import collections
import csv
import gzip
import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent.parent
SEED = ROOT / "api/db/seeds/compendium.json.gz"
MIGRATIONS = ROOT / "api/db/migrations"
DOCUMENT = "srd-2024"

KINDS = {"action": "action", "bonus_action": "bonus", "reaction": "reaction", "legendary_action": "legendary"}

# ---------------------------------------------------------------------------------------------------
# Inputs
# ---------------------------------------------------------------------------------------------------


def existing_kinds():
    """The component kinds the newest effect_components_kind_check allows."""
    newest = None
    for path in sorted(MIGRATIONS.glob("*.sql")):
        text = path.read_text()
        found = re.search(r"effect_components_kind_check CHECK \(kind IN \(([^)]*)\)", text)
        if found:
            newest = (path.name, found.group(1))
    name, body = newest
    return name, set(re.findall(r"'([a-z_]+)'", body))


def seed():
    with gzip.open(SEED) as fh:
        return json.load(fh)


def monsters(data):
    return [m for m in data["monsters"] if m.get("document") == DOCUMENT]


def spell_names(data):
    return sorted({s["name"] for s in data["spells"] if s.get("document") == DOCUMENT}, key=len, reverse=True)


def seeded_spell_effects():
    """The slugs of the spells that own a seeded Effect."""
    out = set()
    for path in sorted(MIGRATIONS.glob("*.sql")):
        out |= set(re.findall(r"\('[a-z0-9-]+', '(?:[^']|'')+', (?:true|false), 'spell', '([a-z0-9-]+)'", path.read_text()))
    return out


def slug(name):
    return re.sub(r"[^a-z0-9]+", "-", name.lower().replace("'", "")).strip("-")


Entry = collections.namedtuple("Entry", "slug monster kind name base text attacks")


def entries(ms):
    out = []
    for m in ms:
        for a in m["actions"]:
            out.append(Entry(m["slug"], m["name"], KINDS[a["type"]], a["name"].strip(), base(a["name"]), a["description"], a["attacks"]))
        for t in m["traits"]:
            out.append(Entry(m["slug"], m["name"], "trait", t["name"].strip(), base(t["name"]), t["description"], []))
    return out


def base(name):
    """A name without its parenthesis: 'Legendary Resistance (3/Day)' is 'Legendary Resistance'."""
    return re.sub(r"\s*\(.*\)\s*$", "", name).strip()


# ---------------------------------------------------------------------------------------------------
# Description shape
# ---------------------------------------------------------------------------------------------------

SIZES = "Tiny|Small|Medium|Large|Huge|Gargantuan"
# Words after "the" that are not the creature's own name.
KEEP = set(
    "target attack hit start end following effect save same damage next condition creature first spell area "
    "triggering cone line emanation ground trigger roll grapple".split()
)


def shape(text):
    """A description with its numbers, dice, sizes and the creature's own name taken out."""
    t = text.replace("’", "'")
    t = re.sub(r"\d+ \(\d+d\d+(?: [+-] \d+)?\)", "DMG", t)
    t = re.sub(r"\b\d+(?= [A-Z][a-z]+ damage)", "DMG", t)
    t = re.sub(r"\b(" + SIZES + r")\b", "SIZE", t)
    t = re.sub(r"\d+", "N", t)

    def creature(m):
        if m.group(1).lower() in KEEP:
            return m.group(0)
        return "the X" + (m.group(2) or "")

    return re.sub(r"\b[Tt]he ([a-z-]+)('s)?\b", creature, t)


HEAD = re.compile(
    r"^(?:Melee|Ranged|Melee or Ranged) Attack Roll: \+N(?: to hit)?(?P<paren> \([^)]*\))?, "
    r"(?:reach N (?:ft\.?|feet\.?)(?: or range N(?:/N)? ft\.)?|range N(?:/N)? (?:ft\.|feet\.))\s*(?:Hit: )?"
)
BASE_DAMAGE = re.compile(r"^DMG \w+ damage(?: plus DMG \w+ damage)?")

# ---------------------------------------------------------------------------------------------------
# Groups. Every result is (group, pattern, kinds needed, exception reason).
# ---------------------------------------------------------------------------------------------------

G_PLAIN = "Plain attack, structured data agrees with the text"
G_PLAIN_WRONG = "Plain attack, structured data disagrees with the text"
G_PLAIN_TEXT = "Plain attack, carried as text only"
G_STATE = "Attack whose damage or Advantage depends on a state"
G_CHARGE = "Attack with a charge rider (moved 10+ or 20+ feet straight toward the target)"
G_PRONE = "Attack that knocks a size-limited target Prone"
G_GRAPPLE = "Attack that grapples (escape DC), with or without Restrained"
G_TIMED = "Attack that puts a condition on the target until a turn boundary"
G_MOVE = "Attack that pushes or pulls the target"
G_SAVE_RIDER = "Attack whose hit forces a saving throw"
G_LYCAN = "Attack that passes on lycanthropy"
G_DRAIN = "Attack that lowers Hit Point maximum or an ability score"
G_SLOW = "Attack that slows, or weakens the target's next roll or turn"
G_IGNITE = "Attack that sets the target burning"
G_ATTACH = "Attack that attaches to or smothers the target"
G_WASTING = "Attack that leaves a lasting poison or curse"
G_STABLE = "Attack that leaves a target dropped to 0 Hit Points Stable and Paralyzed"
G_CORRODE = "Corrodes or destroys armor, weapons and metal"

# One rider clause after the base damage: what to take out of the text, the kinds it needs, its group.
RIDERS = [
    (r" \(with Advantage [^)]*\)", "edge*", G_STATE),
    (r"(?:, or|—or) DMG \w+ damage if the (?:X|target) (?:is Bloodied|had Advantage on the attack roll|is Grappled by the X)(?:—plus DMG \w+ damage)?",
     "attack_gate damage branch*", G_STATE),
    (r", plus DMG \w+ damage if the attack roll had Advantage", "attack_gate damage branch*", G_STATE),
    (r", or DMG \w+ damage if the X moved N\+ feet straight toward the target immediately before the hit",
     "attack_gate damage branch*", G_CHARGE),
    (r"If (?:the )?target is (?:a )?SIZE or smaller(?: creature)? and the X moved N\+ feet straight toward it immediately before the hit, "
     r"the target takes an extra DMG \w+ damage and has the Prone condition\.", "attack_gate branch* damage apply_condition", G_CHARGE),
    (r"If the target is a SIZE or smaller creature and the X moved N\+ feet straight toward it immediately before the hit, "
     r"the target has the Prone condition, and the X can make one Bite attack against it\.", "attack_gate branch* apply_condition sequence", G_CHARGE),
    (r"If the target is a SIZE or smaller creature and the X moved N\+ feet straight toward it immediately before the hit, "
     r"the target has the Prone condition\.", "attack_gate branch* apply_condition", G_CHARGE),
    (r"If the target is a SIZE or smaller creature, it has the Prone condition\.", "attack_gate branch* apply_condition", G_PRONE),
    (r"If the target is a SIZE or smaller creature, it has the Grappled condition \(escape DC N\)(?: from [^.,]*)?"
     r"(?:, and it has the Restrained condition until the grapple ends)?\.", "attack_gate branch* apply_condition", G_GRAPPLE),
    (r"(?:, and the target|The target) has the Grappled condition \(escape DC N\)(?: from [^.,]*)?"
     r"(?:, and it has the Restrained condition until the grapple ends)?\.", "attack_gate apply_condition", G_GRAPPLE),
    (r"While Grappled, the target has the Restrained condition(?: and can't be targeted by the X's Tail)?\.", "apply_condition", G_GRAPPLE),
    (r"Until the grapple ends, the target has the Restrained condition and can't teleport\.", "apply_condition modifier:no_teleport", G_GRAPPLE),
    (r"Ability checks made to escape this grapple have Disadvantage\.", "edge*", G_GRAPPLE),
    (r", and the target has the \w+(?: and \w+)? conditions? until the (?:start|end) of (?:the X's|its) next turn\.",
     "attack_gate apply_condition", G_TIMED),
    (r"(?:Until this poison ends|While Poisoned), the target can't regain Hit Points\.", "modifier:no_healing", G_TIMED),
    (r"If the target is a SIZE or smaller creature, the X(?: mound)? (?:pushes|pulls) the target (?:up to )?N feet straight "
     r"(?:away from|toward) itself, and the target has the Prone condition\.", "attack_gate branch* forced_move apply_condition", G_MOVE),
    (r"If the target is a SIZE or smaller creature, the X(?: mound)? (?:pushes|pulls) the target (?:up to )?N feet straight "
     r"(?:away from|toward) itself\.", "attack_gate branch* forced_move", G_MOVE),
    (r"(?:, and the target's|If the target is a creature, its) Hit Point maximum decreases by an amount equal to the (?:Acid )?damage taken\.",
     "attack_gate modifier:hp_max", G_DRAIN),
    (r"If the target is a (?:creature or a )?flammable object(?: that isn't being worn or carried)?, it starts burning\.",
     "attack_gate apply_condition", G_IGNITE),
    (r"(?:, and the target's|If the target is a creature, its) Speed decreases by N feet until the end of its next turn\.",
     "attack_gate speed_penalty", G_SLOW),
    (r", and the target has Disadvantage on the next attack roll it makes before the end of its next turn\.", "attack_gate edge*", G_SLOW),
    (r"If the target is a Humanoid, it is subjected to the following effect\. Constitution Saving Throw: DC N\. Failure: The target is cursed\. "
     r"If the X target drops to N Hit Points, it instead becomes a \w+ under the DM's control and has N Hit Points\. "
     r"Success: The target is immune to this \w+'s curse for N hours\.",
     "attack_gate branch save_gate apply_condition trigger:drops_to_0 form control modifier:immunity", G_LYCAN),
    (r"If the target is a (?:non-Undead creature|creature that isn't an Undead or elf), it is subjected to the following effect\. "
     r"Constitution Saving Throw: DC N\. Failure: The target has the Paralyzed condition until the end of its next turn\.",
     "attack_gate branch* save_gate apply_condition", G_SAVE_RIDER),
    # Flavour and environment notes with nothing for the engine to compute.
    (r"HitomThe (?:spear|sword) magically returns to the X's hand(?: or hovers within N feet of the X)? immediately after a ranged attack\.", "", ""),
    (r"Being underwater doesn't grant Resistance to this \w+ damage\.", "", ""),
]

# Which rider group names an attack with several riders.
RANK = [G_LYCAN, G_SAVE_RIDER, G_DRAIN, G_CHARGE, G_MOVE, G_GRAPPLE, G_PRONE, G_TIMED, G_SLOW, G_IGNITE, G_STATE]
RIDER_PATTERN = {
    G_STATE: "attack-rider-state", G_CHARGE: "attack-rider-charge", G_PRONE: "attack-rider-condition", G_GRAPPLE: "attack-rider-condition",
    G_TIMED: "attack-rider-condition", G_MOVE: "attack-rider-move", G_SAVE_RIDER: "attack-rider-save", G_LYCAN: "attack-rider-save",
    G_DRAIN: "attack-rider-debuff", G_SLOW: "attack-rider-debuff", G_IGNITE: "attack-rider-condition",
}

# Attacks read by hand: monster | action | group | pattern | kinds needed
ATTACKS_BY_HAND = """
Darkmantle | Crush | {attach} | attack-rider-contain | attack_gate contain apply_condition branch* edge* immobile
Stirge | Proboscis | {attach} | attack-rider-contain | attack_gate contain trigger:turn_start damage
Cloaker | Attach | {attach} | attack-rider-contain | attack_gate contain apply_condition branch* modifier:damage_share
Animated Rug of Smothering | Smother | {attach} | attack-rider-contain | attack_gate choice contain apply_condition branch* trigger:turn_start damage modifier:damage_share
Pirate Captain | Rapier | {slow} | attack-rider-debuff | attack_gate edge*
Worg | Bite | {slow} | attack-rider-debuff | attack_gate edge*
Balor | Lightning Blade | {slow} | attack-rider-debuff | attack_gate modifier:action_economy
Ice Devil | Ice Spear | {slow} | attack-rider-debuff | attack_gate modifier:action_economy speed_penalty
Fire Giant | Hammer Throw | {move} | attack-rider-move | attack_gate forced_move edge*
Otyugh | Bite | {wasting} | attack-rider-debuff | attack_gate apply_condition trigger:rest save_gate modifier:hp_max
Death Dog | Bite | {wasting} | attack-rider-save | attack_gate save_gate apply_condition trigger:clock modifier:hp_max
Mummy | Rotting Fist | {wasting} | attack-rider-debuff | attack_gate apply_condition modifier:no_healing modifier:hp_max trigger:clock trigger:drops_to_0
Mummy Lord | Rotting Fist | {wasting} | attack-rider-debuff | attack_gate apply_condition modifier:no_healing modifier:rest modifier:hp_max trigger:clock trigger:drops_to_0
Rakshasa | Cursed Touch | {wasting} | attack-rider-debuff | attack_gate apply_condition modifier:rest
Incubus | Restless Touch | {wasting} | attack-rider-debuff | attack_gate apply_condition modifier:rest
Pit Fiend | Bite | {wasting} | attack-rider-save | attack_gate save_gate apply_condition modifier:no_healing trigger:turn_start damage
Bearded Devil | Infernal Glaive | {wasting} | attack-rider-save | attack_gate save_gate apply_condition trigger:turn_start damage
Homunculus | Bite | {saverider} | attack-rider-save | attack_gate save_gate apply_condition branch
Cockatrice | Petrifying Bite | {saverider} | attack-rider-save | attack_gate save_gate apply_condition
Shadow | Draining Swipe | {drain} | attack-rider-debuff | attack_gate modifier:ability trigger:drops_to_0 summon*
Gibbering Mouther | Bite | {prone} | attack-rider-condition | attack_gate branch* apply_condition trigger:drops_to_0
Phase Spider | Bite | {stable} | attack-rider-condition | attack_gate trigger:drops_to_0 apply_condition
Vampire Familiar | Umbral Dagger | {stable} | attack-rider-condition | attack_gate trigger:drops_to_0 apply_condition
Black Pudding | Dissolving Pseudopod | {corrode} | item-wear | attack_gate item_wear
Gray Ooze | Pseudopod | {corrode} | item-wear | attack_gate item_wear
Roper | Tentacle | {grapple} | attack-rider-condition | attack_gate apply_condition create_object
Half-Dragon | Claw | {state} | attack-rider-state | attack_gate damage choice
""".format(
    attach=G_ATTACH, slow=G_SLOW, move=G_MOVE, wasting=G_WASTING, saverider=G_SAVE_RIDER, drain=G_DRAIN, prone=G_PRONE, stable=G_STABLE,
    corrode=G_CORRODE, grapple=G_GRAPPLE, state=G_STATE,
)

G_AREA_DMG = "Area save for damage only (breath weapons and bursts)"
G_AREA_COND = "Area save against a condition, push or weakening, no damage"
G_AREA_BOTH = "Area save for damage plus a condition, push or weakening"
G_AREA_STAGED = "Area save in stages: a second failure worsens the condition"
G_T_DMG = "Target save for damage only"
G_T_COND = "Target save against a condition, no damage"
G_T_BOTH = "Target save for damage plus a condition, push or weakening"
G_T_KILL = "Target save that kills or drops a weakened target outright"
G_T_DRAIN = "Target save for damage that lowers Hit Point maximum"
G_T_OBJECT = "Target save against a bond that is itself an object with hit points"
G_T_CONTROL = "Target save that takes control of the target"
G_CONTAIN = "Swallow, engulf or whelm: the target is held inside the creature"
G_ZONE = "Area the creature conjures or carries that lasts"
G_CAST = "Casts one or a few named spells"
G_LIST = "Spellcasting: a list of spells cast at will or a set number of times a day"
G_MULTI_COUNT = "Multiattack: a number of attacks of one kind, or of its attacks in any combination"
G_MULTI = "Multiattack: a fixed mix of different attacks"
G_MULTI_MIX = "Multiattack: attacks mixed with, or swapped for, another action"
G_MULTI_SPELL = "Multiattack: an attack may be swapped for a spell"
G_MOVE_ATTACK = "Moves, then makes one attack"
G_USE = "Makes one attack or uses another of its actions"
G_STANDARD = "Takes Dash, Disengage or Hide"
G_SELF_MOVE = "Moves or leaps on its own, with or without provoking Opportunity Attacks"
G_TELEPORT = "Teleports itself"
G_TELEPORT_PLUS = "Teleports another creature, or between fixed points"
G_ETHEREAL = "Steps to or from the Ethereal Plane"
G_SHAPE = "Shape-Shift"
G_PARRY = "Reaction: raises its AC against the attack that hit"
G_SPLIT = "Reaction: splits into two smaller creatures"
G_SUMMON = "Creates or raises creatures under its control"
G_BUFF = "Bolsters itself or an ally"

# Actions other than attacks, read by hand: name | kind or * | monster or * | group | pattern | kinds needed | exception reason
ACTIONS_BY_HAND = f"""
Baleful Command | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate
Banish | * | * | Target save that banishes until the creature's next turn | save-target-rider | save_gate damage apply_condition teleport*
Bite | * | Vampire | {G_T_DRAIN} | save-target-rider | save_gate damage modifier:hp_max heal summon*
Bite | * | Vampire Spawn | {G_T_DRAIN} | save-target-rider | save_gate damage modifier:hp_max heal
Blinding Breath | * | * | {G_AREA_COND} | save-area-rider | area save_condition save_gate
Blinding Gaze | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition
Blinding Spittle | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate
Cacophony | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition edge*
Cloud of Insects | * | * | {G_T_BOTH} | save-target-rider | save_gate damage save_edge*
Cold Gale | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage forced_move save_gate
Constrict | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition
Consume Life | * | * | {G_T_KILL} | save-target-rider | save_gate damage heal
Consume Memories | * | * | {G_T_DMG} | save-target-damage | save_gate damage
Corrupting Touch | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition
Death Glare | * | * | {G_T_KILL} | save-target-rider | save_gate branch damage
Dominate Mind | * | * | {G_T_CONTROL} | save-target-rider | save_gate apply_condition control trigger:damaged
Dragon's Breath | * | * | {G_AREA_DMG} | save-area-damage | area save_damage save_gate choice
Draining Kiss | * | * | {G_T_DRAIN} | save-target-rider | save_gate damage modifier:hp_max
Dreadful Glare | * | Mummy | {G_T_COND} | save-target-rider | save_gate apply_condition modifier:immunity
Dreadful Glare | * | Mummy Lord | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition
Engulf | * | Shambling Mound | {G_CONTAIN} | contain | save_gate forced_move contain apply_condition trigger:turn_start damage
Engulf | * | Gelatinous Cube | {G_CONTAIN} | contain | movement save_gate damage contain apply_condition trigger:enters trigger:turn_start
Entangling Rope | * | * | {G_T_OBJECT} | save-target-rider | save_gate damage apply_condition create_object
Enthralling Panache | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition
Captain's Charm | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition
Fetid Cloud | * | * | {G_AREA_COND} | save-area-rider | area save_condition save_gate modifier:action_economy
Freezing Burst | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage immobile save_gate
Giggling Magic | * | * | {G_T_BOTH} | save-target-rider | save_gate damage bonus_die*
Heart Sight | * | * | Learns something only the DM can tell | dm-judgment | save_gate manual | What the sprite learns, the target's emotions and alignment, is the DM's to tell; only the save can be computed.
Horrific Visage | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate modifier:immunity
Horror Nimbus | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate modifier:immunity trigger:damaged
Infernal Tail | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition trigger:turn_start
Life Drain | * | Wight | {G_T_DRAIN} | save-target-rider | save_gate damage modifier:hp_max summon*
Mind-Rending Roar | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate
Moan | * | * | {G_AREA_COND} | save-area-rider | area save_condition save_gate modifier:immunity
Nightmare | * | * | {G_T_KILL} | save-target-rider | save_gate branch apply_condition damage trigger:damaged
Noxious Miasma | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_gate modifier:ac
Paralyzing Breath | * | * | {G_AREA_STAGED} | save-area-staged | area save_condition save_gate
Paralyzing Tentacles | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition
Petrifying Breath | * | * | {G_AREA_STAGED} | save-area-staged | area save_condition save_gate
Petrifying Gaze | * | * | {G_AREA_STAGED} | save-area-staged | area save_condition save_gate
Poisonous Spittle | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition
Possession | * | * | {G_T_CONTROL} | save-target-rider | save_gate apply_condition control conceal modifier:immunity
Quick Grapple | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition
Repulsion Breath | * | * | {G_AREA_COND} | save-area-rider | area forced_move save_condition save_gate
Roar | * | Lion | {G_T_COND} | save-target-rider | save_gate apply_condition
Roar | * | Sphinx of Valor | Area save that changes with each use | save-area-rider | area save_damage save_condition save_gate branch*
Scare | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition
Scorching Sands | * | * | {G_T_BOTH} | save-target-rider | save_gate damage speed_penalty*
Shield Bash | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition branch*
Slaying Bow | * | * | {G_T_KILL} | save-target-rider | save_gate branch damage
Sleep Breath | * | * | {G_AREA_STAGED} | save-area-staged | area save_condition save_gate trigger:damaged
Slowing Breath | * | * | {G_AREA_COND} | save-area-rider | area save_gate modifier:action_economy speed_penalty*
Spores | * | * | {G_AREA_COND} | save-area-rider | area save_condition save_gate trigger:turn_start damage
Steam Breath | * | Steam Mephit | {G_AREA_BOTH} | save-area-rider | area save_damage speed_penalty save_gate
Sting | * | Pseudodragon | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition branch
Stunning Screech | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate
Swallow | * | Giant Frog | {G_CONTAIN} | contain | contain apply_condition trigger:turn_end damage
Swallow | * | Giant Toad | {G_CONTAIN} | contain | contain apply_condition trigger:turn_end damage
Swallow | * | * | {G_CONTAIN} | contain | save_gate contain apply_condition trigger:turn_start damage
Tentacle Slam | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition
Thunderclap | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate
Thunderous Bellow | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate
Toxic Ink | * | * | {G_AREA_COND} | save-area-rider | area save_condition save_gate movement
Unsettling Visage | * | * | {G_AREA_COND} | save-area-rider | area save_condition save_gate
Vortex | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition trigger:turn_start modifier:action_economy
Weakening Breath | * | * | {G_AREA_COND} | save-area-rider | area save_gate edge* modifier:damage_dealt
Web | * | * | {G_T_OBJECT} | save-target-rider | save_gate apply_condition create_object
Web Strand | * | * | {G_T_OBJECT} | save-target-rider | save_gate apply_condition create_object
Weight of Years | * | * | {G_T_COND} | save-target-rider | save_gate apply_condition
Whelm | * | * | {G_CONTAIN} | contain | area save_damage save_gate contain apply_condition branch* trigger:turn_start damage
Whirlwind | * | * | {G_T_BOTH} | save-target-rider | save_gate damage forced_move apply_condition
Unnerving Gaze | * | * | Reaction: forces a save on a creature that starts its turn nearby | reaction-other | trigger:starts_turn_near save_gate apply_condition modifier:immunity
Animate Trees | * | * | {G_SUMMON} | summon | summon*
Create Specter | * | * | {G_SUMMON} | summon | summon*
Antennae | * | * | {G_CORRODE} | item-wear | save_gate item_wear
Destroy Metal | * | * | {G_CORRODE} | item-wear | item_wear
Reflexive Antennae | * | * | {G_CORRODE} | item-wear | trigger:hit_by_attack sequence
Aquatic Charge | * | * | {G_SELF_MOVE} | self-move | movement
Bubble Dash | * | * | {G_SELF_MOVE} | self-move | movement
Charge | * | * | {G_SELF_MOVE} | self-move | movement
Leap | * | * | {G_SELF_MOVE} | self-move | movement
Swoop | * | * | {G_SELF_MOVE} | self-move | movement
Prowl | * | * | {G_SELF_MOVE} | self-move | movement sequence
Cunning Action | * | * | {G_STANDARD} | standard-action |
Deathless Agility | * | * | {G_STANDARD} | standard-action |
Nimble Escape | * | * | {G_STANDARD} | standard-action |
Hasten | * | * | {G_STANDARD} | standard-action |
Shadow Stealth | * | * | {G_STANDARD} | standard-action |
Arcane Prowl | * | * | {G_MOVE_ATTACK} | use-other-action | teleport sequence
Pounce | * | * | {G_MOVE_ATTACK} | use-other-action | movement sequence
Charging Horn | * | * | {G_MOVE_ATTACK} | use-other-action | movement sequence
Deathless Strike | * | * | {G_MOVE_ATTACK} | use-other-action | movement sequence
Onslaught | * | * | {G_MOVE_ATTACK} | use-other-action | movement sequence
Rampage | * | * | {G_MOVE_ATTACK} | use-other-action | movement sequence branch*
Lash | * | * | {G_USE} | use-other-action | sequence
Tail Swipe | * | * | {G_USE} | use-other-action | sequence
Necrotic Strike | * | * | {G_USE} | use-other-action | sequence
Storm Bolt | * | * | {G_USE} | use-other-action | sequence
Glare | * | * | {G_USE} | use-other-action | sequence
Psychic Drain | * | * | {G_USE} | use-other-action | sequence heal
Cloaked Flight | * | * | {G_CAST} | cast-named-spell | cast_spell movement
Nightmare Haunting | * | * | {G_CAST} | cast-named-spell | cast_spell modifier:hp_max
Boulder Toss | * | * | {G_AREA_BOTH} | save-area-rider | area save_damage save_condition save_gate branch*
Deadly Leap | * | * | {G_AREA_BOTH} | save-area-rider | movement area save_damage save_condition forced_move save_gate
Fling | * | * | {G_AREA_BOTH} | save-area-rider | forced_move area save_damage save_condition save_gate
Radiant Teleport | * | * | {G_AREA_DMG} | save-area-damage | teleport area save_damage save_gate
Trampling Charge | * | * | {G_T_BOTH} | save-target-rider | movement trigger:enters save_gate damage apply_condition
Conjure Infernal Chain | * | * | {G_T_BOTH} | save-target-rider | save_gate damage apply_condition forced_move branch*
Luring Song | * | * | Area save that charms and compels the target toward the creature | save-area-rider | area save_gate apply_condition compel modifier:immunity trigger:damaged
Create Whirlwind | * | * | {G_ZONE} | zone | area save_gate apply_condition contain trigger:enters trigger:turn_start damage
Darkness Aura | * | * | {G_ZONE} | zone | area create_surface
Ink Cloud | * | Giant Octopus | {G_ZONE} | zone | trigger:damaged area create_surface movement
Ink Cloud | * | Octopus | {G_ZONE} | zone | trigger:ends_turn_near area create_surface movement
World-Shaking Movement | * | * | Moves, then knocks creatures around it Prone and breaks Concentration | use-other-action | movement area apply_condition branch* modifier:concentration
Deathly Teleport | * | * | {G_TELEPORT} | teleport | teleport area damage
Teleport | * | Balor | {G_TELEPORT_PLUS} | teleport | teleport*
Teleport | * | * | {G_TELEPORT} | teleport | teleport
Tree Stride | * | * | {G_TELEPORT_PLUS} | teleport | teleport*
Ethereal Jaunt | * | * | {G_ETHEREAL} | teleport | conceal
Ethereal Stride | * | * | {G_ETHEREAL} | teleport | conceal
Shape-Shift | * | Vampire | {G_SHAPE} | shape-shift | form* modifier:resistance move_rule
Shape-Shift | * | Mimic | {G_SHAPE} | shape-shift | form* conceal
Shape-Shift | * | * | {G_SHAPE} | shape-shift | form*
Parry | * | Black Pudding | {G_SPLIT} | reaction-other | trigger:damaged summon*
Split | * | * | {G_SPLIT} | reaction-other | trigger:damaged summon*
Parry | * | * | {G_PARRY} | reaction-defence | trigger:hit_by_attack modifier:ac
Riposte | * | * | {G_PARRY} | reaction-defence | trigger:hit_by_attack modifier:ac sequence
Protection | * | * | {G_PARRY} | reaction-defence | trigger:hit_by_attack modifier:ac
Whirlwind of Sand | * | * | {G_PARRY} | reaction-defence | trigger:hit_by_attack modifier:ac teleport apply_condition
Deflect Missile | * | * | Reaction: reduces the damage of a ranged hit and may throw it back | reaction-defence | trigger:hit_by_attack modifier:damage_taken save_gate damage
Burst of Ingenuity | * | * | Reaction: adds a bonus to a roll being made | reaction-other | trigger:d20_test modifier:d20
Pursuit | * | * | Reaction: teleports next to a creature that ends its move nearby | reaction-other | trigger:moves_near sequence
Redirect Attack | * | * | Reaction: swaps places with an ally, who takes the attack | reaction-other | trigger:attacked movement modifier:retarget
Shriek | * | * | Shrieks when something comes near | dm-judgment | trigger:moves_near manual | The shriek does nothing but warn whoever the DM decides can hear it within 300 feet.
Reel | * | * | Pulls creatures it already holds toward itself | pull | forced_move
Ignited Illumination | * | * | Sheds light | light | light
Shimmering Shield | * | * | {G_BUFF} | self-buff | temp_hp modifier:ac
War Cry | * | * | {G_BUFF} | self-buff | temp_hp edge
Vanish | * | * | Turns Invisible | self-buff | apply_condition
"""

G_NO_RULE_BREATH = "Breathing: air, water, holding breath"
G_NO_RULE_REVIVE = "Returns to life elsewhere, or days later"
G_NO_RULE_TALK = "Speech, telepathy and bonds"
G_NO_RULE_OTHER = "Other traits with nothing to compute at the table"
G_EDGE = "Advantage or Disadvantage that holds only in a state"
G_MAGIC_RES = "Magic Resistance"
G_NO_OA = "Leaves reach without provoking Opportunity Attacks"
G_SQUEEZE = "Squeezes through gaps, or shares other creatures' spaces"
G_TERRAIN = "Ignores a kind of terrain, or climbs, burrows or passes through it"
G_JUMP = "Fixed jump distances"
G_AURA_DMG = "Aura that damages creatures around it each turn"
G_AURA_SAVE = "Aura that forces a save on creatures that start their turn in it"
G_DEATH = "Explodes when it dies"
G_ABSORB = "Regains hit points from a damage type"
G_REGEN = "Regeneration"
G_ON_DAMAGE = "Weakened by a damage type or a surrounding"
G_MIND = "Mind and detection wards"
G_DM_SENSE = "Senses a kind of thing in the world (magic, metal, gems)"
G_ITEM = "Carries an object with its own AC and hit points"

# Traits, read by hand: name | monster or * | group | pattern | kinds needed | exception reason
TRAITS_BY_HAND = f"""
Amphibious | * | {G_NO_RULE_BREATH} | no-table-rule |
Water Breathing | * | {G_NO_RULE_BREATH} | no-table-rule |
Hold Breath | * | {G_NO_RULE_BREATH} | no-table-rule |
Limited Amphibiousness | * | {G_NO_RULE_BREATH} | no-table-rule |
Celestial Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Demonic Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Diabolical Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Eldritch Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Elemental Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Exalted Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Fiendish Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Hellish Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Undead Restoration | * | {G_NO_RULE_REVIVE} | no-table-rule |
Spirit Jar | * | {G_NO_RULE_REVIVE} | no-table-rule |
Troll Spawn | * | {G_NO_RULE_REVIVE} | no-table-rule |
Speak with Beasts and Plants | * | {G_NO_RULE_TALK} | no-table-rule |
Telepathic Bond | * | {G_NO_RULE_TALK} | no-table-rule |
Vampiric Connection | * | {G_NO_RULE_TALK} | no-table-rule |
Shark Telepathy | * | {G_NO_RULE_TALK} | no-table-rule |
Beast of Burden | * | {G_NO_RULE_OTHER} | no-table-rule |
Ephemeral | * | {G_NO_RULE_OTHER} | no-table-rule |
Incubus Form | * | {G_NO_RULE_OTHER} | no-table-rule |
Succubus Form | * | {G_NO_RULE_OTHER} | no-table-rule |
Legendary Resistance | * | Legendary Resistance | legendary-resistance |
Pack Tactics | * | Pack Tactics | conditional-edge | edge*
Magic Resistance | * | {G_MAGIC_RES} | conditional-edge | save_edge*
Greater Magic Resistance | * | {G_MAGIC_RES} | conditional-edge | save_edge* modifier:immunity
Blood Frenzy | * | {G_EDGE} | conditional-edge | edge*
Bloodied Frenzy | * | {G_EDGE} | conditional-edge | edge* save_edge*
Bloodied Fury | * | {G_EDGE} | conditional-edge | edge*
Sunlight Sensitivity | * | {G_EDGE} | conditional-edge | edge*
Sunlight Weakness | * | {G_EDGE} | conditional-edge | edge* save_edge*
Light Sensitivity | * | {G_EDGE} | conditional-edge | edge*
Blurred Form | * | {G_EDGE} | conditional-edge | edge*
Aversion to Fire | * | {G_EDGE} | conditional-edge | trigger:damaged edge*
Aura of Authority | * | Aura that gives Advantage to the creature and its allies | conditional-edge | area edge* save_edge*
Agile | * | {G_NO_OA} | move-rule | move_rule
Flyby | * | {G_NO_OA} | move-rule | move_rule
Air Form | * | {G_SQUEEZE} | move-rule | move_rule
Water Form | * | {G_SQUEEZE} | move-rule | move_rule
Fire Form | * | {G_SQUEEZE} | move-rule | move_rule trigger:enters damage
Amorphous | * | {G_SQUEEZE} | move-rule | move_rule
Compression | * | {G_SQUEEZE} | move-rule | move_rule
Swarm | * | {G_SQUEEZE} | move-rule | move_rule modifier:no_healing
Abduct | * | Drags a grappled creature at no extra movement | move-rule | move_rule
Earth Glide | * | {G_TERRAIN} | move-rule | move_rule
Tunneler | * | {G_TERRAIN} | move-rule | move_rule
Ice Walk | * | {G_TERRAIN} | move-rule | move_rule
Spider Climb | * | {G_TERRAIN} | move-rule | move_rule
Web Walker | * | {G_TERRAIN} | move-rule | move_rule sense
Incorporeal Movement | * | {G_TERRAIN} | move-rule | move_rule trigger:turn_end damage
Standing Leap | * | {G_JUMP} | move-rule | move_rule
Running Leap | * | {G_JUMP} | move-rule | move_rule
Jumper | * | {G_JUMP} | move-rule | move_rule
Fire Aura | * | {G_AURA_DMG} | aura | trigger:turn_end area damage
Heat Aura | * | {G_AURA_DMG} | aura | trigger:turn_end area damage
Barbed Hide | * | {G_AURA_DMG} | aura | trigger:turn_start damage
Stench | * | {G_AURA_SAVE} | aura | trigger:starts_turn_near area save_gate save_condition modifier:immunity
Fear Aura | * | {G_AURA_SAVE} | aura | trigger:starts_turn_near area save_gate save_condition modifier:immunity
Vile Appearance | * | {G_AURA_SAVE} | aura | trigger:starts_turn_near area save_gate save_condition modifier:immunity
Gibbering | * | {G_AURA_SAVE} | aura | trigger:starts_turn_near area save_gate compel
Mucus Cloud | * | {G_AURA_SAVE} | aura | trigger:turn_end area save_gate apply_condition modifier:no_healing trigger:clock damage
Aberrant Ground | * | Ground around it is Difficult Terrain | zone | area create_surface*
Death Burst | * | {G_DEATH} | on-death | trigger:death area save_damage save_gate
Death Throes | * | {G_DEATH} | on-death | trigger:death area save_damage save_gate
Acid Absorption | * | {G_ABSORB} | self-heal | trigger:damaged heal
Fire Absorption | * | {G_ABSORB} | self-heal | trigger:damaged heal
Lightning Absorption | * | {G_ABSORB} | self-heal | trigger:damaged heal
Regeneration | Troll | {G_REGEN} | self-heal | trigger:turn_start heal trigger:damaged
Regeneration | Troll Limb | {G_REGEN} | self-heal | trigger:turn_start heal trigger:damaged
Regeneration | * | {G_REGEN} | self-heal | trigger:turn_start heal
Freeze | * | {G_ON_DAMAGE} | on-damage | trigger:damaged speed_penalty
Water Susceptibility | * | {G_ON_DAMAGE} | on-damage | trigger:enters damage
Corrosive Form | Black Pudding | {G_CORRODE} | item-wear | trigger:hit_by_attack damage item_wear
Corrosive Form | * | {G_CORRODE} | item-wear | trigger:hit_by_attack item_wear
Adhesive | * | Grapples whatever touches it | on-damage | trigger:hit_by_attack apply_condition edge*
Illumination | * | Sheds light | light | light
Ethereal Sight | * | Sees into the Ethereal Plane | sense | sense
Transparent | * | Goes unnoticed until it moves | passive-modifier | conceal
Invisibility | * | Turns Invisible | passive-modifier | apply_condition
Siege Monster | * | Double damage to objects and structures | passive-modifier | modifier:damage_dealt
Evasion | * | Evasion | passive-modifier | modifier:save_outcome
Reactive | * | Extra Reactions | passive-modifier | modifier:reaction_count
Reactive Heads | * | Extra Reactions | passive-modifier | modifier:reaction_count
Immutable Form | * | {G_MIND} | passive-modifier | modifier:immunity
Inscrutable | * | {G_MIND} | passive-modifier | modifier:immunity edge*
Shielded Mind | * | {G_MIND} | passive-modifier | modifier:immunity
Confer Fire Resistance | * | Gives its rider Resistance to Fire | passive-modifier | modifier:resistance
Draconic Origin | * | Damage type chosen when the creature is placed | passive-modifier | choice
Training | * | Skill chosen when the creature is placed | passive-modifier | choice modifier:skill
Bound | * | Shares the damage its amulet's wearer takes | special | modifier:damage_share
Magic Rope | * | {G_ITEM} | special | create_object
Soul Bag | * | {G_ITEM} | special | create_object
Spell Storing | * | Stores one spell and casts it later | special | cast_spell
Berserk | * | Goes berserk while Bloodied and attacks the nearest creature | special | trigger:turn_start branch* compel
Loathsome Limbs | * | Loses limbs that fight on | special | trigger:turn_end branch* summon* apply_condition
Misty Escape | * | Turns to mist at 0 Hit Points | special | trigger:drops_to_0 form* apply_condition heal
Multiple Heads | * | Heads die and regrow | special | trigger:damaged trigger:turn_end heal resource_change*
Ooze Cube | * | {G_CONTAIN} | contain | contain trigger:enters
Reflective Carapace | * | Shrugs off or reflects spells aimed at it | special | trigger:targeted_by_spell modifier:retarget
Undead Fortitude | * | Saves to stay at 1 Hit Point | special | trigger:drops_to_0 save_gate
Vampire Weakness | * | Vampire weaknesses | dm-judgment | trigger:turn_start trigger:turn_end damage edge* manual | Forbiddance and the stake through the heart hang on the fiction (what counts as a residence, an invitation, a resting place); the sunlight and running-water parts should still be computed.
Wishes | * | Grants wishes | dm-judgment | manual | Whether the genie knows Wish, and what a wish does, is the DM's ruling.
Mimicry | * | Mimics sounds and voices | dm-judgment | manual | What a listener believes is roleplay; only the Insight check is a number.
Probing Telepathy | * | Learns something only the DM can tell | dm-judgment | manual | A creature's greatest desires exist only in the DM's fiction.
Divine Awareness | * | Learns something only the DM can tell | dm-judgment | manual | Whether a statement is a lie is known only to the DM.
Sense Magic | * | {G_DM_SENSE} | dm-judgment | manual | Which objects and creatures are magical is world detail the engine does not hold; the DM tells.
Iron Scent | * | {G_DM_SENSE} | dm-judgment | manual | Where ferrous metal lies is world detail the engine does not hold; the DM tells.
Treasure Sense | * | {G_DM_SENSE} | dm-judgment | manual | Where precious metals and stones lie is world detail the engine does not hold; the DM tells.
"""


def table(text, width):
    rows = []
    for line in text.strip().splitlines():
        cells = [c.strip() for c in line.split("|")]
        cells += [""] * (width - len(cells))
        rows.append(cells)
    return rows


HAND_ATTACKS = {(r[0], r[1]): (r[2], r[3], r[4], "") for r in table(ATTACKS_BY_HAND, 5)}
HAND_ACTIONS = table(ACTIONS_BY_HAND, 7)
HAND_TRAITS = table(TRAITS_BY_HAND, 6)
USED = set()


def by_hand(rows, e, kind_col):
    """The first hand row for an entry: one naming its monster wins over one for every monster."""
    for specific in (True, False):
        for i, r in enumerate(rows):
            name, monster = r[0], r[2] if kind_col else r[1]
            if name != e.base or (monster == "*") == specific or (specific and monster != e.monster):
                continue
            if kind_col and r[1] not in ("*", e.kind):
                continue
            USED.add((id(rows), i))
            rest = r[3:] if kind_col else r[2:]
            return rest[0], rest[1], rest[2], rest[3]
    return None


# ---------------------------------------------------------------------------------------------------
# Classification
# ---------------------------------------------------------------------------------------------------


class Unaccounted(Exception):
    pass


def structured(e):
    """How the structured attack compares with the text: 'agrees', 'differs' or 'missing'."""
    if not e.attacks:
        return "missing"
    at = e.attacks[0]
    hit = re.search(r"Attack Roll: \+(\d+)", e.text)
    dmg = re.search(
        r"(?:ft\.?|feet\.|Hit:) (\d+)(?: \((\d+d\d+)(?: ([+-]) (\d+))?\))? (\w+) damage(?: plus (\d+)(?: \((\d+d\d+)(?: [+-] \d+)?\))? (\w+) damage)?",
        e.text,
    )
    if not hit or not dmg or int(hit.group(1)) != at.get("toHit"):
        return "differs"
    flat, dice, sign, bonus, kind, _, extra_dice, extra_kind = dmg.groups()
    amount = int(bonus or 0) * (-1 if sign == "-" else 1)
    if dice is None:
        amount, dice = int(flat), None
    same = at.get("damageDice") == dice and at.get("damageBonus", 0) == amount and at.get("damageType") == kind.lower()
    extra = at.get("extraDice") == extra_dice and at.get("extraType") == (extra_kind.lower() if extra_kind else None)
    if extra_kind and not extra_dice:
        extra = False  # a flat second damage type has no column to live in
    return "agrees" if same and extra else "differs"


def attack(e):
    hand = HAND_ATTACKS.get((e.monster, e.base))
    if hand:
        USED.add(("attack", e.monster, e.base))
        return hand
    t = shape(e.text)
    head = HEAD.match(t)
    if not head:
        raise Unaccounted("attack head")
    rest = (head.group("paren") or "") + " " + BASE_DAMAGE.sub("", t[head.end():], count=1)
    needs, groups = [], []
    for pattern, kinds, group in RIDERS:
        rest, n = re.subn(pattern, " ", rest)
        if n:
            needs += kinds.split()
            groups.append(group)
    if re.sub(r"[\s.,]", "", rest):
        raise Unaccounted("attack rider: " + rest.strip())
    groups = [g for g in groups if g]
    if not groups:
        state = structured(e)
        group = {"agrees": G_PLAIN, "differs": G_PLAIN_WRONG, "missing": G_PLAIN_TEXT}[state]
        return group, "plain-attack", "", ""
    group = min(groups, key=RANK.index)
    return group, RIDER_PATTERN[group], " ".join(dict.fromkeys(needs)), ""


PURE_DAMAGE = re.compile(
    r"^\w+ Saving Throw: DC N, (?P<target>[^.]*(?:\[Area of Effect\]\|XPHB\|\w+)?[^.]*)\. Failure: DMG \w+ damage(?: plus DMG \w+ damage)?\."
    r"(?: Success: Half damage\.)?"
    r"(?: Failure or Success: (?:the X can't take this action again until the start of its next turn|Being underwater doesn't grant Resistance to this \w+ damage)\.)?$"
)
AREA_WORDS = re.compile(r"Cone|Line|Sphere|Cylinder|Emanation|Cube")


COUNTED = re.compile(
    r"^The [\w' -]+? makes \w+ (?:[A-Z][\w']*(?: [A-Z][\w']*)* attacks|attacks, using [\w' ,-]+ in any combination|[A-Z][\w' ]*? or [A-Z][\w' ]*? attacks)\.$"
)


def multiattack(e):
    if COUNTED.match(e.text):
        return G_MULTI_COUNT, "multiattack-count", "", ""
    if re.search(r"\bto cast\b", e.text):
        return G_MULTI_SPELL, "multiattack-mixed", "sequence cast_spell", ""
    if re.search(r"\buses?\b|\buse of\b|if available|as many|if it used", e.text):
        return G_MULTI_MIX, "multiattack-mixed", "sequence", ""
    return G_MULTI, "multiattack-attacks", "sequence", ""


def action(e):
    if "Attack Roll" in e.text:
        return attack(e)
    if e.base == "Multiattack":
        return multiattack(e)
    if e.base == "Spellcasting":
        return G_LIST, "spellcasting-list", "cast_spell", ""
    hand = by_hand(HAND_ACTIONS, e, True)
    if hand:
        return hand
    t = shape(e.text)
    pure = PURE_DAMAGE.match(t)
    if pure:
        if AREA_WORDS.search(pure.group("target")):
            wide = re.search(r"(\d+)-foot-wide Line", e.text)
            return G_AREA_DMG, "save-area-damage", "area* save_damage save_gate" if wide and wide.group(1) != "5" else "area save_damage save_gate", ""
        return G_T_DMG, "save-target-damage", "save_gate damage", ""
    if re.search(r"\b(?:uses Spellcasting to cast|casts) (?:the )?[A-Z]", e.text):
        return G_CAST, "cast-named-spell", "cast_spell", ""
    raise Unaccounted("action")


def trait(e):
    hand = by_hand(HAND_TRAITS, e, False)
    if not hand:
        raise Unaccounted("trait")
    return hand


TARGETING = re.compile(
    r"each enemy|any enemy|its allies|creature of (?:the [\w-]+'s|its) choice|that isn't an Undead|isn't an Undead|non-Undead creature|"
    r"one Humanoid|each Humanoid|any Beast or Humanoid|target is a Humanoid|one (?:" + SIZES + r") or smaller creature|"
    r"Grappled by (?:the|it)|Charmed by the|Charmed or Grappled|Restrained by its|that has the \w+ condition|that has the Grappled|"
    r"one Frightened creature|that can see the|and can see the|willing demon|to a rider|that has 0 Hit Points|demons succeed automatically|"
    r"Humanoid corpse|it is grappling|creature it is grappling|has a creature Grappled|an enemy it can"
)
ENVIRONMENT = re.compile(
    r"\bunderwater\b|\bsunlight\b|in Bright Light|Dim Light or darkness|running water|Ethereal Plane|moves in water|caused by webs|"
    r"Large or bigger tree|icy surfaces|unworked earth|solid rock|ceilings|a body of water"
)


def other_gaps(e, pattern, needs):
    gaps = []
    if e.kind == "legendary":
        gaps.append("legendary")
    if e.kind == "reaction":
        gaps.append("reaction")
    if e.kind == "bonus" and pattern not in ("plain-attack",) and needs != "teleport":
        gaps.append("activation_cost")
    if e.kind == "action" and needs == "teleport":
        gaps.append("activation_cost")
    if pattern == "legendary-resistance":
        gaps.append("legendary")
    if "can't take this action again until" in e.text:
        gaps.append("cooldown")
    if re.search(r"Recharge \d", e.name):
        gaps.append("recharge")
    if "Short or Long Rest" in e.name or (e.base == "Roar" and "Long Rest" in e.text):
        gaps.append("per_rest")
    if (re.search(r"\d/Day", e.name + e.text) and pattern != "legendary-resistance") or e.base == "Wishes":
        gaps.append("per_day")
    if pattern not in ("plain-attack", "no-table-rule"):
        if TARGETING.search(e.text):
            gaps.append("targeting")
        if ENVIRONMENT.search(e.text) and "doesn't grant Resistance" not in e.text:
            gaps.append("environment")
    if "Attack Roll" in e.text and structured(e) != "agrees":
        gaps.append("seed_data")
    if (e.monster, e.base) == ("Black Pudding", "Parry"):
        gaps.append("seed_data")
    if pattern == "multiattack-count":
        gaps.append("attack_count")
    if e.kind == "trait" and needs:
        gaps.append("trait_owner")
    return gaps


Row = collections.namedtuple("Row", "entry group pattern needs missing gaps exception")

# The order kinds are written in: when it fires, the gate, what lands, then the rest by name.
FIRST = ["trigger", "sequence", "cast_spell", "movement", "teleport", "teleport*", "attack_gate", "area", "area*", "save_gate", "branch", "branch*", "choice",
         "save_damage", "damage", "save_condition", "apply_condition", "forced_move", "contain"]


def token_order(token):
    kind = token.split(":")[0]
    return (FIRST.index(kind) if kind in FIRST else len(FIRST), token)


def classify(es, existing):
    rows, lost = [], []
    for e in es:
        try:
            group, pattern, needs, exception = trait(e) if e.kind == "trait" else action(e)
        except Unaccounted as why:
            lost.append((e, str(why)))
            continue
        kinds = sorted(dict.fromkeys(needs.split()), key=token_order)
        for k in kinds:
            if k.rstrip("*").split(":")[0] in existing and (":" in k):
                raise SystemExit("sub-target on an existing kind: " + k)
        missing = [k for k in kinds if k.endswith("*") or k.split(":")[0] not in existing]
        if exception:
            missing = [k for k in missing if k != "manual"]
        rows.append(Row(e, group, pattern, kinds, missing, other_gaps(e, pattern, needs), exception))
    return rows, lost


# ---------------------------------------------------------------------------------------------------
# Output
# ---------------------------------------------------------------------------------------------------


def kind_of(token):
    """The component kind a token names: modifier:ac is modifier, edge* stays edge*."""
    return token.split(":")[0]


def write_csv(rows):
    with open(HERE / "census-monsters.csv", "w", newline="") as fh:
        w = csv.writer(fh, lineterminator="\n")
        w.writerow(["monster", "monster_name", "kind", "name", "group", "pattern", "kinds_needed", "kinds_missing", "other_gaps",
                    "automatable_with_todays_kinds", "manual_exception", "exception_reason"])
        for r in sorted(rows, key=lambda r: (r.entry.slug, r.entry.kind, r.entry.name)):
            e = r.entry
            w.writerow([e.slug, e.monster, e.kind, e.name, r.group, r.pattern, " ".join(r.needs), " ".join(r.missing), " ".join(r.gaps),
                        "yes" if not r.missing and not r.exception else "no", "yes" if r.exception else "no", r.exception])


ORDER = ["action", "bonus", "reaction", "legendary", "trait"]


def group_table(rows):
    groups = collections.defaultdict(list)
    for r in rows:
        groups[(r.group, r.entry.kind, tuple(r.needs), r.exception)].append(r)
    out = ["| Group (members) | Kind | Entries | Monsters | Kinds needed | Kinds missing today | Other gaps | Pattern | Manual exception |",
           "|---|---|---|---|---|---|---|---|---|"]
    keys = sorted(groups, key=lambda k: (groups[k][0].pattern, k[0], ORDER.index(k[1]), k[2]))
    for key in keys:
        rs = groups[key]
        names = collections.Counter(r.entry.base for r in rs)
        shown = ", ".join(n for n, _ in names.most_common(6))
        if len(names) > 6:
            shown += f", and {len(names) - 6} more"
        gaps = collections.Counter(g for r in rs for g in r.gaps)
        gap_text = ", ".join(g if n == len(rs) else f"{g} ({n})" for g, n in sorted(gaps.items())) or "none"
        needs = " ".join(f"`{k}`" for k in rs[0].needs) or ("none (structured attack)" if rs[0].pattern == "plain-attack" else "none")
        missing = " ".join(f"`{k}`" for k in rs[0].missing) or "none"
        exception = "yes: " + rs[0].exception if rs[0].exception else "no"
        out.append(f"| **{key[0]}** ({shown}) | {key[1]} | {len(rs)} | {len({r.entry.slug for r in rs})} | {needs} | {missing} | {gap_text} | `{rs[0].pattern}` | {exception} |")
    return out, len(keys)


def split(rs):
    a = sum(1 for r in rs if r.entry.kind != "trait")
    return a, len(rs) - a


def pattern_table(rows):
    by = collections.defaultdict(list)
    for r in rows:
        by[r.pattern].append(r)
    out = ["| Pattern | Actions | Traits | Automatable with today's kinds | Proposed exceptions |", "|---|---|---|---|---|"]
    for p, rs in sorted(by.items(), key=lambda kv: -len(kv[1])):
        a, t = split(rs)
        ok = sum(1 for r in rs if not r.missing and not r.exception)
        out.append(f"| `{p}` | {a} | {t} | {ok} | {sum(1 for r in rs if r.exception)} |")
    return out


def missing_table(rows):
    need, sole = collections.defaultdict(list), collections.defaultdict(list)
    for r in rows:
        kinds = {kind_of(k) for k in r.missing}
        for k in kinds:
            need[k].append(r)
        if len(kinds) == 1 and not r.exception:
            sole[next(iter(kinds))].append(r)
    out = ["| Missing kind | Actions needing it | Traits needing it | Actions it alone unlocks | Traits it alone unlocks |", "|---|---|---|---|---|"]
    for k in sorted(need, key=lambda k: (-len(need[k]), k)):
        a, t = split(need[k])
        sa, st = split(sole[k])
        out.append(f"| `{k}` | {a} | {t} | {sa} | {st} |")
    return out, need


def missing_sets(rows):
    """The exact sets of missing kinds, most common first: what has to exist together to unlock a group."""
    by = collections.defaultdict(list)
    for r in rows:
        if r.missing and not r.exception:
            by[frozenset(kind_of(k) for k in r.missing)].append(r)
    out = ["| Missing together | Actions unlocked | Traits unlocked | Mostly |", "|---|---|---|---|"]
    ranked = sorted(by.items(), key=lambda kv: (-len(kv[1]), sorted(kv[0])))
    for kinds, rs in ranked[:SETS_SHOWN]:
        a, t = split(rs)
        mostly = ", ".join(f"`{p}`" for p, _ in collections.Counter(r.pattern for r in rs).most_common(2))
        out.append(f"| {' '.join(f'`{k}`' for k in sorted(kinds, key=token_order))} | {a} | {t} | {mostly} |")
    rest = [r for _, rs in ranked[SETS_SHOWN:] for r in rs]
    a, t = split(rest)
    out.append(f"| {len(ranked) - SETS_SHOWN} other sets | {a} | {t} | |")
    return out


SETS_SHOWN = 24


def sub_targets(rows):
    by = collections.defaultdict(collections.Counter)
    for r in rows:
        for k in set(r.missing):
            if ":" in k:
                by[kind_of(k)][k.split(":")[1]] += 1
    out = []
    for kind, subs in sorted(by.items()):
        out.append(f"- `{kind}`: " + ", ".join(f"`{s}` {n}" for s, n in subs.most_common()))
    return out


def gap_table(rows):
    by = collections.defaultdict(list)
    for r in rows:
        for g in r.gaps:
            by[g].append(r)
    out = ["| Other gap | Actions | Traits |", "|---|---|---|"]
    for g, rs in sorted(by.items(), key=lambda kv: -len(kv[1])):
        a, t = split(rs)
        out.append(f"| `{g}` | {a} | {t} |")
    return out


def numbers(rows, ms, existing, migration):
    acts = [r for r in rows if r.entry.kind != "trait"]
    traits = [r for r in rows if r.entry.kind == "trait"]
    plain = [r for r in acts if r.pattern == "plain-attack"]
    ok = [r for r in rows if not r.missing and not r.exception]
    clean = [r for r in ok if not r.gaps]
    lines = [
        f"- Monsters: {len(ms)}. Actions: {len(acts)} ({', '.join(f'{sum(1 for r in acts if r.entry.kind == k)} {k}' for k in ORDER[:4])}). Traits: {len(traits)}.",
        f"- Component kinds in `{migration}`: {len(existing)}.",
        f"- Plain attacks: {len(plain)} actions. Structured data agrees with the text for {sum(1 for r in plain if r.group == G_PLAIN)}, "
        f"disagrees for {sum(1 for r in plain if r.group == G_PLAIN_WRONG)}, and is absent for {sum(1 for r in plain if r.group == G_PLAIN_TEXT)}.",
        f"- Of the {sum(1 for r in plain if r.group == G_PLAIN_WRONG)} that disagree, {sum(1 for r in plain if r.group == G_PLAIN_WRONG and ' plus ' in r.entry.text and not r.entry.attacks[0].get('extraDice'))} "
        f"lose a second damage type the text gives (`plus N (XdY) Type damage`).",
        f"- Attack-roll actions in all: {sum(1 for r in acts if 'Attack Roll' in r.entry.text)}, of which {sum(1 for r in acts if 'Attack Roll' in r.entry.text and r.pattern != 'plain-attack')} carry a rider.",
        f"- Automatable with today's kinds only: {split(ok)[0]} actions and {split(ok)[1]} traits ({len(ok)} entries). "
        f"With no other gap either: {split(clean)[0]} actions and {split(clean)[1]} traits.",
        f"- Of the {len([r for r in rows if set(r.missing) == {'save_gate'}])} entries whose only missing kind is `save_gate`, "
        f"{len([r for r in rows if set(r.missing) == {'save_gate'} and 'area' in r.needs])} are area saves the engine already resolves but for the DC.",
        f"- Entries missing at least one kind: {split([r for r in rows if r.missing])[0]} actions and {split([r for r in rows if r.missing])[1]} traits.",
        f"- Proposed Manual exceptions: {split([r for r in rows if r.exception])[0]} actions and {split([r for r in rows if r.exception])[1]} traits.",
    ]
    by = collections.defaultdict(list)
    for r in rows:
        by[r.entry.slug].append(r)
    no_kind = [m for m, rs in by.items() if all(not r.missing and not r.exception for r in rs)]
    no_gap = [m for m in no_kind if all(not r.gaps for r in by[m])]
    lines.append(f"- Monsters whose every action and trait needs no new kind: {len(no_kind)} of {len(ms)}; with no other gap either: {len(no_gap)}.")
    return lines


def spells_cast(rows, names):
    """Every spell a monster action casts, by name, with the number of actions that cast it."""
    cast = collections.Counter()
    for r in rows:
        if "cast_spell" not in r.needs:
            continue
        found = set()
        if r.pattern == "spellcasting-list":
            for line in r.entry.text.split("\n"):
                m = re.match(r"- \*\*(.+?):\*\*\s*(.*)", line.strip())
                for s in re.split(r",\s*(?![^()]*\))", m.group(2)) if m else []:
                    if s.strip():
                        found.add(re.sub(r"\s*\(.*\)", "", s).strip().lower())
        else:
            for part in re.findall(r"\b(?:casts?|to cast) ([^.:]*)", r.entry.text):
                for n in names:
                    if re.search(r"\b" + re.escape(n) + r"\b", part):
                        found.add(n.lower())
                        part = part.replace(n, " ")
        cast.update(found)
    return cast


def economy(rows, ms, names, seeded):
    acts = [r for r in rows if r.entry.kind != "trait"]
    leg = [r for r in acts if r.entry.kind == "legendary"]
    spell_list = [r for r in acts if r.pattern == "spellcasting-list"]
    cast = [r for r in rows if "cast_spell" in r.needs]
    breath = [r for r in acts if r.entry.base.endswith("Breath")]
    spells = spells_cast(rows, names)
    automated = sorted(s for s in spells if slug(s) in seeded)
    proper = {n.lower(): n for n in names}
    lair = [m for m in ms if any("in Lair" in t["name"] for t in m["traits"])]
    lines = [
        f"- Recharge: {sum(1 for r in acts if 'recharge' in r.gaps)} actions carry `Recharge N-6` in their name and {sum(1 for r in acts if 'per_rest' in r.gaps)} recharge on a rest; "
        f"{len(breath)} actions named `... Breath` carry no Recharge marker in the seed.",
        f"- Uses per day: {sum(1 for r in rows if 'per_day' in r.gaps)} entries, plus the {sum(1 for r in rows if r.pattern == 'legendary-resistance')} Legendary Resistance traits whose count sits in the trait name.",
        f"- Legendary actions: {len(leg)} on {len({r.entry.slug for r in leg})} monsters; {sum(1 for r in leg if 'cooldown' in r.gaps)} say they cannot be taken again until the creature's next turn. "
        f"Patterns: {', '.join(f'{n} `{p}`' for p, n in collections.Counter(r.pattern for r in leg).most_common())}.",
        f"- Lair: no lair action exists in the seed; {len(lair)} monsters have a Legendary Resistance count that is higher in their lair.",
        f"- Spellcasting: {len(spell_list)} `Spellcasting` actions hold a list of spells; {len(cast)} entries in all need `cast_spell`, "
        f"and for {sum(1 for r in cast if set(r.missing) == {'cast_spell'})} of them it is the only missing kind.",
        f"- Spells those entries cast: {len(spells)} distinct SRD 5.2 spells. {len(automated)} of them own a seeded Effect today "
        f"({', '.join(proper.get(s, s) for s in automated)}). Most cast: {', '.join(f'{proper.get(s, s)} ({n})' for s, n in spells.most_common(8))}.",
    ]
    return lines


def today(rows):
    """The groups no missing kind stands in the way of."""
    by = collections.defaultdict(list)
    for r in rows:
        if not r.missing and not r.exception:
            by[(r.pattern, r.group, r.entry.kind)].append(r)
    out = ["| Group | Kind | Entries | Kinds used | Other gaps still open |", "|---|---|---|---|---|"]
    for (_, group, kind), rs in sorted(by.items(), key=lambda kv: -len(kv[1])):
        gaps = collections.Counter(g for r in rs for g in r.gaps)
        gap_text = ", ".join(g if n == len(rs) else f"{g} ({n})" for g, n in sorted(gaps.items())) or "none"
        used = " ".join(f"`{k}`" for k in sorted({k for r in rs for k in r.needs}, key=token_order)) or "none"
        out.append(f"| {group} | {kind} | {len(rs)} | {used} | {gap_text} |")
    return out


def exceptions(rows):
    by = collections.defaultdict(list)
    for r in rows:
        if r.exception:
            by[(r.entry.base, r.entry.kind, r.exception)].append(r.entry.monster)
    out = ["| Entry | Kind | Monsters | Reason |", "|---|---|---|---|"]
    for (name, kind, reason), who in sorted(by.items()):
        out.append(f"| {name} | {kind} | {', '.join(sorted(who))} | {reason} |")
    return out


def replace_block(text, name, lines):
    start, end = f"<!-- generated:{name} -->", f"<!-- /generated:{name} -->"
    if start not in text or end not in text:
        raise SystemExit("census-monsters.md has no block " + name)
    head, rest = text.split(start, 1)
    _, tail = rest.split(end, 1)
    return head + start + "\n" + "\n".join(lines) + "\n" + end + tail


def main():
    migration, existing = existing_kinds()
    data = seed()
    ms = monsters(data)
    es = entries(ms)
    rows, lost = classify(es, existing)
    if lost:
        for e, why in lost:
            print(f"UNACCOUNTED {e.monster} / {e.name} [{e.kind}] ({why}): {e.text[:200]}", file=sys.stderr)
        raise SystemExit(f"{len(lost)} entries are not accounted for")
    unused = [r for rows_, name in ((HAND_ACTIONS, "action"), (HAND_TRAITS, "trait")) for i, r in enumerate(rows_) if (id(rows_), i) not in USED]
    unused += [k for k in HAND_ATTACKS if ("attack",) + k not in USED]
    if unused:
        raise SystemExit("hand rows that match nothing: " + "; ".join(str(u[:3]) for u in unused))
    assert len(rows) == len(es)
    write_csv(rows)
    table_lines, n_groups = group_table(rows)
    missing_lines, _ = missing_table(rows)
    blocks = {
        "numbers": numbers(rows, ms, existing, migration) + [f"- Rows in the table below: {n_groups}."],
        "patterns": pattern_table(rows),
        "missing": missing_lines,
        "sets": missing_sets(rows),
        "subtargets": sub_targets(rows),
        "gaps": gap_table(rows),
        "economy": economy(rows, ms, spell_names(data), seeded_spell_effects()),
        "today": today(rows),
        "exceptions": exceptions(rows),
        "table": table_lines,
    }
    md = HERE / "census-monsters.md"
    if md.exists():
        text = md.read_text()
        for name, lines in blocks.items():
            text = replace_block(text, name, lines)
        md.write_text(text)
    if "--print" in sys.argv:
        for name, lines in blocks.items():
            print(f"== {name}")
            print("\n".join(lines))


if __name__ == "__main__":
    main()
