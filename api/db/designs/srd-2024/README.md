# SRD 2024 Design files

One YAML file per SRD 5.2 entry states that entry's Effects in the engine's own Part kinds. The files
are read strictly: an unknown field, an entry the seed lacks or a Part a kind refuses stops the read.
They load at server start and through `grimoire import`, right after the snapshot, in one transaction,
and only when the hash over every file here has changed. ARCHITECTURE.md §9 describes the loader.

Paths name the entry: `spells/<slug>.yaml`, `conditions/<slug>.yaml`. Features, species traits, feats,
items, monsters and shared traits load once their link tickets land.

```yaml
entry: spells/fireball        # the file's own path, without .yaml
name: Fireball                # as the seed names it
automation: full              # or exception, listed in _exceptions.yaml
pattern: save-area-damage     # the component pattern the proof harness tests it under
effects:
  - activation:
      concentration: false
      duration: {unit: instant}          # unit, amount and repeat_save
    parts:
      - kind: area                       # a Part kind; its fields follow by name
        shape: sphere
        size_ft: 20
        range_ft: 150
      - kind: save_damage
        ability: dexterity
        dice: 8d6
        damage_type: fire
        half: true
        scaling:                         # how this Part's scalable fields grow
          - {field: dice, axis: slot_level, rule: linear, base: 3, per: 1d6}
    scaling: []                          # how the Activation's duration and concentration grow
```

A Part that holds others lists them under each group's name (`then`, `failure`, or a choice's modes).
An entry with several Effects gives each a `label`; each loads as `<slug>-<label>`.

`_exceptions.yaml` is the approved Manual exception list. Only the maintainer changes it. An entry
marked `exception` must be on it with the same `exception` parts, and a listed entry's file must be
marked `exception`. `features/_no-effect.yaml` and `items/_no-effect.yaml` list entries that need no
Effect, each with its reason: `table_column`, `choice`, `sheet_value` or `plain_gear`.
