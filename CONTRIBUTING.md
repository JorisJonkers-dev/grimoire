# Contributing

Grimoire is open source under the [Attribution Assurance License](LICENSE). Issues and pull requests
are welcome.

## Before you start

- Read [ARCHITECTURE.md](ARCHITECTURE.md) (at least §0, §4 and §18) and use the terms in
  [CONTEXT.md](CONTEXT.md).
- For anything larger than a fix, open an issue first so the design can be agreed.
- Run `mise install`, then `task check` before pushing. CI runs the same targets.

## Pull requests

1. Branch from `main`; one concern per pull request.
2. Title in [Conventional Commits](https://www.conventionalcommits.org/) form (`feat:`, `fix:`,
   `chore:`, `docs:`, `feat!:` for breaking changes). Pull requests are squash-merged and the title
   becomes the commit.
3. Include tests. Rules-engine code needs full coverage and must survive mutation testing.
4. Regenerate and commit generated code (`task gen`) when the OpenAPI contract changes.
5. Do not hand-edit `CHANGELOG.md`; release-please owns it.
6. Never commit secrets, private campaign content, or non-SRD game content.

`Pipeline Complete` is the required status check.

## Security

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md), never in public issues.
