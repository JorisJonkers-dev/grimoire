# Agent contract

The estate-wide conventions live in one place and are **not duplicated here**:

**https://github.com/JorisJonkers-dev/workspace/blob/main/CLAUDE.md**

Read it before doing anything non-trivial in this repository. It covers the
things that most often go wrong, including:

- **Pull request labels.** The estate uses a prefixed taxonomy — `type:`,
  `area:`, `component:`, `priority:`, `status:`. Plain `bug` / `enhancement` /
  `documentation` do **not** exist, and `gh pr create` fails with
  `'bug' not found`. Run `gh label list --repo <owner>/<repo>` once before
  passing `--label`.
- **Verify the value, not the command.** An exit code, a `Ready` condition or
  an accepted object is not evidence that a consumer sees what you intended.
- Traps around workflow runs, `zsh` word-splitting, and detached submodule
  HEADs.

Duplicating that content into every repository guarantees the copies drift, so
this file stays a pointer. Add repo-specific guidance below.

---

## This repository

Grimoire is a **public** repo (Attribution Assurance License). Treat everything you write here as
published.

**Read first:** [ARCHITECTURE.md](ARCHITECTURE.md) is the source of truth and wins over the generic
[blueprints](docs/blueprints/). [CONTEXT.md](CONTEXT.md) is the glossary: use its terms in code,
API names and UI copy (`Combatant`, not `actor`; `Action Log`, not `journal`). ADRs in
[docs/adr/](docs/adr/) are settled — do not reopen them without new facts.

**Never commit** estate-internal infrastructure detail (private hostnames, Vault paths, fleet-infra
file paths), secrets, private campaign or adventure content, or non-SRD game content. Platform
onboarding lives in the private workspace.

**Loop:** `task gen` after touching `openapi/`; `task check` before pushing — it is what CI runs.
`task` with no arguments lists targets.

**Hard rules** (details in ARCHITECTURE.md §6–§18):

- The OpenAPI spec is hand-authored and generated code is never edited by hand.
- `internal/rules` is pure: no I/O, and randomness and time are injected. It carries 100% coverage.
- Unions are sealed interfaces checked by `gochecksumtype`; no `default` branch to dodge exhaustiveness.
- No JSONB. Integrity lives in DB constraints.
- Anything a live Update sends is filtered per audience, with a negative test proving hidden data is absent.
- Every write is attributed (caller + origin) and recoverable, through a Revision or the Action Log.
- No GPL/AGPL code may be embedded.
