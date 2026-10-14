# The automation backlog cut

Research for [#246](https://github.com/JorisJonkers-dev/grimoire/issues/246), part of the map in [#236](https://github.com/JorisJonkers-dev/grimoire/issues/236).

The scripts read the four census documents (the `research/census-*` branches) and the merged gap list (`docs/research/engine-gaps.py` on this branch), and cut the backlog:

1. `cut.py` gives every entry its merged gaps, pattern and weight: spells and Features one unit each, a monster stat block one unit per three actions or traits, an item entry one unit. It writes `units.json`.
2. `plan.py` lists the engine tickets with their blocking edges and packs entries into content batches of about 20 units, by kind, then wave, then pattern (monsters alphabetically within a wave, because a stat block mixes patterns). Shared monster traits get batches of their own that stat blocks wait on. It writes `plan.json`.
3. `render.py` turns the plan into issue titles and bodies (`issues.json`).
4. `publish.py` creates the milestones and issues, then the native blocked-by edges. It is resumable through `published.json` and `edges.json`.

Waves 0 and 1 and every engine ticket are published. The later batches stay in `plan.json` and are cut again, with the kind names as built, once wave 1 lands. `sequence` and `roll_rule` moved from wave 2 to wave 1 in this cut.
