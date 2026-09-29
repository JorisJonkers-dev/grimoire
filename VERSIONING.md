# Versioning & release

Grimoire is released as one version for the whole monorepo. `main` is an integration branch, not a
deploy target.

1. Pull requests are squash-merged with Conventional Commit titles (`feat:`, `fix:`, `chore:`,
   `feat!:` / `BREAKING CHANGE:` for majors).
2. `release.yml` (release-please) keeps a release PR open. Merging it tags `vX.Y.Z`, writes
   `CHANGELOG.md` and bumps `.release-please-manifest.json`.
3. The release publishes the images at that exact version (from the first deploy in M0):
   `ghcr.io/jorisjonkers-dev/grimoire/grimoire-api:X.Y.Z` and `.../grimoire-web:X.Y.Z`.

Pre-1.0, minor is the breaking lever (`bump-minor-pre-major`). 1.0.0 comes when a group can run a
full campaign on it.

The OpenAPI contract has its own compatibility gate: breaking changes against `main` fail CI
(oasdiff), and retiring an endpoint takes three merges (deprecate → remove → replace), see
ARCHITECTURE.md §7.3.

Dependencies are pinned exactly — Go via `go.sum`, JS via `pnpm-lock.yaml` with a one-week minimum
release age, tools via `mise.toml`, GitHub Actions by tag with Renovate/Dependabot bumps.
