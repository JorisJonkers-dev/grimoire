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
| Apps | Installable PWA · Capacitor 8 shell for Android · kiosk browser for the Table |
| Data | PostgreSQL 16+ (normalised, no JSONB) · S3-compatible object storage |

## Documents

| Document | What it is |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | The source of truth: scope, contexts, contracts, data, rules engine, live play, gates, milestones |
| [CONTEXT.md](CONTEXT.md) | The glossary. Use these words in code, API and UI |
| [docs/adr/](docs/adr/) | Decisions that are hard to reverse |
| [docs/blueprints/](docs/blueprints/) | A pointer to the generic Go API and Vue SPA blueprints this project specialises, in `template-go-vue` |
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

## Connecting an AI agent

Grimoire serves MCP over Streamable HTTP at `/mcp`.

- **Identity:** requests are authenticated the same way as the web app. The platform's forward-auth puts the account in `X-User-Id`. Set `GRIMOIRE_OAUTH_ISSUER` to publish `/.well-known/oauth-protected-resource`, so a connector can find the authorization server to sign in with.
- **Access:** every tool that takes a `campaignId` works only for that campaign's DM.
- **Writes:** writes apply at once and return the Revision they recorded. The campaign's **AI activity** page lists the agent's changes, and each one can be undone.
- **Live tools:** live tools act in a running Session with the same authority as the DM's own connection. Examples are `spawn_encounter`, `reveal_area`, `apply_effect`, `adjust_hp` and `run_encounter_check`. Each one returns the Action Log sequence it recorded. `undo_action`, or the session page's Action Log, reverts it.
- **Adding a tool:** mark an OpenAPI operation with `x-mcp: { tool: <name>, entity: <revisioned type> }` and run `task gen`. See [ADR 0007](docs/adr/0007-mcp-tools-run-through-the-rest-router.md).

## Phones and the Android app

- **Install:** Grimoire is an installable PWA. The compendium and Character sheets a device has opened stay readable offline, and the app says when live play is waiting for the connection.
- **Accounts:** Grimoire runs its own Accounts and sessions (ADR-0008). List the subjects that may send the first Account Invite in `GRIMOIRE_ADMIN_SUBJECTS` (comma-separated); after that, Admin Accounts invite from their Account page. Emailed sign-in links point at `GRIMOIRE_BASE_URL` and go out through `GRIMOIRE_SMTP_ADDR` and `GRIMOIRE_SMTP_FROM` (with `GRIMOIRE_SMTP_USERNAME` and `GRIMOIRE_SMTP_PASSWORD` when the server needs them); without an SMTP server the email is written to the log. While a forward-auth proxy still sits in front, Grimoire trusts its `X-User-Id`; set `GRIMOIRE_TRUST_FORWARD_AUTH=false` once it does not, so only a session cookie can say who someone is.
- **External login:** an OIDC provider can be offered beside passwords. Set `GRIMOIRE_OIDC_ISSUER` and `GRIMOIRE_OIDC_CLIENT_ID` (and `GRIMOIRE_OIDC_CLIENT_SECRET` for a confidential client; PKCE is always used), with `GRIMOIRE_BASE_URL` set, and register `<GRIMOIRE_BASE_URL>/oidc/callback` as the redirect URI. Only a login whose ID token lists `GRIMOIRE_OIDC_GRANT_ROLE` (default `SERVICE_GRIMOIRE`) in the `GRIMOIRE_OIDC_ROLES_CLAIM` claim (default `roles`) may sign in; `GRIMOIRE_OIDC_ADMIN_ROLE` (default `ROLE_ADMIN`) also grants that, and makes the Account an Admin at each sign-in. `GRIMOIRE_OIDC_NAME` is what the sign-in page calls it (default: the issuer's host). An Account created from a login keys on the login's own subject, so whatever that subject joined through forward-auth stays its own.
- **Two-step sign-in:** any Account with a password can add an authenticator app (TOTP) with ten recovery codes from its Account page; a password or emailed-link sign-in then asks for a code. An Admin Account holds its Admin powers only in a strong session — one that passed two-step, came from the external login, or arrived through forward-auth — so an Admin who signs in with a password alone must turn two-step on first.
- **Access Tokens:** an Account mints tokens for MCP clients and scripts on its Account page (ADR-0009). A token is sent as `Authorization: Bearer gmt_…`, acts as its Account, and is held to its scopes: `read` for every read, `build` for changing Campaigns and their prep, `play` for acting in Sessions. No token can manage the Account itself (sign-ins, two-step, other tokens), nor hold Admin powers. Each operation's scope is its `x-ogen-operation-group` in the OpenAPI spec.
- **Admin:** the Admin page lists every Account and every unused Invite, and a page per Account shows how it signs in, its Campaigns and its history, with controls to email a sign-in link, make or remove an Admin, reset two-step, and disable or enable it. Disabling ends every session and Access Token at once; nobody removes their own Admin role or disables themselves. Every change lands in the Account's history, which its holder also sees.
- **Members and Characters:** the migration to Accounts turns every Campaign Member into an Account on the same subject (no password, no email yet), and their external login links to it on its first sign-in. Every existing Character becomes a Character owned by that Account plus its one Campaign Character (ADR-0010); a Character then joins further Campaigns from the Characters page with its own progress there, while its name, Portrait and token follow it everywhere.
- **Notifications:** each kind reaches an Account in app, on the devices it opted in on (Web Push, with the VAPID keys above) and by email, as its preferences say. Security email goes out at once; every other email waits for a Digest, sent at most hourly (a single waiting Notice goes as its own email). Every email renders from one template in `internal/platform/mail/letters`, whose golden files are rewritten with `go test ./internal/platform/mail/letters -update`.
- **Release Notes:** an Admin drafts the one Release Note of each full release from the Admin page; the draft lists the Features that release's section of `CHANGELOG.md` (read from `GRIMOIRE_CHANGELOG`, default `CHANGELOG.md`; the image ships it at `/CHANGELOG.md`) records. Published now or scheduled, it rings every bell once live and shows on each Account's Dashboard until they dismiss it.
- **Notifications:** set `GRIMOIRE_VAPID_PUBLIC_KEY`, `GRIMOIRE_VAPID_PRIVATE_KEY` and `GRIMOIRE_VAPID_CONTACT` to send Web Push. A player taps "Notify me on my turn" in a live Session, and the device then hears about their turn and Reaction Prompts while locked. Without the keys the server sends none.
- **Android:** `task android` builds the Capacitor shell as a debug APK (Java 21 and the Android SDK). Set `GRIMOIRE_APP_URL` at build time to load a hosted server instead of the bundled build. The screen stays awake during a live Session.

## Layout

```
api/        Go module: cmd/grimoire (composition root) and internal/<context>
openapi/    The hand-authored OpenAPI 3.1 contract and its lint config
web/        Vue app (arrives with the design system)
design/     Design exports and tokens (arrives with the Claude Design mockups)
docs/       ADRs, and a pointer to the blueprints
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security issues: [SECURITY.md](SECURITY.md).

## License

[Attribution Assurance License](LICENSE). Forks and redistributions must credit **Joris Jonkers**
(<https://jorisjonkers.dev>) visibly in their user interface and in their documentation. SRD content
is used under CC-BY-4.0; see [ATTRIBUTION.md](ATTRIBUTION.md).
