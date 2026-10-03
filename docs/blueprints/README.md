# Blueprints

The generic Go API and Vue + TypeScript SPA blueprints Grimoire specialises live in
[`JorisJonkers-dev/template-go-vue`](https://github.com/JorisJonkers-dev/template-go-vue/tree/main/docs/blueprints),
the estate's Go + Vue template, so the standard has one home. Where they disagree with
[ARCHITECTURE.md](../../ARCHITECTURE.md), ARCHITECTURE.md wins.

| Blueprint | Used for |
|---|---|
| [go-api.md](https://github.com/JorisJonkers-dev/template-go-vue/blob/main/docs/blueprints/go-api.md) | `api/`: hexagonal Go API, spec-first OpenAPI (ogen), sqlc, goose |
| [vue-ts-spa.md](https://github.com/JorisJonkers-dev/template-go-vue/blob/main/docs/blueprints/vue-ts-spa.md) | `web/`: Vue 3 + TypeScript SPA with a generated, validated client |

Rails, Rust, .NET and Spring/Kotlin variants were evaluated and not chosen
([ADR-0001](../adr/0001-go-backend-not-estate-kotlin.md)).
