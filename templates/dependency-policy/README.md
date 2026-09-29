# Dependency Policy Templates

These templates are copied into target repositories during onboarding. Replace
any `{{...}}` placeholders with repo-specific values before committing them.

Policy invariants:

- Renovate delegates to `github>JorisJonkers-dev/renovate-config`.
- Dependabot waits 7 days before proposing newly published third-party
  versions, and proposes JorisJonkers-dev releases immediately.
- Dependabot groups minor and patch version updates.
- Dependabot keeps security updates separate.

`renovate.json.tmpl` is the default JorisJonkers-dev preset entrypoint.
`dependabot.yml.tmpl` is available for repos that use Dependabot directly or as
a fallback.
