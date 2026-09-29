# Architecture decision records

Decisions that are hard to reverse, surprising without context, and the result of a real trade-off.
Numbered sequentially; one short file each.

| ADR | Decision |
|---|---|
| [0001](0001-go-backend-not-estate-kotlin.md) | Go for the backend, not the estate's Kotlin/Spring |
| [0002](0002-attribution-assurance-license.md) | Public repo under the Attribution Assurance License |
| [0003](0003-single-replica-live-runtime.md) | Live Sessions run in one goroutine on a single API replica |
| [0004](0004-server-only-rules.md) | Rules run only on the server; the client keeps hex geometry alone |
