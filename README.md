# Grimoire

A self-hosted D&D 5e table companion. Players use their phones, the DM uses a laptop or tablet, and
a TV shows the shared table. The server computes the rules — modifiers, ranges, hit chances,
effects, fog of war — and keeps every hidden thing hidden. Combat takes its cues from Baldur's Gate 3
(hotbar, action economy, hit-chance previews, reaction prompts, roll cards) with far less animation.
Players can still roll real dice: the app shows exactly which dice to throw and does the arithmetic.

A DM can also prep and run sessions from an AI agent through Grimoire's MCP server: generate
encounters from per-campaign pools, roll random encounters on rests and travel, stock town shops,
and import their own adventures.

> **Status:** pre-alpha. The repository currently holds the architecture, conventions and an empty
> skeleton. Design mockups come next, then milestone M1 (see [ARCHITECTURE.md §20](ARCHITECTURE.md#20-milestones)).

## Stack

| Part | Tech |
|---|---|
| API | Go 1.26 · spec-first OpenAPI 3.1 (ogen) · sqlc · goose + squawk · WebSocket live runtime · MCP |
| Web | Vue 3.5 · TypeScript (strict) · Vite · generated client + Zod · TanStack Query · Pinia · PixiJS + SVG map |
| Apps | Installable PWA · Capacitor 7 shells for Android/iOS · kiosk browser for the Table |
| Data | PostgreSQL 16+ (normalised, no JSONB) · S3-compatible object storage |

## Documents

| Document | What it is |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | The source of truth: scope, contexts, contracts, data, rules engine, live play, gates, milestones |
| [CONTEXT.md](CONTEXT.md) | The glossary. Use these words in code, API and UI |
| [docs/adr/](docs/adr/) | Decisions that are hard to reverse |
| [docs/blueprints/](docs/blueprints/) | Generic Go API and Vue SPA blueprints this project specialises |
| [CLAUDE.md](CLAUDE.md) | Rules for AI agents working in this repo |
| [ATTRIBUTION.md](ATTRIBUTION.md) | Credits for SRD content and third-party work |

## Development

Toolchain versions are pinned in [`mise.toml`](mise.toml); every workflow is a
[Task](https://taskfile.dev) target.

```sh
mise install        # Go, Node, pnpm, task, sqlc, squawk, golangci-lint, actionlint
task dev            # Postgres (Docker), API with a dev identity, Vite
task                # list targets
task check          # everything CI runs on a pull request
task test           # unit tests
```

## Layout

```
api/        Go module: cmd/grimoire (composition root) and internal/<context>
openapi/    The hand-authored OpenAPI 3.1 contract and its lint config
web/        Vue app (arrives with the design system)
design/     Design exports and tokens (arrives with the Claude Design mockups)
docs/       ADRs and blueprints
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security issues: [SECURITY.md](SECURITY.md).

## License

[Attribution Assurance License](LICENSE). Forks and redistributions must credit **Joris Jonkers**
(<https://jorisjonkers.dev>) visibly in their user interface and in their documentation. SRD content
is used under CC-BY-4.0; see [ATTRIBUTION.md](ATTRIBUTION.md).
