# Census: the Effect components SRD 5.2 monster actions and traits need

Research for [#239](https://github.com/JorisJonkers-dev/grimoire/issues/239), part of the map in
[#236](https://github.com/JorisJonkers-dev/grimoire/issues/236). Read against `main` at `92a4a7a`
(release 1.3.0), 2026-10-12.

**Question.** For every action, bonus action, reaction, legendary action and trait of the SRD 5.2
monsters in the Compendium seed: which Effect component kinds does Automation Level **full** need,
which does the engine have, and which are missing?

**Short answer.** 276 of the 989 actions are plain attacks that need no Effect at all, and 182 of
those resolve correctly today. Another 110 actions and 113 traits need no new kind. The other 603
actions and 203 traits miss at least one kind. Three kinds on their own unlock 254 actions
(`cast_spell`, `save_gate`, `sequence`); one cluster of four (`attack_gate`, `apply_condition`,
`damage` and a wider `branch`) unlocks the attack riders; three small extensions (`move_rule`,
`edge`, `save_edge`) unlock most traits. 14 entries are proposed for the Manual exception list.

Files:

- [`census-monsters.csv`](census-monsters.csv): one line per action or trait, 1,315 in all, each
  mapped to its group.
- [`census-monsters.py`](census-monsters.py): the script that reads the seed, puts every entry in a
  group, and writes the CSV and every generated block below. An entry nothing accounts for stops the
  run, so nothing is sampled.

`docs/research/` is new; the repository had no place for research notes yet.

## 1. Sources and method

| Source | What it settles |
|---|---|
| `api/db/seeds/compendium.json.gz`, key `monsters`, `document == "srd-2024"` | The 331 monsters: 989 actions (`name`, `description`, `type`, `attacks`) and 326 traits (`name`, `description`). |
| `api/db/migrations/00077_visibility.sql`, `effect_components_kind_check` | The 29 component kinds a seeded Effect can carry today. |
| `api/internal/rules/effects/effects.go`, `shape.go`, `light.go` | What each kind holds and how it folds. `AreaSave` and `Light` exist in the Go union only. |
| `api/internal/compendium/pgstore/effects.go` | `AreaSave` and `Light` are never written to the catalogue's tables: "The Effect builder's parts live in a spell's design". |
| `api/internal/play/pgstore/statblocks.go` (`Monster`) | What a seed monster brings into play: AC, hit points, saves, senses, attacks. No attacks-per-action, no spell save DC, no legendary data. |
| `api/internal/play/live/area.go`, `magic.go`, `effects.go`, `legend.go` | How an area Effect, a teleport, an applied Effect and a legendary action resolve. |
| `api/internal/rules/monsterbuild/monsterbuild.go`, `api/internal/play/pgstore/monsters.go` | What a Homebrew creature can declare that a seed monster cannot: Multiattack count, Recharge, legendary and lair actions. |
| `api/internal/rules/tactics/tactics.go` | Suggested Action chooses among attacks by reach and range only. |
| `api/internal/compendium/open5e/entries.go` | The importer reads `name`, `desc`, `action_type` and `attacks` of an action and nothing else. |
| `ARCHITECTURE.md` §9.3, §9.4 | The component union the engine is heading for, and the Manual fallback. |

Method. The script groups entries by normalised name and by the shape of the description (numbers,
dice, sizes and the creature's own name taken out). Attack rolls are parsed: the attack line and base
damage are removed and every remaining clause must be consumed by a rider rule, or the attack must have
a hand-written row. Saves that only deal damage are recognised by shape. Every other action and every
trait has a hand-written row, keyed by name and, where one name hides different rules, by monster.
All 989 actions and 326 traits were read; the hand rows are in the script.

Three words used below:

- **Kind**: an Effect component kind. A name ending in `*` is a kind the engine has that lacks a
  parameter the entry needs. `modifier:ac` and `trigger:turn_start` name what a `modifier` changes or
  when a `trigger` fires; they count as one kind each in the rankings.
- **Other gap**: something missing that is not a component kind: Targeting, action economy, seed data.
  Defined in §5.
- **Automatable with today's kinds**: no kind is missing. An other gap may still be open.

## 2. Summary

<!-- generated:numbers -->
- Monsters: 331. Actions: 989 (807 action, 76 bonus, 24 reaction, 82 legendary). Traits: 326.
- Component kinds in `00077_visibility.sql`: 29.
- Plain attacks: 276 actions. Structured data agrees with the text for 182, disagrees for 75, and is absent for 19.
- Of the 75 that disagree, 66 lose a second damage type the text gives (`plus N (XdY) Type damage`).
- Attack-roll actions in all: 423, of which 147 carry a rider.
- Automatable with today's kinds only: 386 actions and 113 traits (499 entries). With no other gap either: 184 actions and 80 traits.
- Of the 77 entries whose only missing kind is `save_gate`, 76 are area saves the engine already resolves but for the DC.
- Entries missing at least one kind: 603 actions and 203 traits.
- Proposed Manual exceptions: 2 actions and 12 traits.
- Monsters whose every action and trait needs no new kind: 48 of 331; with no other gap either: 18.
- Rows in the table below: 235.
<!-- /generated:numbers -->

### 2.1 Plain attacks

A plain attack is an attack roll with one or two damage types and nothing else. It needs no Effect:
the engine resolves it from the structured `attacks` row (`MonsterAttackRows`, then `attack.Outcome`).
276 actions are plain attacks. The engine resolves 182 of them as the text reads. It resolves 75 with
damage that differs from the text, in 66 cases because the second damage type is missing from the
structured row. It cannot resolve 19 at all, because the seed has the attack as text only. The other
147 attack rolls carry a rider and are counted under their rider's pattern.

### 2.2 Count per pattern

A pattern is shared by entries that resolve the same way. One pattern can span several rows of the
table when its members need slightly different kinds.

<!-- generated:patterns -->
| Pattern | Actions | Traits | Automatable with today's kinds | Proposed exceptions |
|---|---|---|---|---|
| `plain-attack` | 276 | 0 | 276 | 0 |
| `multiattack-count` | 91 | 0 | 91 | 0 |
| `no-table-rule` | 0 | 80 | 80 | 0 |
| `attack-rider-condition` | 69 | 0 | 0 | 0 |
| `conditional-edge` | 0 | 67 | 0 | 0 |
| `move-rule` | 0 | 64 | 0 | 0 |
| `multiattack-mixed` | 59 | 0 | 0 | 0 |
| `cast-named-spell` | 58 | 0 | 0 | 0 |
| `save-area-damage` | 54 | 0 | 0 | 0 |
| `save-target-rider` | 49 | 0 | 0 | 0 |
| `spellcasting-list` | 48 | 0 | 0 | 0 |
| `save-area-rider` | 39 | 0 | 0 | 0 |
| `use-other-action` | 33 | 0 | 0 | 0 |
| `legendary-resistance` | 0 | 32 | 32 | 0 |
| `multiattack-attacks` | 28 | 0 | 0 | 0 |
| `attack-rider-state` | 20 | 0 | 0 | 0 |
| `passive-modifier` | 0 | 19 | 1 | 0 |
| `attack-rider-charge` | 18 | 0 | 0 | 0 |
| `attack-rider-debuff` | 16 | 0 | 0 | 0 |
| `dm-judgment` | 2 | 12 | 0 | 14 |
| `shape-shift` | 13 | 0 | 0 | 0 |
| `aura` | 0 | 12 | 0 | 0 |
| `standard-action` | 12 | 0 | 12 | 0 |
| `attack-rider-save` | 12 | 0 | 0 | 0 |
| `self-move` | 12 | 0 | 0 | 0 |
| `special` | 0 | 12 | 0 | 0 |
| `save-area-staged` | 11 | 0 | 0 | 0 |
| `reaction-defence` | 11 | 0 | 0 | 0 |
| `contain` | 10 | 1 | 0 | 0 |
| `teleport` | 8 | 0 | 3 | 0 |
| `self-heal` | 0 | 8 | 0 | 0 |
| `save-target-damage` | 7 | 0 | 0 | 0 |
| `item-wear` | 5 | 2 | 0 | 0 |
| `light` | 1 | 5 | 0 | 0 |
| `attack-rider-move` | 6 | 0 | 0 | 0 |
| `on-death` | 0 | 6 | 0 | 0 |
| `reaction-other` | 6 | 0 | 0 | 0 |
| `zone` | 4 | 1 | 1 | 0 |
| `attack-rider-contain` | 4 | 0 | 0 | 0 |
| `on-damage` | 0 | 3 | 0 | 0 |
| `self-buff` | 3 | 0 | 1 | 0 |
| `pull` | 2 | 0 | 2 | 0 |
| `sense` | 0 | 2 | 0 | 0 |
| `summon` | 2 | 0 | 0 | 0 |
<!-- /generated:patterns -->

### 2.3 Missing kinds, ranked

"Needing it" counts every entry that lists the kind as missing. "Alone unlocks" counts entries for
which it is the only missing kind. Proposed exceptions are left out of the unlock counts.

<!-- generated:missing -->
| Missing kind | Actions needing it | Traits needing it | Actions it alone unlocks | Traits it alone unlocks |
|---|---|---|---|---|
| `save_gate` | 186 | 14 | 76 | 0 |
| `apply_condition` | 158 | 5 | 1 | 1 |
| `attack_gate` | 141 | 0 | 2 | 0 |
| `cast_spell` | 126 | 1 | 103 | 1 |
| `sequence` | 124 | 0 | 75 | 0 |
| `trigger` | 61 | 48 | 0 | 1 |
| `damage` | 87 | 16 | 1 | 0 |
| `branch*` | 93 | 3 | 0 | 0 |
| `modifier` | 63 | 31 | 1 | 15 |
| `move_rule` | 1 | 64 | 0 | 47 |
| `edge*` | 20 | 37 | 6 | 28 |
| `movement` | 45 | 0 | 11 | 0 |
| `save_edge*` | 2 | 38 | 0 | 34 |
| `contain` | 15 | 1 | 0 | 0 |
| `form*` | 13 | 1 | 11 | 0 |
| `heal` | 4 | 10 | 0 | 0 |
| `summon*` | 7 | 1 | 2 | 0 |
| `control` | 7 | 0 | 0 | 0 |
| `sense` | 0 | 7 | 0 | 2 |
| `create_object` | 4 | 2 | 0 | 2 |
| `item_wear` | 4 | 2 | 1 | 0 |
| `light` | 1 | 5 | 1 | 5 |
| `speed_penalty*` | 6 | 0 | 0 | 0 |
| `conceal` | 4 | 1 | 2 | 1 |
| `area*` | 4 | 0 | 0 | 0 |
| `compel` | 1 | 3 | 0 | 0 |
| `teleport*` | 4 | 0 | 2 | 0 |
| `bonus_die*` | 2 | 0 | 0 | 0 |
| `create_surface*` | 0 | 1 | 0 | 1 |
| `resource_change*` | 0 | 1 | 0 | 0 |
<!-- /generated:missing -->

Kinds are seldom missing alone. The sets below are what has to exist together before a group of
entries reaches full.

<!-- generated:sets -->
| Missing together | Actions unlocked | Traits unlocked | Mostly |
|---|---|---|---|
| `cast_spell` | 103 | 1 | `cast-named-spell`, `spellcasting-list` |
| `save_gate` | 76 | 0 | `save-area-damage`, `save-area-rider` |
| `sequence` | 75 | 0 | `multiattack-mixed`, `multiattack-attacks` |
| `move_rule` | 0 | 47 | `move-rule` |
| `attack_gate` `branch*` `apply_condition` | 45 | 0 | `attack-rider-condition`, `attack-rider-charge` |
| `edge*` | 6 | 28 | `conditional-edge`, `attack-rider-state` |
| `save_edge*` | 0 | 34 | `conditional-edge` |
| `sequence` `movement` | 22 | 0 | `use-other-action`, `self-move` |
| `sequence` `cast_spell` | 20 | 0 | `multiattack-mixed` |
| `attack_gate` `apply_condition` | 19 | 0 | `attack-rider-condition` |
| `modifier` | 1 | 15 | `passive-modifier`, `special` |
| `attack_gate` `branch*` `damage` | 13 | 0 | `attack-rider-state`, `attack-rider-charge` |
| `attack_gate` `branch*` `damage` `apply_condition` | 12 | 0 | `attack-rider-charge`, `attack-rider-condition` |
| `save_gate` `damage` `apply_condition` | 12 | 0 | `save-target-rider` |
| `trigger` `save_gate` | 4 | 8 | `on-death`, `save-area-staged` |
| `form*` | 11 | 0 | `shape-shift` |
| `movement` | 11 | 0 | `self-move` |
| `trigger` `modifier` | 9 | 1 | `reaction-defence`, `reaction-other` |
| `save_gate` `apply_condition` | 9 | 0 | `save-target-rider` |
| `save_gate` `damage` | 9 | 0 | `save-target-damage`, `save-target-rider` |
| `trigger` `heal` | 0 | 8 | `self-heal` |
| `trigger` `save_gate` `damage` `apply_condition` `contain` | 7 | 0 | `contain`, `zone` |
| `trigger` `damage` | 0 | 7 | `aura`, `on-damage` |
| `modifier` `move_rule` | 0 | 7 | `move-rule` |
| 98 other sets | 137 | 45 | |
<!-- /generated:sets -->

What a `modifier` changes and when a `trigger` fires, by number of entries:

<!-- generated:subtargets -->
- `modifier`: `immunity` 24, `no_healing` 13, `ac` 13, `hp_max` 12, `action_economy` 8, `damage_dealt` 8, `damage_share` 3, `rest` 3, `retarget` 2, `reaction_count` 2, `resistance` 2, `save_outcome` 1, `skill` 1, `ability` 1, `d20` 1, `damage_taken` 1, `no_teleport` 1, `concentration` 1
- `trigger`: `turn_start` 25, `damaged` 20, `turn_end` 16, `hit_by_attack` 15, `drops_to_0` 14, `death` 6, `enters` 6, `starts_turn_near` 6, `clock` 4, `moves_near` 2, `attacked` 1, `ends_turn_near` 1, `rest` 1, `d20_test` 1, `targeted_by_spell` 1
<!-- /generated:subtargets -->

Reading the two tables together:

1. **`cast_spell`** is the largest single unlock, but it adds no behaviour of its own. An entry that
   casts Fireball is as automated as Fireball is. It depends on the spells census.
2. **`save_gate`** unlocks 76 area saves the engine already resolves except for the DC, breath weapons
   first among them, and stands in front of 124 more entries.
3. **`sequence`** unlocks the Multiattacks that are a fixed mix of attacks or swap an attack for
   another action, and every legendary action of the form "makes one Rend attack".
4. **`attack_gate`, `apply_condition`, `damage` and `branch*`** only pay off together: 94 attack
   riders need two to four of them and nothing else.
5. **`move_rule`, `save_edge*` and `edge*`** are small and unlock 112 traits between them, Magic
   Resistance and Pack Tactics included.
6. **`trigger` and `modifier`** are needed widely (109 and 94 entries) but unlock little alone: they
   are the tail of almost every one-off.

### 2.4 Recharge, legendary and lair economy, spellcasting

<!-- generated:economy -->
- Recharge: 2 actions carry `Recharge N-6` in their name and 2 recharge on a rest; 72 actions named `... Breath` carry no Recharge marker in the seed.
- Uses per day: 51 entries, plus the 32 Legendary Resistance traits whose count sits in the trait name.
- Legendary actions: 82 on 30 monsters; 41 say they cannot be taken again until the creature's next turn. Patterns: 31 `use-other-action`, 27 `cast-named-spell`, 11 `save-target-rider`, 9 `save-area-rider`, 2 `save-area-damage`, 1 `teleport`, 1 `self-buff`.
- Lair: no lair action exists in the seed; 27 monsters have a Legendary Resistance count that is higher in their lair.
- Spellcasting: 48 `Spellcasting` actions hold a list of spells; 127 entries in all need `cast_spell`, and for 104 of them it is the only missing kind.
- Spells those entries cast: 113 distinct SRD 5.2 spells. 13 of them own a seeded Effect today (Animate Dead, Bless, Cone of Cold, Counterspell, Dispel Magic, Faerie Fire, Fireball, Lightning Bolt, Misty Step, Shatter, Spirit Guardians, Thunderwave, Wall of Fire). Most cast: Detect Magic (33), Invisibility (16), Scorching Ray (12), Shapechange (12), Guiding Bolt (12), Mind Spike (12), Detect Thoughts (10), Minor Illusion (9).
<!-- /generated:economy -->

**Recharge.** The seed keeps a Recharge marker only where it sits in the action's name, and that is
two actions (Basilisk's Petrifying Gaze, Blink Dog's Teleport), plus two that come back on a rest
(Cloaker's Phantasms, Sphinx of Valor's Roar). None of the 72 breath actions, on 58 monsters, has one
in name or text. A Homebrew creature's action can
declare `Recharge` 4 to 6 (`monsterbuild.Action`), but live play has no recharge roll: the only
recharging it knows is magic item charges after a rest. Recharge is therefore both a seed gap and a
runtime gap, and it is not a component kind.

**Legendary.** Live play already keeps the economy: uses per round, a cost per action, a window that
opens right after another creature's turn, and a Legendary Resistance counter (`live/legend.go`). Two
things keep SRD monsters out of it. Only a token built from a Homebrew Design gets that data; the stat
block read for a seed monster sets none. And taking a legendary action writes a Manual prompt with the
action's text rather than resolving anything. The seed holds no number of legendary actions per round.
Half of the 82 legendary actions also carry their own limit ("can't take this action again until the
start of its next turn"), which the runtime does not know.

**Lair.** The SRD 5.2 monsters in the seed have no lair actions. Lair shows up only as a second count
in a trait name, "Legendary Resistance (3/Day, or 4/Day in Lair)", on 27 monsters. Automating it takes
a mark on the Combatant that says it is in its lair, nothing else. The runtime's lair action on
initiative count 20 is used by Homebrew creatures only.

**Spellcasting.** SRD 5.2 has no spellcasting trait; it has a `Spellcasting` action that lists spells
cast at will or a set number of times a day, on 48 monsters. Another 79 entries cast one or a few
named spells as an action, Bonus Action, reaction or legendary action, or swap an attack for a spell
inside Multiattack. All of them only point at spells. They need `cast_spell`, a per-day counter that
works for a monster (`resource_change` today only reaches a Character), and the spell's own Effect:
of the 113 spells they name, 13 own a seeded Effect today.

**Suggested Action.** `tactics.Suggest` picks among a creature's attacks by reach and range. An action
automated through an Effect is invisible to it, so 178 monsters with Multiattack and 72 breath actions
would resolve when the DM picks them and still never be proposed. That choice is already listed as
unspecified on the map; this census only gives its size.

### 2.5 Automatable with today's kinds only

499 entries miss no kind: 386 actions and 113 traits. Only 264 of them have no other gap either.

<!-- generated:today -->
| Group | Kind | Entries | Kinds used | Other gaps still open |
|---|---|---|---|---|
| Plain attack, structured data agrees with the text | action | 182 | none | none |
| Multiattack: a number of attacks of one kind, or of its attacks in any combination | action | 91 | none | attack_count |
| Plain attack, structured data disagrees with the text | action | 75 | none | seed_data |
| Breathing: air, water, holding breath | trait | 47 | none | none |
| Legendary Resistance | trait | 32 | none | legendary |
| Returns to life elsewhere, or days later | trait | 25 | none | none |
| Plain attack, carried as text only | action | 19 | none | seed_data |
| Takes Dash, Disengage or Hide | bonus | 12 | none | activation_cost, environment (1) |
| Speech, telepathy and bonds | trait | 4 | none | none |
| Other traits with nothing to compute at the table | trait | 4 | none | none |
| Teleports itself | bonus | 2 | `teleport` | recharge (1) |
| Area the creature conjures or carries that lasts | action | 1 | `area` `create_surface` | none |
| Pulls creatures it already holds toward itself | bonus | 1 | `forced_move` | activation_cost, targeting |
| Bolsters itself or an ally | bonus | 1 | `edge` `temp_hp` | activation_cost, targeting |
| Damage type chosen when the creature is placed | trait | 1 | `choice` | trait_owner |
| Teleports itself | action | 1 | `teleport` | activation_cost |
| Pulls creatures it already holds toward itself | action | 1 | `forced_move` | targeting |
<!-- /generated:today -->

Two things in that table are judgments rather than counts:

- The 80 `no-table-rule` traits have nothing the engine tracks: what a creature breathes, where it
  returns to life after it dies, whom it can speak to. They can reach full with an Effect that has no
  components. They are not proposed as exceptions, because no DM ruling is needed during play. The
  maintainer may prefer to list them.
- The 91 `multiattack-count` actions are "makes N attacks" of one kind or in any combination, N at
  most 4. The engine has attacks-per-action for exactly that, capped at 4, and a Homebrew creature
  sets it. A seed monster's stat block does not, so each one attacks once per action today.

### 2.6 Proposed Manual exceptions

Proposed only where the outcome is the DM's judgment by nature. An entry that is merely hard is listed
as a gap instead: possession, lycanthropy, swallowing, a hydra's heads and a vampire's mist form are
all gaps.

<!-- generated:exceptions -->
| Entry | Kind | Monsters | Reason |
|---|---|---|---|
| Divine Awareness | trait | Planetar, Solar | Whether a statement is a lie is known only to the DM. |
| Heart Sight | action | Sprite | What the sprite learns, the target's emotions and alignment, is the DM's to tell; only the save can be computed. |
| Iron Scent | trait | Rust Monster | Where ferrous metal lies is world detail the engine does not hold; the DM tells. |
| Mimicry | trait | Green Hag, Raven | What a listener believes is roleplay; only the Insight check is a number. |
| Probing Telepathy | trait | Aboleth | A creature's greatest desires exist only in the DM's fiction. |
| Sense Magic | trait | Chuul | Which objects and creatures are magical is world detail the engine does not hold; the DM tells. |
| Shriek | reaction | Shrieker Fungus | The shriek does nothing but warn whoever the DM decides can hear it within 300 feet. |
| Treasure Sense | trait | Xorn | Where precious metals and stones lie is world detail the engine does not hold; the DM tells. |
| Vampire Weakness | trait | Vampire, Vampire Spawn | Forbiddance and the stake through the heart hang on the fiction (what counts as a residence, an invitation, a resting place); the sunlight and running-water parts should still be computed. |
| Wishes | trait | Djinni, Efreeti | Whether the genie knows Wish, and what a wish does, is the DM's ruling. |
<!-- /generated:exceptions -->

Vampire Weakness is a partial exception: two of its four bullets are fiction-driven, and the entry
would stay partial rather than full.

## 3. The table

One row per group and kind of entry. A group that spans different sets of kinds has one row per set.
"Other gaps" with a number in brackets apply to that many of the row's entries.

<!-- generated:table -->
| Group (members) | Kind | Entries | Monsters | Kinds needed | Kinds missing today | Other gaps | Pattern | Manual exception |
|---|---|---|---|---|---|---|---|---|
| **Attack with a charge rider (moved 10+ or 20+ feet straight toward the target)** (Gore, Hooves) | action | 4 | 4 | `attack_gate` `branch*` `apply_condition` | `attack_gate` `branch*` `apply_condition` | none | `attack-rider-charge` | no |
| **Attack with a charge rider (moved 10+ or 20+ feet straight toward the target)** (Ram) | action | 2 | 2 | `attack_gate` `branch*` `damage` | `attack_gate` `branch*` `damage` | seed_data (1) | `attack-rider-charge` | no |
| **Attack with a charge rider (moved 10+ or 20+ feet straight toward the target)** (Gore, Ram, Hooves, Tusk) | action | 11 | 11 | `attack_gate` `branch*` `damage` `apply_condition` | `attack_gate` `branch*` `damage` `apply_condition` | seed_data (1) | `attack-rider-charge` | no |
| **Attack with a charge rider (moved 10+ or 20+ feet straight toward the target)** (Claws) | action | 1 | 1 | `sequence` `attack_gate` `branch*` `apply_condition` | `sequence` `attack_gate` `branch*` `apply_condition` | none | `attack-rider-charge` | no |
| **Attack that grapples (escape DC), with or without Restrained** (Tentacle) | action | 1 | 1 | `attack_gate` `apply_condition` | `attack_gate` `apply_condition` | none | `attack-rider-condition` | no |
| **Attack that grapples (escape DC), with or without Restrained** (Tentacle) | action | 1 | 1 | `attack_gate` `apply_condition` `create_object` | `attack_gate` `apply_condition` `create_object` | seed_data | `attack-rider-condition` | no |
| **Attack that grapples (escape DC), with or without Restrained** (Bite) | action | 1 | 1 | `attack_gate` `apply_condition` `modifier:no_teleport` | `attack_gate` `apply_condition` `modifier:no_teleport` | none | `attack-rider-condition` | no |
| **Attack that grapples (escape DC), with or without Restrained** (Bite, Claw, Tentacle, Pincer, Tentacles, Claws, and 5 more) | action | 22 | 22 | `attack_gate` `branch*` `apply_condition` | `attack_gate` `branch*` `apply_condition` | seed_data (3) | `attack-rider-condition` | no |
| **Attack that grapples (escape DC), with or without Restrained** (Bite, Pseudopod) | action | 2 | 2 | `attack_gate` `branch*` `apply_condition` `edge*` | `attack_gate` `branch*` `apply_condition` `edge*` | seed_data, targeting (1) | `attack-rider-condition` | no |
| **Attack that knocks a size-limited target Prone** (Tail, Bite, Claw, Ram, Storm Bolt, Rock Launch, and 5 more) | action | 18 | 18 | `attack_gate` `branch*` `apply_condition` | `attack_gate` `branch*` `apply_condition` | seed_data (1) | `attack-rider-condition` | no |
| **Attack that knocks a size-limited target Prone** (Swarm of Grasping Hands) | action | 1 | 1 | `attack_gate` `branch*` `damage` `apply_condition` | `attack_gate` `branch*` `damage` `apply_condition` | none | `attack-rider-condition` | no |
| **Attack that knocks a size-limited target Prone** (Bite) | action | 1 | 1 | `trigger:drops_to_0` `attack_gate` `branch*` `apply_condition` | `trigger:drops_to_0` `attack_gate` `branch*` `apply_condition` | none | `attack-rider-condition` | no |
| **Attack that leaves a target dropped to 0 Hit Points Stable and Paralyzed** (Bite, Umbral Dagger) | action | 2 | 2 | `trigger:drops_to_0` `attack_gate` `apply_condition` | `trigger:drops_to_0` `attack_gate` `apply_condition` | seed_data | `attack-rider-condition` | no |
| **Attack that puts a condition on the target until a turn boundary** (Bite, Shortsword, Thundercloud, Gouge, Trash Lob, Paralyzing Touch, and 5 more) | action | 14 | 14 | `attack_gate` `apply_condition` | `attack_gate` `apply_condition` | seed_data (4) | `attack-rider-condition` | no |
| **Attack that puts a condition on the target until a turn boundary** (Beard, Infernal Sting) | action | 2 | 2 | `attack_gate` `apply_condition` `modifier:no_healing` | `attack_gate` `apply_condition` `modifier:no_healing` | seed_data (1) | `attack-rider-condition` | no |
| **Attack that sets the target burning** (Hurl Flame, Burn, Touch) | action | 4 | 4 | `attack_gate` `apply_condition` | `attack_gate` `apply_condition` | none | `attack-rider-condition` | no |
| **Attack that attaches to or smothers the target** (Crush) | action | 1 | 1 | `attack_gate` `branch*` `apply_condition` `contain` `edge*` `immobile` | `attack_gate` `branch*` `apply_condition` `contain` `edge*` | none | `attack-rider-contain` | no |
| **Attack that attaches to or smothers the target** (Attach) | action | 1 | 1 | `attack_gate` `branch*` `apply_condition` `contain` `modifier:damage_share` | `attack_gate` `branch*` `apply_condition` `contain` `modifier:damage_share` | none | `attack-rider-contain` | no |
| **Attack that attaches to or smothers the target** (Smother) | action | 1 | 1 | `trigger:turn_start` `attack_gate` `branch*` `choice` `damage` `apply_condition` `contain` `modifier:damage_share` | `trigger:turn_start` `attack_gate` `branch*` `damage` `apply_condition` `contain` `modifier:damage_share` | none | `attack-rider-contain` | no |
| **Attack that attaches to or smothers the target** (Proboscis) | action | 1 | 1 | `trigger:turn_start` `attack_gate` `damage` `contain` | `trigger:turn_start` `attack_gate` `damage` `contain` | none | `attack-rider-contain` | no |
| **Attack that leaves a lasting poison or curse** (Restless Touch, Cursed Touch) | action | 2 | 2 | `attack_gate` `apply_condition` `modifier:rest` | `attack_gate` `apply_condition` `modifier:rest` | seed_data (1) | `attack-rider-debuff` | no |
| **Attack that leaves a lasting poison or curse** (Rotting Fist) | action | 1 | 1 | `trigger:clock` `trigger:drops_to_0` `attack_gate` `apply_condition` `modifier:hp_max` `modifier:no_healing` | `trigger:clock` `trigger:drops_to_0` `attack_gate` `apply_condition` `modifier:hp_max` `modifier:no_healing` | seed_data | `attack-rider-debuff` | no |
| **Attack that leaves a lasting poison or curse** (Rotting Fist) | action | 1 | 1 | `trigger:clock` `trigger:drops_to_0` `attack_gate` `apply_condition` `modifier:hp_max` `modifier:no_healing` `modifier:rest` | `trigger:clock` `trigger:drops_to_0` `attack_gate` `apply_condition` `modifier:hp_max` `modifier:no_healing` `modifier:rest` | seed_data | `attack-rider-debuff` | no |
| **Attack that leaves a lasting poison or curse** (Bite) | action | 1 | 1 | `trigger:rest` `attack_gate` `save_gate` `apply_condition` `modifier:hp_max` | `trigger:rest` `attack_gate` `save_gate` `apply_condition` `modifier:hp_max` | none | `attack-rider-debuff` | no |
| **Attack that lowers Hit Point maximum or an ability score** (Life Drain, Slam) | action | 3 | 3 | `attack_gate` `modifier:hp_max` | `attack_gate` `modifier:hp_max` | seed_data (1) | `attack-rider-debuff` | no |
| **Attack that lowers Hit Point maximum or an ability score** (Draining Swipe) | action | 1 | 1 | `trigger:drops_to_0` `attack_gate` `modifier:ability` `summon*` | `trigger:drops_to_0` `attack_gate` `modifier:ability` `summon*` | none | `attack-rider-debuff` | no |
| **Attack that slows, or weakens the target's next roll or turn** (Morningstar, Rapier, Bite) | action | 3 | 3 | `attack_gate` `edge*` | `attack_gate` `edge*` | none | `attack-rider-debuff` | no |
| **Attack that slows, or weakens the target's next roll or turn** (Lightning Blade) | action | 1 | 1 | `attack_gate` `modifier:action_economy` | `attack_gate` `modifier:action_economy` | seed_data | `attack-rider-debuff` | no |
| **Attack that slows, or weakens the target's next roll or turn** (Ice Spear) | action | 1 | 1 | `attack_gate` `modifier:action_economy` `speed_penalty` | `attack_gate` `modifier:action_economy` | seed_data | `attack-rider-debuff` | no |
| **Attack that slows, or weakens the target's next roll or turn** (Great Bow, Ocean Spear) | action | 2 | 2 | `attack_gate` `speed_penalty` | `attack_gate` | seed_data | `attack-rider-debuff` | no |
| **Attack that pushes or pulls the target** (Flame Whip) | action | 1 | 1 | `attack_gate` `branch*` `apply_condition` `forced_move` | `attack_gate` `branch*` `apply_condition` | seed_data | `attack-rider-move` | no |
| **Attack that pushes or pulls the target** (Harpoon, Hooves, Charged Tendril, Warhammer) | action | 4 | 4 | `attack_gate` `branch*` `forced_move` | `attack_gate` `branch*` | seed_data (1) | `attack-rider-move` | no |
| **Attack that pushes or pulls the target** (Hammer Throw) | action | 1 | 1 | `attack_gate` `forced_move` `edge*` | `attack_gate` `edge*` | seed_data | `attack-rider-move` | no |
| **Attack that leaves a lasting poison or curse** (Bite) | action | 1 | 1 | `trigger:clock` `attack_gate` `save_gate` `apply_condition` `modifier:hp_max` | `trigger:clock` `attack_gate` `save_gate` `apply_condition` `modifier:hp_max` | none | `attack-rider-save` | no |
| **Attack that leaves a lasting poison or curse** (Infernal Glaive) | action | 1 | 1 | `trigger:turn_start` `attack_gate` `save_gate` `damage` `apply_condition` | `trigger:turn_start` `attack_gate` `save_gate` `damage` `apply_condition` | none | `attack-rider-save` | no |
| **Attack that leaves a lasting poison or curse** (Bite) | action | 1 | 1 | `trigger:turn_start` `attack_gate` `save_gate` `damage` `apply_condition` `modifier:no_healing` | `trigger:turn_start` `attack_gate` `save_gate` `damage` `apply_condition` `modifier:no_healing` | none | `attack-rider-save` | no |
| **Attack that passes on lycanthropy** (Bite, Gore) | action | 5 | 5 | `trigger:drops_to_0` `attack_gate` `save_gate` `branch` `apply_condition` `control` `form` `modifier:immunity` | `trigger:drops_to_0` `attack_gate` `save_gate` `apply_condition` `control` `modifier:immunity` | targeting | `attack-rider-save` | no |
| **Attack whose hit forces a saving throw** (Petrifying Bite) | action | 1 | 1 | `attack_gate` `save_gate` `apply_condition` | `attack_gate` `save_gate` `apply_condition` | none | `attack-rider-save` | no |
| **Attack whose hit forces a saving throw** (Bite) | action | 1 | 1 | `attack_gate` `save_gate` `branch` `apply_condition` | `attack_gate` `save_gate` `apply_condition` | seed_data | `attack-rider-save` | no |
| **Attack whose hit forces a saving throw** (Claw) | action | 2 | 2 | `attack_gate` `save_gate` `branch*` `apply_condition` | `attack_gate` `save_gate` `branch*` `apply_condition` | targeting | `attack-rider-save` | no |
| **Attack whose damage or Advantage depends on a state** (Bites, Scimitar, Shortbow, Beak, Bite, Beaks) | action | 11 | 9 | `attack_gate` `branch*` `damage` | `attack_gate` `branch*` `damage` | none | `attack-rider-state` | no |
| **Attack whose damage or Advantage depends on a state** (Bite, Bites) | action | 2 | 2 | `attack_gate` `branch*` `damage` `edge*` | `attack_gate` `branch*` `damage` `edge*` | seed_data, targeting (1) | `attack-rider-state` | no |
| **Attack whose damage or Advantage depends on a state** (Claw) | action | 1 | 1 | `attack_gate` `choice` `damage` | `attack_gate` `damage` | none | `attack-rider-state` | no |
| **Attack whose damage or Advantage depends on a state** (Bite, Morningstar, Light Hammer, Slam) | action | 6 | 6 | `edge*` | `edge*` | seed_data, targeting (2) | `attack-rider-state` | no |
| **Aura that damages creatures around it each turn** (Fire Aura, Heat Aura) | trait | 5 | 5 | `trigger:turn_end` `area` `damage` | `trigger:turn_end` `damage` | targeting (2), trait_owner | `aura` | no |
| **Aura that damages creatures around it each turn** (Barbed Hide) | trait | 1 | 1 | `trigger:turn_start` `damage` | `trigger:turn_start` `damage` | targeting, trait_owner | `aura` | no |
| **Aura that forces a save on creatures that start their turn in it** (Mucus Cloud) | trait | 1 | 1 | `trigger:clock` `trigger:turn_end` `area` `save_gate` `damage` `apply_condition` `modifier:no_healing` | `trigger:clock` `trigger:turn_end` `save_gate` `damage` `apply_condition` `modifier:no_healing` | environment, trait_owner | `aura` | no |
| **Aura that forces a save on creatures that start their turn in it** (Gibbering) | trait | 1 | 1 | `trigger:starts_turn_near` `area` `save_gate` `compel` | `trigger:starts_turn_near` `save_gate` `compel` | trait_owner | `aura` | no |
| **Aura that forces a save on creatures that start their turn in it** (Stench, Fear Aura, Vile Appearance) | trait | 4 | 4 | `trigger:starts_turn_near` `area` `save_gate` `save_condition` `modifier:immunity` | `trigger:starts_turn_near` `save_gate` `modifier:immunity` | targeting (2), trait_owner | `aura` | no |
| **Casts one or a few named spells** (Invisibility, Read Thoughts, Sleep, Etherealness, Ice Wall, Fog Cloud, and 3 more) | action | 11 | 11 | `cast_spell` | `cast_spell` | per_day (1) | `cast-named-spell` | no |
| **Casts one or a few named spells** (Nightmare Haunting) | action | 1 | 1 | `cast_spell` `modifier:hp_max` | `cast_spell` `modifier:hp_max` | environment, per_day | `cast-named-spell` | no |
| **Casts one or a few named spells** (Divine Aid, Misty Step, Phantasms, Spiritual Weapon, Magic of the Spider Queen, Invisibility, and 3 more) | bonus | 16 | 16 | `cast_spell` | `cast_spell` | activation_cost, environment (1), per_rest (1), targeting (1) | `cast-named-spell` | no |
| **Casts one or a few named spells** (Protective Magic) | reaction | 3 | 3 | `cast_spell` | `cast_spell` | reaction | `cast-named-spell` | no |
| **Casts one or a few named spells** (Frightful Presence, Guiding Light, Sonic Boom, Blazing Light, Mind Jolt, Mind Invasion, and 6 more) | legendary | 25 | 23 | `cast_spell` | `cast_spell` | cooldown (17), legendary | `cast-named-spell` | no |
| **Casts one or a few named spells** (Cloaked Flight) | legendary | 2 | 2 | `cast_spell` `movement` | `cast_spell` `movement` | cooldown, legendary | `cast-named-spell` | no |
| **Advantage or Disadvantage that holds only in a state** (Sunlight Sensitivity, Bloodied Fury, Light Sensitivity, Blood Frenzy, Blurred Form) | trait | 10 | 10 | `edge*` | `edge*` | environment (6), trait_owner | `conditional-edge` | no |
| **Advantage or Disadvantage that holds only in a state** (Bloodied Frenzy, Sunlight Weakness) | trait | 2 | 2 | `edge*` `save_edge*` | `edge*` `save_edge*` | environment (1), trait_owner | `conditional-edge` | no |
| **Advantage or Disadvantage that holds only in a state** (Aversion to Fire) | trait | 1 | 1 | `trigger:damaged` `edge*` | `trigger:damaged` `edge*` | trait_owner | `conditional-edge` | no |
| **Aura that gives Advantage to the creature and its allies** (Aura of Authority) | trait | 1 | 1 | `area` `edge*` `save_edge*` | `edge*` `save_edge*` | targeting, trait_owner | `conditional-edge` | no |
| **Magic Resistance** (Greater Magic Resistance) | trait | 1 | 1 | `modifier:immunity` `save_edge*` | `modifier:immunity` `save_edge*` | trait_owner | `conditional-edge` | no |
| **Magic Resistance** (Magic Resistance) | trait | 34 | 34 | `save_edge*` | `save_edge*` | trait_owner | `conditional-edge` | no |
| **Pack Tactics** (Pack Tactics) | trait | 18 | 18 | `edge*` | `edge*` | trait_owner | `conditional-edge` | no |
| **Swallow, engulf or whelm: the target is held inside the creature** (Engulf) | action | 1 | 1 | `trigger:enters` `trigger:turn_start` `movement` `save_gate` `damage` `apply_condition` `contain` | `trigger:enters` `trigger:turn_start` `movement` `save_gate` `damage` `apply_condition` `contain` | none | `contain` | no |
| **Swallow, engulf or whelm: the target is held inside the creature** (Swallow) | action | 2 | 2 | `trigger:turn_end` `damage` `apply_condition` `contain` | `trigger:turn_end` `damage` `apply_condition` `contain` | targeting | `contain` | no |
| **Swallow, engulf or whelm: the target is held inside the creature** (Whelm) | action | 1 | 1 | `trigger:turn_start` `area` `save_gate` `branch*` `save_damage` `damage` `apply_condition` `contain` | `trigger:turn_start` `save_gate` `branch*` `damage` `apply_condition` `contain` | none | `contain` | no |
| **Swallow, engulf or whelm: the target is held inside the creature** (Swallow) | action | 1 | 1 | `trigger:turn_start` `save_gate` `damage` `apply_condition` `contain` | `trigger:turn_start` `save_gate` `damage` `apply_condition` `contain` | targeting | `contain` | no |
| **Swallow, engulf or whelm: the target is held inside the creature** (Engulf) | action | 1 | 1 | `trigger:turn_start` `save_gate` `damage` `apply_condition` `forced_move` `contain` | `trigger:turn_start` `save_gate` `damage` `apply_condition` `contain` | targeting | `contain` | no |
| **Swallow, engulf or whelm: the target is held inside the creature** (Swallow) | bonus | 4 | 4 | `trigger:turn_start` `save_gate` `damage` `apply_condition` `contain` | `trigger:turn_start` `save_gate` `damage` `apply_condition` `contain` | activation_cost, targeting | `contain` | no |
| **Swallow, engulf or whelm: the target is held inside the creature** (Ooze Cube) | trait | 1 | 1 | `trigger:enters` `contain` | `trigger:enters` `contain` | trait_owner | `contain` | no |
| **Grants wishes** (Wishes) | trait | 2 | 2 | `manual` | none | per_day, trait_owner | `dm-judgment` | yes: Whether the genie knows Wish, and what a wish does, is the DM's ruling. |
| **Learns something only the DM can tell** (Heart Sight) | action | 1 | 1 | `save_gate` `manual` | `save_gate` | none | `dm-judgment` | yes: What the sprite learns, the target's emotions and alignment, is the DM's to tell; only the save can be computed. |
| **Learns something only the DM can tell** (Probing Telepathy) | trait | 1 | 1 | `manual` | none | trait_owner | `dm-judgment` | yes: A creature's greatest desires exist only in the DM's fiction. |
| **Learns something only the DM can tell** (Divine Awareness) | trait | 2 | 2 | `manual` | none | trait_owner | `dm-judgment` | yes: Whether a statement is a lie is known only to the DM. |
| **Mimics sounds and voices** (Mimicry) | trait | 2 | 2 | `manual` | none | trait_owner | `dm-judgment` | yes: What a listener believes is roleplay; only the Insight check is a number. |
| **Senses a kind of thing in the world (magic, metal, gems)** (Sense Magic) | trait | 1 | 1 | `manual` | none | trait_owner | `dm-judgment` | yes: Which objects and creatures are magical is world detail the engine does not hold; the DM tells. |
| **Senses a kind of thing in the world (magic, metal, gems)** (Iron Scent) | trait | 1 | 1 | `manual` | none | trait_owner | `dm-judgment` | yes: Where ferrous metal lies is world detail the engine does not hold; the DM tells. |
| **Senses a kind of thing in the world (magic, metal, gems)** (Treasure Sense) | trait | 1 | 1 | `manual` | none | trait_owner | `dm-judgment` | yes: Where precious metals and stones lie is world detail the engine does not hold; the DM tells. |
| **Shrieks when something comes near** (Shriek) | reaction | 1 | 1 | `trigger:moves_near` `manual` | `trigger:moves_near` | reaction | `dm-judgment` | yes: The shriek does nothing but warn whoever the DM decides can hear it within 300 feet. |
| **Vampire weaknesses** (Vampire Weakness) | trait | 2 | 2 | `trigger:turn_end` `trigger:turn_start` `damage` `edge*` `manual` | `trigger:turn_end` `trigger:turn_start` `damage` `edge*` | environment, trait_owner | `dm-judgment` | yes: Forbiddance and the stake through the heart hang on the fiction (what counts as a residence, an invitation, a resting place); the sunlight and running-water parts should still be computed. |
| **Corrodes or destroys armor, weapons and metal** (Dissolving Pseudopod, Pseudopod) | action | 2 | 2 | `attack_gate` `item_wear` | `attack_gate` `item_wear` | none | `item-wear` | no |
| **Corrodes or destroys armor, weapons and metal** (Destroy Metal) | action | 1 | 1 | `item_wear` | `item_wear` | none | `item-wear` | no |
| **Corrodes or destroys armor, weapons and metal** (Antennae) | action | 1 | 1 | `save_gate` `item_wear` | `save_gate` `item_wear` | none | `item-wear` | no |
| **Corrodes or destroys armor, weapons and metal** (Reflexive Antennae) | reaction | 1 | 1 | `trigger:hit_by_attack` `sequence` | `trigger:hit_by_attack` `sequence` | reaction | `item-wear` | no |
| **Corrodes or destroys armor, weapons and metal** (Corrosive Form) | trait | 1 | 1 | `trigger:hit_by_attack` `damage` `item_wear` | `trigger:hit_by_attack` `damage` `item_wear` | trait_owner | `item-wear` | no |
| **Corrodes or destroys armor, weapons and metal** (Corrosive Form) | trait | 1 | 1 | `trigger:hit_by_attack` `item_wear` | `trigger:hit_by_attack` `item_wear` | trait_owner | `item-wear` | no |
| **Legendary Resistance** (Legendary Resistance) | trait | 32 | 32 | none | none | legendary | `legendary-resistance` | no |
| **Sheds light** (Ignited Illumination) | bonus | 1 | 1 | `light` | `light` | activation_cost | `light` | no |
| **Sheds light** (Illumination) | trait | 5 | 5 | `light` | `light` | trait_owner | `light` | no |
| **Drags a grappled creature at no extra movement** (Abduct) | trait | 2 | 2 | `move_rule` | `move_rule` | targeting, trait_owner | `move-rule` | no |
| **Fixed jump distances** (Standing Leap, Running Leap, Jumper) | trait | 6 | 6 | `move_rule` | `move_rule` | trait_owner | `move-rule` | no |
| **Ignores a kind of terrain, or climbs, burrows or passes through it** (Spider Climb, Ice Walk, Tunneler, Earth Glide) | trait | 22 | 22 | `move_rule` | `move_rule` | environment, trait_owner | `move-rule` | no |
| **Ignores a kind of terrain, or climbs, burrows or passes through it** (Web Walker) | trait | 5 | 5 | `move_rule` `sense` | `move_rule` `sense` | environment, trait_owner | `move-rule` | no |
| **Ignores a kind of terrain, or climbs, burrows or passes through it** (Incorporeal Movement) | trait | 4 | 4 | `trigger:turn_end` `damage` `move_rule` | `trigger:turn_end` `damage` `move_rule` | trait_owner | `move-rule` | no |
| **Leaves reach without provoking Opportunity Attacks** (Flyby, Agile) | trait | 9 | 9 | `move_rule` | `move_rule` | trait_owner | `move-rule` | no |
| **Squeezes through gaps, or shares other creatures' spaces** (Swarm) | trait | 7 | 7 | `modifier:no_healing` `move_rule` | `modifier:no_healing` `move_rule` | trait_owner | `move-rule` | no |
| **Squeezes through gaps, or shares other creatures' spaces** (Amorphous, Air Form, Compression, Water Form) | trait | 8 | 8 | `move_rule` | `move_rule` | trait_owner | `move-rule` | no |
| **Squeezes through gaps, or shares other creatures' spaces** (Fire Form) | trait | 1 | 1 | `trigger:enters` `damage` `move_rule` | `trigger:enters` `damage` `move_rule` | trait_owner | `move-rule` | no |
| **Multiattack: a fixed mix of different attacks** (Multiattack) | action | 28 | 28 | `sequence` | `sequence` | none | `multiattack-attacks` | no |
| **Multiattack: a number of attacks of one kind, or of its attacks in any combination** (Multiattack) | action | 91 | 91 | none | none | attack_count | `multiattack-count` | no |
| **Multiattack: an attack may be swapped for a spell** (Multiattack) | action | 20 | 20 | `sequence` `cast_spell` | `sequence` `cast_spell` | none | `multiattack-mixed` | no |
| **Multiattack: attacks mixed with, or swapped for, another action** (Multiattack) | action | 39 | 39 | `sequence` | `sequence` | none | `multiattack-mixed` | no |
| **Breathing: air, water, holding breath** (Amphibious, Water Breathing, Hold Breath, Limited Amphibiousness) | trait | 47 | 47 | none | none | none | `no-table-rule` | no |
| **Other traits with nothing to compute at the table** (Succubus Form, Beast of Burden, Incubus Form, Ephemeral) | trait | 4 | 4 | none | none | none | `no-table-rule` | no |
| **Returns to life elsewhere, or days later** (Diabolical Restoration, Demonic Restoration, Exalted Restoration, Elemental Restoration, Fiendish Restoration, Eldritch Restoration, and 5 more) | trait | 25 | 25 | none | none | none | `no-table-rule` | no |
| **Speech, telepathy and bonds** (Speak with Beasts and Plants, Telepathic Bond, Shark Telepathy, Vampiric Connection) | trait | 4 | 4 | none | none | none | `no-table-rule` | no |
| **Grapples whatever touches it** (Adhesive) | trait | 1 | 1 | `trigger:hit_by_attack` `apply_condition` `edge*` | `trigger:hit_by_attack` `apply_condition` `edge*` | trait_owner | `on-damage` | no |
| **Weakened by a damage type or a surrounding** (Freeze) | trait | 1 | 1 | `trigger:damaged` `speed_penalty` | `trigger:damaged` | trait_owner | `on-damage` | no |
| **Weakened by a damage type or a surrounding** (Water Susceptibility) | trait | 1 | 1 | `trigger:enters` `damage` | `trigger:enters` `damage` | environment, trait_owner | `on-damage` | no |
| **Explodes when it dies** (Death Burst, Death Throes) | trait | 6 | 6 | `trigger:death` `area` `save_gate` `save_damage` | `trigger:death` `save_gate` | trait_owner | `on-death` | no |
| **Damage type chosen when the creature is placed** (Draconic Origin) | trait | 1 | 1 | `choice` | none | trait_owner | `passive-modifier` | no |
| **Double damage to objects and structures** (Siege Monster) | trait | 4 | 4 | `modifier:damage_dealt` | `modifier:damage_dealt` | trait_owner | `passive-modifier` | no |
| **Evasion** (Evasion) | trait | 1 | 1 | `modifier:save_outcome` | `modifier:save_outcome` | trait_owner | `passive-modifier` | no |
| **Extra Reactions** (Reactive Heads, Reactive) | trait | 2 | 2 | `modifier:reaction_count` | `modifier:reaction_count` | trait_owner | `passive-modifier` | no |
| **Gives its rider Resistance to Fire** (Confer Fire Resistance) | trait | 1 | 1 | `modifier:resistance` | `modifier:resistance` | targeting, trait_owner | `passive-modifier` | no |
| **Goes unnoticed until it moves** (Transparent) | trait | 1 | 1 | `conceal` | `conceal` | trait_owner | `passive-modifier` | no |
| **Mind and detection wards** (Inscrutable) | trait | 2 | 2 | `edge*` `modifier:immunity` | `edge*` `modifier:immunity` | trait_owner | `passive-modifier` | no |
| **Mind and detection wards** (Immutable Form, Shielded Mind) | trait | 5 | 5 | `modifier:immunity` | `modifier:immunity` | trait_owner | `passive-modifier` | no |
| **Skill chosen when the creature is placed** (Training) | trait | 1 | 1 | `choice` `modifier:skill` | `modifier:skill` | trait_owner | `passive-modifier` | no |
| **Turns Invisible** (Invisibility) | trait | 1 | 1 | `apply_condition` | `apply_condition` | trait_owner | `passive-modifier` | no |
| **Plain attack, carried as text only** (Bite, Talons, Rake, Scratch, Claw, Storm Blade, and 2 more) | action | 19 | 19 | none (structured attack) | none | seed_data | `plain-attack` | no |
| **Plain attack, structured data agrees with the text** (Rend, Bite, Claw, Slam, Hooves, Beak, and 57 more) | action | 182 | 160 | none (structured attack) | none | none | `plain-attack` | no |
| **Plain attack, structured data disagrees with the text** (Bite, Rend, Claw, Sting, Pact Blade, Slam, and 33 more) | action | 75 | 71 | none (structured attack) | none | seed_data | `plain-attack` | no |
| **Pulls creatures it already holds toward itself** (Reel) | action | 1 | 1 | `forced_move` | none | targeting | `pull` | no |
| **Pulls creatures it already holds toward itself** (Reel) | bonus | 1 | 1 | `forced_move` | none | activation_cost, targeting | `pull` | no |
| **Reaction: raises its AC against the attack that hit** (Parry, Protection) | reaction | 8 | 8 | `trigger:hit_by_attack` `modifier:ac` | `trigger:hit_by_attack` `modifier:ac` | reaction | `reaction-defence` | no |
| **Reaction: raises its AC against the attack that hit** (Riposte) | reaction | 1 | 1 | `trigger:hit_by_attack` `sequence` `modifier:ac` | `trigger:hit_by_attack` `sequence` `modifier:ac` | reaction | `reaction-defence` | no |
| **Reaction: raises its AC against the attack that hit** (Whirlwind of Sand) | reaction | 1 | 1 | `trigger:hit_by_attack` `teleport` `apply_condition` `modifier:ac` | `trigger:hit_by_attack` `apply_condition` `modifier:ac` | reaction, targeting | `reaction-defence` | no |
| **Reaction: reduces the damage of a ranged hit and may throw it back** (Deflect Missile) | reaction | 1 | 1 | `trigger:hit_by_attack` `save_gate` `damage` `modifier:damage_taken` | `trigger:hit_by_attack` `save_gate` `damage` `modifier:damage_taken` | reaction | `reaction-defence` | no |
| **Reaction: adds a bonus to a roll being made** (Burst of Ingenuity) | reaction | 1 | 1 | `trigger:d20_test` `modifier:d20` | `trigger:d20_test` `modifier:d20` | reaction | `reaction-other` | no |
| **Reaction: forces a save on a creature that starts its turn nearby** (Unnerving Gaze) | reaction | 1 | 1 | `trigger:starts_turn_near` `save_gate` `apply_condition` `modifier:immunity` | `trigger:starts_turn_near` `save_gate` `apply_condition` `modifier:immunity` | reaction, targeting | `reaction-other` | no |
| **Reaction: splits into two smaller creatures** (Parry, Split) | reaction | 2 | 2 | `trigger:damaged` `summon*` | `trigger:damaged` `summon*` | reaction, seed_data (1) | `reaction-other` | no |
| **Reaction: swaps places with an ally, who takes the attack** (Redirect Attack) | reaction | 1 | 1 | `trigger:attacked` `movement` `modifier:retarget` | `trigger:attacked` `movement` `modifier:retarget` | reaction | `reaction-other` | no |
| **Reaction: teleports next to a creature that ends its move nearby** (Pursuit) | reaction | 1 | 1 | `trigger:moves_near` `sequence` | `trigger:moves_near` `sequence` | reaction | `reaction-other` | no |
| **Area save for damage only (breath weapons and bursts)** (Dragon's Breath) | action | 1 | 1 | `area` `save_gate` `choice` `save_damage` | `save_gate` | none | `save-area-damage` | no |
| **Area save for damage only (breath weapons and bursts)** (Fire Breath, Cold Breath, Lightning Breath, Acid Breath, Poison Breath, Acid Spray, and 4 more) | action | 47 | 47 | `area` `save_gate` `save_damage` | `save_gate` | targeting (1) | `save-area-damage` | no |
| **Area save for damage only (breath weapons and bursts)** (Acid Breath, Lightning Breath) | action | 4 | 4 | `area*` `save_gate` `save_damage` | `area*` `save_gate` | none | `save-area-damage` | no |
| **Area save for damage only (breath weapons and bursts)** (Disrupt Life) | legendary | 1 | 1 | `area` `save_gate` `save_damage` | `save_gate` | cooldown, legendary, targeting | `save-area-damage` | no |
| **Area save for damage only (breath weapons and bursts)** (Radiant Teleport) | legendary | 1 | 1 | `teleport` `area` `save_gate` `save_damage` | `save_gate` | legendary | `save-area-damage` | no |
| **Area save against a condition, push or weakening, no damage** (Weakening Breath) | action | 4 | 4 | `area` `save_gate` `edge*` `modifier:damage_dealt` | `save_gate` `edge*` `modifier:damage_dealt` | none | `save-area-rider` | no |
| **Area save against a condition, push or weakening, no damage** (Slowing Breath) | action | 4 | 4 | `area` `save_gate` `modifier:action_economy` `speed_penalty*` | `save_gate` `modifier:action_economy` `speed_penalty*` | none | `save-area-rider` | no |
| **Area save against a condition, push or weakening, no damage** (Unsettling Visage, Blinding Breath) | action | 2 | 2 | `area` `save_gate` `save_condition` | `save_gate` | targeting (1) | `save-area-rider` | no |
| **Area save against a condition, push or weakening, no damage** (Repulsion Breath) | action | 4 | 4 | `area` `save_gate` `save_condition` `forced_move` | `save_gate` | none | `save-area-rider` | no |
| **Area save against a condition, push or weakening, no damage** (Fetid Cloud) | action | 1 | 1 | `area` `save_gate` `save_condition` `modifier:action_economy` | `save_gate` `modifier:action_economy` | none | `save-area-rider` | no |
| **Area save against a condition, push or weakening, no damage** (Spores) | action | 1 | 1 | `trigger:turn_start` `area` `save_gate` `damage` `save_condition` | `trigger:turn_start` `save_gate` `damage` | none | `save-area-rider` | no |
| **Area save against a condition, push or weakening, no damage** (Moan) | bonus | 1 | 1 | `area` `save_gate` `save_condition` `modifier:immunity` | `save_gate` `modifier:immunity` | activation_cost | `save-area-rider` | no |
| **Area save against a condition, push or weakening, no damage** (Toxic Ink) | legendary | 1 | 1 | `movement` `area` `save_gate` `save_condition` | `movement` `save_gate` | cooldown, environment, legendary | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Boulder Toss) | action | 1 | 1 | `area` `save_gate` `branch*` `save_damage` `save_condition` | `save_gate` `branch*` | none | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Blinding Spittle, Baleful Command, Mind-Rending Roar, Thunderous Bellow, Stunning Screech) | action | 5 | 5 | `area` `save_gate` `save_damage` `save_condition` | `save_gate` | targeting (3) | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Fling) | action | 1 | 1 | `area` `save_gate` `save_damage` `save_condition` `forced_move` | `save_gate` | targeting | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Horrific Visage) | action | 1 | 1 | `area` `save_gate` `save_damage` `save_condition` `modifier:immunity` | `save_gate` `modifier:immunity` | targeting | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Steam Breath) | action | 1 | 1 | `area` `save_gate` `save_damage` `speed_penalty` | `save_gate` | none | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Deadly Leap) | action | 1 | 1 | `movement` `area` `save_gate` `save_damage` `save_condition` `forced_move` | `movement` `save_gate` | none | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Horror Nimbus) | bonus | 1 | 1 | `trigger:damaged` `area` `save_gate` `save_damage` `save_condition` `modifier:immunity` | `trigger:damaged` `save_gate` `modifier:immunity` | activation_cost | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Cold Gale) | legendary | 2 | 2 | `area` `save_gate` `save_damage` `forced_move` | `save_gate` | cooldown, legendary | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Freezing Burst) | legendary | 2 | 2 | `area` `save_gate` `save_damage` `immobile` | `save_gate` | cooldown, legendary | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Noxious Miasma) | legendary | 2 | 2 | `area` `save_gate` `save_damage` `modifier:ac` | `save_gate` `modifier:ac` | cooldown, legendary | `save-area-rider` | no |
| **Area save for damage plus a condition, push or weakening** (Thunderclap) | legendary | 2 | 2 | `area` `save_gate` `save_damage` `save_condition` | `save_gate` | legendary | `save-area-rider` | no |
| **Area save that changes with each use** (Roar) | action | 1 | 1 | `area` `save_gate` `branch*` `save_damage` `save_condition` | `save_gate` `branch*` | per_rest, targeting | `save-area-rider` | no |
| **Area save that charms and compels the target toward the creature** (Luring Song) | action | 1 | 1 | `trigger:damaged` `area` `save_gate` `apply_condition` `compel` `modifier:immunity` | `trigger:damaged` `save_gate` `apply_condition` `compel` `modifier:immunity` | targeting | `save-area-rider` | no |
| **Area save in stages: a second failure worsens the condition** (Paralyzing Breath, Petrifying Breath) | action | 5 | 5 | `area` `save_gate` `save_condition` | `save_gate` | none | `save-area-staged` | no |
| **Area save in stages: a second failure worsens the condition** (Sleep Breath) | action | 4 | 4 | `trigger:damaged` `area` `save_gate` `save_condition` | `trigger:damaged` `save_gate` | none | `save-area-staged` | no |
| **Area save in stages: a second failure worsens the condition** (Petrifying Gaze) | bonus | 2 | 2 | `area` `save_gate` `save_condition` | `save_gate` | activation_cost, recharge (1) | `save-area-staged` | no |
| **Target save for damage only** (Consume Memories, Pummel, Lightning Strike, Mockery) | action | 4 | 4 | `save_gate` `damage` | `save_gate` `damage` | targeting (2) | `save-target-damage` | no |
| **Target save for damage only** (Trample) | bonus | 3 | 3 | `save_gate` `damage` | `save_gate` `damage` | activation_cost, targeting | `save-target-damage` | no |
| **Target save against a bond that is itself an object with hit points** (Web Strand, Web) | action | 2 | 2 | `save_gate` `apply_condition` `create_object` | `save_gate` `apply_condition` `create_object` | targeting (1) | `save-target-rider` | no |
| **Target save against a bond that is itself an object with hit points** (Entangling Rope) | action | 1 | 1 | `save_gate` `damage` `apply_condition` `create_object` | `save_gate` `damage` `apply_condition` `create_object` | none | `save-target-rider` | no |
| **Target save against a condition, no damage** (Paralyzing Tentacles, Roar, Enthralling Panache, Scare) | action | 4 | 4 | `save_gate` `apply_condition` | `save_gate` `apply_condition` | targeting (1) | `save-target-rider` | no |
| **Target save against a condition, no damage** (Cacophony) | action | 1 | 1 | `save_gate` `apply_condition` `edge*` | `save_gate` `apply_condition` `edge*` | none | `save-target-rider` | no |
| **Target save against a condition, no damage** (Dreadful Glare) | action | 1 | 1 | `save_gate` `apply_condition` `modifier:immunity` | `save_gate` `apply_condition` `modifier:immunity` | none | `save-target-rider` | no |
| **Target save against a condition, no damage** (Quick Grapple, Captain's Charm) | bonus | 2 | 2 | `save_gate` `apply_condition` | `save_gate` `apply_condition` | activation_cost, targeting (1) | `save-target-rider` | no |
| **Target save against a condition, no damage** (Weight of Years, Blinding Gaze) | legendary | 3 | 3 | `save_gate` `apply_condition` | `save_gate` `apply_condition` | cooldown, legendary | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Sting) | action | 1 | 1 | `save_gate` `branch` `damage` `apply_condition` | `save_gate` `damage` `apply_condition` | none | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Shield Bash) | action | 1 | 1 | `save_gate` `branch*` `damage` `apply_condition` | `save_gate` `branch*` `damage` `apply_condition` | none | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Conjure Infernal Chain) | action | 1 | 1 | `save_gate` `branch*` `damage` `apply_condition` `forced_move` | `save_gate` `branch*` `damage` `apply_condition` | none | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Constrict, Poisonous Spittle, Corrupting Touch, Dreadful Glare, Tentacle Slam) | action | 10 | 10 | `save_gate` `damage` `apply_condition` | `save_gate` `damage` `apply_condition` | targeting (7) | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Whirlwind) | action | 1 | 1 | `save_gate` `damage` `apply_condition` `forced_move` | `save_gate` `damage` `apply_condition` | targeting | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Infernal Tail) | action | 1 | 1 | `trigger:turn_start` `save_gate` `damage` `apply_condition` | `trigger:turn_start` `save_gate` `damage` `apply_condition` | none | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Vortex) | action | 1 | 1 | `trigger:turn_start` `save_gate` `damage` `apply_condition` `modifier:action_economy` | `trigger:turn_start` `save_gate` `damage` `apply_condition` `modifier:action_economy` | targeting | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Trampling Charge) | bonus | 1 | 1 | `trigger:enters` `movement` `save_gate` `damage` `apply_condition` | `trigger:enters` `movement` `save_gate` `damage` `apply_condition` | activation_cost | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Giggling Magic) | legendary | 2 | 2 | `save_gate` `damage` `bonus_die*` | `save_gate` `damage` `bonus_die*` | cooldown, legendary | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Cloud of Insects) | legendary | 2 | 2 | `save_gate` `damage` `save_edge*` | `save_gate` `damage` `save_edge*` | cooldown, legendary | `save-target-rider` | no |
| **Target save for damage plus a condition, push or weakening** (Scorching Sands) | legendary | 2 | 2 | `save_gate` `damage` `speed_penalty*` | `save_gate` `damage` `speed_penalty*` | cooldown, legendary | `save-target-rider` | no |
| **Target save for damage that lowers Hit Point maximum** (Bite) | action | 1 | 1 | `save_gate` `damage` `heal` `modifier:hp_max` | `save_gate` `damage` `heal` `modifier:hp_max` | targeting | `save-target-rider` | no |
| **Target save for damage that lowers Hit Point maximum** (Bite) | action | 1 | 1 | `save_gate` `damage` `heal` `modifier:hp_max` `summon*` | `save_gate` `damage` `heal` `modifier:hp_max` `summon*` | targeting | `save-target-rider` | no |
| **Target save for damage that lowers Hit Point maximum** (Draining Kiss) | action | 1 | 1 | `save_gate` `damage` `modifier:hp_max` | `save_gate` `damage` `modifier:hp_max` | targeting | `save-target-rider` | no |
| **Target save for damage that lowers Hit Point maximum** (Life Drain) | action | 1 | 1 | `save_gate` `damage` `modifier:hp_max` `summon*` | `save_gate` `damage` `modifier:hp_max` `summon*` | none | `save-target-rider` | no |
| **Target save that banishes until the creature's next turn** (Banish) | legendary | 2 | 2 | `teleport*` `save_gate` `damage` `apply_condition` | `teleport*` `save_gate` `damage` `apply_condition` | cooldown, legendary | `save-target-rider` | no |
| **Target save that kills or drops a weakened target outright** (Death Glare, Slaying Bow) | action | 2 | 2 | `save_gate` `branch` `damage` | `save_gate` `damage` | targeting (1) | `save-target-rider` | no |
| **Target save that kills or drops a weakened target outright** (Consume Life) | bonus | 1 | 1 | `save_gate` `damage` `heal` | `save_gate` `damage` `heal` | activation_cost, targeting | `save-target-rider` | no |
| **Target save that kills or drops a weakened target outright** (Nightmare) | bonus | 1 | 1 | `trigger:damaged` `save_gate` `branch` `damage` `apply_condition` | `trigger:damaged` `save_gate` `damage` `apply_condition` | activation_cost | `save-target-rider` | no |
| **Target save that takes control of the target** (Possession) | action | 1 | 1 | `save_gate` `apply_condition` `conceal` `control` `modifier:immunity` | `save_gate` `apply_condition` `conceal` `control` `modifier:immunity` | targeting | `save-target-rider` | no |
| **Target save that takes control of the target** (Dominate Mind) | action | 1 | 1 | `trigger:damaged` `save_gate` `apply_condition` `control` | `trigger:damaged` `save_gate` `apply_condition` `control` | none | `save-target-rider` | no |
| **Bolsters itself or an ally** (War Cry) | bonus | 1 | 1 | `edge` `temp_hp` | none | activation_cost, targeting | `self-buff` | no |
| **Bolsters itself or an ally** (Shimmering Shield) | legendary | 1 | 1 | `modifier:ac` `temp_hp` | `modifier:ac` | cooldown, legendary | `self-buff` | no |
| **Turns Invisible** (Vanish) | bonus | 1 | 1 | `apply_condition` | `apply_condition` | activation_cost | `self-buff` | no |
| **Regains hit points from a damage type** (Lightning Absorption, Acid Absorption, Fire Absorption) | trait | 4 | 4 | `trigger:damaged` `heal` | `trigger:damaged` `heal` | trait_owner | `self-heal` | no |
| **Regeneration** (Regeneration) | trait | 2 | 2 | `trigger:damaged` `trigger:turn_start` `heal` | `trigger:damaged` `trigger:turn_start` `heal` | trait_owner | `self-heal` | no |
| **Regeneration** (Regeneration) | trait | 2 | 2 | `trigger:turn_start` `heal` | `trigger:turn_start` `heal` | trait_owner | `self-heal` | no |
| **Moves or leaps on its own, with or without provoking Opportunity Attacks** (Bubble Dash) | action | 1 | 1 | `movement` | `movement` | environment | `self-move` | no |
| **Moves or leaps on its own, with or without provoking Opportunity Attacks** (Leap, Charge, Bubble Dash, Swoop, Aquatic Charge) | bonus | 10 | 10 | `movement` | `movement` | activation_cost, environment (1), targeting (4) | `self-move` | no |
| **Moves or leaps on its own, with or without provoking Opportunity Attacks** (Prowl) | bonus | 1 | 1 | `sequence` `movement` | `sequence` `movement` | activation_cost | `self-move` | no |
| **Sees into the Ethereal Plane** (Ethereal Sight) | trait | 2 | 2 | `sense` | `sense` | environment, trait_owner | `sense` | no |
| **Shape-Shift** (Shape-Shift) | action | 3 | 3 | `form*` | `form*` | none | `shape-shift` | no |
| **Shape-Shift** (Shape-Shift) | bonus | 1 | 1 | `conceal` `form*` | `conceal` `form*` | activation_cost | `shape-shift` | no |
| **Shape-Shift** (Shape-Shift) | bonus | 8 | 8 | `form*` | `form*` | activation_cost | `shape-shift` | no |
| **Shape-Shift** (Shape-Shift) | bonus | 1 | 1 | `form*` `modifier:resistance` `move_rule` | `form*` `modifier:resistance` `move_rule` | activation_cost, environment | `shape-shift` | no |
| **Carries an object with its own AC and hit points** (Magic Rope, Soul Bag) | trait | 2 | 2 | `create_object` | `create_object` | trait_owner | `special` | no |
| **Goes berserk while Bloodied and attacks the nearest creature** (Berserk) | trait | 2 | 2 | `trigger:turn_start` `branch*` `compel` | `trigger:turn_start` `branch*` `compel` | trait_owner | `special` | no |
| **Heads die and regrow** (Multiple Heads) | trait | 1 | 1 | `trigger:damaged` `trigger:turn_end` `heal` `resource_change*` | `trigger:damaged` `trigger:turn_end` `heal` `resource_change*` | trait_owner | `special` | no |
| **Loses limbs that fight on** (Loathsome Limbs) | trait | 1 | 1 | `trigger:turn_end` `branch*` `apply_condition` `summon*` | `trigger:turn_end` `branch*` `apply_condition` `summon*` | per_day, trait_owner | `special` | no |
| **Saves to stay at 1 Hit Point** (Undead Fortitude) | trait | 2 | 2 | `trigger:drops_to_0` `save_gate` | `trigger:drops_to_0` `save_gate` | trait_owner | `special` | no |
| **Shares the damage its amulet's wearer takes** (Bound) | trait | 1 | 1 | `modifier:damage_share` | `modifier:damage_share` | trait_owner | `special` | no |
| **Shrugs off or reflects spells aimed at it** (Reflective Carapace) | trait | 1 | 1 | `trigger:targeted_by_spell` `modifier:retarget` | `trigger:targeted_by_spell` `modifier:retarget` | trait_owner | `special` | no |
| **Stores one spell and casts it later** (Spell Storing) | trait | 1 | 1 | `cast_spell` | `cast_spell` | trait_owner | `special` | no |
| **Turns to mist at 0 Hit Points** (Misty Escape) | trait | 1 | 1 | `trigger:drops_to_0` `apply_condition` `form*` `heal` | `trigger:drops_to_0` `apply_condition` `form*` `heal` | trait_owner | `special` | no |
| **Spellcasting: a list of spells cast at will or a set number of times a day** (Spellcasting) | action | 48 | 48 | `cast_spell` | `cast_spell` | per_day (46) | `spellcasting-list` | no |
| **Takes Dash, Disengage or Hide** (Nimble Escape, Cunning Action, Deathless Agility, Hasten, Shadow Stealth) | bonus | 12 | 12 | none | none | activation_cost, environment (1) | `standard-action` | no |
| **Creates or raises creatures under its control** (Animate Trees, Create Specter) | action | 2 | 2 | `summon*` | `summon*` | targeting (1) | `summon` | no |
| **Steps to or from the Ethereal Plane** (Ethereal Stride) | action | 1 | 1 | `conceal` | `conceal` | environment | `teleport` | no |
| **Steps to or from the Ethereal Plane** (Ethereal Jaunt) | bonus | 1 | 1 | `conceal` | `conceal` | activation_cost, environment | `teleport` | no |
| **Teleports another creature, or between fixed points** (Teleport, Tree Stride) | bonus | 2 | 2 | `teleport*` | `teleport*` | activation_cost, environment (1), targeting (1) | `teleport` | no |
| **Teleports itself** (Teleport) | action | 1 | 1 | `teleport` | none | activation_cost | `teleport` | no |
| **Teleports itself** (Teleport) | bonus | 2 | 2 | `teleport` | none | recharge (1) | `teleport` | no |
| **Teleports itself** (Deathly Teleport) | legendary | 1 | 1 | `teleport` `area` `damage` | `damage` | legendary | `teleport` | no |
| **Makes one attack or uses another of its actions** (Tail Swipe, Lash, Storm Bolt, Glare, Necrotic Strike) | legendary | 6 | 5 | `sequence` | `sequence` | cooldown (1), legendary | `use-other-action` | no |
| **Makes one attack or uses another of its actions** (Psychic Drain) | legendary | 1 | 1 | `sequence` `heal` | `sequence` `heal` | legendary, targeting | `use-other-action` | no |
| **Moves, then knocks creatures around it Prone and breaks Concentration** (World-Shaking Movement) | legendary | 1 | 1 | `movement` `area` `branch*` `apply_condition` `modifier:concentration` | `movement` `branch*` `apply_condition` `modifier:concentration` | cooldown, legendary | `use-other-action` | no |
| **Moves, then makes one attack** (Rampage) | bonus | 2 | 2 | `sequence` `movement` `branch*` | `sequence` `movement` `branch*` | activation_cost | `use-other-action` | no |
| **Moves, then makes one attack** (Pounce, Onslaught, Charging Horn, Deathless Strike) | legendary | 21 | 21 | `sequence` `movement` | `sequence` `movement` | legendary | `use-other-action` | no |
| **Moves, then makes one attack** (Arcane Prowl) | legendary | 2 | 2 | `sequence` `teleport` | `sequence` | legendary | `use-other-action` | no |
| **Area the creature conjures or carries that lasts** (Darkness Aura) | action | 1 | 1 | `area` `create_surface` | none | none | `zone` | no |
| **Area the creature conjures or carries that lasts** (Create Whirlwind) | action | 1 | 1 | `trigger:enters` `trigger:turn_start` `area` `save_gate` `damage` `apply_condition` `contain` | `trigger:enters` `trigger:turn_start` `save_gate` `damage` `apply_condition` `contain` | none | `zone` | no |
| **Area the creature conjures or carries that lasts** (Ink Cloud) | reaction | 1 | 1 | `trigger:damaged` `movement` `area` `create_surface` | `trigger:damaged` `movement` | environment, reaction | `zone` | no |
| **Area the creature conjures or carries that lasts** (Ink Cloud) | reaction | 1 | 1 | `trigger:ends_turn_near` `movement` `area` `create_surface` | `trigger:ends_turn_near` `movement` | environment, reaction | `zone` | no |
| **Ground around it is Difficult Terrain** (Aberrant Ground) | trait | 1 | 1 | `area` `create_surface*` | `create_surface*` | trait_owner | `zone` | no |
<!-- /generated:table -->

## 4. Missing kinds

### 4.1 New kinds

- **`save_gate`**: a saving throw that carries its own DC and can be aimed at one creature as well as
  at everyone an `area` covers, with what follows a failure, a success, either, or a second failure
  under it. ARCHITECTURE.md §9.3 calls it `SaveGate`. Today `save_damage` and `save_condition`
  resolve only inside an area cast, and the DC is the caster's spell save DC, which a seed monster
  does not have, so it falls back to 13.
- **`attack_gate`**: ties an Effect to one of the creature's attacks so that its parts fire on a hit
  (§9.3 `AttackGate`). Today a structured attack resolves to hit and damage and nothing can hang on it.
- **`damage`**: damage dealt outright, without an area save of its own: extra or alternative dice on
  a hit, damage at the start of a turn, damage after a `save_gate`. Today `save_damage` is tied to an
  area and `extra_damage` to hits by an Effect's source on its bearer.
- **`apply_condition`**: puts a condition, or another named Effect, on one creature with its own end:
  a turn boundary, an escape DC, a repeated save, a length of time. Today `save_condition` does this
  only for everyone who fails an area save.
- **`trigger`**: fires its parts when something happens rather than when the Effect is used: the
  start or end of a turn, being hit or damaged, dying, dropping to 0 Hit Points, a creature entering
  or starting its turn nearby (§9.3 `Trigger`). Today `reacts` knows one trigger, `damaged`, and
  hands the DM an instruction.
- **`modifier`**: a lasting change to a number or a rule of the bearer (§9.3 `Modifier`): AC, Hit
  Point maximum, an ability score, healing blocked, limits on actions and reactions, immunity to a
  named effect, damage dealt or taken.
- **`sequence`**: takes named actions of the same stat block as one use: a fixed mix of attacks, an
  attack swapped for another action, "makes one Rend attack" as a legendary action.
- **`cast_spell`**: casts a named spell's Effect with the monster's own save DC, attack bonus and
  slot level, without components, at will or a set number of times a day.
- **`movement`**: the creature moves as part of the Effect: up to its Speed or half of it, flying,
  swimming or burrowing, a jump, with or without provoking Opportunity Attacks (§9.3 `Movement`
  beyond teleport and push).
- **`move_rule`**: a standing rule for how the bearer moves: no Opportunity Attack when it leaves
  reach, squeezing through gaps, sharing another creature's space, ignoring a kind of terrain,
  climbing, burrowing, fixed jump distances. A sibling of `move_cost`.
- **`heal`**: gives hit points back: a fixed amount each turn, the damage just dealt, dice (§9.3
  `Heal`). Today only `temp_hp` exists.
- **`contain`**: holds a creature inside or on another: it moves with its holder, has Total Cover,
  carries conditions while held, and gets out by a damage threshold and a save, an escape check, or
  the holder's death. Has a capacity.
- **`control`**: hands a Combatant to another Controller or side while an Effect lasts.
- **`compel`**: dictates what the bearer does with its turn: approach the source, attack the nearest
  creature, move at random, do nothing.
- **`conceal`**: gives a Visibility Quality (Invisible, Disguised, Ethereal, unnoticed until it
  moves). The mirror of `reveal`.
- **`sense`**: gives a Sense or widens what one pierces (§9.3 `Sense`).
- **`create_object`**: an object in play with its own AC and hit points whose destruction ends
  something: a web strand, a magic rope, a roper's tentacle (§9.3 `CreateObject`).
- **`item_wear`**: wears down or destroys an Item a creature wears or wields: a stacking penalty to
  the AC it gives or to attack rolls, destruction at a threshold, undone by Mending.
- **`light`**: sheds bright and dim light. It exists in the Go union for the Homebrew spell builder
  but has no component row, so a seeded Effect cannot carry it.

### 4.2 Kinds the engine has that lack a parameter

- **`branch*`**: conditions it cannot test: the target's size, the target or the source being
  Bloodied, the attack roll having had Advantage, the source having moved a distance straight toward
  the target, a creature *not* being of a type, the target being held by the source. Today it tests
  `fails_by`, `hp_at_most`, `first_each_turn` and `creature_is`.
- **`edge*`**: an advantage or disadvantage that holds only in a state (an ally within 5 feet of the
  target, Bloodied, in sunlight, the target below its Hit Point maximum or Grappled by the source),
  that covers ability checks, or that is spent by the next roll.
- **`save_edge*`**: every ability at once, limited to spells and other magical effects, or an
  automatic success.
- **`form*`**: a Form that keeps the creature's own stat block and changes only its size, Speed or
  appearance, chosen from a fixed list.
- **`summon*`**: a Summon with an altered stat block, hit points split between copies, a cap on how
  many, or one that rises later from a corpse.
- **`teleport*`**: teleporting another creature, or to a destination with a constraint.
- **`speed_penalty*`**: halving Speed rather than taking feet off it.
- **`area*`**: a Line wider than 5 feet.
- **`bonus_die*`**: a die subtracted rather than added, and on ability checks.
- **`create_surface*`**: plain Difficult Terrain. Every Surface kind today brings something more.
- **`resource_change*`**: a counter on a monster. Resources belong to Characters today.

## 5. Other gaps

<!-- generated:gaps -->
| Other gap | Actions | Traits |
|---|---|---|
| `trait_owner` | 0 | 214 |
| `seed_data` | 131 | 0 |
| `legendary` | 82 | 32 |
| `attack_count` | 91 | 0 |
| `activation_cost` | 75 | 0 |
| `targeting` | 65 | 9 |
| `environment` | 12 | 40 |
| `per_day` | 48 | 3 |
| `cooldown` | 41 | 0 |
| `reaction` | 24 | 0 |
| `recharge` | 2 | 0 |
| `per_rest` | 2 | 0 |
<!-- /generated:gaps -->

- **`trait_owner`**: an Effect's owner is one of spell, feature, item property, condition, monster
  action, surface, rule variant, trap or roll table result. A monster trait is none of them, and
  nothing puts a trait's Effect on a Combatant when it is placed. Every trait with rules needs this.
- **`seed_data`**: the seed is wrong or incomplete for this entry. See §6.
- **`legendary`**: the legendary economy exists for Homebrew creatures and is not fed for seed
  monsters. See §2.4.
- **`activation_cost`**: an Effect does not say what it costs to use. Casting an area always spends
  the action, a teleport always the Bonus Action, and a standard action such as Disengage always the
  action. A Bonus Action breath, an action teleport and Nimble Escape all need the cost declared.
- **`targeting`**: Targeting reaches "everyone in an area, friend or foe" or "the token the DM
  picks". These entries need a side filter (each enemy, creatures of its choice), a creature-type
  filter, or a state filter (Grappled by the source, Prone, Frightened, a size limit).
- **`environment`**: the rule depends on a fact about the surroundings the engine may not hold:
  sunlight, underwater, a web, ice, the Ethereal Plane.
- **`per_day`, `per_rest`, `recharge`, `cooldown`**: limited uses for a monster's action.
- **`reaction`**: the entry is a reaction; it needs a Reaction Prompt for a trigger other than the
  ones the engine offers today (Opportunity Attack, Shield, Counterspell, a readied attack,
  `damaged`).
- **`attack_count`**: the stat block read for a seed monster sets no attacks-per-action.

## 6. Seed findings

Found while counting; none needs an engine change.

- **33 attack rolls have no structured attack**: 19 plain attacks and 14 with a rider. The text
  varies from the usual form in each: flat damage with no dice ("1 Piercing damage"), "feet" for
  "ft.", or a note in brackets after the attack bonus.
- **75 plain attacks have structured data that disagrees with the text.** 66 lose the second damage
  type. The rest: second damage dice that differ from the text (Adult Bronze Dragon, Adult Green
  Dragon, Ancient Blue Dragon, Ancient White Dragon and Young Blue Dragon's Rend), a negative damage
  bonus stored as positive (Baboon and Jackal's Bite, `1d4 - 1`), a second damage type with no dice
  (Young Silver Dragon's Rend), and a missing damage type (Adult Blue Dragon's Rend). Rider-bearing
  attacks have the same faults; the CSV marks each with `seed_data`.
- **Recharge is absent** from every breath action. See §2.4.
- **No legendary actions per round** are recorded.
- **Black Pudding's reaction is named Parry** but carries the text of Split.
- **Markup is left in descriptions**: `Sphere [Area of Effect]|XPHB|Sphere`, `Cover|XPHB|Total
  Cover`, `HitomThe spear`, `Hitazard burning`.
- One Compendium monster has no actions, and 127 have no traits.

## 7. What could not be verified

- **The SRD 5.2 text itself.** The repository holds no copy; the seed is the only source read. Where
  the seed looks wrong (no Recharge on breath weapons, Parry on a Black Pudding) the claim rests on
  the seed disagreeing with itself or with how the rules are usually printed, not on a comparison
  with the published document.
- **What the upstream source offers.** The seed names Open5e v2 as its source. Whether it exposes
  Recharge, legendary action counts or costs in fields the importer does not read could not be
  checked: the API was not reachable from where this ran.
- **Running behaviour.** Nothing was executed against the engine. Every statement about what resolves
  today comes from reading the code named in §1.
- **Darkness Aura.** It is counted as automatable with `area` and `create_surface`. A concentration
  emanation follows its caster, but whether a Surface it laid follows too was not traced.
- **Temporary Hit Points given as dice.** `temp_hp` takes a fixed amount. War Cry and Shimmering
  Shield print an average with dice; they are counted as covered on the assumption that the average
  is used.
- **Flat-damage attacks that do have a structured row** (Octopus's Tentacles) were not traced through
  the damage roll.
- **One-off entries.** For the entries read by hand, the kinds listed are a reading of the rules
  text, not a design. A different decomposition would move a few entries between `modifier`,
  `trigger` and `apply_condition` without changing the ranking.

## 8. Running the census again

```sh
python3 docs/research/census-monsters.py
```

It reads the component kinds from the newest `effect_components_kind_check`, so a kind added by a
later migration drops out of the missing column by itself. It rewrites the CSV and every generated
block in this file. A new action or trait in the seed that no rule covers stops the run and is named.
