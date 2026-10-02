---
title: Grimoire v2 — accounts, Library and Homebrew, full rules automation, redesigned play
status: ready-for-agent
date: 2026-10-01
sources: CONTEXT.md, docs/adr/0001–0013, docs/rules/visibility.md, docs/reviews/2026-10-01-bg3-and-homebrew-gaps.md, the Claude Design canvas "Grimoire UI mockups"
---

# Grimoire v2

## Problem Statement

Grimoire v1 runs a single live Session well, but almost everything around it is missing or thin.

**Accounts and characters**
- Players can only reach Grimoire through one external sign-in. They have no Account of their own, no Friends and no way to talk outside a Session.
- Characters belong to one Campaign, so a player cannot take Tamsin from one table to another.

**Homebrew and the rules engine**
- DMs cannot build anything reusable. Every creature, Location, Shop and spell has to be remade per Campaign.
- Players cannot suggest Homebrew at all.
- The rules engine automates a small, hard-coded set of spells and two kinds of reaction. Anything a table actually homebrews cannot be expressed: a lantern spell that reveals the invisible, a bow that grants a cantrip, a subclass, a monster with legendary actions, a lingering-injury rule.

**Missing 5e and 2024 features**
- Core 5e and 2024 features are absent or partial: most conditions and exhaustion, the 2024 actions, Weapon Mastery, Extra Attack, rests and camp, level-up beyond level 1, multiclassing, spell preparation, inventory with individual items, attunement, traps and locks, summons and Wild Shape.
- Players who know Baldur's Gate 3 expect those features, along with its conveniences: an editable action bar, a clear turn order, confirm-before-move, reaction settings, and loot and inventory screens.

**Look, maps and communication**
- The visual design is inconsistent between pages, and parts read as "booklike" and heavy.
- Maps beyond the battle grid are not supported: there is no world map with fog for what the party has found, and no travel estimates.
- There is no way to tell players about new releases, to notify them, or to email them.

## Solution

Grimoire v2 delivers a full redesign plus the systems needed to run a long campaign with heavy homebrew. Everything stays on SRD 5.2 rules text, with original content or user-supplied content for anything else.

**Accounts and social**
- Grimoire owns its own Accounts. Admins invite people; jorisjonkers.dev OIDC can be linked for those who hold the Grimoire permission (ADR-0008).
- Accounts carry Friends, Conversations, Notifications (in app, push, email), Access Tokens for MCP (ADR-0009), Dice Sets and preferences.

**Characters**
- Characters belong to Accounts and carry per-Campaign progress (ADR-0010).
- The character sheet is semi-recognisable as the 5e sheet.
- Creation and level-up wizards in the style of BG3 cover multiclassing, spells, feats and Weapon Mastery.
- Inventory has equipment slots, individual item instances, attunement, weight and a Party Stash.

**Library and Homebrew**
- Each DM has a Library, and there is a Shared Library on top, with Collections and Campaign Overrides (ADR-0011).
- Homebrew builders exist for spells, items, subclasses, classes, species, backgrounds, feats, monsters, conditions, Resources, Roll Tables and Rule Variants. All of them are built from data-driven Effects and Item Properties the rules engine executes.
- Players suggest Homebrew through Proposals, which the DM reviews.

**Rules engine: full automation from data**
- The rules engine executes Effects, Item Properties, Features and Resources stored as data, with a Manual fallback the DM resolves.
- Complete SRD 5.2 coverage:
  - conditions and exhaustion;
  - 2024 actions and Weapon Mastery;
  - rests, Hit Dice, Heroic Inspiration;
  - summons, Forms, traps, locks and map objects;
  - Visibility Qualities against Senses;
  - Faction Standing feeding social checks, prices, reactions and encounters (ADR-0013).

**Live play, redesigned**
- The hex map is always underneath. See-through compact panels sit on top.
- One roster is shared by party, DM and TV. Initiative is revealed with a slide-in, followed by an "It's your turn" banner.
- Every player can edit their own action bar. Every move is confirmed.
- Areas of effect are drawn as tinted hexes.
- Phones get swipeable pages over a full-bleed bottom bar, and pinch zoom.
- The DM sees and controls more through the same components.
- Dice are GPU 3D with server results (ADR-0012).

**Maps and the world**
- World maps are pictures — the painted Default World or an upload — under a calibrated, see-through hex grid that gives travel estimates.
- Fog follows Found Maps: dimmed where the party has not been, dark without the world map.

**Communication**
- Release Notes, a Digest, emails with one consistent template, and a Public Compendium with guides.

## User Stories

### Accounts, sign-in and administration

1. As a new person, I want to set up an Account from an Admin's invite link, so that I can join Grimoire without an external identity.
2. As a person with a jorisjonkers.dev login and the Grimoire permission, I want to sign in with it, so that I don't need another password.
3. As an Account holder, I want to link and unlink my jorisjonkers.dev login from my Account, in either direction, so that I can use whichever sign-in suits me.
4. As an Account holder, I want my jorisjonkers.dev-provided fields shown read-only while my Username and Nickname stay editable, so that I know which data comes from where.
5. As an Account holder, I want optional two-step sign-in with an authenticator app and recovery codes, so that my Account is protected.
6. As an Admin, I want two-step sign-in to be required for me, so that admin powers are protected.
7. As an Account holder, I want to enter a one-time code in a single numeric field that my phone can autofill, so that two-step sign-in is quick.
8. As an Account holder who forgot my password, I want an email sign-in link, so that I can recover my Account.
9. As an Account holder, I want an email when someone signs in from a new device, so that I notice misuse.
10. As an Account holder, I want to see and revoke my signed-in devices, so that I can sign out a lost phone.
11. As an Account holder, I want to mint Access Tokens with scopes and expiry, so that MCP tools and scripts can work for me without my password.
12. As an Account holder, I want to revoke an Access Token and see when it was last used, so that I stay in control.
13. As an Admin, I want one list of all Accounts, including invited ones that have not yet activated, so that I see everyone in one place.
14. As an Admin, I want a page per Account with sign-in methods, Campaigns, history and controls (send sign-in link, make Admin, disable), so that I can manage one person in depth.
15. As an Admin, I want to create a new invite that is closed by default and expires after a chosen time, so that invites don't linger.
16. As an Admin, I want to promote another Account to Admin, so that I can share the work.
17. As an Admin, I want a read-only, audited view into any Campaign, so that I can help without changing anything.
18. As an Admin, I want to review the Shared Library, so that only safe and licensed content is shared.
19. As an Admin, I want to write one Release Note per full release, drafted from merged features, so that people learn what's new.
20. As an Account holder, I want a profile with Nickname, Avatar and a per-Campaign display-name override, so that each table calls me what fits.

### Friends, Conversations and Notifications

21. As an Account holder, I want to send, accept and decline Friend requests, so that I can play with people I know.
22. As a Friend, I want 1:1 and group Conversations, so that we can plan outside Sessions.
23. As a Friend, I want to mention a Character, Proposal or Location in a Conversation, so that we can discuss game content directly.
24. As an Account holder, I want Notifications in app, by push and by email, per kind, so that I control interruptions.
25. As an Account holder, I want emails bundled into a Digest at most once an hour, except security emails, so that I'm not flooded.
26. As an Account holder, I want a bell with unread Notifications and an action on each, such as Review, so that I can act straight away.
27. As a DM, I want a Notification when a player submits a Proposal or a Join Request, so that I can respond.
28. As a player, I want a Notification when my character can level up, so that I don't miss it.
29. As an Account holder, I want a Release Note shown once per release on my Dashboard, so that I learn about new features.

### Dashboard and search

30. As an Account holder, I want a Dashboard listing what needs me before the next Session, so that I can prepare.
31. As an Account holder, I want to join a live Session from the Dashboard in one click, so that I'm at the table fast.
32. As an Account holder, I want universal search with previews of results across the compendium, Library, Campaigns and people, so that I find things without navigating.

### Campaigns

33. As a DM, I want to create a Campaign with rules (ruleset SRD 5.2, ability method, starting level, advancement by XP or Milestone, Rule Variants), so that the table plays my way.
34. As a DM, I want to invite players to a Campaign and approve Join Requests with a starting level, so that I control the table.
35. As a DM, I want Campaign Prep and Session Prep with things carried over between Sessions, so that nothing falls through.
36. As a DM, I want to schedule Sessions with reminders, so that players show up.
37. As a DM, I want encounter tables, pools, loot tables and encounter checks per Region, so that random events are fair and quick.
38. As a DM, I want Settlements, Shops with Stock, a Price Guide and restocking, so that commerce works.
39. As a DM, I want Factions with Standing per party and Personal Standing per character, so that the world reacts to the party.
40. As a DM, I want Grimoire to suggest Standing Changes from events, which I confirm, edit or dismiss, choosing whether players see the reason, so that I stay in charge.
41. As a player, I want to see the Standing tier of each Faction I've met, and the reasons my DM shared, so that I understand the world's reactions.
42. As a DM, I want to start Factions from a catalogue of generic archetypes (thieves' guild, city watch and so on), so that I'm fast.
43. As a DM, I want a game clock (date, time of day), so that "dawn", "1 hour" and travel time are computed.
44. As a DM, I want marching order and formation, so that travel and ambush targeting know who is in front.
45. As a DM, I want Quests with steps and status in the Journal, so that the party remembers what it's doing.
46. As a player, I want Lore entries unlocked by reading books and letters found in play, so that the world builds up.

### Characters

47. As a player, I want my Characters to belong to my Account and join several Campaigns, each with its own progress, so that one character can travel.
48. As a player, I want a creation wizard (Species, Class, Background, Ability scores, Skills and choices, Equipment, Appearance, Name and story, Review) that saves drafts, so that I can build carefully.
49. As a player, I want class tiles with details (hit die, main ability, saves, skills, armour, weapons, Weapon Mastery, what I get by my starting level), so that I can choose well.
50. As a player, I want point buy, standard array or rolling, as the DM allows, with the background's ability increases, so that my scores follow the rules.
51. As a player, I want the class's main ability highlighted and suggestions for my build, so that a new player doesn't build a weak character.
52. As a player, I want a character sheet laid out like the familiar 5e sheet (vitals, abilities with saves, skills with proficiency and expertise markers, attacks, features and traits, proficiencies), so that I recognise it.
53. As a player, I want to switch the sheet between my Campaigns, so that I see the right level and items.
54. As a player, I want to apply damage, healing and temporary HP from the sheet, so that bookkeeping is quick.
55. As a player, I want a Backstory with mentions of Locations and NPCs, which I can propose to a Campaign, so that the DM can weave it in.
56. As a player, I want a Portrait and a Token Icon, so that I'm recognisable on the map.
57. As a player, I want a level-up wizard covering multiclass prerequisites, subclass, ASI or feat, Fighting Style, Expertise, spells and cantrips learned or swapped, and HP, so that levelling is complete.
58. As a player, I want level-up to unlock at the next long rest, unless my DM holds it, so that it matches the table's pace.
59. As a player, I want to retrain my build with DM approval, keeping history, so that I can fix choices I regret.
60. As a player, I want Heroic Inspiration I can spend on a reroll or pass to an ally, so that I can use the 2024 rule.

### Inventory and items

61. As a player, I want equipment slots around my character (head, cloak, body, hands, feet, amulet, two rings, main hand, off hand, ranged, ammunition, instrument), so that I see what I wear.
62. As a player, I want a bag of individual items with filters, sorting, weight and carrying capacity, so that I manage gear like in BG3.
63. As a player, I want each item to be its own instance, with charges, attunement, an identified state and a custom name, so that my bow isn't just "a longbow".
64. As a player, I want at most three attuned items, with attunement requirements enforced, so that the rules hold.
65. As a player, I want to drag items between my bag, slots, other characters and the Party Stash, so that sharing is easy.
66. As a player, I want item cards with use actions (Drink, Give, Throw), so that I act from the inventory.
67. As a player, I want weapon sets with a quick swap, so that switching from bow to sword is one tap.
68. As a party, we want loot piles from fights and chests, with claim, need or greed, or DM assignment, and an even coin split, so that loot doesn't stall the table.
69. As a player, I want unidentified items until Identify, a short rest of study or a DM reveal, so that loot keeps its mystery.
70. As a player, I want a two-sided trade view with shopkeepers and other players, so that bartering is clear.
71. As a player, I want shop prices to reflect my Faction Standing and haggling, so that relationships matter.

### Library, Collections, Homebrew and Proposals

72. As a DM, I want my own Library of creatures, NPCs, Locations, Shops, items, spells, tables and more, linked into my Campaigns, so that I build once and reuse.
73. As a DM, I want Campaign Overrides on linked Library entries, so that one Campaign can differ without forking.
74. As a DM, I want to pin a Library entry's Revision per Campaign, so that edits don't change a Campaign mid-session.
75. As a DM, I want Collections (for example "Feywild expansion"), so that I can switch sets of Homebrew on per Campaign.
76. As a DM, I want to submit Library entries to the Shared Library, approved by an Admin with an IP check, so that good content spreads safely.
77. As a player, I want to propose Homebrew (spell, item, species, background, feat, subclass and more) to my DM, so that my ideas reach the table.
78. As a DM, I want to review a Proposal as a side-by-side diff, edit it, approve it into my Library and Campaign Collection, ask for changes or decline it, so that I stay in control.
79. As an author, I want rules text generated from structured parts, so that Homebrew reads like the SRD.
80. As an author, I want an Estimated Challenge for creatures and a Price Check for items, so that I avoid broken content.
81. As an author, I want import and export of Homebrew as JSON in Grimoire's own schema, so that I can back up and share.

### The effect builder (spells, features, abilities)

82. As an author, I want to choose Targeting: self, one creature, several, sphere, cone, line, cube, cylinder, emanation, wall or ring, with size, range and origin, so that areas are exact.
83. As an author, I want to choose who it touches (everyone, creatures I choose, allies only, enemies only, objects too) and filter by creature type, so that effects hit the right targets.
84. As an author, I want an area that moves with a creature (an emanation) or that the caster can move, so that spells like a moving lantern work.
85. As an author, I want typed parts: damage, healing, temporary HP, save, attack, condition, modifier, light, reveal, sense, movement (push, pull, teleport), surface, summon, transform, create object or wall, dispel, counter, grant action or feature, resource change, resistance change, and manual, so that nearly anything can be expressed.
86. As an author, I want each part to say when it fires (on cast, on entering, start or end of a turn, on hit, when hit, on a critical, on a kill, on a rest) and whom it touches, so that timing is exact.
87. As an author, I want conditions and branches inside effects ("if the save fails by 5 or more", "if HP 50 or fewer", "first time each turn", "creature type X"), so that complex spells work.
88. As an author, I want choice parts (the caster picks a mode), so that spells with options work.
89. As an author, I want durations: instant, rounds, minutes, hours, until dispelled, until a rest, permanent, and "until the end of its next turn", with repeat saves, so that effects end correctly.
90. As an author, I want concentration, ritual, components (with linked Material Components and costs) and casting time (action, bonus action, reaction with a structured trigger, minutes), so that spell metadata is complete.
91. As an author, I want Scaling by slot level, character level, class level or a level-table column, so that cantrips, Sneak Attack and auras scale.
92. As an author, I want a preview of the area drawn as tinted hexes on a map, so that I see what is hit.
93. As an author, I want parts listed as rows separated by rules, with drag to reorder and no boxed cards, so that the editor stays readable.

### The item builder

94. As an author, I want to choose the kind of item (weapon, armour, shield, helmet, cloak, gloves, boots, amulet, ring, clothing, instrument, ammunition, potion, scroll, throwable, coating, wand, container, trinket) and a base item, so that defaults come from the SRD.
95. As an author, I want rarity, enchantment +0 to +3, weight, value and attunement (with class, species, background or alignment requirements), so that the basics are set.
96. As an author, I want weapon properties and Weapon Mastery, including custom properties and masteries, so that weapons can be anything.
97. As an author, I want one-click Skill Boosts (advantage, +1d4, flat bonus, proficiency, expertise) and one-click granted cantrips, so that common magic is instant.
98. As an author, I want a catalogue of Item Properties across attack, defence, abilities and skills, spells and magic, movement and senses, light and visibility, triggers, use and charges, restrictions and drawbacks, sets and storage, so that I match everything BG3 offers and more.
99. As an author, I want Charges with recharge schedules (dawn, long rest, short rest, rolled), so that wands and staffs work.
100. As an author, I want consumables (single use, until a long rest, weapon coatings for N hits, area on impact, scrolls, special ammunition), so that potions, grenades and oils work.
101. As an author, I want curses, items that can't be removed, hidden properties revealed on attunement or identify, and sentient items with personality and goals, so that items have stories.
102. As an author, I want items that grow with character level or story beats, so that legacy items work.
103. As an author, I want set bonuses across items, so that armour sets are rewarding.
104. As an author, I want containers (capacity, weightless, holds only one kind), so that bags of holding work.
105. As an author, I want firearm options (misfire, reload, burst, ammunition recovery), so that common homebrew works.
106. As an author, I want a live item card as players will see it, so that the result reads well.

### Classes, subclasses, species, backgrounds, feats

107. As an author, I want a subclass builder with Features gated by level, uses, Resources, choices and Effects, so that the most common homebrew is supported.
108. As an author, I want a class builder (hit die, proficiencies, level table with custom columns, spellcasting progression including custom slot tables, subclass levels), so that whole classes can be added.
109. As an author, I want a generic Resource (name, die, maximum by level, recharge on short rest, long rest, dawn, initiative or a roll), so that points, dice and pools work for any class.
110. As an author, I want choices and prerequisites (pick one of N, level, ability score, spellcasting, feat), so that fighting styles, invocations and feats work.
111. As an author, I want a species builder (size choice, creature type, speeds, senses, lineages, innate spells by level, resistances), so that new peoples can be added.
112. As an author, I want a background builder in the 2024 shape (three ability options, origin feat, skills, tool, equipment or gold), so that backgrounds follow the current rules.
113. As an author, I want feats with prerequisites, repeatable flags and ability increases, so that feats are complete.

### Monsters and NPCs

114. As an author, I want a monster builder with stat block, multiattack composition, actions, bonus actions, reactions, legendary actions (count, costs), Legendary Resistance, recharge, lair actions at initiative 20, regional effects, mythic or second phases, auras, swarms, damage thresholds and spellcasting "X per day", so that boss fights work.
115. As a DM, I want legendary and lair actions offered in the turn order between turns, so that I don't forget them.
116. As a DM, I want Tactics per creature (from Int, simple, cunning, off), producing Suggested Actions shown under the token, so that running many creatures is easy.
117. As a DM, I want creatures and objects to carry Visibility Qualities (Hidden, Invisible, Disguised, Illusory, Ethereal, Obscured, Secret), so that the party sees only what it should.
118. As a DM, I want NPCs with personal attitude toward each character, so that relationships beyond Factions matter.
119. As a DM, I want companions and hirelings who travel with the party, controlled by a player or by me, so that retainers work.

### Rules engine coverage (SRD 5.2)

120. As a table, we want every SRD condition, including 2024 exhaustion levels with stacking, sources and durations, so that statuses are always right.
121. As a table, we want the 2024 actions (Attack, Dash, Disengage, Dodge, Help, Hide, Influence, Magic, Ready, Search, Study, Utilize), so that every choice is in the bar.
122. As a table, we want Grapple and Shove as the 2024 save-based Unarmed Strike options, with the drag movement cost, so that grappling works.
123. As a table, we want Extra Attack, moving between attacks, the Nick off-hand attack and one free object interaction per turn, so that martial turns are correct.
124. As a table, we want Weapon Mastery (Cleave, Graze, Nick, Push, Sap, Slow, Topple, Vex) as hotbar actions, so that martial play has the 2024 depth.
125. As a table, we want readied actions with structured triggers, so that Ready works.
126. As a table, we want the downed flow: 0 HP, Unconscious, death saves, stabilise with Medicine or a spell, healing that wakes, massive damage, revival windows, so that dying is handled.
127. As a table, we want short rests with Hit Dice spending and long rests that can be interrupted, triggering Encounter Checks, level-up, spell preparation, Charges and restock, so that the rest economy works.
128. As a table, we want optional camp supplies as the long-rest cost, so that gritty games work.
129. As a caster, I want to prepare spells after a long rest, with always-prepared spells, ritual casting and spellbook copying with cost and time, so that casters play correctly.
130. As a caster, I want concentration tracked, with saves on damage and warnings when a new concentration spell would end the current one, so that I don't lose spells by accident.
131. As a summoner, I want summoned creatures to act on my turn or their own, commanded with a bonus action, ending with concentration and shown in the roster, so that summons work.
132. As a druid, I want Wild Shape and Polymorph as Forms with their own stat block, temporary HP, kept features and a swapped hotbar, reverting at 0 HP or on dismiss, so that shapechanging works.
133. As a table, we want traps (trigger, detection DC, disarm with tools, Effect, reset) and locks (DC, key, thieves' tools, Knock, breaking), so that dungeons work.
134. As a table, we want interactable map objects (doors, levers, chests, barrels, destructible objects with AC and HP, curtains), so that environments matter.
135. As a table, we want surfaces including obscuring clouds (fog, darkness, stinking cloud), plant growth, spikes, mud, lava and consecrated ground, plus author-defined surfaces, so that terrain plays like BG3.
136. As a table, we want elevation on hexes, climbing and falling damage, and an optional high-ground rule, so that terrain height matters.
137. As a table, we want jumping, climbing, throwing objects or creatures and forced-movement previews, so that physical actions are supported.
138. As a table, we want line of sight, cover and opportunity-attack warnings shown along a planned path, so that we don't stumble into reactions.
139. As a table, we want a sneak mode with group Stealth and the passive Perception reach of noticed creatures drawn as tinted hexes, so that infiltration plays well.
140. As a DM, I want a turn-based exploration mode without starting Combat, so that tense moments are structured.
141. As a table, we want group and shared initiative options, and monsters sharing an initiative, so that large fights run quickly.
142. As a table, we want Visibility Qualities resolved against Senses (sight, darkvision, blindsight, tremorsense, truesight) and Reveal effects, as documented in the rules reference, so that hidden and disguised things behave correctly.
143. As a table, we want Influence checks against an NPC's attitude, with Faction Standing as an advantage or disadvantage source, so that social scenes have teeth.
144. As a table, we want mounts (controlled or independent, mounted movement, falling off), so that riders work.
145. As a table, we want vehicles and ships (hull, crew stations, speed on the world map), so that naval and caravan play is possible.

### Rule Variants, Roll Tables, Tracks, downtime

146. As a DM, I want Rule Variants as switches (flanking, critical fumbles, critical variants, gritty or epic rest, short-rest cap, massive damage, hidden death saves, healing surges, slow natural healing, initiative variants, morale, encumbrance variants, exhaustion variants), so that the table plays its way.
147. As a DM, I want to author my own Rule Variants from hook points (on a natural 1, on a critical, on dropping to 0, on a rest, on casting) to Effects or Roll Tables, so that house rules are automated.
148. As a DM, I want Roll Tables in the Library (dice, ranges, results that apply Effects or Items), so that fumbles, injuries, surges, madness, trinkets, weather and rumours are a click away.
149. As a DM, I want lingering injuries that survive rests until a specific cure, so that gritty play works.
150. As a DM, I want custom Tracks (sanity, stress, honour, piety, corruption, renown, notoriety) with thresholds that trigger Effects or Roll Tables, so that campaign-specific systems work.
151. As a DM, I want spell points and other alternative casting systems as Resources, so that variants are supported.
152. As a player, I want downtime days as a Resource and downtime activities (crafting, training, work, research), so that time between adventures matters.
153. As a player, I want crafting from Recipes (ingredients, tools, time, cost), including potions and scrolls where the SRD allows, so that BG3-style alchemy exists.

### Live play: shared components

154. As anyone at the table, I want the map always underneath, with see-through, compact panels on top, so that the screen doesn't feel crowded.
155. As anyone at the table, I want one roster (party, DM and TV variants) floating at the top, centred, with a drop shadow, and a name, health bar and status icons under each face, so that the turn order reads at a glance.
156. As anyone at the table, I want a wide roster to swipe sideways and keep the active face in view automatically, so that big fights still work on phones.
157. As anyone at the table, I want to tap a face to see its effects (source, description, duration), so that statuses are explained.
158. As a player, I want enemy health shown only roughly while the DM sees exact numbers and hidden creatures, so that secrets stay secret.
159. As a table, we want initiative numbers shown once, when the order is set, with the faces sliding into order and the numbers fading, so that the reveal feels like an event.
160. As a player, I want a large old-style "It's your turn" banner to pop up in the middle of my screen and fade, so that I never miss my turn.
161. As a player, I want my action economy (action, bonus action, reaction, movement, slot gems, class Resources) always visible, with per-kind glyphs, so that I know what I have left.
162. As a player, I want unusable actions greyed out with the reason shown, so that I understand why.
163. As a player, I want to arrange my own action bar: on desktop through a drawer with drag, two bars and keys 1–0; on phones by holding a tile to wiggle-edit, with + to add and a four-tile quick bar on the map, so that the bar fits how I play.
164. As a player, I want new spells and items added to the end of my bar without reordering anything, so that muscle memory survives.
165. As a player, I want every move confirmed (path, cost, warnings, then Confirm or Cancel) whether I chose Move or tapped a hex, so that I never misclick into danger.
166. As a player, I want a target preview (hit chance, damage range, cover, sources of advantage) before attacking, so that I choose well.
167. As a player, I want areas of effect shown as tinted hexes before I cast, so that I see who is hit.
168. As a player, I want health bars and, for the DM, Suggested Actions shown under tokens on the map, so that the map carries the key information.
169. As a player, I want Roll Cards that accept physical dice through the number keyboard or roll for me, so that I can roll my own dice.
170. As a player, I want critical hits and fumbles celebrated (gold for a natural 20, red for a natural 1), so that big moments land.
171. As a player, I want Reaction Prompts with a timer, plus per-character reaction settings (always ask, always use, never, with conditions), so that the table isn't held up.
172. As a player, I want Heroic Inspiration, Bless and similar dice shown in the roll breakdown, so that I see why I hit.
173. As a player, I want dice to roll in from the screen edge, glide to the centre and show the total large, with critical styling, falling back to 2D with reduced motion, so that rolling is satisfying.
174. As a player, I want my Dice Set (preset or uploaded pattern aligned on the unwrapped die, shareable with Friends or everyone) used for my rolls, so that dice feel like mine.

### Live play: phone

175. As a player on a phone, I want swipeable pages (Map, Actions, Spells, Character, Party) over the map, so that I reach everything quickly.
176. As a player on a phone, I want the bottom bar full-bleed on the screen edge, sheets attached flush above it, and my remaining resources floating above with no box, so that the layout feels native.
177. As a player on a phone, I want pinch to zoom and no zoom buttons, so that it works like any map app.
178. As a player on a phone, I want one-line spell rows grouped by level, with slot gems in the headers, so that the list is compact.

### Live play: DM

179. As a DM, I want the same roster, tokens and bars as players, plus hidden creatures, exact HP and Suggested Actions, so that I learn one interface.
180. As a DM, I want to switch which creature I control, or select several, so that running groups is quick.
181. As a DM, I want a creature panel with stats, Tactics and agent (MCP) notes with Undo, so that I can adjust quickly.
182. As a DM, I want Encounter Zones drawn as tinted hexes, with detection progress and Spring or Hold controls, so that ambushes run themselves.
183. As a DM, I want to place hidden creatures, draw zones and reveal areas, so that prep continues live.
184. As a DM, I want a phone remote for the Table Display (follow turn, party, ping, zoom, blackout) and for running creatures, so that I can step away from the laptop.
185. As a DM, I want named checkpoints and rewind to the start of a round, plus an optional no-undo mode, so that mistakes are recoverable or not, by choice.
186. As a DM, I want to show or hide DCs per Campaign, so that the table chooses its style.
187. As a DM, I want to run a split party across maps, with the Table Display following one group, so that splits are manageable.

### Table Display (TV)

188. As a table, we want the TV to show the same roster, the map in party view, the last roll with 3D dice and DM captions, so that everyone watches the same story.
189. As a table, we want the initiative reveal and "It's your turn" on the TV, so that the room shares the moment.
190. As a table, we want travel shown on the world map with route, distance, time and the encounter check, so that journeys feel real.

### Maps and world

191. As a DM, I want battle maps as hex tiles with terrain, elevation, cover, light and surfaces, at a per-Map scale, so that tactics are exact.
192. As a DM, I want world maps as a picture (the painted Default World or an upload) with a see-through hex grid calibrated by dragging two points onto a known distance, so that travel estimates are right.
193. As a DM, I want to set grid kind (hexes, squares, off), size in miles and strength, so that the grid fits the art.
194. As a player, I want to measure routes on the world map in hexes, miles and hours at the party's pace, so that we can plan.
195. As a player, I want the world map dimmed where we have never been, and dark except for found local maps and the road walked when we have no world map, so that discovery matters.
196. As a DM, I want to see everything, including encounter regions and secret places, so that I can run the world.
197. As a table, we want Fog with remembered and currently visible layers, Party Vision and strict light, so that exploration is fair.

### Public Compendium

198. As anyone, I want a public, searchable SRD compendium with guides (loot suggestions, spells and attacks by level), so that I can look things up without an Account.
199. As anyone, I want to copy a link to an entry, so that I can share it.

### Look, accessibility and devices

200. As anyone, I want one consistent component system ("Soft"): 5 px controls, 8 px panels, gold underline for the current tab, brass primary buttons, floating-label fields, flush dropdowns, rows rather than cards, round avatars and hex enemies, so that every page feels like the same app.
201. As anyone, I want fields judged only after I leave them, green when valid and red with the reason when not, so that forms are calm.
202. As anyone, I want pickers that filter as I type and search the server with a debounce for large lists, so that choosing is fast.
203. As anyone, I want desktop pages that fill the width with even gutters and a full-width footer, so that nothing looks detached.
204. As anyone, I want colour-blind-safe palettes, text size, a dyslexia-friendly font option, screen-reader announcements for turns and rolls, reduced motion and haptics control, so that everyone can play.
205. As a player with a gamepad or keyboard, I want full shortcuts, so that I can play without a mouse.
206. As a table, we want a difficulty preset per Campaign, so that new players have an easier time.
207. As a player, I want an optional, disclosed karmic smoothing for auto-rolls only, so that tables that want it can have it.

### Emails

208. As an Account holder, I want emails (invite, sign-in link, confirm, new sign-in, two-step reset, Account disabled, Friend request, Conversation, Session reminder, Proposal to DM, Proposal decision, level-up, Release Note, Digest) in one dark, centred, minimal template, so that mail is recognisable and calm.

### MCP and agents

209. As a DM, I want MCP tools to create and edit Campaigns, encounters, maps, Homebrew and Standing Changes on my behalf with my Access Token, with every write attributed and recoverable, so that agents can build with me safely.
210. As a DM, I want agent-proposed Standing Changes and Homebrew to stay pending until I confirm, so that agents never change the world behind my back.

## Implementation Decisions

### Ground rules

- Settled decisions are respected, not reopened: ADR-0001 (Go backend), 0003 (single replica), 0004 (server-only rules), 0005 (goose and squawk), 0006 (UUIDs), 0007 (MCP through the REST router), 0008 (own Accounts with linkable OIDC), 0009 (Grimoire Access Tokens), 0010 (Characters owned by Accounts), 0011 (linked Library with Campaign Overrides), 0012 (GPU dice with server results) and 0013 (Faction Standing).
- Content is SRD 5.2 only. Anything else ships as an original mechanism plus original examples, or is supplied by users. The 2014 ruleset option is removed from new Campaigns; existing data is migrated to 5.2 names (Focus Points, Deflect Attacks, Divine Smite as a spell, and so on).
- No JSONB. Typed columns, child tables and DB constraints carry integrity. Every write is attributed and recoverable through a Revision or the Action Log. Live Updates are filtered per audience, with a negative test for every hidden field.

### Bounded contexts and modules

- **identity** (new): Accounts, credentials, two-step sign-in, OIDC links, sessions and devices, Access Tokens, invites, Admin role. It replaces the forward-auth identity used by campaign members, with a one-time migration of existing members to Accounts.
- **social** (new): Friends, Conversations, mentions, Notifications (in app, push, email channels), Digest scheduling, Release Notes.
- **library** (new): Library entries, Shared Library, Collections, Campaign links and Campaign Overrides, Revisions with per-Campaign pins, Proposals and reviews, import and export in Grimoire's own schema.
- **compendium**: SRD 5.2 import. The typed effect tables become the single representation of anything that does something, owned by exactly one of spell, feature, item property, condition, monster action, surface, rule variant, trap or roll-table result.
- **rules** (pure): expanded as listed below. No I/O; randomness and time are injected; 100% coverage stays.
- **campaign**: Campaign rules and Rule Variants; Characters as Campaign Characters linked to Account-owned Characters; Factions and Standing; NPC attitude; game clock; marching order; Quests and Lore; Journal.
- **inventory** (new, inside campaign): item instances, containers (character, Party Stash, shop stock, loot pile, nested containers), equipment slots, attunement, currency.
- **prep**: Regions, encounter tables and pools, loot tables, encounter checks, Settlements, Shops and Stock, Price Guide, Encounter Zones, traps and locks as placeable map objects.
- **play**: the live runtime gains roster audiences, initiative reveal, turn banner events, action bar layouts, move confirmation, reaction settings, summons and Forms, legendary and lair actions, loot piles, checkpoints, split-party groups and exploration turn mode.
- **maps**: per-Map scale and grid calibration; picture maps (upload or the painted Default World) with an overlay grid; hex battle maps; Found Maps and Fog.
- **web**: the Soft component system as shared components, used by every page; the live-play shells for desktop, phone, DM and TV; the builders.

### Rules engine additions (pure Go, sealed unions)

- **Effect components**, extending the existing set:
  - New components: temporary HP, teleport, forced movement, transform (Form), create object or wall, dispel, counter, redirect, grant action or feature, resource change, resistance change, light, reveal, sense, summon, choice and conditional branch.
  - Durations: instant, rounds, minutes, hours, until dispelled, until a rest, permanent, end of next turn; plus repeat saves.
  - Scaling axes: slot level, character level, class level, level-table column.
- **Targeting**:
  - Shapes: self, creatures (count), sphere, cone, line, cube, cylinder, emanation (moving with a creature), wall (segments, length, height, thickness), ring.
  - Side filter: anyone, allies, enemies, caster's choice, objects.
  - Also a creature-type filter and whether sight is required.
  - Rasterised to hexes for previews and resolution.
- **Item Properties**: a sealed union mapped onto Effect components, plus item-only kinds: enchantment, set bonus, container rules, charges and recharge, consumable mode, curse, hidden-until state, growth by level or beat, sentience.
- **Feature, Resource, Choice and Prerequisite** as rules value objects. Classes, subclasses, species, backgrounds and feats are data made of Features. Spellcasting progression is a table, which allows custom slot tables and spell points as a Resource.
- **Conditions**: the full SRD catalogue as data. Exhaustion is a stacking condition with per-level effects and a variant switch.
- **TurnEconomy**:
  - The 2024 actions.
  - Attack counts (Extra Attack), movement split across attacks, the Nick off-hand attack, one free object interaction, equip and unequip during an attack.
  - Readied actions with triggers.
  - Legendary action points and lair actions in the round.
- **Weapon Mastery** effects as data attached to weapons and characters.
- **ReactionTriggers**: generalised to any Effect with a reaction casting time or a trigger. Each Controller has reaction settings (ask, auto, never, with conditions) that resolve before a prompt is raised.
- **Death and dying**, **Rest** (short, long, interrupted, camp supplies), **Heroic Inspiration** and **Concentration** (with a conflict warning) as services.
- **Summons and Forms** as Combatant overlays with an owner link and their own economy rules.
- **Visibility**: Visibility Qualities against Senses, exactly per the rules reference. Reveal strips qualities in its area.
- **Faction Standing**: the tier modifies social checks, prices, first reactions and encounter weights.
- **Rule Variants and hooks**: a catalogue of switches plus author-defined hooks to Effects or Roll Tables.
- **Roll Tables, Tracks, Recipes and downtime activities** as pure resolution functions.
- **Travel**: distance on calibrated world maps, pace, game clock advance, encounter checks per leg.
- **Price Check and Estimated Challenge** as pure estimators.

### Data and schema (summary; detailed migrations per slice)

- **identity schema**:
  - Tables: `accounts`, `credentials`, `totp_factors`, `recovery_codes`, `oidc_links`, `account_sessions`, `access_tokens` (hashed, scoped, expiring), `invites`.
  - Admin is a role on the account.
- **social schema**: `friendships`, `friend_requests`, `conversations`, `conversation_members`, `messages`, `message_mentions`, `notifications`, `notification_preferences` (per kind and channel), `push_subscriptions`, `release_notes`.
- **library schema**:
  - Tables: `library_entries` (owner account, kind, exactly one typed body table per kind), `collections`, `collection_entries`, `campaign_links` (with optional pinned revision), `campaign_overrides`, `shared_library_reviews` (including an IP-check outcome), `proposals`, `proposal_reviews`.
  - Revisions per entry use the existing revisions pattern.
- **Characters**:
  - Characters move to Account ownership; `campaign_characters` hold per-Campaign level, XP, HP, Resources, spells prepared and conditions.
  - `character_classes` supports multiclassing.
  - Also: `character_features`, `character_choices`, `character_resources`, `heroic_inspiration`, `action_bar_layouts` (per character, per bar, ordered slots, key bindings), `reaction_settings`.
- **Inventory**:
  - `item_instances` (base item or homebrew item, custom name, charges, identified, attuned, equipped slot, container).
  - Containers have exactly one owner kind, and nesting is supported.
  - Attunement is limited to three per character by a constraint plus a check in the use case.
- **Effects and builders**:
  - Effect tables are extended with the new component tables.
  - New tables: `item_properties`, `features`, `resources`, `choices`, `prerequisites`, `monster_legendary_actions`, `monster_lair_actions`, `roll_tables`, `roll_table_results`, `tracks`, `track_thresholds`, `rule_variants`, `rule_variant_hooks`, `recipes`, `downtime_activities`.
- **Campaign**: `factions`, `faction_archetypes` (seeded), `standings`, `personal_standings`, `standing_changes` (status pending/confirmed/dismissed, shared-with-players flag), `npc_attitudes`, `game_clock`, `marching_orders`, `quests`, `quest_steps`, `lore_entries`.
- **Maps**:
  - Maps gain `kind` (battle, local, world), `scale` (feet or miles per hex), `grid_kind`, `grid_strength`, `grid_origin`, `grid_rotation` and an optional image key.
  - Objects: `map_objects` (door, lever, chest, barrel, curtain, destructible, with AC and HP), `traps`, `locks`.
  - Visibility: `visibility_qualities` on tokens and objects.
- **Play**: `checkpoints`, `party_groups` (split party), `summon_links`, `form_overlays`, `loot_piles`, `loot_claims`, `exploration_mode` on the live session.

### API contracts

- The OpenAPI spec stays hand-authored and is the source for REST and MCP.
- New resource groups: accounts and sign-in, social, library and proposals, homebrew builders (one resource per kind, with a shared parts sub-resource), characters and campaign characters, inventory, factions, rule variants, roll tables, tracks, maps with calibration, Public Compendium (unauthenticated, rate-limited).
- **Live play messages**:
  - New Update kinds: initiative reveal (with rolls, then order), turn started (drives the banner), roster changes per audience, move previews and confirmations, reaction settings outcomes, summons and Forms, legendary and lair action windows, loot pile changes, checkpoint created and restored.
  - Each Update kind is filtered per audience: party, each player, DM, Table Display.
- **MCP**: tools mirror the REST resources for Library, Homebrew, prep, maps and Standing Changes. Writes create Revisions or pending items that the DM confirms.

### Frontend

- **Shared component system**: field, picker, server picker, choices, values, navigation, tabs, rows, chips, dialogs, toasts, hover cards, stat blocks.
- **In-play components**: roster strip, effects card, turn banner, economy glyphs, resource pips, action tiles and bar editor, move confirm, target preview, Roll Card, Reaction Prompt, area tint, token with health bar and note, item card, Standing tier chip and meter.
- **Live-play shells**: desktop player, phone player (swipe track, full-bleed bottom bar, floating resources, pinch zoom), DM desktop, DM phone remote, Table Display.
- **Pages**: Dashboard, search, Admin, Account, Friends, Characters (sheet, inventory, creation and level-up wizards), Library and builders, Campaign (prep, factions, proposals, encounters), Compendium (public and in-app), Shops, Locations, Maps (world and local).
- **Maps**: rendered as hex layers. World maps render the image under a calibrated SVG or canvas grid with soft fog masks. Areas of effect, zones and regions are always tinted hexes, never circles.
- **Dice**: a WebGL worker with physics, server results forced by material swap, Dice Sets as texture atlases, and a 2D fallback.

## Testing Decisions

**What makes a good test**
- A good test exercises behaviour through a public seam with realistic inputs and asserts observable outcomes. It does not assert internal calls or private state.
- Tests use the project glossary in names.
- Hidden data gets negative tests: a payload for an audience must not contain what that audience may not see.

**Seams, highest first. These are the seams this spec commits to:**
1. **The HTTP API, in process.** The ogen server runs with testcontainers Postgres and migrations applied. All REST and MCP behaviour is tested here: Accounts, Library, Proposals, builders, inventory, Factions, maps, campaign prep. Prior art: the platform httpapi tests, contract tests and pgstore adapter tests.
2. **The live SessionRuntime.** It is driven by scripted command sequences with fixed seeds, asserting Updates per audience, replay equal to live, and `-race`. All live-play behaviour is tested here: roster and initiative reveal, turn banner events, move confirmation, reactions and settings, summons, Forms, legendary actions, loot piles, checkpoints, split party, exploration mode. Prior art: the play/live test suite (combat, reaction, area, walk, zones, inventory, table, tactics, effects).
3. **The pure rules package.** Table-driven and property tests cover effect folding, Targeting rasterisation, conditions and exhaustion, TurnEconomy with Extra Attack and mastery, rests, concentration, visibility against senses, standing modifiers, travel distance on calibrated grids, Price Check and Estimated Challenge, Roll Tables and Rule Variant hooks. 100% coverage and mutation testing on changed packages. Prior art: the rules package tests (attack, combat, dice, effects, hex, surface, surprise, tactics, travel, vision, loot, shops, encounters).
4. **Web.** Vitest covers domain and composables. Playwright runs multi-client scenarios (DM, two players, Table Display) across the phone, tablet, desktop and TV matrix, with axe on every surface and fog negatives in the DOM.

**Required scenarios include**
- Accounts:
  - invite to activated Account;
  - OIDC link both ways;
  - two-step required for Admins;
  - Access Token scopes enforced on MCP.
- Library and Homebrew:
  - Proposal approved into the DM's Library and linked into the Campaign Collection;
  - Campaign Override wins over the base;
  - a pinned Revision is unaffected by later edits.
- Builders: Marsh Lantern resolves light, reveal, charm on Undead and Fey only, and radiant damage at start of turn. Ashwood Longbow grants Light at will and spends and recharges Hunter's Mark charges.
- Rules:
  - exhaustion stacks to death;
  - Extra Attack with moving between attacks;
  - Vex grants advantage;
  - Grapple save and drag cost;
  - a readied action fires on its trigger.
- Combat and rest flows:
  - Shield set to auto only when it turns a hit into a miss;
  - short rest Hit Dice;
  - an interrupted long rest does not unlock level-up;
  - Wild Shape reverts at 0 HP with overflow damage;
  - a summon ends with concentration.
- Play:
  - initiative reveal order and ties;
  - roster audiences, with enemy HP coarse for players, exact for the DM, hidden creatures absent for players;
  - move confirm required.
- World and economy:
  - a calibrated world map gives the expected travel time;
  - Found Maps fog;
  - Standing tier changes social roll advantage and shop prices;
  - a Standing Change stays pending until the DM confirms.
- Items and inventory: attunement limited to three; a loot pile claim conflict resolved deterministically.

## Out of Scope

- Non-SRD game content of any kind: named factions, monsters, subclasses and settings from published products; mind-flayer-style progression; non-SRD encounter or price tables until verified as SRD 5.2.
- An LLM inside Grimoire (ADR and ARCHITECTURE: AI only through MCP clients).
- Video or voice chat.
- Real-time (non turn-based) combat.
- 3D maps or a 3D character model; the character preview stays a 2D figure or Portrait.
- Importing non-SRD text from third-party character builders. Unknown entries become Manual stubs.
- Native iOS. Android ships through Capacitor as today.

## Further Notes

- **ARCHITECTURE.md must be updated in the same delivery to match the ADRs**:
  - identity per ADR-0008/0009, not forward-auth;
  - goose, not Atlas;
  - SRD 5.2 only;
  - level-up at the next long rest, XP split among Combat participants;
  - the effect component vocabulary unified with the glossary;
  - per-Map grid scale;
  - motion and dice rules per ADR-0012;
  - the chosen visual direction, not "dark parchment".
- **The glossary needs these terms before code uses them**: Resource, Feature, Rest, Item Instance (with Attunement), Rule Variant, Roll Table, Trap, Lock, Map Object, Form, Summon, Heroic Inspiration, Weapon Mastery, Track, Game Clock, Marching Order, Checkpoint, Quest, Lore. Also rename "aura" to "emanation" (SRD 5.2) and reconcile the Visibility Qualities with the rules reference (Obscured covers darkness and heavy obscurement).
- **Fix the inconsistencies the review found**:
  - the three versions of Marsh Lantern;
  - "Estimated Challenge" used for spells (spells get their own balance check);
  - the undefined "Board";
  - the Proposal approval path, which is: copied once into the DM's Library, then linked into the Campaign Collection.
- **Delivery order**, recommended because later slices depend on earlier ones:
  1. identity and Account-owned Characters;
  2. data-driven Effects, item instances and the pure engine additions;
  3. Library, Proposals and builders;
  4. live-play redesign and shared components;
  5. maps and world;
  6. factions, rule variants, roll tables, tracks, downtime;
  7. social, notifications, emails, Release Notes, Public Compendium;
  8. accessibility, difficulty, karmic dice, gamepad.
  Each slice ships behind its own migrations and tests at the seams above.
- The canvas "Grimoire UI mockups" is the visual reference for every page and component named here.
- The gap review (docs/reviews/2026-10-01-bg3-and-homebrew-gaps.md) lists every item this spec absorbs, with priorities, for slicing into issues.
