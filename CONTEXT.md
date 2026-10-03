# Grimoire

A private D&D 5e table companion for the owner's own groups: reference compendium, campaign
knowledge base, fully automated rules, and a live, server-authoritative play runtime used on phones,
laptops and a shared screen.

## Language

### Table & people

**Account**:
A person's identity in Grimoire, signed in with a Username and password, through jorisjonkers.dev, or either once both are linked.
_Avoid_: User, profile, login

**Username**:
An Account's unique sign-in handle, chosen by its owner.
_Avoid_: Login, handle

**Nickname**:
The name an Account shows to others; a Member may override it within one Campaign.
_Avoid_: Display name, alias

**Access Token**:
A revocable credential an Account mints (or approves for an AI connector) so a tool can act as that Account, limited by scopes and an expiry.
_Avoid_: API key, PAT, MCP key

**Avatar**:
An Account's own picture, shown in the account menu and member lists.
_Avoid_: Portrait (that belongs to a Character), profile picture

**Admin**:
An Account that manages Grimoire itself rather than one Campaign, either through the jorisjonkers.dev admin role or because another Admin promoted it.
_Avoid_: Owner, superuser, DM

**Account Invite**:
A one-time link an Admin creates, and may email, so a new person can set up an Account, optionally joining a Campaign as a Player.
_Avoid_: Sign-up link, registration

**Campaign Invite**:
A link a DM creates so someone with an Account joins that Campaign as a Player.
_Avoid_: Invite (ambiguous), join code

**Friend**:
An Account linked to another by a mutual, accepted friend request; friends can see each other and start Conversations.
_Avoid_: Contact, follower, connection

**Conversation**:
Messages between Friends outside a live Session, one-to-one or a small group.
_Avoid_: DM (that is the Dungeon Master), Whisper (that is inside a Session), chat, inbox

**Campaign**:
One ongoing game with its own party, world, rule settings and history.
_Avoid_: Game, world, adventure

**DM**:
The member of a Campaign who runs it and sees the truth.
_Avoid_: GM, Admin (that is site-wide), owner

**Player**:
A member of a Campaign who plays one or more Characters.
_Avoid_: User, participant

**Member**:
An Account's participation in a Campaign, as DM or Player.
_Avoid_: Account (the person, not the participation), user

**Character**:
A hero owned by an Account: identity, Portrait, build and Backstory, playable in several Campaigns.
_Avoid_: PC, hero, sheet (the sheet is a view)

**Campaign Character**:
A Character's progress in one Campaign: level, hit points, inventory, spells and history there.
_Avoid_: Character copy, run, instance

**Backstory**:
A Character's history, which may name Locations and NPCs the Player drafts and the DM can adopt into a Campaign.
_Avoid_: Bio, lore

**Join Request**:
A Player's request to bring a Character into a Campaign; on approval the DM's starting level and allowed Collections shape the new Campaign Character.
_Avoid_: Application, enrolment

**Advancement**:
How a Campaign's Characters level up, chosen per Campaign: by XP or by Milestone (the DM grants levels).
_Avoid_: Progression, levelling mode

**XP Award**:
Experience given to Campaign Characters, either split evenly among a Combat's participants for defeated creatures or granted by the DM for anything else.
_Avoid_: Experience drop, loot XP

**Level-up**:
Raising a Campaign Character one level through the guided wizard, unlocked at the next long rest after reaching the XP threshold (or when the DM grants it); the DM can hold it and is told when it becomes available.
_Avoid_: Ding, promotion

**Feature**:
Something a class, subclass, species, background, feat or item gives a character at a level: uses, a Resource, choices, prerequisites and the Effects it applies.
_Avoid_: Ability, perk, trait (except for species traits in rules text)

**Resource**:
A named pool a Feature spends and refills, with a maximum by level and a recharge (short rest, long rest, dawn, rolling initiative, or a roll), such as Focus Points, Rage or Sorcery Points.
_Avoid_: Points, charges (reserved for items), mana

**Weapon Mastery**:
A weapon's mastery property (Cleave, Graze, Nick, Push, Sap, Slow, Topple, Vex) that a character can use once they have mastered that kind of weapon.
_Avoid_: Weapon action, weapon skill

**Heroic Inspiration**:
A reroll a character holds and spends on one die, granted by the DM or by features, and passable to an ally.
_Avoid_: Inspiration point, hero point

**Rest**:
A short or long break the party agrees to take. A short rest spends Hit Dice; a long rest can be interrupted and, when completed, refills Resources and Charges, unlocks Level-up and can trigger an Encounter Check.
_Avoid_: Camp (one way to take a long rest), sleep, downtime

**Form**:
A stat block a Combatant takes on over its own, as with Wild Shape or Polymorph, with its own hit points and actions until it reverts.
_Avoid_: Shape, transformation, polymorph (as the model term)

**Summon**:
A creature an Effect brings in under its caster's control, acting on the caster's turn or its own and ending when the Effect ends.
_Avoid_: Minion, pet, conjuration (as the model term)

**Controller**:
The account currently driving a Combatant; normally its owner, reassignable by the DM.
_Avoid_: Driver, operator

**Table Display**:
The shared screen (usually a TV) showing the party view, steered remotely by the DM.
_Avoid_: Table view, TV mode, second screen

**Portrait**:
A Character's picture, uploaded by its Player.
_Avoid_: Avatar (that belongs to an Account), profile picture

**Token Icon**:
The image drawn inside a Character's Token, cropped from the Portrait or uploaded separately.
_Avoid_: Avatar, token image

**Notification**:
A message to one Account about something that needs their attention or happened to them, shown in the account bar, on the Dashboard and per Campaign, and optionally pushed or emailed per kind.
_Avoid_: Alert, message, toast (a toast is transient UI)

**Dashboard**:
An Account's home page: what needs attention, upcoming Sessions, their Characters and Campaigns.
_Avoid_: Home, overview, landing page

**Digest**:
One email that gathers an Account's unread Notifications and messages since the last one, sent on the schedule the Account chooses.
_Avoid_: Newsletter, summary mail

**Release Note**:
What changed in one minor or major Grimoire release, drafted from the release and published by an Admin; each Account sees it once.
_Avoid_: Changelog (the raw list), announcement, patch notes

### Play

**Session**:
One evening of play, numbered and dated, moving through planned, live, paused and ended, with a recap.
_Avoid_: Game night, play session, live session

**Encounter**:
A prepared fight: which creatures, how many, and its difficulty budget.
_Avoid_: Battle, fight template

**Exploration**:
The mode of a live Session with no noticed hostile creatures: no initiative, everyone moves freely.
_Avoid_: Free roam, out of combat

**Suggested Action**:
The action and target the rules propose for a creature on its turn, from its tactics.
_Avoid_: AI move, auto-attack

**Tactics**:
How a creature picks Suggested Actions: Simple, Cunning or Off, derived from Intelligence unless overridden.
_Avoid_: AI level, behaviour

**Combat**:
A running fight inside a live Session, with rounds, initiative and turns.
_Avoid_: Encounter (for the running instance), battle

**Combatant**:
Any creature taking part in a Combat, whether Character, NPC or monster.
_Avoid_: Actor, unit, token

**Checkpoint**:
A named point in a live Session's Action Log the DM can rewind to.
_Avoid_: Save, snapshot

**Group**:
A part of a split party that plays on a map of its own, in a Session of its own, with its own Party Vision. The DM sees every Group; the Table Display follows one.
_Avoid_: Sub-party, team, squad

**Token**:
The marker that shows a Combatant or object on a Map.
_Avoid_: Piece, figure

**Reaction Prompt**:
A timed request to a Controller to use a reaction before the triggering action resolves.
_Avoid_: Interrupt, popup

**Roll Request**:
What the rules need rolled for one purpose: the exact dice, the reason, and every modifier source.
_Avoid_: Dice request, check request

**Dice Set**:
The look an Account gives its dice, per die type: a preset pattern or an uploaded image placed on the die, with numbers on their own layer; a d100 is a ball die or a percentile pair. Its owner may share it with Friends or with everyone; others then use a copy they cannot edit, which they keep if sharing stops. A set with an uploaded image is checked before everyone can see it.
_Avoid_: Dice skin, dice theme

**Roll Card**:
The on-screen form of a Roll Request, filled in from physical dice or auto-rolled with a tap.
_Avoid_: Dice dialog, roll popup

**Effect**:
Anything that changes what a creature can do or what happens to it, described as typed components: damage, healing, save, condition, modifier, light, reveal, sense, movement, surface, summon, trigger. Each component says when it fires (on cast, on entering, at the start or end of a turn) and whom it touches within its Targeting.
_Avoid_: Buff, debuff, status (as model terms)

**Targeting**:
What an Effect can reach: self, one or several creatures, or an area (sphere, cone, line, cube, cylinder, emanation, wall, ring) at a range, filtered by side (anyone, allies only, enemies only, creatures of the caster's choice, objects) and by creature type, with or without needing sight.
_Avoid_: Range type, AoE (as the model term)

**Visibility Quality**:
Something that keeps a creature or object from being seen for what it is: Hidden, Invisible, Disguised, Illusory, Ethereal, Obscured or Secret. Each has a check or a Sense that sees through it.
_Avoid_: Stealth state, cloak, camouflage (as the model term)

**Sense**:
A way of perceiving with a range: sight, darkvision, blindsight, tremorsense or truesight. Senses decide which Visibility Qualities a character pierces, and so what reaches Party Vision.
_Avoid_: Vision type, perception mode

**Disguised**:
A Visibility Quality where a creature or object shows a false identity; truesight, a successful Insight or Investigation check, or a reveal Effect shows the true one.
_Avoid_: Hidden identity, shapechanged (as the model term)

**Reveal**:
An Effect component that strips chosen Visibility Qualities inside its area, such as outlining invisible creatures or ending Hidden.
_Avoid_: Detect, expose, dispel (as the model term)

**Scaling**:
How an Effect grows with the spell slot it is cast with or the caster's level: more dice, targets, duration or area.
_Avoid_: Upcasting (the act, not the data), higher-level text

**Material Component**:
A specific item a spell needs, with its cost and whether casting consumes it.
_Avoid_: Reagent, ingredient

**Manual Effect**:
The part of an Effect the rules cannot compute yet, resolved by the DM from a prompt.
_Avoid_: Unsupported effect, custom effect

**Action Log**:
The append-only, ordered record of every change in a live Session, used for undo and the combat log.
_Avoid_: Journal, event log, history

**Journal**:
A Player's own record of Quests, Lore and notes, fed from Boards and the KB.
_Avoid_: Action Log, quest log, diary

**Handout**:
An image or text the DM pushes to selected Players or the Table.
_Avoid_: Note, card

**Whisper**:
A chat message inside a live Session, visible only to its chosen recipients.
_Avoid_: DM, private message

**Party Stash**:
Items and currency held by the party as a whole rather than by one Character.
_Avoid_: Shared inventory, bank

**Item Instance**:
One particular item a Character, the Party Stash, a Shop or a loot pile holds, with its own name, Charges, attunement, identified state and equipped slot.
_Avoid_: Item stack, inventory row

**Attunement**:
The bond a character forms with an Item Instance that requires it; a character holds at most three.
_Avoid_: Binding, equip (a separate state)

**Quest**:
A goal the party is pursuing, with steps and a status, shown in the Journal.
_Avoid_: Mission, task, objective

**Lore**:
World knowledge the party has unlocked, for example by reading a book or letter found in play.
_Avoid_: Codex, wiki

**Board**:
A planning board of lanes and cards, linked into the KB, that the DM uses for quests and threads.
_Avoid_: Kanban (as the user-facing name), Map (a board is never a map)

**Travel Leg**:
One movement of the party along a route on a world Map, which may trigger an Encounter Check.
_Avoid_: Journey, trip

### Prep & random encounters

**Session Prep**:
Prep the DM attaches to one upcoming Session; whatever goes unused carries over to the next.
_Avoid_: Session notes, plan

**Campaign Prep**:
Prep not tied to a Session (villains, factions, secrets, Locations) that any Session can draw on.
_Avoid_: General notes, world bible

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

**Location**:
Any named place: a village, a temple, a forest, a homeland. Maps, Backstories and Settlements refer to Locations.
_Avoid_: Place, landmark, Region (that is an Encounter-Table area)

**Settlement**:
An inhabited Location on a world Map, with a size and a wealth tier.
_Avoid_: Town, city, village (as distinct concepts)

**Shop**:
A trader in a Settlement with a type, an owner NPC, and Stock.
_Avoid_: Store, merchant (for the place)

**Price Guide**:
Reference prices: SRD list prices, typical ranges per rarity, settlement-wealth markup, and what suits each party level.
_Avoid_: Price list, value table

**Game Clock**:
The Campaign's in-world date and time, advanced by travel, rests and downtime, that decides "dawn", hour-long durations and restocks.
_Avoid_: Calendar, timeline

**Marching Order**:
The order the party travels and explores in, used for who meets a trap or an ambush first.
_Avoid_: Formation, party order

**Rule Variant**:
An optional or house rule a DM switches on for a Campaign, such as flanking or critical fumbles, built in or authored from hook points that apply Effects or Roll Tables.
_Avoid_: House rule (as the model term), setting, option

**Roll Table**:
A Library table of dice ranges whose results can apply Effects or give Items, used for fumbles, injuries, surges, madness, trinkets, weather and rumours.
_Avoid_: Random table, d100 table

**Track**:
A Campaign-specific score such as sanity, stress, honour or renown, with thresholds that trigger Effects or Roll Tables.
_Avoid_: Meter, counter, stat

**Faction**:
An organisation NPCs, Shops and Locations can belong to, with goals, territory and a Standing towards the party. Grimoire ships Faction Archetypes and the Default World's own Factions; the named factions of published settings are not included.
_Avoid_: Guild, organisation, group (as the general term)

**Faction Archetype**:
A generic Faction template a DM copies and names, such as a thieves' guild, city watch, temple, merchant league, druid circle, arcane college, noble house, mercenary company, knightly order, cult or smuggling ring.
_Avoid_: Preset faction, template faction

**Standing**:
How a Faction regards the party, in five tiers from Hostile through Unfriendly, Neutral and Friendly to Allied. A character may carry a Personal Standing that overrides the party's. Standing shapes social checks against members, prices in member Shops, NPC first reactions and Encounter Table weights in the Faction's territory.
_Avoid_: Reputation, renown, enmity, faction score

**Personal Standing**:
A character's own Standing with a Faction, used instead of the party's for that character.
_Avoid_: Individual reputation, grudge

**Standing Change**:
A move up or down in Standing with a reason. Grimoire suggests one after events such as killing a member or finishing a job; the DM confirms or edits it and chooses whether Players see the reason.
_Avoid_: Reputation event, faction XP

**Stock**:
The items and quantities a Shop currently sells, generated from Loot Tables and replenished on restock.
_Avoid_: Inventory (reserved for what a Character carries)

**Library**:
An Account's own store of reusable creatures, NPCs, Homebrew, Factions, prep tables, Roll Tables, places and maps, usable in any Campaign its owner runs.
_Avoid_: Collection (a grouping inside a Library), vault, catalogue

**Shared Library**:
The site-wide Library every DM can use; DMs submit entries and an Admin approves them.
_Avoid_: Global library, public library

**Collection**:
A named group of Library entries, such as a homebrew expansion, that a DM switches on per Campaign.
_Avoid_: Pack, bundle, module

**Campaign Collection**:
The Collection every Campaign has for entries made or approved just for it, such as accepted Proposals.
_Avoid_: Campaign homebrew, local rules

**Proposal**:
A Player's request that the DM accept a new or changed entry (Homebrew, a Location, an NPC) into a Campaign; the DM approves, asks for changes, or declines. An approved Proposal is copied once into the DM's Library and then linked into the Campaign Collection.
_Avoid_: Suggestion (reserved for Suggested Action), request, submission

**Homebrew**:
Rules content that is not from the SRD (a spell, item, species, background, feat, class or condition), kept in a Library.
_Avoid_: Custom content, house rules (those are rule settings)

**Item Property**:
A typed part of an item: an enchantment bonus, extra damage, a resistance, a granted cantrip or spell, a Skill Boost, a sense, a speed, a trigger, a set bonus, a curse. Homebrew items are built from Item Properties on top of a base item, the way spells are built from Effects.
_Avoid_: Enchantment (one kind of property), affix, perk

**Skill Boost**:
A one-step Item Property on a skill: advantage, +1d4, a flat bonus, proficiency or expertise.
_Avoid_: Skill buff, skill cantrip

**Charges**:
Uses an item holds, spent by its granted spells and actions and regained on a schedule (dawn, a long rest, a short rest, or a rolled number at dawn).
_Avoid_: Uses (as the model term), ammo

**Price Check**:
Grimoire comparing a homebrew item's properties with its rarity and the Price Guide, and saying whether the rarity and value fit.
_Avoid_: Balance score, power rating

**Campaign Override**:
Campaign-specific state layered on a linked Library entry (HP, location, attitude, death) while the base stays shared.
_Avoid_: Local copy, fork

**Estimated Challenge**:
The challenge rating and XP Grimoire proposes for a Homebrew creature from its defences and offence, which the author may accept or override.
_Avoid_: Suggested CR (Suggested is reserved), auto CR

**Public Compendium**:
The read-only Compendium anyone can browse without signing in: SRD entries plus reference guides such as the Price Guide, loot by party level, spell lists and weapon and attack tables; never Homebrew or Campaign data.
_Avoid_: Guest mode, open wiki, suggestions (reserved for Suggested Action)

**Automation Level**:
How much of an entry's behaviour the rules compute: full, partial, or manual.
_Avoid_: Support level, coverage

**Revision**:
A recorded version of a piece of prep data, with its author, that the DM can compare and restore.
_Avoid_: Version, history entry

### Space

**Map**:
An image with a calibrated hex grid; either a world map or a local tactical map. A local Map's hexes are always 5 feet across; a world Map has its own scale in miles to a cell, and its grid can be drawn as hexes, as squares or not at all.
_Avoid_: Board (reserved), battlemap

**Default World**:
The original, SRD-safe setting that ships with Grimoire: a world Map with towns and cities, each with local Maps and named Locations, ready to reuse.
_Avoid_: Standard world, official setting, Forgotten Realms

**Found Map**:
A Map the party has obtained in play; Players see only Found Maps, and a Found local Map also shows its outline on the world Map.
_Avoid_: Unlocked map, discovered map (Fog covers what was seen)

**Fog**:
The server-enforced split between what exists on a Map and what the party perceives: never seen, remembered, or visible now.
_Avoid_: Hidden layer, mask

**Party Vision**:
Everything any party member can perceive right now, shared by every Player and the Table Display.
_Avoid_: Line of sight (for the combined set), team vision

**Encounter Zone**:
An area on a local Map holding hidden creatures that springs when the party comes within range or the DM triggers it.
_Avoid_: Trap zone, trigger area

**Surprised**:
Unaware of the threat when Combat starts, so initiative is rolled at disadvantage.
_Avoid_: Ambushed, caught off guard

**Surface**:
A hex-level terrain effect such as fire, grease, water, ice or web that the rules engine applies.
_Avoid_: Hazard, terrain effect

**Map Object**:
An interactable thing on a local Map, such as a door, lever, chest, barrel or curtain, with its own armour class and hit points.
_Avoid_: Prop, interactable, entity

**Trap**:
A hidden Map Object with a trigger area, a detection DC, a way to disarm it and an Effect.
_Avoid_: Hazard (a Surface), snare

**Lock**:
What keeps a door or container shut: a DC, an optional key, and ways through such as thieves' tools, Knock or breaking it.
_Avoid_: Seal, latch
