# Gap review: Baldur's Gate 3 parity and homebrew expressiveness (1 October 2026)

A read-only review of the glossary, the decision records, the Go code and the canvas boards. Priorities: P1 before launch, P2 soon after, P3 nice to have.

## Code compared with the design

- Effects are a hard-coded map of 11 entries in `rules/effects/effects.go`. The homebrew effect builder needs data-driven effect definitions first.
- Items have no per-item instances (`container_items` is keyed by slug). Charges, attunement, equipped, identified and cursed states have nowhere to live.
- Reaction kinds are a fixed check of two (`opportunity_attack`, `shield`). Characters are still campaign-scoped, which contradicts ADR-0010.

## Baldur's Gate 3 parity

P1:
- All conditions, including 2024 exhaustion levels and stacking.
- The missing 2024 actions: Ready, Search, Study, Influence, Utilize, Magic, and Grapple or Shove as a save.
- Attacks left: Extra Attack and the Nick off-hand.
- Weapon Mastery.
- Reaction settings per character.
- A full level-up with multiclassing.
- Spell preparation and a spellbook.
- Rest flow and camp.
- Inventory (now on the canvas).
- Loot distribution.
- Traps and locks.
- The downed-and-stabilise flow.
- Heroic Inspiration.
- Summons.
- Wild Shape and Polymorph forms.

P2:
- Line of sight, cover and opportunity-attack warnings on paths.
- Elevation and high ground.
- Jump, climb and throw.
- A sneak mode with perception reach.
- A turn-based exploration toggle.
- Marching order.
- Group initiative.
- NPC attitude and Influence.
- Interactable map objects.
- Richer surfaces and clouds.
- Identifying items.
- Trading.
- Weapon sets.
- A game clock.
- Checkpoints.
- Journal and quests.
- A split party.
- Companions.
- Respec.
- Accessibility.

P3:
- Difficulty presets.
- Karmic dice.
- Showing DCs.
- Dyes.
- Gamepad.
- Coatings from surfaces.
- NPC approval.

## Homebrew the model cannot yet express

P1:
- Data-driven effects.
- Missing component kinds: temporary HP, teleport, forced movement, transform, walls, dispel, counter, grant feature, resources, choices, conditionals and durations.
- Wall and ring shapes, and moving emanations.
- Reaction-triggered casting.
- Ritual and spell metadata.
- A subclass builder.
- A class builder.
- A generic Resource.
- Scaling by class level.
- Choices and prerequisites.
- A species builder.
- Custom conditions.
- A monster builder with legendary and lair actions, recharge and phases.
- Rule variants as toggles.
- Roll tables.

P2:
- User-authored rule hooks.
- Lingering injuries.
- Custom tracks such as sanity or honour.
- Spell points.
- Firearms.
- Custom weapon properties and masteries.
- Item-instance states.
- Growing, sentient and hidden-property items.
- Crafting and downtime.
- Mounts.
- A background builder.
- Pinning a revision per campaign.
- Homebrew import and export.

P3:
- Vehicles.
- Mass combat.
- Hirelings.
- Custom damage and creature types.
- Turn-economy variants.
- Advancement variants.
- A balance check for spells.

## Content that must be original

- Battle Master is not in SRD 5.2; the boards now show the Champion.
- Do not use anything modelled on mind-flayer progression.
- Wild Magic, Beast Master, Artificer, and DMG-only variants such as spell points, madness, lingering injuries and Bastions: ship the mechanism with original examples only.
- Shared Library approval needs an IP check.
- Check that the 2024 encounter-budget and magic-item price tables are in SRD 5.2's Gameplay Toolbox before shipping them.

## Inconsistencies found

- ARCHITECTURE.md still describes forward-auth identity and Atlas migrations (superseded by ADR-0008, ADR-0009 and ADR-0005).
- It still describes a 5.1/5.2 ruleset blend.
- Some boards used 2014 rules: Ki, Deflect Missiles, Divine Smite, Healing Word, Inflict Wounds, Shove, potions and Spiritual Weapon. Fixed on the canvas on 1 October 2026.
- Level-up timing and XP-ledger wording differ between ARCHITECTURE.md and the glossary.
- The effect component vocabulary has four different versions (glossary, effect builder, ARCHITECTURE.md, code).
- Marsh Lantern differs between three boards.
- Surface lists differ between documents.
- "Aura" in the glossary versus "emanation" in SRD 5.2.
- The visibility qualities differ between the glossary and `docs/rules/visibility.md`.
- "Estimated Challenge" is used for a spell.
- "Board" is undefined.
- The proposal approval path is worded three ways.
- ARCHITECTURE.md's motion and parchment notes contradict ADR-0012 and the chosen style.
- Grid scale needs to be set per Map.
- Tamsin's HP and Fireball's owner were inconsistent; both are fixed.

## Suggested new glossary terms

Resource, Feature, Rest, Item instance (with Attunement), Rule Variant, Roll Table, Trap, Lock, Map Object, Form, Summon, Heroic Inspiration, Weapon Mastery, Track, Game Clock, Marching Order, Checkpoint, Quest.
