# Engine gaps: the four census results merged

Research for [#243](https://github.com/JorisJonkers-dev/grimoire/issues/243), part of the map in [#236](https://github.com/JorisJonkers-dev/grimoire/issues/236).

The four census documents (on the `research/census-*` branches) each named the Effect component kinds their entries lack, and each chose its own names. [`engine-gaps.py`](engine-gaps.py) maps those names onto one list by definition, groups the list into waves, and counts again across every entry.

- [`engine-gaps.tables.md`](engine-gaps.tables.md): every merged gap with the entries that need it per census and the census names it absorbs, then the entries with no gap left after each wave.
- [`engine-gaps.csv`](engine-gaps.csv): one line per census row with its merged gaps and the wave it becomes buildable in. The batch cut reads this.

## How to read the numbers

- 2,317 entries in 1,237 rows: 339 spells, 420 Features, 1,315 monster actions and traits (grouped), 243 item entries. 61 are proposed Manual exceptions and are left out of the counts, as the census documents proposed them; the later ruling on Notes moves some of those back in.
- An entry's wave is the latest wave among the gaps it needs.
- "No gap left" counts component kinds only. It does not count what the census documents call other gaps: a monster trait owning an Effect, the legendary economy, Feature Choices and Resources the schema lacks, wrong seed data. An entry in wave 0 with no missing kind still waits on Activation and on its owner link.
- `cast_spell` finishes an entry only once the spell it casts is itself automated.

## Limits

The mapping is a reading of four sets of definitions written by four sessions; a gap that is designed differently will move entries between neighbours such as `trigger`, `modifier` and `roll_rule`. Nothing here was run against the engine.
