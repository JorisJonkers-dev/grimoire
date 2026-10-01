# Architecture decision records

Decisions that are hard to reverse, surprising without context, and the result of a real trade-off.
Numbered sequentially; one short file each.

| ADR | Decision |
|---|---|
| [0001](0001-go-backend-not-estate-kotlin.md) | Go for the backend, not the estate's Kotlin/Spring |
| [0002](0002-attribution-assurance-license.md) | Public repo under the Attribution Assurance License |
| [0003](0003-single-replica-live-runtime.md) | Live Sessions run in one goroutine on a single API replica |
| [0004](0004-server-only-rules.md) | Rules run only on the server; the client keeps hex geometry alone |
| [0005](0005-goose-and-squawk-for-migrations.md) | goose and squawk for migrations, not Atlas |
| [0006](0006-uuid-identifiers-and-tokens-in-bodies.md) | UUIDs for API identifiers; tokens only in request bodies |
| [0007](0007-mcp-tools-run-through-the-rest-router.md) | MCP tools are spec operations served through the REST router |
| [0008](0008-own-accounts-with-linkable-oidc.md) | Grimoire owns its Accounts; jorisjonkers.dev is one linkable sign-in method |
| [0009](0009-grimoire-issues-its-own-access-tokens.md) | Grimoire issues its own Access Tokens for MCP and tools |
| [0010](0010-characters-owned-by-accounts.md) | Characters belong to Accounts; each Campaign holds its own progress |
| [0011](0011-linked-library-with-campaign-overrides.md) | Library entries are linked into Campaigns, with Campaign Overrides |
| [0012](0012-gpu-rendered-dice-with-server-results.md) | Dice are 3D on the GPU; the server decides the result |
| [0013](0013-faction-standing-is-tiered-and-dm-confirmed.md) | Factions ship as archetypes; Standing is tiered and the DM confirms changes |
