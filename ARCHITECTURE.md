# Grimoire — Architecture

> A private, self-hosted D&D 5e table companion: reference compendium, campaign knowledge base,
> **fully automated rules**, BG3-inspired live play on phones, laptops and a shared screen, and an
> **MCP server** so the DM can prep and run sessions from an AI agent.
>
> **Go 1.26 API + Vue 3.5 SPA (PWA + Capacitor) + PostgreSQL 16+**, one public monorepo
> `JorisJonkers-dev/grimoire`, deployed on the JorisJonkers-dev estate platform.

**Status.** Settled 2026-09-29 in a design grilling session. This document supersedes the earlier
Grimoire design (Rails) and specialises the generic
[Go API](docs/blueprints/go-api.md) and [Vue + TS SPA](docs/blueprints/vue-ts-spa.md) blueprints for this product. Where this document and a generic blueprint
disagree, **this document wins**. Vocabulary lives in [`CONTEXT.md`](CONTEXT.md); decisions that are hard to reverse have
ADRs in [`docs/adr/`](docs/adr/).

**How to read it.** §1–§3 say what we build and why. §4–§17 say how, one concern per section.
§18–§22 cover delivery, milestones and the rules agents follow. Nothing here is aspirational unless
marked *(later)*; everything else is in scope for the milestone named next to it.

---

## 0. Decision summary

| Area | Decision | Section |
|---|---|---|
| Users | Private groups; **Grimoire Accounts** created by Admin invite, optionally linked to a jorisjonkers.dev login ([ADR-0008](docs/adr/0008-own-accounts-with-linkable-oidc.md)); DM invites Players per Campaign | §1 |
| Scope | Everything in the old Grimoire spec **plus** BG3-style combat UX, random encounters, shops, AI/MCP, loot, rest/camp, handouts, chat, journal; towns/images later | §2 |
| Rules | **Full automation**; typed effects with a **manual fallback** for anything not yet modelled | §9 |
| Dice | **Roll Card**: shows the exact dice to throw; player types the faces or taps to auto-roll | §10 |
| Grid | Hex, axial coordinates, scale per Map (5 ft on battle maps, miles on world maps), with elevation, cover and Surfaces | §12 |
| Backend | **Go 1.27**, spec-first OpenAPI 3.1 (**ogen**), **sqlc**, **goose** + **squawk** ([ADR-0005](docs/adr/0005-goose-and-squawk-for-migrations.md)), goroutine per live Session | §6 |
| Contract | One OpenAPI 3.1 spec for REST **and** WebSocket messages; MCP tools call the same use cases | §7 |
| Rules on client | **Server-only**; client keeps hex↔pixel geometry only, pinned by golden fixtures | §9.9 |
| Frontend | Vue 3.5 + TS strict, hey-api client + Zod, TanStack Query, Pinia; hybrid **PixiJS canvas + SVG** map | §15 |
| Devices | Responsive **PWA**, wrapped in **Capacitor 7** (Android/iOS) like agents-ui; Table surface in a kiosk browser | §15.7 |
| Connectivity | Online for live play; compendium and sheets cached for offline reading | §15.7 |
| Design | **Claude Design mocks first** (M0), own BG3-inspired theme, vue-web-commons for plumbing only | §16, §23 |
| Play | Exploration without initiative, combat with initiative on the TV, one hotbar for players and DM, suggested enemy actions, Encounter Zones with surprise | §23 |
| Runtime | **Single replica** + Postgres advisory lock; ports ready for multi-replica | §11.7 |
| AI | MCP server **inside the Go API** at `/mcp`, authenticated with Grimoire **Access Tokens** ([ADR-0009](docs/adr/0009-grimoire-issues-its-own-access-tokens.md)); writes apply **immediately**, undone via Revisions/Action Log | §14 |
| Generation | Deterministic seeded generators in Go; Go-native OSS embeds; Watabou/Azgaar exports imported; FMG and ComfyUI sidecars *(later)* | §13 |
| Hosting | Estate k3s/Flux via `platform/deployment.yml`, shared Postgres, Garage S3; Grimoire handles its own sign-in | §19 |
| Tooling | Monorepo with **mise + Taskfile** | §5 |
| Gates | **Tiered**: contract, types, lint, tests, fog negatives and device matrix block every PR; full mutation nightly | §17 |
| License | Public repo under the **Attribution Assurance License**; forks must credit the author visibly in their UI | §21 |
| i18n | English, all strings through vue-i18n | §15.8 |

---

## 1. Product

### 1.1 What Grimoire is

A companion for an in-person (or hybrid) table. Players use their phones, the DM uses a laptop or
tablet, and a TV shows the shared **Table** view. The server owns the truth: it computes every
modifier, range, hit chance and effect, hides what the party has not discovered, and keeps an Action Log
of everything that happened. The feel borrows from **Baldur's Gate 3** — action economy pips, a
hotbar, hit-chance previews, reaction prompts, roll cards with a modifier breakdown — but with
**minimal animation**: state changes are shown clearly and quickly, not cinematically.

### 1.2 Users & tenancy

- **Private groups only.** No public sign-up. Grimoire owns its **Accounts**: an Admin invites a
  person, who sets up a Username, Nickname and password, or signs in with a linked jorisjonkers.dev
  login if they hold the Grimoire permission. Admins must use two-step sign-in. Agents and scripts
  use scoped **Access Tokens** ([ADR-0008](docs/adr/0008-own-accounts-with-linkable-oidc.md),
  [ADR-0009](docs/adr/0009-grimoire-issues-its-own-access-tokens.md)).
- **Characters belong to Accounts** and join Campaigns as Campaign Characters with their own progress
  ([ADR-0010](docs/adr/0010-characters-owned-by-accounts.md)).
- A **Campaign** has members with a role: **DM** (one or more co-DMs) or **Player**. Membership is by
  invite.
- A **Controller** drives a Combatant; normally its owner, reassignable by the DM (absent players,
  delegates).

### 1.3 Surfaces

| Surface | Device | Role | Purpose |
|---|---|---|---|
| **Player** | phone first, laptop | one Character | sheet, hotbar, movement, Roll Cards, Reaction Prompts, journal, chat, inventory |
| **DM** | laptop / tablet | full control | truth view, all Combatants, prep, encounter checks, fog, overrides, undo, generators |
| **Table Display** | TV / big screen, kiosk browser | shared, no controls; steered remotely from any DM session | party view only: discovered map, tokens, initiative rail (whenever enemies are present), roll results, level-up moments, handouts |

### 1.4 Principles

1. **Server is the single source of truth.** Clients send intents; the server validates, mutates,
   logs, and broadcasts role-filtered state.
2. **Hidden data never leaves the server.** Fog, hidden tokens, DM notes and secret checks are
   absent from Player/Table payloads, not hidden in CSS.
3. **Compute, don't track.** If the rules say it, the engine computes it. What the engine cannot
   model yet degrades to a visible manual step, never to silent wrongness.
4. **Contracts first.** One OpenAPI 3.1 document drives the Go server, the TS client, the Zod
   validators, the WebSocket message types and the MCP tool schemas.
5. **Rapid loop.** Fast builds, fast tests, fast responses. Every gate must run locally in seconds to
   a few minutes.
6. **Private content stays out of git.** Adventures, homebrew and campaign data live in Postgres and
   object storage, never in the repository.

### 1.5 Non-goals

- Public multi-tenant hosting, payments, a marketplace.
- Voice/video (people are in the room or on their own call).
- Full offline play. Live play requires a connection.
- Hosting or redistributing non-SRD commercial content.

---

## 2. Feature catalogue

Every feature below is in scope. The milestone tag says when it lands (§20). "Grimoire" marks
features carried over from the earlier spec.

### 2.1 Compendium
- SRD **5.2 (2024)** only. The earlier 5.1 option is retired; existing data is migrated to the 2024 names and rules
- Spells, classes, subclasses, class levels and features, species, backgrounds, feats, conditions,
  monsters (every stat NOT NULL, kill XP), weapons with properties and masteries, armor, equipment,
  magic items, tools, languages — M1
- Import from **Open5e v2** (primary) with **5e-bits** cross-check; idempotent; snapshot pinned — M1
- **Homebrew authoring** in the same typed shapes (UI + MCP), campaign-scoped — M3
- Compendium browser with filters, search (`pg_trgm`), nested concept tooltips (BG3-style: hovering
  "Prone" inside a spell description opens the condition) — M1/M3
- Attribution surface (CC-BY-4.0, OGL 1.0a where applicable) — M1
- **Automation coverage report**: which SRD entries are fully automated vs manual fallback — M2

### 2.2 Rules engine (full automation)
- Dice, Roll Requests, advantage/disadvantage, crits, rerolls (Lucky, Halfling Luck), minimums
  (Reliable Talent), bonus dice (Bless, Guidance, Bardic Inspiration) — M2
- Ability scores, modifiers, proficiency, expertise, saves, skills, passive scores — M2
- Ability generation: standard array, point buy, 4d6-drop-lowest via Roll Cards — M3
- Attack resolution with every advantage/disadvantage source, cover, elevation, resistances,
  vulnerabilities, immunities — M2/M5
- **Typed effects**: damage, healing, saves, conditions, modifiers, areas, durations,
  concentration, triggers; **manual fallback** — M2 onward
- Conditions and buffs auto-applied to PCs and NPCs alike (Grimoire) — M2
- Initiative with **simultaneous ties**, optional tiebreak, group/popcorn initiative options
  (Grimoire) — M2
- Movement: reachable hexes, difficult terrain, elevation, climbing, swimming, flying, **jump**,
  **shove**, **dash/disengage/dodge/help/hide** — M2/M5
- Line of sight, **cover** (half, three-quarters, total), **vision & lighting** (darkvision,
  blindsight, truesight, devil's sight, light sources, obscurement), union/intersection Table vision
  (Grimoire) — M5
- **Surfaces** (fire, grease, water, ice, web, acid, poison cloud, darkness) and their interactions
  (grease + fire, water + lightning/cold) — M5
- **AoE templates** (cone, sphere, cube, cylinder, line, emanation) on the hex grid — M5
- **Reactions**: opportunity attacks, Shield, Counterspell, Hellish Rebuke, readied actions — M5
- Throwing items and potions (incl. the `thrown_potion_heals` option) — M5
- Encounter budget (2024 DMG), CR→XP — M2
- Optional/homebrew **rule catalogue** toggled per Campaign (Grimoire §26) — M5
- Rests: short/long, hit dice, spell slots, class resources, gritty/epic variants — M7

### 2.3 Campaign
- Campaigns, members, roles, invites — M3
- **Character builder** (BG3-style wizard): species, class, background, ability scores, feats,
  spells, gear, validated against the ruleset — M3
- **Level-up flow**: unlocks at the next long rest after the threshold, unless the DM holds it; applied out of combat — M3
- Out-of-combat editing (swap cantrips, prepare spells, retrain), edit-locked in an active Combat
  (Grimoire §28) — M3
- **D&D Beyond import** (unofficial public JSON; unknowns flagged) — M8
- NPCs, factions, locations, KB entries with typed links (Grimoire) — M3/M7
- Sessions: numbered, dated, recap, summary — M3/M4
- XP split equally among a Combat's participants, or Milestone advancement — M5
- **Boards** (Kanban quest/planning boards linked into the KB) (Grimoire) — M7
- **Handouts** pushed to phones and the Table; **party chat** and **whispers**; per-player
  **journal** fed from boards/KB — M7
- **Inventory** with weight/encumbrance and drag/drop, **party stash**, currency — M6
- **Revisions** of all prep data with author (DM or MCP client), diff and restore — M3 onward

### 2.4 Prep, world & generation
- **Encounters** (prepared) with difficulty budget — M3
- **Encounter Tables** per Region, **Encounter Pools** with level bands, **Loot Tables** — M6
- **Encounter Checks** on rest, travel leg, or DM action; DM can force a check, force an encounter,
  schedule a check, or pick the result; per-table secret/open visibility — M6
- **Settlements**, **Shops**, **Stock** generated from Loot Tables by settlement size and wealth;
  haggling via Roll Cards; restock on rest/time — M6
- Generators: encounters, loot, shops, NPC names/personalities, dungeons/battlemaps — M6/M8
- Import **Watabou** (MFCG, Village, One Page Dungeon) and **Azgaar FMG** exports — M8
- Procedural **towns** with districts, shops and NPCs *(later, M9+)*
- Procedural/AI **images** for portraits, shops and scenes via ComfyUI sidecar *(later)*
- **Private adventure import** via MCP into Maps, Regions, NPCs, Encounters and KB — M8

### 2.5 Live play
- Session lifecycle planned → live → paused → ended (Grimoire) — M4
- World map travel along routes, party marker, "no map" mode (Grimoire) — M6
- Local tactical maps with Fog, regions (manual / on-enter / line-of-sight reveal), doors,
  locked edges (Grimoire) — M4/M5
- Tokens incl. Large+ sizes, hidden tokens — M4
- Combat: initiative, turns, rounds, **hotbar**, **action economy pips**, **hit % + damage preview**,
  **path preview**, **AoE preview**, **Reaction Prompts**, **Roll Cards** — M4/M5
- DM challenge rolls ("give me a Wisdom save") (Grimoire §28) — M4
- Control reassignment (Grimoire §24) — M4
- DM **undo** via the Action Log — M4
- Shared "Level up!" moment on Table + phones (Grimoire §25) — M5

### 2.6 AI & MCP
- MCP server at `/mcp` (Streamable HTTP), Grimoire Access Tokens, DM-scoped tools — M4
- Tools for prep (encounters, pools, loot, shops, NPCs, KB, homebrew), live management (spawn
  encounter, apply effect, move token, reveal fog, run encounter check), and lookup — M4 onward
- Immediate writes, attributed to the MCP client, recoverable via Revisions/Action Log — M4

---

## 3. System overview

```
             ┌──────────────────── phones / laptops / TV ────────────────────┐
             │  Vue 3 SPA (PWA)   ·   Capacitor 7 shell (Android/iOS)        │
             └──────────────┬───────────────────────────────┬────────────────┘
                            │ HTTPS REST + WebSocket         │
                   ┌────────▼────────┐                       │
DM's AI agent ────▶│ Traefik         │ Grimoire sessions, OIDC link to jorisjonkers.dev,
 (Claude, MCP)     └────────┬────────┘   Access Tokens for MCP
                            │
          ┌─────────────────▼──────────────────────────────────────────┐
          │ grimoire-api (Go, single binary, single replica)            │
          │  inbound: REST (ogen) · WS hub · MCP (/mcp) · CLI · jobs    │
          │  app/use cases ─▶ domain (compendium · rules · campaign ·   │
          │                   prep · play · generation)                 │
          │  outbound: sqlc repos · S3 (Garage) · generator sidecars    │
          └──────┬──────────────────────────┬──────────────────────────┘
                 │                          │
        ┌────────▼────────┐        ┌────────▼────────┐      (later)
        │ Postgres 16+    │        │ Garage S3       │   FMG sidecar (Node)
        │ grimoire_db     │        │ grimoire-assets │   ComfyUI (GPU)
        └─────────────────┘        └─────────────────┘
```

- The SPA is embedded in the API binary and served by it, so cookies and WebSockets are
  same-origin and there is one image to deploy.
- REST, WebSocket and MCP are three **inbound adapters over the same use cases**. There is no
  business logic in any of them.

---

## 4. Bounded contexts

Six contexts, each a package tree under `api/internal/<context>/` with hexagonal layers inside.
Contexts communicate **by value** (IDs, slugs, value objects), never by sharing rows across a
context line.

| Context | Owns | Depends on |
|---|---|---|
| **compendium** | SRD + homebrew reference data, ruleset blend, import | shared |
| **rules** | Pure rules engine: dice, effects, attacks, movement, vision, surfaces, AoE, initiative, budgets | shared, compendium (public read port only) |
| **campaign** | Campaigns, members, characters, NPCs, KB, boards, sessions (durable record), maps (definitions), inventory, handouts, chat, revisions | shared, compendium, rules |
| **prep** | Encounters, Encounter Tables/Pools, Loot Tables, Regions, Settlements, Shops, Stock, Encounter Checks | shared, compendium, rules, campaign |
| **play** | Live Session runtime: combat, positions, effects, reveal, reactions, rolls, Action Log, projections | shared, rules, compendium, campaign, prep |
| **generation** | Seeded generators (encounters, loot, shops, names, dungeons), import of Watabou/FMG exports, sidecar gateways | shared, compendium, rules |

**identity** is its own context (ADR-0008): Accounts, credentials, two-step factors, OIDC links,
sessions, Access Tokens and invites. Campaign members reference Accounts.

```
compendium ◀── rules ◀── campaign ◀── prep ◀── play
     ▲           ▲          ▲          ▲
     └──────── generation ──┘          │
                  (writes via prep/campaign use cases)
```

**Durable vs live (Grimoire's key split, kept).** Campaign owns the durable Session record, Map
definitions, persisted reveal and the KB. Play owns the live instance while a Session is live and
writes the recap and discovered state back when it ends.

---

## 5. Repository layout & developer loop

Public monorepo `JorisJonkers-dev/grimoire`, topic `workspace-services`, bootstrapped from
`repo-template` (its source-available LICENSE is replaced, §21).

```
grimoire/
├── CONTEXT.md                     # glossary (no implementation detail)
├── ARCHITECTURE.md                # this document
├── ATTRIBUTION.md                 # SRD / OGL / Watabou / Azgaar / OSS credits (machine-readable)
├── LICENSE                        # Attribution Assurance License
├── mise.toml                      # pins go, node, pnpm, task, sqlc, squawk, vacuum, golangci-lint
├── Taskfile.yml                   # the only entry points: dev, gen, check, test, e2e, fix
├── openapi/
│   ├── v1/openapi.yaml            # HAND-AUTHORED source of truth (REST + WS message components)
│   ├── v1/asyncapi.yaml           # channel docs; $refs components from openapi.yaml
│   ├── redocly.yaml  .spectral.yaml
│   └── fixtures/                  # golden request/response + hex geometry fixtures (§9.9)
├── api/                           # Go module
│   ├── go.mod  sqlc.yaml  .golangci.yml  .custom-gcl.yml
│   ├── cmd/grimoire/main.go       # composition root: `serve`, `migrate`, `import`, `gen`
│   ├── internal/
│   │   ├── shared/                # IDs, Result helpers, clock, rng, errors
│   │   ├── compendium/{domain,app,adapters/{persistence,sources}}/
│   │   ├── rules/{dice,effects,combat,spatial,vision,surfaces,budget,rest}/   # pure
│   │   ├── campaign/{domain,app,adapters/persistence}/
│   │   ├── prep/{domain,app,adapters/persistence}/
│   │   ├── play/{domain,app,runtime,projection,adapters/{persistence,broadcast}}/
│   │   ├── generation/{domain,app,adapters/{sidecar,imports}}/
│   │   └── platform/{api(ogen),db(sqlc),httpx,ws,mcp,auth,storage,otel}/
│   └── db/{schema,migrations,queries,seeds}/
├── web/                           # Vue 3 SPA (pnpm)
│   ├── package.json  pnpm-lock.yaml  vite.config.ts  eslint.config.ts  capacitor.config.ts
│   ├── src/{app,shared,features/<feature>,infrastructure/api(GENERATED),render,realtime}/
│   └── tests/{unit,component,e2e}/
├── design/                        # Claude Design exports, tokens.json, component inventory
├── platform/                      # deployment.yml, images.lock.json, production.env, render-local.sh
├── docs/{adr,runbooks}/
└── .github/workflows/
```

**One command per loop** (Taskfile):

| Task | Does |
|---|---|
| `task dev` | Postgres (compose/testcontainers), API with live reload (`air`), Vite HMR, seeded compendium |
| `task gen` | spec lint → ogen → sqlc → hey-api → golden fixtures; idempotent |
| `task check` | the full PR gate set locally (§17) |
| `task test` | Go unit + adapter tests, Vitest |
| `task e2e` | Playwright device matrix + multi-client scenarios against a local stack |
| `task fix` | gofumpt, golangci-lint --fix, eslint --fix, prettier |

`mise install` is the only setup step. No Nx/Turbo; Go and pnpm caches do the work.

---

## 6. Backend — Go

Follows the [Go API blueprint](docs/blueprints/go-api.md) in full except where stated. Summary of what binds here:

### 6.1 Stack

```
Go 1.27
github.com/ogen-go/ogen                 # server + types + validation from openapi.yaml
github.com/jackc/pgx/v5                 # driver + pgxpool; sqlc generates queries
github.com/pressly/goose/v3             # forward-only SQL migrations embedded in the binary (squawk lints them)
github.com/coder/websocket              # live play transport
github.com/modelcontextprotocol/go-sdk  # MCP server (Streamable HTTP) — verify version in M0
github.com/riverqueue/river             # Postgres-backed jobs (imports, restocks, scheduled checks)
github.com/aws/aws-sdk-go-v2/service/s3 # Garage S3 for uploads
go.opentelemetry.io/otel                # traces/metrics; log/slog for logs
# dev: golangci-lint v2 + nilaway, gofumpt, gremlins, go-test-coverage, testcontainers-go,
#      pgregory.net/rapid (property tests), air (live reload)
```

### 6.2 Dependency rule

Inbound adapters (ogen handlers, WS hub, MCP server, CLI, River workers) → application (use cases)
→ domain (entities, value objects, ports). Outbound adapters (sqlc repositories, S3, sidecar
gateways) implement ports. `rules` imports nothing but `shared` and the compendium's read port.
Enforced by `internal/` visibility plus **depguard** rules per context; a violation fails CI.

### 6.3 Typing gates

The Go blueprint's §6 applies unchanged and matters more here than anywhere, because the rules
engine is a large set of "one of N" types:

- **Sealed interfaces + `gochecksumtype`** for every union: `Effect`, `Command`, `Update`,
  `RollOutcome`, `TriggerKind`, `EntrySource`, `Surface`. No `default` branch may dodge
  exhaustiveness.
- **`exhaustive`** on every typed enum switch; **`exhaustruct`** on value objects; **NilAway** on
  the whole module.
- Value objects have unexported fields and constructors returning `(T, error)`.
- Branded IDs: `type CampaignID int64`, `type CombatantID int64`, … never raw `int64` across
  a port.

### 6.4 Use cases

One struct per business action with `Handle(ctx, Input) (Output, error)`. Errors are typed
(`ErrNotFound`, `ErrForbidden`, `ErrRuleViolation{Reason}`, `ErrConflict`) and mapped once to
RFC 9457 problem+json (REST), an error frame (WS) or an MCP tool error. Every mutating use case takes
a `Caller` (member id + role + origin: `ui` | `mcp:<client>` | `system`) so revisions and the
Action Log record who did it.

### 6.5 Determinism

All randomness goes through an injected `rng.Source` seeded per operation; seeds are logged. A
replay of the Action Log with the same seeds reproduces identical state — this is how undo, tests and
generator rerolls stay exact. The clock is injected the same way.

---

## 7. Contracts

### 7.1 One spec, three transports

`openapi/v1/openapi.yaml` is **hand-authored** and the source of truth for:

1. **REST** paths (`/api/v1/...`) → ogen server interfaces and types.
2. **WebSocket messages**: every `Command` and `Update` is a component schema with a `kind`
   discriminator (`oneOf` + `discriminator`). ogen generates Go types; hey-api generates TS types
   and Zod validators. `asyncapi.yaml` documents the channel and `$ref`s these components; a test
   asserts every component referenced by AsyncAPI exists and every `Command`/`Update` variant is
   referenced.
3. **MCP tools**: tool input/output JSON Schemas are **generated** from the same components at build
   time (`task gen`), so an MCP tool and its REST twin cannot disagree.

### 7.2 Pipeline

```
1. spec gate    redocly lint (recommended-strict) && vacuum lint --fail-severity error && spectral lint (OWASP)
2. generate     ogen (Go) · hey-api (TS + Zod + vue-query) · MCP schema export
3. drift        git diff --exit-code api/internal/platform/api web/src/infrastructure/api openapi/generated
4. compat       api-contract-checks (oasdiff breaking vs main) — estate composite action
5. compile      go build ./... && vue-tsc --noEmit
```

### 7.3 Conventions

- Versioned `/api/v1`; keyset pagination on every collection; ETags on compendium reads.
- RFC 9457 problem+json for every error, declared per operation (4xx required by the linter).
- Retiring an endpoint takes three merges (deprecate → remove → replace), because oasdiff tolerates
  removing only paths already marked `deprecated` on `main` (estate lesson from agents-api).
- WebSocket messages are **additive**: never repurpose a field; a new behaviour is a new `kind`.
- No `publish-api-clients` for now: the only consumer (web) lives in the same repo and generates
  from the local spec. Publishing is a one-line workflow addition if another consumer appears.

---

## 8. Data

### 8.1 Principles (hard rules)

- PostgreSQL 16+, database `grimoire_db` on the estate's shared instance; schemas `compendium`,
  `campaign`, `prep`, `play`, `ops`.
- **No JSONB.** Typed columns, lookup tables + FK, child tables, join tables. The only exception is
  an opaque third-party blob kept for provenance (e.g. a raw Watabou export in S3, referenced by key),
  with an inline reason.
- **Integrity in the database.** NOT NULL, FKs, CHECK / enums, unique indexes,
  `CHECK (num_nonnulls(...) = 1)` for "exactly one of".
- Cross-context references to compendium entities are **two columns** `(slug, ruleset)`, never a FK
  (anti-corruption boundary).
- Speed comes from indexes (btree on FKs and filters, composite, covering, partial) — not documents.
- **goose** applies forward-only SQL migrations embedded in the binary (`grimoire migrate`);
  **squawk** lints them in CI and blocks locking or destructive changes; two-phase drops; backfills
  are separate migrations or River jobs ([ADR-0005](docs/adr/0005-goose-and-squawk-for-migrations.md)).
- sqlc queries live in `db/queries/<context>/*.sql`; no ORM, no lazy loading, so N+1 is
  structurally absent. A pgx tracer asserts query counts in adapter tests for list endpoints.

### 8.2 Schema inventory

The Grimoire (Rails) DDL is the baseline for compendium, campaign maps and play tables; it carries
over with the changes marked **Δ**. New tables are marked **new**. Agents write goose migrations from
this inventory plus the Open5e v2 field set.

**compendium**
- `documents` (key, license, ruleset_year, precedence, attribution); **Δ** `campaign_id` NULLABLE for
  campaign-scoped homebrew documents
- Lookups: `ability_scores`, `skills`, `damage_types`, `magic_schools`, `sizes`, `creature_types`,
  `movement_modes`, `sense_types`, `languages`, `proficiencies`; enum `die_faces`
- `conditions` + `condition_effects` **Δ** replaced by the typed effect tables below
- `spells`, `spell_classes`, `spell_damage`; `classes`, `class_saving_throws`,
  `class_proficiencies`, `class_levels`, `class_level_spell_slots`, `class_level_resources`,
  `subclasses`, `class_features`; `species` + speeds/senses/traits; `backgrounds`, `feats`
- `monsters` + speeds/senses/saves/skills/damage_relations/condition_immunities/languages/traits/
  actions/attacks; `cr_xp`
- `weapons` + `weapon_properties`, `weapon_masteries`, `weapon_damage`; `armor`; `equipment`;
  `magic_items` + `magic_item_effects`; `item_prices` (**new**: base price, rarity band)
- **new — typed effects** (§9.3): `effect_definitions` (owner: exactly one of spell / feature / item /
  condition / monster_action / surface / rule_option; activation, range, target shape, duration,
  concentration, automation_level `full|partial|manual`), `effect_damage`, `effect_heal`,
  `effect_saves`, `effect_attack`, `effect_apply_condition`, `effect_modifiers`,
  `effect_area` (shape, size_ft), `effect_surface`, `effect_triggers`, `effect_scaling`
- **new** `automation_coverage` materialized view (entity → automation_level) for the coverage report

**campaign**
- **Δ** `members`: `(campaign_id, account_id, display_name_override, role dm|player)`; unique
  `(campaign_id, account_id)`; Accounts live in the identity schema (ADR-0008)
- `campaigns` (progression_mode `xp|milestone`, table_vision_mode, reaction_timeout_s
  **new**, rolling_default **new** `physical|auto`)
- `characters` + `character_classes`, `character_ability_scores`, `character_skills`,
  `character_spells`, `character_senses`, `character_resources` (**new**: slots, hit dice, class
  resources, current/max), `character_feats` (**new**)
- **new** inventory: `containers` (character / party_stash / shop_stock / loot_drop — exactly one
  owner), `container_items` (item slug+ruleset or homebrew id, quantity, equipped, attuned),
  `currency_balances`
- `npcs` (**Δ** + `personality_*` typed columns, `settlement_id`, `faction_id`)
- `kb_entries`, `kb_links`; `boards`, `board_lanes`, `board_cards`
- `sessions` (durable: number, planned_on, status, recap, summary)
- `xp_awards`, `xp_thresholds`, `pending_level_ups`
- `rule_options`, `campaign_rule_settings`
- maps: `maps` (**Δ** + `storage_key` for the Garage object), `map_regions`, `map_region_hexes`,
  `map_terrain` (**Δ** + `elevation_ft`, `cover` `none|half|three_quarters|total`),
  `map_light_cells`, `map_nodes`, `map_edges`, `map_tokens`, `map_reveals`
- **new** `handouts` + `handout_recipients`; `chat_messages` (channel: party | whisper; recipients
  as child rows); `journal_entries`
- **new** `revisions` (entity_type, entity_id, revision_no, caller_subject, origin, created_at) +
  per-entity snapshot tables `<entity>_revisions` with the same typed columns as the live row
  (no JSONB); restore = copy back in one transaction

**prep** (**new** schema)
- `encounters`, `encounter_monsters` (moved from campaign)
- `regions` (campaign, world map, name, danger tier), `region_hexes` or `region_map_nodes`
- `encounter_tables` (region, check_chance_pct, die, visibility `secret|open`), `encounter_table_entries`
  (weight, exactly one of encounter_id / pool_id / `nothing`)
- `encounter_pools` (level_min, level_max), `encounter_pool_members` (monster slug+ruleset or
  homebrew, weight, min/max count), `encounter_pool_tags`
- `loot_tables`, `loot_table_entries` (weight, exactly one of item / currency / nested table,
  quantity dice), attachments to monsters/pools/encounters/shop types
- `encounter_checks` (trigger `rest|travel_leg|dm`, mode `normal|force_encounter|pick`, scheduled_for
  `next_rest|next_travel|at_time`, seed, rolled value, result entry, visibility)
- `settlements` (map node, size tier, wealth tier), `settlement_districts` *(later)*
- `shops` (settlement, type, owner npc, markup_pct, restock rule), `shop_types` + `shop_type_loot_tables`

**play**
- As Grimoire §8.5: `sessions` (live runtime row), `combatants`, `control_assignments`,
  `combats` (**Δ** renamed from `encounters`), `combatant_positions` (**Δ** + `elevation_ft`),
  `initiative_entries`, `combatant_effects` (**Δ** references `effect_definitions` or custom),
  `combatant_effect_modifiers`, `combatant_senses`, `light_sources`, `revealed_hexes`
- **new** `surfaces_active` (hex, kind, source effect, expires_round), `turn_economy` (combatant,
  action, bonus_action, reaction, movement_remaining_ft), `pending_reactions` (trigger, eligible
  combatant, expires_at, status)
- **new** `roll_requests` (who must roll, dice spec, reason, mode chosen `physical|auto`, status) +
  `roll_request_modifiers` (the breakdown rows shown on the Roll Card)
- Typed Action Log: `actions` (sequence, kind, caller member, caller origin, seed) + one detail table
  per kind (`action_token_moved`, `action_roll_resolved`, `action_attack`, `action_attack_damage`,
  `action_effect_applied`, `action_effect_removed`, `action_turn_advanced`, `action_hexes_revealed`,
  `action_reaction_resolved`, `action_surface_changed`, `action_encounter_spawned`, …)

**ops** — River job tables, `idempotency_keys` (command nonce → result), advisory-lock bookkeeping.

### 8.3 Object storage

Garage bucket `grimoire-assets` (private) for map images, handouts, portraits and raw imports.
Keys are content-addressed (`sha256/<hash>.<ext>`); rows hold the key. The API serves images through
short-lived signed URLs or a streaming endpoint that checks campaign membership — never a public
bucket, because private adventure maps live here. Display variants are generated by a River job
(`govips`/libvips) *(M4)*.

---

## 9. Rules engine

Pure Go under `api/internal/rules/`. No I/O, no DB, no clock or rng except injected. This is where
coverage (100%), mutation testing and property tests concentrate.

### 9.1 Core value objects

`Dice` (count, faces, modifier; parse "2d6+3"), `DiceSpec` (a list of dice groups plus keep-highest/
lowest rules — advantage is `2d20kh1`), `AbilityScore`/`Modifier`, `ProficiencyBonus`,
`ChallengeRating`, `DamageExpression` (dice + type), `HexCoord` (axial q, r), `HexLayout`,
`Feet`, `Elevation`.

### 9.2 Roll model

A **Roll Request** is what the engine needs from the table: `DiceSpec` + purpose (`attack`, `damage`,
`save`, `check`, `initiative`, `death_save`, `hit_die`, `haggle`, `encounter_check`) + a
**breakdown** of every modifier source (proficiency, ability, Bless +1d4, cover −2 on the target's
side, advantage from the Help action, …). The engine resolves a request from **faces**, regardless
of whether the faces were typed by a player or produced by the server's seeded rng. Physical and auto
rolls are therefore identical downstream (Grimoire §21.5, kept).

Bonus dice (Bless, Guidance, Bardic Inspiration) are part of the same `DiceSpec`, so the Roll Card
asks for them together ("1d20 + 1d4"). Rerolls (Lucky, Halfling) create a follow-up request.

### 9.3 Typed effects — how full automation works

Everything that does something — spells, class features, feats, items, monster actions, conditions,
surfaces, optional rules — is described as **data**: one `effect_definition` with typed component
rows. The Go side is a sealed union:

```go
type Component interface{ isComponent() }
type Damage struct{ Dice DiceSpec; Type DamageType; OnSave SaveOutcome /* half|none */ }
type Heal struct{ Dice DiceSpec; Temp bool }                 // Temp = temporary hit points
type SaveGate struct{ Ability Ability; DC DCSource }        // spell DC, fixed, or formula
type AttackGate struct{ Kind AttackKind; Bonus BonusSource }
type ApplyCondition struct{ Condition ConditionRef; Duration Duration; EndsOnSave bool }
type Modifier struct{ Target ModTarget; Value ModValue; Scope Scope }   // to_hit, ac, speed, adv, resist, ...
type Light struct{ BrightFt, DimFt Feet; Colour Colour; Sunlight bool }
type Reveal struct{ Strips []VisibilityQuality }            // Hidden, Invisible, Disguised, ...
type Sense struct{ Kind SenseKind; RangeFt Feet }
type Movement struct{ Kind MoveKind; DistanceFt Feet }      // push, pull, teleport, drag
type CreateSurface struct{ Kind SurfaceKind; Duration Duration }
type Summon struct{ Creature CreatureRef; Count CountSpec; Acts SummonTurn }
type Transform struct{ Form FormRef }                       // Wild Shape, Polymorph
type CreateObject struct{ Object ObjectRef }                // walls, barriers
type Dispel struct{ Level LevelSpec }
type Counter struct{ Level LevelSpec }
type GrantFeature struct{ Feature FeatureRef }
type ResourceChange struct{ Resource ResourceRef; Delta ModValue }
type Choice struct{ Options [][]Component }                 // the caster picks a mode
type Branch struct{ If Condition; Then, Else []Component }  // "fails by 5+", "HP <= 50", type filter
type Trigger struct{ On TriggerKind; Then []Component }     // reactions, "start of turn", "on hit"
type Scaling struct{ Per ScalingAxis; Add []Component }      // slot, character level, class level, table column
type Manual struct{ Instruction string }                     // the fallback (§9.4)
```

Every Effect also carries **Targeting** (self, creatures, sphere, cone, line, cube, cylinder,
emanation, wall, ring; size, range, origin; side filter; creature-type filter; sight required) and a
**Duration** (instant, rounds, minutes, hours, until dispelled, until a rest, permanent, end of next
turn, with repeat saves). These names match the glossary's Effect, Targeting and Visibility Quality
entries; Item Properties, Features, conditions, surfaces, traps, Rule Variants and Roll Table results
are all owners of Effects.

The **EffectEngine** folds a Combatant's active effects into an immutable `EffectProfile` (to-hit,
damage bonuses by type, AC delta, speed delta, advantage/disadvantage sources with reasons, save
modifiers, resistances, granted/forbidden actions, senses). `AttackResolver`, `SaveResolver`,
`MovementCalculator` and `VisionCalculator` consume profiles only; no service special-cases an item
or a spell by name.

Import maps Open5e fields to components where the data allows (damage dice, save ability, area,
condition names). What cannot be mapped automatically gets `automation_level = partial|manual`; the
coverage report (§2.1) lists them, and completing them is ordinary work (a migration of component
rows + tests), usually done through MCP by an agent and reviewed via Revisions.

### 9.4 Manual fallback

A `Manual` component resolves as a **DM prompt** in the live flow: the engine does everything it can
(rolls, saves, damage it understands) and then shows the DM the instruction plus quick actions
(apply condition, adjust HP, place surface). Players see "resolving…" until the DM confirms. The
Action Log records it like any other action. Nothing silently skips.

### 9.5 Combat services

- **InitiativeOrder** — ranks with simultaneous ties; optional tiebreak; group/popcorn variants.
- **TurnEconomy** — action, bonus action, reaction, movement, free object interaction; features that
  grant extra actions (Action Surge) are effects.
- **AttackResolver** — face(s) in, outcome out: advantage sources, cover, elevation (high ground +2
  is an optional rule; the default 5e treats it via cover/LOS), crit range, resistances,
  vulnerabilities, immunities, damage by type.
- **SaveResolver**, **CheckResolver** — same shape.
- **ReactionTriggers** — given an action about to resolve, returns the eligible reactions per
  Combatant (opportunity attack on leaving reach, Shield on being hit, Counterspell on a cast in
  60 ft, readied actions).
- **Concentration** — damage triggers a Con save request; failure ends linked effects.
- **Death & dying** — death saves, stabilise, massive damage (+ `massive_damage_instakill` option).

### 9.6 Spatial services

- **HexCoord / HexLayout** — axial coordinates, pointy-top; hex↔pixel conversion; distance.
- **MovementCalculator** — Dijkstra over hexes with terrain cost, elevation change (climb costs
  double without a climb speed), occupied hexes (allies passable, enemies not), Surfaces (difficult,
  dangerous), jump distance from Strength, flight/swim speeds. Returns the reachable set and a path
  per target hex with a cost breakdown.
- **LineOfSight / Cover** — hex ray casting with elevation; `map_terrain.cover` and intervening
  creatures produce half / three-quarters / total cover.
- **AreaTemplates** — cone, sphere, cube, cylinder, line, emanation, wall and ring rasterised to
  hexes at the Map's scale; origin rules per shape. Areas are drawn as tinted hexes, never circles.
- **ForcedMovement** — shove, push, pull; collisions and falling damage.

### 9.7 Vision & lighting

Grimoire §10.10/§27 carries over: per-Combatant senses, ambient + dynamic light, obscurement,
sight-blocking terrain → visible hex set. **Vision is shared by the whole party**: every Player and
the Table Display see the union of what any party member perceives right now. Sight is strict: an
unlit hex outside every sense is black, not hinted. Results feed Fog projection (§11.5), not client
rendering decisions.

### 9.8 Surfaces

`SurfaceKind` is a sealed enum (fire, grease, water, ice, web, acid, poison, darkness, difficult
rubble). A **SurfaceInteractions** table (data + tests) defines combinations: grease + fire → fire;
water + lightning → electrified water; water + cold → ice; fire + web → burn away. Entering or
starting a turn in a surface fires its effect components. Surfaces expire by duration and are
logged.

### 9.9 Rules on the client

Rules run **only on the server**. The client asks for previews (reachable hexes, path cost, hit
chance, expected damage, AoE targets) via cheap `Preview*` commands that never mutate state and are
answered in a few ms. The client keeps exactly one piece of geometry: hex↔pixel conversion for
rendering, in TS. **Golden fixtures** in `openapi/fixtures/hex/*.json` are generated by the Go
`HexLayout` and asserted by Vitest, so the two implementations cannot drift.

### 9.10 Budgets, progression, rests

`EncounterBudget` (2024 DMG, data-driven tables), CR→XP, XP thresholds, `RestService` (short/long,
hit dice, slot and resource recovery, gritty/epic variants, `slow_natural_healing`,
`healing_surges`). All tables are loaded from compendium data; nothing hardcoded beyond defaults for
tests.

### 9.11 Optional & homebrew rules

The Grimoire §26 catalogue carries over as `RuleSettings` injected into resolvers. Each toggle maps
to effect components or a resolver branch, with tests on both states. Adding a rule = seed row +
settings flag + component/branch + tests (+ mutation on the math).

---

## 10. Roll Cards (dice UX contract)

The table rolls **real dice by default**, and the app does the arithmetic.

1. A Roll Request is created (by the engine, the DM, or a reaction) and routed to the Controller's
   device(s) and, if open, the Table.
2. The **Roll Card** shows: the purpose ("Longsword attack vs Goblin"), **dice graphics for exactly
   what to throw** ("2 × d20, keep highest — advantage: Help from Lae'zel", "+ 1 × d4 — Bless"),
   and the modifier breakdown (BG3-style, each source named).
3. Each die offers two explicit buttons: **Roll for me** (server-seeded, logged; the die tumbles
   briefly and lands on the server's result) and **I rolled it** (a number pad from 1 to the die
   size). Mixing is allowed per die; "roll the rest for me" fills whatever is still empty. With
   advantage the kept d20 is highlighted and the dropped one dimmed.
4. The server resolves and broadcasts the outcome. Attack → hit/miss/crit → only then a damage Roll
   Card (crit shows doubled dice).
5. The DM can auto-roll for NPCs by default (per-NPC or per-campaign setting) and can force-resolve a
   stalled card.

Dice are drawn as shaded polyhedra on a felt tray; animation is a sub-second tumble that honours
reduced motion.
The Roll Card is the same component on every surface; the Table shows it read-only.

---

## 11. Live play runtime

### 11.1 Authority & flow

```
client intent (Command) ─▶ WS hub ─▶ Session goroutine (single writer)
                                     │ load state (cached) · validate via rules
                                     ├─ illegal → Rejected{reason}   (client rolls back optimistic UI)
                                     ├─ needs input → RollRequest / ReactionPrompt (flow pauses)
                                     └─ legal → one DB transaction:
                                              mutate play.* rows + append typed action (sequence++)
                                              commit ─▶ project per audience ─▶ broadcast Updates
```

### 11.2 One goroutine per live Session

A `SessionRuntime` goroutine owns an in-memory copy of the live state and reads commands off one
channel (single writer, no locks). Every accepted command is written through to Postgres in **one
transaction** before any broadcast; memory is a cache of committed state, never ahead of it. On
start (or after a restart) the runtime rebuilds from the `play.*` tables. `go test -race` guards the
hub.

### 11.3 Commands

The Grimoire §21.3 catalogue carries over, plus:

| Command | Effect | Engine |
|---|---|---|
| `StartSession` / `PauseSession` / `EndSession` | status, recap write-back | state machine |
| `SetActiveMap`, `PlaceToken`, `HideToken`, `RevealHexes`, `RevealRegion` | map state | fog rules |
| `StartCombat` (from Encounter, Pool draw, or ad hoc), `RollInitiative`, `EndTurn`, `EndCombat` | combat | InitiativeOrder, TurnEconomy |
| `PreviewMove`, `PreviewAttack`, `PreviewArea` | none (answered only to the sender) | Movement, Attack, Area |
| `MoveToken`, `Jump`, `Shove`, `Dash`, `Disengage`, `Dodge`, `Help`, `Hide` | positions, economy | Movement, ForcedMovement |
| `UseAction` (attack / spell / feature / item, incl. throw) | effects, HP, surfaces | EffectEngine + resolvers |
| `SubmitRoll` (faces or auto per die) | resolves a Roll Request | Roll model |
| `RespondReaction` (use / decline) | resolves a Reaction Prompt | ReactionTriggers |
| `ApplyEffect` / `RemoveEffect` / `AdjustHP` (DM) | overrides | EffectEngine |
| `ReassignControl` | control_assignments | authority |
| `DMChallenge` | Roll Request | Check/Save resolver |
| `RunEncounterCheck` / `ScheduleEncounterCheck` | prep → play | §13.2 |
| `Undo(toSequence)` (DM) | replay to sequence | Action Log |

Every command carries a client `nonce` (idempotent via `ops.idempotency_keys`) and the sender's
last seen `sequence`.

### 11.4 Reaction Prompts

When `ReactionTriggers` finds eligible reactions, the runtime **pauses** the triggering action,
sends a Reaction Prompt to each eligible Controller, and waits up to the campaign's
`reaction_timeout_s` (default 10 s). Timeout = decline. The DM can force-resolve. Multiple eligible
reactions are collected in parallel and applied in rules order. The prompt shows what triggered it
and the expected effect (e.g. Shield: "AC 15 → 20, the attack (18) would miss").

### 11.5 Projections & Fog

After each commit the runtime builds **per-audience projections**:

- **DM**: truth.
- **Player (per member)**: the party's shared vision and fog layers (never seen, remembered,
  visible now), own Character in full, allies' public state, only noticed enemies, only open
  Encounter Checks, own whispers/handouts.
- **Table Display**: the same party view, no controls, no secrets, framed by the DM's camera and
  scene.

Updates are computed from the projection, so a hidden token or undiscovered hex is **absent from
the payload**. Changes in visibility (a reveal) are sent as `Revealed{...}` updates that include
newly visible entities. This is the security boundary; §17 has negative tests for it.

### 11.6 Sequence, reconnect, undo

- Each Update carries the Session `sequence`. A client that sees a gap requests `Resync`, which
  returns its current projection (no log replay).
- Undo replays the Action Log from the last checkpoint to the target sequence with the logged seeds
  onto freshly reset state, in one transaction, then broadcasts a full `Resync` to everyone.
- Multi-step resolutions (a Fireball on six creatures with saves) commit as one transaction per step
  that needs input, and broadcast only committed steps, so no client sees a half-applied state.

### 11.7 Single replica now, multi-replica ready

The API runs as **one replica**. Two ports make scale-out a matter of adapters, not a rewrite:

- `SessionOwner` — acquires a Postgres **advisory lock** per live Session before starting its
  goroutine. Now: in-process adapter; the lock still exists so a rolling deploy can never run two
  owners. Later: other replicas forward commands to the owner.
- `Broadcaster` — fans Updates to subscribers. Now: in-process hub. Later: Postgres
  `LISTEN/NOTIFY` (payload = session id + sequence; subscribers read the projection) or NATS.

Deployment uses `strategy: Recreate`-equivalent semantics for the API (the manifest's rollout
settings) so the lock hand-off is clean; the WS client reconnects with backoff and resyncs.

---

## 12. Maps & space

- **Hex grid**, pointy-top, axial coordinates, **scale per Map**: 5 ft per hex on battle maps, miles
  per hex on world maps. Maps are calibrated (orientation, hex size px, origin, scale) so server and
  client agree (golden fixtures). A world map is a picture (the painted Default World or an upload)
  under a see-through grid the DM calibrates by placing two points a known distance apart.
- **World maps**: nodes (locations, Settlements), edges (routes, travel time, danger), Regions with
  Encounter Tables; party marker; "no map" mode. Travelling an edge is a **travel leg**, which can
  trigger Encounter Checks.
- **Local maps**: per-hex terrain cost, blocked, blocks-sight, **elevation_ft**, **cover**, light;
  regions with reveal rules; nodes/edges for rooms and doors; tokens with sizes.
- **Fog** layers: truth (DM) and discovered (persisted in `campaign.map_reveals`, so what was seen
  stays seen across Sessions).
- **Uploads**: images to Garage via a signed upload; variants by a River job.
- **Imports** *(M8)*: Watabou MFCG/Village GeoJSON and One Page Dungeon JSON, and Azgaar FMG
  GeoJSON/JSON become Maps, nodes, Regions and Settlements; the raw export is kept in S3 for
  provenance and attribution.

---

## 13. Prep & generation

### 13.1 Revisions

Every prep write (UI, MCP or generator) creates a **Revision** with caller and origin. The DM can
list, diff and restore. Revisions are how immediate MCP writes stay safe (§14.3).

### 13.2 Encounter Checks

- **Triggers**: short/long rest (`RestService` asks prep for the current Region's table), each
  travel leg, or the DM.
- **DM declarations**: force a check now; force an encounter (skip the chance, never Nothing);
  schedule a check (next rest, next travel leg, or an in-game time); pick the exact result.
- **Resolution**: roll the chance die (secret or open per table, via a Roll Request so physical
  dice work too), then a weighted draw over entries: a prepared Encounter, a Pool draw, or Nothing.
- **Pool draw**: `EncounterGenerator` fills the 2024 XP budget for the party's current levels and
  a target difficulty from the Pool's members (weights, min/max counts, tags), seeded and logged.
  Loot is rolled from attached Loot Tables when the Combat ends.
- The result can interrupt a rest (the rest resolves partially per the rules).

### 13.3 Loot

`LootGenerator` walks Loot Tables (weights, nested tables, quantity dice, currency) seeded per roll.
Drops land in a `loot_drop` container on the map or in the party stash; players drag items into
their inventory.

### 13.4 Settlements, Shops, Stock

- A Settlement has a size tier and wealth tier; Shops have a type, owner NPC, markup and restock
  rule.
- `ShopGenerator` fills Stock from the Shop type's Loot Tables, scaled by size and wealth; prices
  from `item_prices` × markup; magic items by rarity band (verify SRD 5.2 carries the price table;
  otherwise DM-set defaults).
- **Haggling** is a Roll Card (Persuasion/Deception/Intimidation vs the owner); the result adjusts
  price within configured bounds.
- **Restock** runs on long rest or elapsed in-game days (River job).
- Rerolling Stock creates a Revision.

### 13.5 Generator port & OSS integration

`generation` exposes a `Generator[T]` port per kind with seeded, pure implementations. Integration
modes (licenses verified 2026-09-29; the repo is AAL-licensed, so **no GPL/AGPL code is embedded**):

| Source | License | Mode |
|---|---|---|
| SolarLune/dngn (dungeons), zfedoran/go-wfc (battlemap tiles) | MIT | embed in Go |
| skeeto/fantasyname or ironarachne/namegen (names) | Unlicense / Apache-2.0 | embed/port in Go |
| Eigengrau's Generator, Cellule/dndGenerator (shop/NPC tables) | MIT | port tables to seed data |
| DnDGen TreasureGen (treasure tables) | MIT | port tables |
| Watabou MFCG / Village / One Page Dungeon | closed code, outputs free to use | import exports, credit |
| Azgaar Fantasy Map Generator | MIT | import exports (M8); Node sidecar for server-side world gen *(later)* |
| ComfyUI + FLUX.1 schnell (Apache-2.0) / SDXL | GPL-3.0 app, separate service | HTTP sidecar on a GPU node *(later)* |
| TownGeneratorOS, settlemaker, nortantis, MapTool | GPL/AGPL | inspiration only |

The FMG sidecar is a TypeScript runtime in production, which the Vue-TS blueprint otherwise
forbids for backends; it is accepted as an **isolated generator container** with no access to
campaign data beyond the request, pinned image digest, and pnpm supply-chain settings (§19.4).

---

## 14. AI & MCP

### 14.1 Server

- MCP over **Streamable HTTP** at `/mcp`, served by the same Go binary, calling the same use cases
  as REST. No separate MCP service.
- **Auth**: Grimoire **Access Tokens** (ADR-0009): scoped, expiring, revocable, minted by the Account
  holder in the UI. Tools run as that Account and check its Campaign role.
- Tool scope: every tool takes a `campaign` argument and requires the caller to be DM there
  (player-level tools *(later)*).

### 14.2 Tool catalogue (initial)

| Group | Tools |
|---|---|
| Lookup | `search_compendium`, `get_entity`, `get_campaign_state`, `get_session_state` (DM projection), `get_automation_coverage` |
| Prep | `upsert_encounter`, `upsert_encounter_pool`, `upsert_encounter_table`, `upsert_loot_table`, `upsert_settlement`, `upsert_shop`, `reroll_stock`, `upsert_npc`, `upsert_kb_entry`, `link_kb`, `upsert_homebrew`, `complete_effect_definition` |
| Generate | `generate_encounter` (pool/budget/seed), `generate_loot`, `generate_shop`, `generate_npc`, `generate_dungeon` |
| Import | `import_adventure_section` (map + regions + NPCs + encounters + KB from agent-extracted content), `import_watabou`, `import_fmg` |
| Live | `run_encounter_check`, `schedule_encounter_check`, `spawn_encounter`, `move_token`, `apply_effect`, `adjust_hp`, `reveal`, `push_handout`, `award_xp`, `undo` |
| History | `list_revisions`, `restore_revision`, `get_action_log` |

Tool schemas are generated from OpenAPI components (§7.1). Every tool response includes the created
Revision ids or Action Log sequence so the agent can undo precisely.

### 14.3 Immediate writes, safely

Writes apply immediately (decided). Safety comes from: Revisions for prep data, the Action Log
for live play, `origin = mcp:<client>` on both, rate limits per member on `/mcp`, and a DM-visible
activity feed of MCP actions in the DM surface.

### 14.4 No LLM inside Grimoire

Grimoire holds no model API key. Generation is deterministic and seeded in Go; the DM's own agent
provides creativity (names, flavour, curation, adventure extraction) through MCP. Private adventure
content (e.g. an owned module PDF) is read by the agent on the DM's machine and written through MCP
into Postgres/S3 — it never enters the repository.

---

## 15. Frontend

Follows the [Vue + TS SPA blueprint](docs/blueprints/vue-ts-spa.md) in full except where stated.

### 15.1 Stack

Vue 3.5, TypeScript 5.9 strictest, Vite 8, Vitest 4, vue-tsc, pnpm 11 (security defaults on),
`@hey-api/openapi-ts` (pinned exact) with sdk + Zod + `@tanstack/vue-query`, Pinia (client state
only), vue-router, vue-i18n, PixiJS 8 for the map canvas, `@vueuse/core`, Capacitor 7.
Reused from `@jorisjonkers-dev/vue-web-commons`: `useAuth`, `api-runtime`, Vite/ESLint presets,
nginx preset, Faro observability. **Not** reused: its visual components and tokens (§16).

### 15.2 Layers & slices

presentation → application (composables, query hooks, stores) → domain (pure TS) → infrastructure
(generated client, WS client). Feature slices: `compendium`, `character`, `campaign`, `kb`, `boards`,
`prep`, `session`, `combat`, `map`, `rolls`, `inventory`, `shops`, `chat`, `handouts`, `journal`,
`settings`. `eslint-plugin-boundaries` enforces both.

### 15.3 Realtime client

`realtime/` holds one WebSocket per live Session with typed `Command`/`Update` from the generated
client and Zod validation on every incoming frame. A `sessionStore` applies Updates by `sequence`,
requests `Resync` on a gap, and exposes read-only projections to components. Optimistic UI only for
the sender's own intents (a planned move), rolled back on `Rejected`. Reconnect with backoff;
visible connection state.

### 15.4 Map rendering (hybrid)

- **PixiJS canvas layer**: map image, elevation shading, Surfaces, light/darkness, Fog. Redraws only
  on projection change.
- **SVG overlay**: hex hit-targets, tokens, selection, path preview, AoE preview, reach highlights,
  initiative badges. Every interactive element is a DOM node with a stable `data-testid`, so
  Playwright/Vitest assert the DOM; the canvas exposes a `window.__grimoireScene` test hook in
  non-production builds for layer assertions.
- Pan/zoom with pointer + touch gestures; tap targets ≥ 44 px at the default zoom on phones.

### 15.5 BG3-inspired combat UI (component inventory)

Hotbar (attacks, spells, items, actions; greyed with reason when unavailable), action economy pips,
concentration indicator, spell slot and resource pips, hit % + damage range on target hover/tap,
path preview with feet cost and difficult terrain, AoE template with affected-creature highlights,
Reaction Prompt with countdown, Roll Card (§10), initiative rail with portraits and tied ranks,
condition icons with nested tooltips, turn banner ("Your turn"), combat log (from the Action Log),
DM override drawer. Animation budget: ≤ 200 ms for UI transitions; the exceptions are the dice
(ADR-0012: roll in, glide to centre, total), the initiative reveal and the "It's your turn" banner.
Reduced motion replaces all three with an immediate result.

### 15.6 Surfaces & responsive contract

| Breakpoint | Primary surface | Layout |
|---|---|---|
| Phone ~390×844 | Player | map full-screen with bottom hotbar; sheet as a drawer; Roll Card as a bottom sheet |
| Tablet ~1024×768 | DM | split: initiative + map + selected statblock |
| Desktop ≥1280 | DM / Player | multi-pane |
| TV ≥1920 landscape | Table | map-dominant, large initiative rail, big type, no controls |

Container queries compose shared components per breakpoint. No hover-only affordances on Player or
Table.

### 15.7 PWA & Capacitor

- **PWA**: installable, full-screen, Screen Wake Lock during a live Session; service worker caches
  the app shell, compendium reads and the member's own sheets for offline reading. Live play shows an
  explicit offline state.
- **Capacitor 7** shells for Android/iOS, following agents-ui: the WebView loads the hosted URL and
  keeps `*.jorisjonkers.dev` in-app so the Grimoire session cookie works; adds native push (turn
  notifications, Reaction Prompts while locked), haptics on "your turn", and keep-awake. APK built
  in CI; iOS simulator build in release, store builds *(later)*.
- **Table**: any browser in kiosk mode on the TV; a `/table/<session>` route with a join code.

### 15.8 i18n & a11y

All strings through vue-i18n (English now). axe in Playwright is a gate; colour is never the only
signal (hit/miss also by icon/text); focus management for Roll Cards and prompts.

---

## 16. Design process — Claude Design first

M0 starts with design, before any UI code:

1. **Claude Design mocks** of the three surfaces and the key flows: character sheet + hotbar on a
   phone, a full combat turn (move → attack → Roll Card → damage), a Reaction Prompt, a fog reveal
   on the Table, DM encounter check + spawn, shop with haggling, level-up moment.
2. A **design system** extracted from the approved mocks: tokens (`design/tokens.json` → CSS
   variables), typography, iconography (conditions, damage types, economy pips), component
   inventory matching §15.5.
3. The Vue build implements components from that inventory; Playwright responsive snapshots are
   compared against the approved states.

Visual direction: BG3-inspired dark fantasy UI with high legibility on phones and TVs, restrained
motion. Grimoire has its **own theme**; vue-web-commons provides plumbing only.

---

## 17. Testing & gates

### 17.1 Layers

- **Rules unit tests** (Go, no I/O): table-driven + **property tests** (`rapid`) for dice, movement,
  AoE rasterisation, LOS, initiative, effect folding, budgets. **100% statement coverage** on
  `internal/rules/...`; **gremlins** mutation testing.
- **Use case tests** with in-memory fakes of ports.
- **Adapter tests**: sqlc repositories against **testcontainers Postgres** with goose migrations
  applied; query-count assertions on list endpoints; S3 against a MinIO/Garage container;
  `httptest` for Open5e/5e-bits (recorded fixtures, no network).
- **Contract tests**: ogen server in-process; every operation's examples validate against the spec;
  golden fixtures for hex geometry and WS message samples shared with the web tests.
- **Runtime tests**: `SessionRuntime` driven by scripted command sequences with fixed seeds; replay
  equals live state; `go test -race`.
- **Fog / authority negatives (blocking security tests)**: for each Update kind and each audience,
  hidden tokens, undiscovered hexes, secret checks and DM notes are **absent from the serialized
  payload**; non-Controllers cannot act for a Combatant; MCP tools refuse non-DM callers.
- **Web**: Vitest for domain/composables (100% on domain), Vue Test Utils for components, MSW from
  the spec.
- **E2E**: Playwright device matrix (phone, tablet, desktop, TV) plus **multi-client scenarios** in
  one test (DM + two Players + Table): move propagation, illegal move rollback, tied initiative,
  Roll Card physical and auto, Reaction Prompt timeout, fog negative in the DOM, reconnect/resync,
  encounter check secret vs open. axe on every surface. Responsive snapshots reviewed as diffs.

### 17.2 Gates (tiered)

**Every PR (blocking):**

- the contract chain from §7.2 (spec lint, generate, drift, oasdiff, compile)
- `golangci-lint` (incl. nilaway, exhaustive, gochecksumtype, exhaustruct, depguard, gosec)
- `go test -race ./...`, with coverage 100% on `rules` and a global floor of 98% (web: 98% lines, 95% branches)
- gremlins on **changed** `rules` packages
- `squawk` on migrations and `migration-guard`
- `govulncheck`
- `vue-tsc`, ESLint (boundaries + a11y), Vitest with coverage
- Playwright device matrix + multi-client scenarios + axe + fog negatives
- `pnpm audit` with frozen lockfile

**Nightly / dispatch:** full gremlins run, full Playwright snapshot suite, import against live
Open5e into a scratch DB (drift report), Capacitor Android build.

### 17.3 Speed budget

`task check` < 5 min on a laptop; Go unit tests < 30 s; PR CI wall time < 10 min. Because the repo is
public, standard runners are free, so CI **may** split into parallel jobs for wall-clock speed; the
estate's one-job rule targets private-repo billing and does not apply here.

---

## 18. Conventions for agents

### 18.1 Recipes

**Add a compendium entity:** goose migration → sqlc queries → domain type + port → repository →
import mapping (Open5e + 5e-bits cross-check) → effect components if it does anything → use case →
OpenAPI paths → `task gen` → handler → tests (unit, adapter, contract) → attribution.

**Add a rules behaviour / effect component:** new variant in the sealed union (compiler + linter
point at every switch) → EffectEngine/resolver handling → property + table tests → gremlins clean →
map import data to it → coverage report shrinks.

**Add a live command:** OpenAPI component (`Command` variant + resulting `Update` variants) →
`task gen` → domain command + validation via rules → runtime handler (one transaction + typed
Action Log detail table) → projection rules per audience → fog negative test → Playwright multi-client
scenario → web handler in `sessionStore`.

**Add an MCP tool:** reuse or add a use case → OpenAPI component for input/output → `task gen` →
register tool in `platform/mcp` with DM authorization → response carries Revision ids/sequence →
tool test with a fake MCP client.

**Add a generator:** pure `Generator[T]` with seed → golden output tests per seed → use case that
writes through Revisions → UI + MCP entry points.

### 18.2 Definition of Done

- All §17.2 PR gates green.
- New unions exhaustive without `default`.
- New data enforced by DB constraints.
- New live Updates projected per audience, with a fog negative test.
- New writes attributed (caller + origin) and recoverable (Revision or Action Log).
- Spec, generated code and fixtures committed.
- Strings in i18n.
- New SRD/OSS content attributed in `ATTRIBUTION.md`.

### 18.3 Style

Imperative use-case names (`MoveToken`, `RunEncounterCheck`); ports named `…Repository`,
`…Gateway`, `…Generator`; glossary terms from `CONTEXT.md` in code, UI copy and API names (e.g.
`Combatant`, never `actor`). Sparse comments; reasoning in PR bodies and ADRs.

---

## 19. Platform, deployment & operations

### 19.1 Deployment

- One image, `grimoire`: a distroless static Go binary that also serves the built SPA (embedded at
  build time, SPA fallback, immutable hashed assets, strict CSP). Built and pushed by `publish.yml`
  on every release tag; deployed via `platform/deployment.yml`
  (v2) with pinned digests in `images.lock.json`, through the estate's publish → `deploy/production`
  → Flux flow.
- Route: `grimoire.jorisjonkers.dev`. Until Grimoire Accounts ship the route stays behind the
  estate's forward-auth; afterwards Grimoire handles sign-in itself (ADR-0008), `/mcp` takes Access
  Tokens (ADR-0009), and `/healthz`, `/readyz` and the Public Compendium are anonymous.
- **Platform onboarding** (identity allow-lists, database and credentials, object storage bucket,
  native login redirect) happens in the estate's private infrastructure repositories. The checklist
  lives there, not in this public repo. Each step is verified by reading the state back, not by exit
  codes.
- Migrations run as a one-shot `grimoire-api migrate` job before rollout (manifest
  `migrationPolicy`), with `rollbackTargetRetention` set.

### 19.2 CI (GitHub Actions)

The repo-template skeleton (`ci.yml` ending in `Pipeline Complete`, release-please, rulesets,
add-to-project) plus:

- a **`go-ci`** reusable workflow in `github-workflows` (new; setup-go with cache, golangci-lint,
  race tests, coverage gate, gremlins-changed, govulncheck, squawk, testcontainers)
- the existing `node-ci` for `web/`
- `api-contract-checks` for oasdiff
- Playwright job with the device matrix
- Android APK workflow modelled on agents-ui

The first real CI run must be checked by `event` and `headSha`, not by `status` alone.

### 19.3 Security

- Fog is enforced server-side (§11.5), with negative tests.
- Authorization in every use case: member of campaign; DM for DM operations; Controller for
  Combatant commands.
- Rate limits per member on commands and `/mcp`.
- CSP locked to self + API + WS.
- Signed, short-lived URLs for assets.
- Private content lives only in Postgres and S3.
- Secrets via the platform secret store, never in the repo.
- Generic user-facing errors; detail goes to logs and traces.

### 19.4 Supply chain

- **Go:** `go.sum` committed; `GOFLAGS=-mod=readonly`; `go mod verify`; `govulncheck`; minimal deps.
- **JS:** pnpm 11 defaults (`minimumReleaseAge` ~1 week, blocked build scripts with allowlist,
  `blockExoticSubdeps`), `--frozen-lockfile`, `pnpm audit`.
- **Dependency updates:** Renovate via the estate preset, with cooldown.
- **Actions:** pinned to SHAs.

### 19.5 Observability

- **Logs:** `log/slog` JSON logs to Loki, with request/session/command ids.
- **Traces and metrics:** OTel traces to the estate collector. Metrics per command kind: latency,
  rejects, reaction timeouts, WS connections.
- **Frontend:** Faro, correlated by trace id.
- **Dashboard:** live Sessions, command p95, and resync rate.

---

## 20. Milestones

Each milestone ends with all gates green, and with a demo on phone + TV where UI is involved.

| M | Scope | Exit criteria |
|---|---|---|
| **M0** | Claude Design mocks + design system; repo from template (AAL license, CONTEXT, ADRs); mise/Taskfile; Go + Vue skeletons; spec pipeline; every §17.2 gate green on an empty domain; deploy a hello-world through the estate platform; verify auth-api OAuth support for MCP | mocks approved; `grimoire.jorisjonkers.dev/healthz` live behind forward-auth; WS handshake passes CORS |
| **M1** | Compendium schema + Open5e/5e-bits import + blend resolver + read API + browser + attribution | all SRD entities browsable; import idempotent; cross-check report |
| **M2** | Rules core: dice/Roll Requests, typed effects, EffectEngine, attack/save/check resolvers, initiative, hex/movement, budgets; automation coverage report | property + mutation gates green; coverage report published |
| **M3** | Campaigns, members, invites, character builder, level-up, out-of-combat editing, NPCs, KB, Encounters, homebrew, Revisions | a party of four built on phones; restore of any prep edit |
| **M4** | Live Session spine: WS, runtime goroutine, Action Log, projections, Fog, tokens, maps upload, turns, Roll Cards, DM challenge, undo, control reassignment; **MCP server + DM tools** | multi-client E2E green; fog negatives green; DM runs a session from Claude |
| **M5** | Full combat UX: hotbar, pips, hit %, path + AoE previews, reactions, surfaces, elevation/cover, LOS/vision/lighting, throwables, optional rules, XP + level-up moment | a scripted 5-round fight with every mechanic, on the device matrix |
| **M6** | Prep: Regions, Encounter Tables/Pools/Loot, Encounter Checks (all triggers and declarations), world-map travel, Settlements/Shops/Stock/haggling, inventory + stash + loot | a rest and a travel leg roll encounters; a town with three generated shops |
| **M7** | KB depth, boards, handouts, chat/whispers, journal, rest/camp, resources | full between-session loop |
| **M8** | D&D Beyond import, adventure import via MCP, Watabou/FMG imports, dungeon/battlemap generation, Capacitor push | a module section imported end to end |
| **M9+** *(later)* | Procedural towns, FMG sidecar, ComfyUI images, multi-replica runtime, store builds | per ADR |

---

## 21. Licensing & attribution

- **Code**: **Attribution Assurance License** (OSI-approved, BSD-style). Forks must keep the notice
  and **prominently credit the original author in their user interface**. Grimoire shows the credit
  in its own About/footer as the reference implementation. Replaces repo-template's
  source-available LICENSE for this repo only.
- **SRD 5.2**: CC-BY-4.0, with the required attribution in `ATTRIBUTION.md` and the in-app
  attribution surface. 5e-bits data is under OGL 1.0a: include the OGL text and Section 15 when it is
  used.
- **Excluded from the SRD** (e.g. Artificer, Aasimar, Beholder): only as campaign-scoped homebrew,
  never shipped in the repo.
- **Watabou/Azgaar outputs**: credited on the imported Map and in `ATTRIBUTION.md`.
- **Embedded OSS**: listed with licenses in `ATTRIBUTION.md`; CI checks licenses of Go modules
  (`go-licenses`) and npm packages against an allowlist (no GPL/AGPL).
- **Private adventures and homebrew**: never in the repo (§1.4).

---

## 22. Risks & open items

| Risk | Mitigation |
|---|---|
| MCP clients expect OAuth discovery that Access Tokens don't provide | Access Tokens work as bearer tokens in Claude connectors and scripts today; add OAuth discovery only if a client requires it |
| Full automation scope is enormous | typed effects + manual fallback + coverage report make progress incremental and visible |
| Single replica restarts drop live WS connections | fast startup (static binary), advisory-lock hand-off, client auto-resync; multi-replica ADR ready |
| Go lacks native sum types | gochecksumtype/exhaustive as blocking gates; review forbids `default` escapes |
| D&D Beyond JSON is unofficial | import is optional, isolated behind a port, failures reported not fatal |
| SRD 5.2 magic item price table availability | verify in M6; DM-set defaults otherwise |
| No Go reusable workflow in the estate yet | add `go-ci` to github-workflows in M0 |
| PixiJS canvas harder to test | SVG overlay carries all interaction; scene test hook; screenshot diffs |

---

## 23. Play modes & interaction model

Settled in the design review of the Claude Design mockups (§16). These rules bind every surface.

### 23.1 Exploration and combat

- **Exploration** is the default whenever no hostile creature is noticed. There is **no
  initiative**: everyone taps a visible, reachable hex and their token **walks** the path at a fixed
  walking pace on every surface, so the Table Display sees them arrive a moment later.
- **Combat** starts the instant a hostile creature is noticed, an Encounter Zone springs, or the DM
  starts it. The **initiative rail appears on the Table Display**, the DM console and every phone,
  showing whose turn it is; tied ranks act together in any order.

### 23.2 Maps: world and local

- A campaign has **world maps** (locations, routes, Regions) and **local maps** (hex tactical
  maps). Everyone can switch **World ↔ Local**; the DM also switches what the Table Display shows.
- Local fog has three layers for players: **never seen** (black), **remembered** (dimmed, what the
  party saw before, persisted across Sessions) and **visible now** (lit). The DM sees everything,
  with never-seen areas hatched.
- **Strict sight**: light sources, darkvision and line of sight decide what is visible. What the
  party cannot perceive is not drawn and is absent from the payload (§11.5).

### 23.3 Encounter Zones, perception and surprise

- The DM places **hidden creatures** (with Stealth) and draws an **Encounter Zone** on a local map.
- A zone springs **automatically** when any party member comes within its radius, or when the DM
  chooses (**Spring it now / Hold off**, or DM-trigger only zones).
- On triggering: passive Perception is compared with Stealth first; anyone who missed gets a
  Perception Roll Card; the DC stays hidden. A noticed creature is revealed to the whole party.
- Anyone still unaware when combat starts is **surprised** (2024 rules: initiative at
  disadvantage), shown on the initiative rail.

### 23.4 One way to act

- The DM acts for creatures with **the same hotbar, target preview and Roll Card** players use.
  There are no separate DM action pages; tapping any token acts as it. HP and effect edits live
  under the same hotbar.
- **Suggested actions** for creatures (`rules/tactics`, pure and tested):
  - **Simple** (Int 11 or less): attack the nearest visible enemy.
  - **Cunning** (Int 12 or more): if it has a ranged attack, target the enemy it has *seen* deal
    the most damage from range; otherwise nearest.
  - **Never too clever**: only observed information (no hidden HP or AC), no coordinated focus
    fire across creatures.
  - Per-creature override: From Int / Simple / Cunning / Off. The suggestion shows its reason and
    one tap (**Use suggestion**) performs it.

### 23.5 Table Display remote

The Table Display is steered from any DM session, phone or desktop: camera (**follow turn**,
**show party**, free pan/zoom), **scene** (local map, world map, handout, title card), **ping** and
**blackout**. It only ever renders the party view.

### 23.6 Portraits and token icons

Players upload a **portrait** and choose a **token icon** (crop from the portrait, a separate
upload, or initials). Images are campaign-private objects (§8.3). The party ring (round) or enemy
frame (hexagonal) is always drawn over the icon so allegiance stays readable without colour.

### 23.7 Visual language

The **Soft** component system on a dark ground (not parchment): 5 px controls, 8 px panels,
brass primary buttons, a gold underline for the current tab, floating-label fields, rows instead of
cards. Cinzel for titles, Marcellus for labels and navigation, Alegreya Sans for UI and numbers,
Alegreya for rules and lore. In play the map is always underneath and panels are see-through glass.
Party tokens are round, enemies hexagonal, hidden creatures dashed (DM only). Touch targets are at
least 44 px. The Claude Design canvas "Grimoire UI mockups" is the visual reference.

---

## Appendix A — Relationship to the source documents

- **The earlier Grimoire design (Rails)**: its domain (§1, §3, §8 tables, §9–§10 rules, §14.1 surfaces,
  §21–§28 live play, maps, boards, control, XP, optional rules, vision, challenges) is carried
  forward with the changes above. Its stack sections (Rails, RBS/Steep, Packwerk, Kamal, Solid
  Cable, Rails auth) are superseded.
- **[Go API blueprint](docs/blueprints/go-api.md)**: adopted for the API. Deviations: realtime is required, not optional; one
  spec also carries WS and MCP schemas; property tests added; CI may parallelise (public repo).
- **[Vue + TS SPA blueprint](docs/blueprints/vue-ts-spa.md)**: adopted for the web app. Additions: realtime client, hybrid canvas/SVG
  map, PWA + Capacitor shells, i18n. The FMG sidecar exception to "no TS backend" is scoped in §13.5.
- **Rails, Rust, .NET and Spring/Kotlin blueprints**: evaluated; not chosen ([ADR-0001](docs/adr/0001-go-backend-not-estate-kotlin.md)).
