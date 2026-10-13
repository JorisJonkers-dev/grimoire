# What Open5e holds for the facts the seed lacks or has wrong

Research for [#254](https://github.com/JorisJonkers-dev/grimoire/issues/254), part of the map in
[#236](https://github.com/JorisJonkers-dev/grimoire/issues/236). Read against `main` at `92a4a7a`
(release 1.3.0), 2026-10-13.

**Question.** For each fact the importer does not read today, or that the four census sessions found
wrong in the Compendium seed: does Open5e v2 hold it in structured form, as text only, or not at
all? Is the defect upstream or in our importer? Is the published SRD 5.2 document needed as a second
source?

**Short answer.**

- The seed is a faithful copy of the live API. Every wrong or missing text the censuses found is
  wrong or missing upstream too. The importer mangles nothing.
- The importer does leave structured data unread. Recharge and uses per day on actions sit in
  `usage_limits`, complete and in agreement with the SRD. Reading that field is the whole fix.
- Open5e has no legendary action uses per round anywhere. The SRD prints it on all 30 legendary stat
  blocks. This is the one monster fact that needs the SRD.
- SRD 5.2 has no lair actions. Nothing is missing there.
- Upstream fixed a good part of the text defects on 2026-10-09 to 2026-10-11 (spell text, the two
  wands, attack damage types). Those fixes sit on the `staging` branch and are not in the live API
  yet. A fresh snapshot after the next Open5e release picks them up with no importer change.
- Three class features, the spell flags, the magic flag on five items and one monster name stay
  wrong upstream and need a hand correction, three of them with text from the SRD.

## 1. Sources

Each finding below names the source it rests on by these labels.

| Label | Source | Read how |
|---|---|---|
| **seed** | `api/db/seeds/compendium.json.gz`, document `srd-2024`, snapshot of 2026-09-29 | Unpacked and read whole. |
| **live** | Open5e v2 API, `https://api.open5e.com/v2/`, release v2.2.3 of 2026-10-03 | Every `srd-2024` row of `creatures`, `spells`, `items`, `magicitems`, `classes`, `feats` and `species` fetched with curl on 2026-10-13. The API answered at once; the earlier sessions' timeouts did not recur. |
| **staging** | [`open5e/open5e-api`](https://github.com/open5e/open5e-api), default branch `staging` at `f9efac38` (2026-10-11): the fixtures in `data/v2/wizards-of-the-coast/srd-2024/` and the models in `api_v2/models/` | Sparse clone, read as JSON and source. Not released: `staging` is 82 commits ahead of `main`, which the live API serves. |
| **SRD** | `SRD_CC_v5.2.1.pdf` from `https://media.dndbeyond.com/compendium-images/srd/5.2/` (CC-BY-4.0), and `SRD_CC_v5.2.pdf` where noted | Text extracted with `pdftotext`, searched by entry name. |

Two checks tie them together.

- **seed against live:** no difference in any action name or text of the 331 monsters, in `desc`,
  `higher_level`, `attack_roll`, `saving_throw_ability` or `damage_roll` of the 339 spells, in any
  item description, or in any class feature name or text. A snapshot taken today would be the same
  seed.
- **live against staging:** differences are listed in the table. They are all fixes.

## 2. The table

Columns: what the seed has; what Open5e v2 has, as **structured**, **text only** or **absent**, live
first and `staging` where it differs; where the defect sits; whether the SRD document is needed.

### 2.1 Monsters

| Fact | Seed | Open5e v2 | Defect sits | SRD needed |
|---|---|---|---|---|
| Recharge on a roll | No field. The marker survives in two action names (Basilisk's Petrifying Gaze, Blink Dog's Teleport). | **Structured**, live and staging: `actions[].usage_limits`. Adult Red Dragon, Fire Breath: `{"type": "RECHARGE_ON_ROLL", "param": 5}`. 86 actions carry it: 73 as `RECHARGE_ON_ROLL`, 13 as `RECHARGE`, both with `param` 4, 5 or 6 as the lowest roll that recharges. The SRD has 86 `(Recharge N-6)` markers too, 52 of them on breath weapons in both. Name and number agree for the 83 markers the text extraction kept on one line. | Importer: `usage_limits` is not read. | No |
| Recharge after a rest | Marker in the name only: Cloaker's `Phantasms (Recharge after a Short or Long Rest)`. | Live: **text only**, in the name; `usage_limits` is `null`. Staging: `uses_type: "RECHARGE"` with `uses_param: null`. The model has a `RECHARGE_AFTER_REST` type that no SRD 5.2 row uses. One action; the SRD has one such marker. | Upstream. The name can be parsed. | No |
| Uses per day on an action | Nothing. Action names are clean and the field is unread. | **Structured**, live and staging: `usage_limits`. Aboleth, Dominate Mind: `{"type": "PER_DAY", "param": 2}`. 27 actions, matching the SRD's markers on actions. | Importer. | No |
| Uses per day on a trait | The trait name, as upstream. | **Text only**, in the trait name: `Legendary Resistance (3/Day, or 4/Day in Lair)`. A trait has `name` and `desc` and no other field. 33 traits: 32 Legendary Resistance and the Troll's `Loathsome Limbs (4/Day)`. | Upstream has no field. The name can be parsed. | No |
| Spells cast a number of times per day | The action text, as upstream. | **Text only**, inside `desc` of the 48 `Spellcasting` actions: `- **1/Day Each:** Fireball`. `usage_limits` is `null` on all 48. | Upstream has no field. The text can be parsed. | No |
| Legendary action uses per round | Nothing. | **Absent**, live and staging. No field on the creature or the action, and the phrase occurs nowhere in the data. The SRD prints `Legendary Action Uses: 3 (4 in Lair)` on 27 stat blocks and `Legendary Action Uses: 3` on 3 (Solar, Tarrasque, Unicorn). | Upstream lacks it. | **Yes**: 30 values. They follow one rule: 3, and 4 in the lair exactly where the Legendary Resistance trait name says `in Lair`. |
| Cost of a legendary action | Nothing. | **Structured**: `actions[].legendary_action_cost`, `1` on all 82 legendary actions. That is right: the SRD has no "Costs N Actions" anywhere. Live also returns `1` on 751 ordinary actions; staging clears those to `null`. Tell a legendary action by `action_type`, never by this field. | Importer does not read it; nothing is lost, since every cost is 1. | No |
| A legendary action that cannot be repeated in a round | The action text, as upstream. | **Text only**: "can't take this action again until the start of its next turn" in `desc` of 41 of the 82. | Upstream has no field. The sentence is fixed and can be matched. | No |
| Lair actions | None. | **Absent**, and rightly. The model has a `LAIR_ACTION` type; no SRD 5.2 row uses it; the SRD does not contain the words "lair action". In SRD 5.2 a lair changes three numbers only: the Legendary Resistance count (in the trait name), legendary action uses (absent, row above) and XP (`CR 17 (XP 18,000, or 20,000 in lair)` on 27 stat blocks; Open5e holds one `experience_points` value). | Not a defect. | Only for the uses and the XP in a lair. |
| Second damage type on an attack | 31 attacks have `extraType`. The censuses count 66 plain attacks that lose it. | Live: **structured** for 30 attacks (Adult Red Dragon, Rend: `extra_damage_die_count: 2`, `extra_damage_die_type: "D4"`, `extra_damage_type: fire`). **Text only** for 88: live leaves `damage_type` `null` on 350 of 390 attacks and puts the first type in `extra_damage_type`, so the second has nowhere to go. Basilisk, Bite: `damage_type: null`, `extra_damage_type: piercing`, text "plus 7 (2d6) Poison damage". Staging: the type is **structured** for all 118 (`damage_type: piercing`, `extra_damage_type: poison`); the dice are right for 26, wrong for 5 and missing for 87. A branch not yet merged (`1004-fix-attack-damage-dice`) fills the dice. | Upstream. | No. The action text holds type and dice. |
| Attack rolls with no structured attack | 33 actions with an attack roll in the text and no attack row. | **Text only**, live and staging: `attacks: []` on the same 33. Badger, Bite: "Melee Attack Roll: +2, reach 5 ft. 1 Piercing damage." Each has flat damage, "feet" for "ft." or a note after the bonus. The unmerged dice branch leaves all 33 without a row. | Upstream. The text can be parsed. | No |
| Wrong numbers on an attack | As upstream. | **Structured but wrong**, live and staging: second damage dice on the Rend of five dragons (Young Blue Dragon: `1` `D6` stored, text "5 (1d10) Lightning"); `damage_bonus: 1` for "1d4 - 1" on Baboon and Jackal. The unmerged dice branch fixes all seven. The text is right in each. | Upstream. | No |
| Black Pudding's reaction | Named Parry, with the text of Split. | Live and staging: `name: "Parry"` on the row keyed `black-pudding-split`. The SRD names it Split. | Upstream. | To confirm the name only. |
| Markup left in action text | `Sphere [Area of Effect]\|XPHB\|Sphere` and the like. | Live and staging: the same, in 25 actions. | Upstream. | No |

### 2.2 Spells

| Fact | Seed | Open5e v2 | Defect sits | SRD needed |
|---|---|---|---|---|
| Attack roll flag | `attackRoll` true for 42 spells. | **Structured but unreliable**, live and staging: `attack_roll`. Bless: `true`. Of the 42 spells flagged, 24 have no spell attack in their text; they mention attack rolls for another reason. Three that do make one are `false` (Arcane Hand, Conjure Fey, Spiritual Weapon). | Upstream. | No. "Make a melee spell attack" or "ranged spell attack" in the text decides it. |
| Save ability | `saveAbility` set for 128 spells. | **Structured, mostly right**: `saving_throw_ability`. For 127 of the 128 it equals the first save the text names. Freedom of Movement has `"constitution"`, `damage_roll: "10d6"`, `damage_types: ["cold"]`, `shape_type: "sphere"`, `shape_size: 60`: field for field the values of Freezing Sphere, the next spell in the SRD. Six spells name a save in their text and have an empty field (Arcane Hand, Control Water, Dispel Evil and Good, Glyph of Warding, Holy Aura, Symbol). Live and staging agree. | Upstream. | No |
| Damage roll | `damageRoll` set for 119 spells. | **Structured but loose**: `damage_roll` holds the first dice in the text, damage or not. Bless `1d4`, Blink `1d6`, Teleport `1d100`, Cure Wounds `2d8`. 24 of the 119 have an empty `damage_types`. Live and staging agree. | Upstream. Requiring a damage type filters most, but wrongly drops Chromatic Orb, Sorcerous Burst, Dragon's Breath and Conjure Elemental (the type is chosen) and keeps Freedom of Movement. | No |
| Empty description | `greater-invisibility` is empty. | Live: `desc: ""`. Staging: "A creature you touch has the Invisible condition until the spell ends.", as in the SRD. | Upstream; fixed, not released. | No, once released. |
| Cut descriptions: Moonbeam, Conjure Celestial | Moonbeam lacks its first paragraph; Conjure Celestial lacks the two lights. | Live: the same. Staging: both complete, checked against the SRD. Staging also completes Antimagic Field, Command, Hallow and Knock. | Upstream; fixed, not released. | No, once released. |
| Cut description: Telekinesis | Stops at "such as manipulating a simple tool,". | Live and staging: the same. **The SRD stops there too**, in both the 5.2 and the 5.2.1 PDF; Telepathic Bond follows mid-sentence. | The published document. | It does not help. |
| Stray heading in Tsunami | Ends with `### U-Z Spells`. | Live and staging: the same. | Upstream. | No |
| Missing higher-level text | `chain-lightning` and `dissonant-whispers` have none. | Live: `higher_level: ""` and no `casting_options`. Staging: both have the text, and rows for each slot level (Chain Lightning `target_count` 5 to 7; Dissonant Whispers `damage_roll` `4d6` to `11d6`). The SRD has both sentences. | Upstream; fixed, not released. | No, once released. |
| Scaling that is not damage dice | `scaling` rows for damage only; 56 spells have higher-level text and no row. | **Structured**: `casting_options[]` also carries `target_count` (164 rows), `duration` (59), `shape_size` (30), `concentration` (18), `range` (3) and `desc` (87). Bless: `{"type": "slot_level_2", "target_count": 2}`. | Importer: only `damage_roll` is read. | No |

### 2.3 Items

| Fact | Seed | Open5e v2 | Defect sits | SRD needed |
|---|---|---|---|---|
| Magic flag | `magic` false for Ioun Stone, Potion of Giant Strength, Potion of Healing, Potions of Healing and Spell Scroll. | **Structured only by endpoint.** Open5e keeps two models: `Item` (`/v2/items/`) and `MagicItem` (`/v2/magicitems/`), and the importer sets `magic` by which endpoint a row came from. The five rows are `Item` rows, live and staging. An `Item` has no `rarity`, `requires_attunement` or `attunement_detail` at all. The SRD prints three of them as "Rarity Varies", and Ioun Stone as "Requires Attunement". | Upstream files them as mundane. No field lets the importer tell. | For the rarity and attunement line of each. |
| Rows that bundle variants | Ioun Stone is one row with fourteen stones. | **Text only**: one row, with each stone in `desc` as "Absorption (Very Rare). While this pale lavender ellipsoid ...". Potion of Giant Strength and Potions of Healing hold a table in `desc`. The SRD prints each as one entry too, so upstream follows the document. Upstream does split some families (five Belts of Giant Strength, fourteen Ammunition of Slaying). | Neither; it is how the SRD is laid out. | No. The text of every variant is in the row already. |
| `wand-of-web` text | Carries the Wand of Fear table and Regaining Charges paragraph. | Live: the same. Staging: "This wand has 7 charges. While holding it, you can expend 1 charge to cast Web (save DC 13) from it." and its own Regaining Charges paragraph, as in the SRD. Staging also gives Wand of Fear its missing paragraph back. | Upstream; fixed, not released. | No, once released. |

### 2.4 Features

| Fact | Seed | Open5e v2 | Defect sits | SRD needed |
|---|---|---|---|---|
| `circle-of-the-land_natures-ward` | Text of Nature's Sanctuary. | Live and staging: the same. A feature has `name`, `desc` and its levels; nothing structured. The SRD text (immunity to Poisoned, a Resistance by land type, and a table) is nowhere in Open5e. | Upstream. | **Yes** |
| `circle-of-the-land_natures-sanctuary` | Text of Natural Recovery. | Live and staging: the same. The right text exists upstream, on the Nature's Ward row. | Upstream. | To confirm; the text can be moved from the other row. |
| Remarkable Athlete | One sentence. | Live and staging: the same 99 characters. The SRD has a second sentence: after a Critical Hit, movement of up to half Speed without provoking Opportunity Attacks. | Upstream. | **Yes** |

## 3. Summary

**An importer change alone fixes:**

- Recharge on a roll and uses per day on actions: read `usage_limits` (86 and 27 actions). Treat
  `RECHARGE` and `RECHARGE_ON_ROLL` alike; `param` is the lowest roll that recharges.
- The cost of a legendary action: read `legendary_action_cost` where `action_type` is
  `LEGENDARY_ACTION`. Every value is 1.
- Spell scaling beyond damage dice: read the other `casting_options` fields.
- By parsing text the importer already stores, with no new source: the 33 attack rolls that have no
  attack row; the second damage type and dice of the 88 attacks that lose it; uses per day in trait
  names and in `Spellcasting` lists; the rest-recharge marker in one action name; the repeat limit
  on 41 legendary actions; the spell attack flag, from "spell attack" in the description.

**A new snapshot after the next Open5e release fixes, with no importer change:**

- The empty description of Greater Invisibility and the cut ones of Moonbeam and Conjure Celestial.
- The higher-level text and scaling rows of Chain Lightning and Dissonant Whispers.
- Wand of Web and Wand of Fear.
- The second damage type of every attack, as a type. Its dice follow only when the unmerged dice
  branch lands. The importer's swap of `extra_damage_type` into `damageType` then stops firing,
  which is harmless.

Until that release the same data can be read from the fixtures on `staging`.

**The SRD 5.2 document is needed as a second source for:**

- Legendary action uses per round: 30 monsters, all 3, and 4 in the lair for 27.
- XP in a lair, if it is wanted: 27 monsters.
- The text of Nature's Ward and the second sentence of Remarkable Athlete.
- The rarity and attunement line of the five items upstream files as mundane.

It is the same document under the same CC-BY-4.0 attribution the seed already carries. Open5e keeps
both PDFs in its repository under `data/raw_sources/srd_5_2/`.

**Hand correction, because no source or rule gives it:**

- The magic flag, rarity and attunement of Ioun Stone, Potion of Giant Strength, Potion of Healing,
  Potions of Healing and Spell Scroll.
- Splitting rows that bundle variants. The text is in the row; the split is authoring.
- Freedom of Movement: clear the save, damage and area fields.
- The damage roll of spells whose dice are not damage, as a list. No rule separates them cleanly.
- The six spells with a save in the text and an empty save field, if they are wanted.
- Black Pudding's reaction name. The two Circle of the Land features and Remarkable Athlete, with
  SRD text. Tsunami's stray heading. The 25 actions with markup left in them.
- Telekinesis stays cut. The published SRD ends mid-sentence, so there is no licensed text to add.

## 4. Other findings

- **`document__key` is ignored on two endpoints.** `/v2/items/` and `/v2/magicitems/` return every
  document's rows whatever `document__key` says (440 and 2,322). The importer already filters by
  document after fetching, so the seed is right. `/v2/magicitems/?document=srd-2024` returns the 760.
- **The seed's document is SRD 5.2; the published one is now 5.2.1.** Upstream lists 15 magic items
  that 5.2.1 added and Open5e lacks (Cloak of Invisibility, Potion of Vitality, Sending Stones and
  others), and Horn of Valhalla stored as Legendary where its rarity varies by horn.
- **The Spell Scroll row is the equipment entry** (cantrip and level 1). The magic item entry with
  its table of levels, save DCs and attack bonuses is not in Open5e.
- **Creatures printed inside a spell or item are not in Open5e:** Draconic Spirit, Giant Insect,
  Otherworldly Steed, Avatar of Death.
- **Melee-or-ranged attacks** are one attack row with both `reach` and `range` in SRD 5.2 (Lich,
  Eldritch Burst: `reach: 5`, `range: 120`). Upstream is deciding whether to split them into two
  rows; a split would change the count of attacks per action from one to two.
- **Fields the importer does not read and nobody asked about:** `limited_to_form` on actions
  ("Vampire Form Only", 10 distinct values), `order_in_statblock`, `initiative_bonus` on creatures,
  and `feature_type` on class features, which tells a class table column
  (`CLASS_TABLE_DATA`, `SPELL_SLOTS`, `CORE_TRAITS_TABLE`) from a real feature. The census's
  odd rows `cleric/core-traits` named Cleric Subclasses and `druid/wild-shape-uses` named Cantrips
  Known are such table rows with a wrong name upstream.
- **Spell `target_type` and `shape_type` are not worth reading yet.** `target_type` is `creature`
  for 338 of 339 spells, and Moonbeam and Conjure Celestial have no shape.

## 5. Not verified

- **The whole SRD was not compared with the whole of Open5e.** Only the entries the censuses named
  were read in the PDF, plus the counts of Recharge, per-day and legendary markers. Other cut or
  wrong texts may exist.
- **Counts taken from the PDF rest on extracted text.** A marker split across a line or column can
  be missed. The Recharge count was taken two ways and agrees; three Recharge markers and one per-day
  marker were matched by count only, not by name.
- **When Open5e will release `staging`** is not known. The fixes are merged there and nothing says
  when the live API will serve them.
- **The unmerged dice branch** was read for the seven wrong attacks, the count of second damage
  dice (127 of 128 filled) and the 33 missing rows, not as a whole.
- **Whether upstream knows about the three feature defects, Freedom of Movement and Black Pudding.**
  A search of its issues for those names found nothing. No issue was filed from here.
- **The census counts of 66 attacks and 24 spells** were not reproduced with the census scripts.
  This note counts 88 attacks across all actions, riders included, and 24 spells by its own match.
- **No parser was written.** "Can be parsed" means the text has one regular form in the rows read,
  not that a parser was run against all of them.
