# Go for the backend, not the estate's Kotlin/Spring

The estate's services are Kotlin/Spring and every reusable workflow, convention plugin and client publisher already exists for them, so Kotlin was the path of least resistance. Grimoire's top priority is a rapid loop — fast builds, fast tests, fast responses, easy implementation — and Go (ogen spec-first, sqlc, Atlas, a goroutine per live Session) wins on every one of those; Kotlin's Gradle/JVM loop is the slowest of the candidates. We accept the costs: a new `go-ci` reusable workflow, and sum types enforced by `gochecksumtype`/`exhaustive` linters instead of the compiler.

## Considered Options

- **Rust** — strongest types and a rules engine that could compile to wasm, rejected for the slowest compile loop.
- **Rails + rswag** — fastest to write and liked for its contract tests, rejected for slow request specs, bolt-on typing (Steep friction seen in an earlier project) and the lowest response speed.
- **C#/.NET 10** — balanced, best realtime (SignalR), rejected for introducing a third ecosystem with no estate support.
