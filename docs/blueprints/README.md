# Blueprints

Generic, product-agnostic architecture blueprints that Grimoire specialises. Where they disagree with
[ARCHITECTURE.md](../../ARCHITECTURE.md), ARCHITECTURE.md wins.

| Blueprint | Used for |
|---|---|
| [go-api.md](go-api.md) | `api/` — hexagonal Go API, spec-first OpenAPI (ogen), sqlc, Atlas |
| [vue-ts-spa.md](vue-ts-spa.md) | `web/` — Vue 3 + TypeScript SPA with a generated, validated client |

Rails, Rust, .NET and Spring/Kotlin variants were evaluated and not chosen
([ADR-0001](../adr/0001-go-backend-not-estate-kotlin.md)).
