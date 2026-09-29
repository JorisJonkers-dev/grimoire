# minimal-service — worked example

The smallest service that integrates with the deploy platform: one stateless
workload, a mandatory health probe, an acknowledged rollback-retention policy,
and nothing else — no HTTP routes, no stateful storage, no raw manifests.

`platform/render-local.sh` here is the `render-local.sh.tmpl` template rendered
with the values below; `scripts/validate-templates.sh` re-renders the template
and fails on any drift between the two.

| Placeholder | Value |
|-------------|-------|
| `service_name` | `minimal-service` |
| `service_namespace` | `minimal-service` |
| `schema_version` | `0.16.0` |
| `context_ref` | `ghcr.io/jorisjonkers-dev/cluster-deploy-context-public@sha256:1111…` (placeholder digest — when onboarding a real service, fetch the live pin from `https://raw.githubusercontent.com/JorisJonkers-dev/homelab-deploy/main/.context-pins.yaml`) |
| `ghcr_owner` | `jorisjonkers-dev` |
| `image_alias` | `minimal-service` |

## Expected scorecard

Running the scorecard against this fixture must produce exactly
[`expected-scorecard.json`](expected-scorecard.json):

```bash
./deploy/render-local.sh --scorecard-only
cat out/scorecard.md
```

| Check | Status |
|-------|--------|
| schema_pinned | pass |
| context_pinned | pass |
| no_latest_images | pass |
| health_declared | pass |
| route_owner_authmode_declared | not_applicable |
| rollback_retention_acknowledged | pass |
| no_raw_secrets | pass |
| stateful_policy_declared | not_applicable |
| raw_manifests_guarded | not_applicable |
| npm_signatures_verified | pass |

The three `not_applicable` results are the point of this example:

- no `routes[]` entries → `route_owner_authmode_declared = not_applicable`
- no `stateful: true` workload → `stateful_policy_declared = not_applicable`
- no `rawManifests` block → `raw_manifests_guarded = not_applicable`

`images.lock.json` uses the canonical **object** form (alias → digest-pinned
ref). The transitional array form is accepted by the CI normalizer but must
not be committed.
