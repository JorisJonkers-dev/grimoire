# Grimoire

A private D&D 5e table companion for the owner's own groups: reference compendium, campaign
knowledge base, fully automated rules, and a live, server-authoritative play runtime used on phones,
laptops and a shared screen.

## Language

### Table & people

**Campaign**:
One ongoing game with its own party, world, rule settings and history.
_Avoid_: Game, world, adventure

**DM**:
The member of a Campaign who runs it and sees the truth.
_Avoid_: GM, admin, owner

**Player**:
A member of a Campaign who plays one or more Characters.
_Avoid_: User, participant

**Member**:
An estate account's participation in a Campaign, as DM or Player.
_Avoid_: Account, user

**Controller**:
The account currently driving a Combatant; normally its owner, reassignable by the DM.
_Avoid_: Driver, operator

### Play

**Session**:
One evening of play, numbered and dated, moving through planned, live, paused and ended, with a recap.
_Avoid_: Game night, play session, live session

**Encounter**:
A prepared fight: which creatures, how many, and its difficulty budget.
_Avoid_: Battle, fight template

**Combat**:
A running fight inside a live Session, with rounds, initiative and turns.
_Avoid_: Encounter (for the running instance), battle

**Combatant**:
Any creature taking part in a Combat, whether Character, NPC or monster.
_Avoid_: Actor, unit, token

**Token**:
The marker that shows a Combatant or object on a Map.
_Avoid_: Piece, figure

**Reaction Prompt**:
A timed request to a Controller to use a reaction before the triggering action resolves.
_Avoid_: Interrupt, popup

**Roll Request**:
What the rules need rolled for one purpose: the exact dice, the reason, and every modifier source.
_Avoid_: Dice request, check request

**Roll Card**:
The on-screen form of a Roll Request, filled in from physical dice or auto-rolled with a tap.
_Avoid_: Dice dialog, roll popup

**Effect**:
Anything that changes what a creature can do or what happens to it, described as typed components (damage, save, condition, modifier, area, surface, trigger).
_Avoid_: Buff, debuff, status (as model terms)

**Manual Effect**:
The part of an Effect the rules cannot compute yet, resolved by the DM from a prompt.
_Avoid_: Unsupported effect, custom effect

**Action Log**:
The append-only, ordered record of every change in a live Session, used for undo and the combat log.
_Avoid_: Journal, event log, history

**Journal**:
A Player's own record of quests and notes, fed from boards and the KB.
_Avoid_: Action Log, quest log, diary

**Handout**:
An image or text the DM pushes to selected Players or the Table.
_Avoid_: Note, card

**Whisper**:
A chat message visible only to its chosen recipients.
_Avoid_: DM, private message

**Party Stash**:
Items and currency held by the party as a whole rather than by one Character.
_Avoid_: Shared inventory, bank

**Travel Leg**:
One movement of the party along a route on a world Map, which may trigger an Encounter Check.
_Avoid_: Journey, trip

### Prep & random encounters

**Region**:
An area of a world Map with its own Encounter Table.
_Avoid_: Zone, biome, area

**Encounter Table**:
A per-Region chance of an encounter plus weighted entries, each a prepared Encounter, a Pool draw, or Nothing.
_Avoid_: Random table, wandering monster table

**Encounter Pool**:
A weighted set of creatures with tags and a level band, from which the generator builds an Encounter for the party's level.
_Avoid_: Monster list, spawn pool

**Loot Table**:
Weighted items and currency by CR tier or tag, attached to creatures, Pools or Encounters.
_Avoid_: Drop table, treasure list

**Encounter Check**:
One roll against an Encounter Table, triggered by a rest, a travel leg, or the DM.
_Avoid_: Random encounter roll, wandering check

**Settlement**:
A named inhabited place on a world Map, with a size and a wealth tier.
_Avoid_: Town, city, village (as distinct concepts)

**Shop**:
A trader in a Settlement with a type, an owner NPC, and Stock.
_Avoid_: Store, merchant (for the place)

**Stock**:
The items and quantities a Shop currently sells, generated from Loot Tables and replenished on restock.
_Avoid_: Inventory (reserved for what a Character carries)

**Automation Level**:
How much of an entry's behaviour the rules compute: full, partial, or manual.
_Avoid_: Support level, coverage

**Revision**:
A recorded version of a piece of prep data, with its author, that the DM can compare and restore.
_Avoid_: Version, history entry

### Space

**Map**:
An image with a calibrated hex grid; either a world map or a local tactical map.
_Avoid_: Board (reserved), battlemap

**Fog**:
The server-enforced split between what exists on a Map and what the party has discovered.
_Avoid_: Hidden layer, mask

**Surface**:
A hex-level terrain effect such as fire, grease, water, ice or web that the rules engine applies.
_Avoid_: Hazard, terrain effect
