# Bootstrapping a new JorisJonkers-dev repo from this template

1. **Create the repo from the template** (GitHub "Use this template", or
   `gh repo create JorisJonkers-dev/<name> --template JorisJonkers-dev/repo-template
   --private`).
2. **Apply the branch ruleset** so `Pipeline Complete` is required:
   ```bash
   scripts/apply-ruleset.sh JorisJonkers-dev/<name>
   ```
3. **Wire the real CI**: replace the placeholder `Lint`/`Test`/`Coverage`/`Build`
   **steps** in `.github/workflows/ci.yml` with this repo's real checks, or copy
   the nearest caller template from `templates/workflows/`.

   Keep them as steps in the one `Pipeline Complete` job. GitHub bills every job
   rounded up to a full minute, so splitting fast checks across jobs multiplies
   the bill by the job count while the work stays the same — in this estate that
   rounding, not compute, was the large majority of private-repo Actions spend.

   Add a second job only when you must: a reusable workflow (`uses:`) cannot be
   a step. When you do, gate it from the `Pipeline Complete` job with
   `needs:` plus a final step that fails unless its result is `success` — see
   the comment at the top of `ci.yml`. Compare against `success` rather than
   `failure`, so a skipped or cancelled job fails too.
4. **Wire the coverage gate**: every repo must enforce **>=80% line coverage**.
   - Gradle: apply JaCoCo and run `jacocoTestCoverageVerification` with a line
     coverage minimum of `0.80`.
   - Node: run Vitest with c8 coverage enforcement, for example `c8 --lines 80
     vitest run`.
5. **Set the release artifact**: edit `release.yml`'s `publish` job for this
   repo's artifact type (Maven / npm / image), and set `release-type` in
   `release-please-config.json` accordingly. New GitHub Packages publish as
   public on this account, but verify package visibility after the first
   publish.
6. **Set the starting version** in `.release-please-manifest.json`.
7. **Install private-repo hooks** when GitHub branch protection is process-gated:
   ```bash
   scripts/install-git-hooks.sh
   ```
8. **CODEOWNERS / README**: adjust owners and replace this README's body with
   the repo's purpose.
9. **Set the `workspace-<group>` topic** on the repo. Submodule placement in
   the [`workspace`](https://github.com/JorisJonkers-dev/workspace) monorepo is
   derived from this topic, giving `<group>/<repo>`:

   ```sh
   gh repo edit <owner>/<repo> --add-topic workspace-services
   ```

   Valid groups are listed in `workspace/.workspace/groups.txt`:
   `services`, `ui`, `libs`, `platform`, `tooling`, `tests`, `tools`, `data`.

   A repo with no such topic lands in `inbox/` rather than blocking the sync -
   **so forgetting this fails quietly**. Confirm placement by reading the path
   in `workspace/.gitmodules` after the next daily sync, not by checking that
   the topic is set.

10. **Adjust `CLAUDE.md`** for the repo. The template ships a short pointer to
    the estate-wide agent contract in `workspace/CLAUDE.md`; add anything
    specific to this repo below it, and leave the pointer in place.

11. **Choose dependency policy depth**:
   - Keep the root `renovate.json` and `.github/dependabot.yml` for the default
     JorisJonkers-dev policy.
   - Copy richer stack-specific variants from `templates/dependency-policy/`
     when the repo needs Dependabot ecosystems, dependency-review, Scorecard, or
     advanced CodeQL setup.
12. **Choose root tooling presets** from `templates/root-tooling/` when the repo
   has frontend linting, local hooks, Stryker mutation testing, ADRs, or docs
   indexes.
13. **Opt into platform/deploy config validation** only for repos that carry
    platform config: copy
    `templates/platform-config-validation/platform-config-validate.yml.tmpl` to
    `.github/workflows/platform-config-validate.yml`. It calls
    `JorisJonkers-dev/github-workflows/.github/workflows/platform-config-validate.yml@v0.7.3`
    with `schema-kind: auto` and platform/deploy YAML globs.
14. **Opt into Project, hygiene, or deploy bundle callers** by copying the
    relevant file from `templates/workflows/` after replacing
    `{{github_workflows_ref}}` with a published `github-workflows` tag. The
    reusable workflow must exist at that tag.
15. **Review Docker pattern skeletons** in `templates/docker-patterns/` only as
    design references. They are not production Dockerfiles.
16. **Validate template assets**:
    ```bash
    scripts/validate-templates.sh
    ```

## What this template provides

| File | Purpose |
| --- | --- |
| `.github/workflows/ci.yml` | One CI pipeline, one job, named `Pipeline Complete` — the only required check. Includes a coverage gate placeholder |
| `.github/workflows/add-to-project.yml` | Event-driven project boarding. **Delete it in a private repo** — the org-wide sweep covers you there and this costs a billable minute per issue and pull request |
| `.github/workflows/release.yml` | release-please tag/release + artifact publish |
| `.github/rulesets/main.json` + `scripts/apply-ruleset.sh` | Org ruleset as code (requires `Pipeline Complete`) |
| `.github/CODEOWNERS`, `dependabot.yml`, `renovate.json` | Ownership + dependency automation |
| `templates/dependency-policy/` | Parameterized Dependabot, Renovate, dependency-review, Scorecard, and CodeQL policy templates |
| `templates/release-please/` | Release-please config and manifest archetypes |
| `templates/workflows/` | Reusable workflow caller templates pinned to published `JorisJonkers-dev/github-workflows` tags, one per stack: jvm, node, python, nix, gitops, docker, tofu |
| `scripts/install-git-hooks.sh`, `templates/push-protection/` | Private-repo pre-push and commit-msg hooks |
| `templates/root-tooling/` | Root editor, prettier, ESLint, Stryker, lint-staged, Husky, gitleaks, docs, and ADR presets |
| `templates/platform-config-validation/` | Opt-in workflow template for `@jorisjonkers-dev/deploy-config-schema` validation |
| `templates/docker-patterns/` | Design-only Dockerfile and entrypoint skeleton fixtures |
| `scripts/validate-templates.sh` | Local validation for template syntax, pins, hooks, policy invariants, and source-value leakage |
| `release-please-config.json`, `.release-please-manifest.json` | Versioning state |
| `CONTRIBUTING.md`, `VERSIONING.md`, `SECURITY.md` | Lean repo-local conventions and security references |
| `.editorconfig`, `.gitattributes`, `.gitignore`, `.gitleaks.toml`, `LICENSE` | Baseline hygiene |
