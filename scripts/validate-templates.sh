#!/usr/bin/env bash
# Validate repo-template static assets without network access.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

fail() {
  echo "validate-templates: $*" >&2
  exit 1
}

log() {
  echo "validate-templates: $*"
}

json_files=(
  .github/rulesets/main.json
  .release-please-manifest.json
  release-please-config.json
  renovate.json
  templates/dependency-policy/renovate.json.tmpl
  templates/release-please/simple.json.tmpl
  templates/release-please/node.json.tmpl
  templates/release-please/java-gradle.json.tmpl
  templates/release-please/manifest-default.json.tmpl
  templates/release-please/manifest-agents.json.tmpl
  templates/root-tooling/package/pnpm-package.json
  templates/root-tooling/package/yarn-package.json
  templates/root-tooling/hooks/lintstagedrc.json.tmpl
  templates/root-tooling/prettierrc.json.tmpl
  templates/platform-deploy/platform/images.lock.json.tmpl
  templates/platform-deploy/examples/minimal-service/platform/images.lock.json
  templates/platform-deploy/examples/minimal-service/expected-scorecard.json
)

workflow_template_files=(templates/workflows/*.yml.tmpl)
released_workflow_template_files=(
  templates/workflows/jvm-lib-ci.yml.tmpl
  templates/workflows/jvm-service-ci.yml.tmpl
  templates/workflows/gradle-plugin-ci.yml.tmpl
  templates/workflows/ts-lib-ci.yml.tmpl
  templates/workflows/vue-app-ci.yml.tmpl
  templates/workflows/python-ci.yml.tmpl
  templates/workflows/nix-ci.yml.tmpl
  templates/workflows/gitops-ci.yml.tmpl
)
placeholder_workflow_template_files=(
  templates/workflows/add-to-project.yml.tmpl
  templates/workflows/repository-hygiene.yml.tmpl
  templates/workflows/deploy-bundle.yml.tmpl
)

log "checking JSON syntax"
python3 - "$ROOT" "${json_files[@]}" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for rel in sys.argv[2:]:
    path = root / rel
    with path.open("r", encoding="utf-8") as fh:
        json.load(fh)
PY

yaml_files=(
  .github/dependabot.yml
  .github/workflows/ci.yml
  .github/workflows/release.yml
  templates/dependency-policy/dependabot.yml.tmpl
  templates/dependency-policy/dependency-review.yml.tmpl
  templates/dependency-policy/scorecard.yml.tmpl
  # codeql.yml.tmpl is intentionally absent: its {{manual_timeout_minutes}}
  # placeholder renders to a bare integer, so the pre-substitution template is
  # not parseable YAML (quoting it would change the rendered type).
  templates/dependency-policy/codeql-config.yml.tmpl
  templates/platform-config-validation/platform-config-validate.yml.tmpl
  templates/platform-deploy/workflows/release.yml.tmpl
  templates/platform-deploy/workflows/publish.yml.tmpl
  templates/platform-deploy/workflows/deploy-preview.yml.tmpl
  templates/platform-deploy/platform/deployment.yml.tmpl
  templates/platform-deploy/examples/minimal-service/platform/deployment.yml
)

log "checking YAML syntax when a local parser is available"
python3 - "$ROOT" "${yaml_files[@]}" "${workflow_template_files[@]}" <<'PY'
import importlib.util
import pathlib
import sys

if importlib.util.find_spec("yaml") is None:
    print("validate-templates: PyYAML unavailable; skipped YAML parser check")
    raise SystemExit(0)

import yaml

root = pathlib.Path(sys.argv[1])
for rel in sys.argv[2:]:
    path = root / rel
    with path.open("r", encoding="utf-8") as fh:
        yaml.safe_load(fh)
PY

log "checking dependency policy invariants"
grep -R "default-days: 7" .github/dependabot.yml templates/dependency-policy/dependabot.yml.tmpl >/dev/null \
  || fail "Dependabot policy must include 7-day cooldown"
grep -R "update-types: \\[minor, patch\\]" templates/dependency-policy/dependabot.yml.tmpl >/dev/null \
  || fail "Dependabot template must group minor and patch updates"
grep -R "security-updates" templates/dependency-policy/dependabot.yml.tmpl .github/dependabot.yml >/dev/null \
  || fail "Dependabot policy must group security updates"

log "checking Renovate shared preset"
python3 - "$ROOT" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
expected = {
    "$schema": "https://docs.renovatebot.com/renovate-schema.json",
    "extends": ["github>JorisJonkers-dev/renovate-config"],
}
for rel in ("renovate.json", "templates/dependency-policy/renovate.json.tmpl"):
    actual = json.loads((root / rel).read_text(encoding="utf-8"))
    if actual != expected:
        raise SystemExit(f"{rel} must use the shared JorisJonkers-dev Renovate preset")
PY

log "checking release-please templates"
grep -F '"bootstrap-sha": "a2095b6de581575eaa896eea056963889b893770"' release-please-config.json >/dev/null \
  || fail "repo release-please config must carry the migration bootstrap sha"
grep -R '"bootstrap-sha": "{{bootstrap_sha}}"' templates/release-please/*.json.tmpl >/dev/null \
  || fail "release-please templates must expose bootstrap_sha"
grep -F '{ ".": "0.1.0" }' templates/release-please/manifest-default.json.tmpl >/dev/null \
  || fail "default release-please manifest template must start at 0.1.0"
grep -F '{ ".": "0.16.0" }' templates/release-please/manifest-agents.json.tmpl >/dev/null \
  || fail "agents release-please manifest template must start at 0.16.0"

log "checking root tooling invariants"
grep -R "config/detekt/detekt.yml" templates/root-tooling >/dev/null \
  || fail "Detekt default path must align with gradle-conventions"
grep -R "lint-staged" templates/root-tooling/hooks templates/root-tooling/package >/dev/null \
  || fail "lint-staged preset missing"
grep -R "prettier --check" templates/root-tooling/package templates/root-tooling/hooks >/dev/null \
  || fail "format check preset missing"
grep -R '"license": "LicenseRef-JorisJonkers-Proprietary-1.0"' templates/root-tooling/package >/dev/null \
  || fail "package presets must carry the proprietary LicenseRef"
grep -F "ignoreStatic: true" templates/root-tooling/stryker.config.mjs.tmpl >/dev/null \
  || fail "Stryker preset must ignore static mutants by default"

log "checking platform config validation template"
grep -F "uses: JorisJonkers-dev/github-workflows/.github/workflows/platform-config-validate.yml@v0.7.3" templates/platform-config-validation/platform-config-validate.yml.tmpl >/dev/null \
  || fail "platform config validation template must call the reusable workflow by ref"
grep -F "schema-kind: auto" templates/platform-config-validation/platform-config-validate.yml.tmpl >/dev/null \
  || fail "platform config validation template must default schema-kind to auto"
grep -F "platform/**/*.yaml" templates/platform-config-validation/platform-config-validate.yml.tmpl >/dev/null \
  || fail "platform config validation template must include platform YAML globs"
grep -F "deploy/**/*.yaml" templates/platform-config-validation/platform-config-validate.yml.tmpl >/dev/null \
  || fail "platform config validation template must include deploy YAML globs"

log "checking workflow caller templates"
for rel in "${released_workflow_template_files[@]}"; do
  [[ -f "$rel" ]] || fail "missing workflow caller template: $rel"
  grep -F "Pipeline Complete" "$rel" >/dev/null \
    || fail "$rel must include the Pipeline Complete aggregator"
  grep -F "re-actors/alls-green@release/v1" "$rel" >/dev/null \
    || fail "$rel must aggregate reusable workflow jobs"
  grep -E "JorisJonkers-dev/github-workflows/.github/workflows/.+@v0\\.7\\.3" "$rel" >/dev/null \
    || fail "$rel must pin released reusable workflows to v0.7.3"
done
for rel in "${placeholder_workflow_template_files[@]}"; do
  [[ -f "$rel" ]] || fail "missing workflow caller template: $rel"
  grep -F "@{{github_workflows_ref}}" "$rel" >/dev/null \
    || fail "$rel must require a rendered published github-workflows ref"
done
if rg -n 'JorisJonkers-dev/github-workflows/.github/workflows/.+@(main|master|HEAD)' templates/workflows templates/platform-config-validation templates/platform-deploy; then
  fail "reusable workflow templates must not use moving refs"
fi

log "checking hook shell syntax"
bash -n scripts/install-git-hooks.sh templates/push-protection/hooks/*

log "checking Docker pattern template structure"
python3 - "$ROOT" <<'PY'
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
required_files = {
    "templates/docker-patterns/jvm/Dockerfile.jvm.tmpl": [
        "FROM ${JVM_BUILD_IMAGE} AS build",
        "FROM ${JVM_RUNTIME_IMAGE} AS runtime",
        "{{jvm_dependency_resolution_command}}",
        "{{jvm_build_command}}",
        "{{runtime_user_create_command}}",
        "COPY --from=build",
        "USER ${APP_UID}:${APP_GID}",
        "HEALTHCHECK",
        "ENTRYPOINT",
        "CMD",
    ],
    "templates/docker-patterns/jvm/Dockerfile.crac-jvm.tmpl": [
        "FROM ${JVM_BUILD_IMAGE} AS build",
        "FROM ${CRAC_RUNTIME_IMAGE} AS crac-base",
        "FROM crac-base AS train",
        "FROM crac-base AS runtime",
        "{{checkpoint_dir}}",
        "CRAC_MODE",
        "VOLUME",
        "HEALTHCHECK",
    ],
    "templates/docker-patterns/vue/Dockerfile.vue-build-runtime.tmpl": [
        "FROM ${NODE_BUILD_IMAGE} AS deps",
        "FROM deps AS build",
        "FROM ${NGINX_RUNTIME_IMAGE} AS runtime",
        "{{package_manager_install_command}}",
        "{{frontend_build_command}}",
        "COPY --from=build",
        "HEALTHCHECK",
    ],
    "templates/docker-patterns/vue/Dockerfile.pnpm-build.tmpl": [
        "FROM ${NODE_BUILD_IMAGE} AS deps",
        "FROM deps AS build",
        "FROM ${NGINX_RUNTIME_IMAGE} AS runtime",
        "{{pnpm_bootstrap_command}}",
        "pnpm install --frozen-lockfile",
        "pnpm {{pnpm_build_filter}} {{frontend_build_script}}",
        "COPY --from=build",
        "HEALTHCHECK",
    ],
    "templates/docker-patterns/vue/Dockerfile.yarn-build.tmpl": [
        "FROM ${NODE_BUILD_IMAGE} AS deps",
        "FROM deps AS build",
        "FROM ${NGINX_RUNTIME_IMAGE} AS runtime",
        "corepack enable",
        "yarn install --immutable",
        "yarn {{frontend_build_script}}",
        "COPY --from=build",
        "HEALTHCHECK",
    ],
    "templates/docker-patterns/nginx/Dockerfile.privileged-nginx.tmpl": [
        "FROM ${NGINX_RUNTIME_IMAGE} AS runtime",
        "COPY {{nginx_config_path}}",
        "COPY {{static_assets_dir}}",
        "EXPOSE {{privileged_listen_port}}",
        "HEALTHCHECK",
        "CMD",
    ],
    "templates/docker-patterns/nginx/Dockerfile.unprivileged-nginx.tmpl": [
        "FROM ${NGINX_RUNTIME_IMAGE} AS runtime",
        "COPY --chown=${NGINX_UID}:${NGINX_GID}",
        "USER ${NGINX_UID}:${NGINX_GID}",
        "EXPOSE {{unprivileged_listen_port}}",
        "HEALTHCHECK",
        "CMD",
    ],
    "templates/docker-patterns/entrypoints/otel-entrypoint.sh.tmpl": [
        "#!/usr/bin/env sh",
        "set -eu",
        "OTEL_RESOURCE_ATTRIBUTES",
        "otel_agent_arg",
        "exec \"$@\"",
    ],
    "templates/docker-patterns/entrypoints/crac-entrypoint.sh.tmpl": [
        "#!/usr/bin/env sh",
        "set -eu",
        "case \"${mode}\" in",
        "train)",
        "restore)",
        "run)",
        "exec java",
    ],
}

for rel, markers in required_files.items():
    path = root / rel
    if not path.exists():
        raise SystemExit(f"missing Docker pattern template: {rel}")
    text = path.read_text(encoding="utf-8")
    if "DESIGN-ONLY SKELETON" in text:
        raise SystemExit(f"Docker pattern template is still design-only: {rel}")
    missing = [marker for marker in markers if marker not in text]
    if missing:
        raise SystemExit(f"{rel} missing structural markers: {', '.join(missing)}")

template_count = len(list((root / "templates/docker-patterns").glob("**/*.tmpl")))
if template_count < len(required_files):
    raise SystemExit("expected all Docker pattern templates to be present")
if (root / "templates/docker-patterns/Dockerfile.tmpl").exists():
    raise SystemExit("single shared Dockerfile template is not allowed")

placeholder_re = re.compile(r"\{\{[a-zA-Z0-9_]+\}\}")
placeholders = set()
for path in (root / "templates/docker-patterns").glob("**/*.tmpl"):
    placeholders.update(placeholder_re.findall(path.read_text(encoding="utf-8")))

docs = [
    root / "templates/docker-patterns/README.md",
]
for doc in docs:
    text = doc.read_text(encoding="utf-8")
    missing = sorted(ph for ph in placeholders if ph not in text)
    if missing:
        raise SystemExit(f"{doc.relative_to(root)} missing placeholder docs: {', '.join(missing)}")
PY

log "checking Docker entrypoint shell syntax"
for entrypoint_template in templates/docker-patterns/entrypoints/*.tmpl; do
  sh -n "$entrypoint_template"
done

log "checking platform deploy templates"
platform_deploy_files=(
  templates/platform-deploy/README.md
  templates/platform-deploy/PLATFORM.md.tmpl
  templates/platform-deploy/workflows/release.yml.tmpl
  templates/platform-deploy/workflows/publish.yml.tmpl
  templates/platform-deploy/workflows/deploy-preview.yml.tmpl
  templates/platform-deploy/platform/deployment.yml.tmpl
  templates/platform-deploy/platform/production.env.tmpl
  templates/platform-deploy/platform/images.lock.json.tmpl
  templates/platform-deploy/platform/render-local.sh.tmpl
  templates/platform-deploy/examples/minimal-service/PLATFORM.md
  templates/platform-deploy/examples/minimal-service/platform/deployment.yml
  templates/platform-deploy/examples/minimal-service/platform/images.lock.json
  templates/platform-deploy/examples/minimal-service/platform/render-local.sh
  templates/platform-deploy/examples/minimal-service/expected-scorecard.json
)
for rel in "${platform_deploy_files[@]}"; do
  [[ -f "$rel" ]] || fail "missing platform deploy file: $rel"
done

# Released reusable workflows must be pinned to the released tag.
grep -F "container-publish.yml@f6c2969d7f1f4555da3b2cf46ce6a9b364c471b3" templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null \
  || fail "publish.yml.tmpl must pin container-publish.yml to f6c2969d7f1f4555da3b2cf46ce6a9b364c471b3 # v0.12.0"
grep -F "deploy-artifact.yml@f6c2969d7f1f4555da3b2cf46ce6a9b364c471b3" templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null \
  || fail "publish.yml.tmpl must pin deploy-artifact.yml to f6c2969d7f1f4555da3b2cf46ce6a9b364c471b3 # v0.12.0"
grep -F "deploy-validate.yml@f6c2969d7f1f4555da3b2cf46ce6a9b364c471b3" templates/platform-deploy/workflows/deploy-preview.yml.tmpl >/dev/null \
  || fail "deploy-preview.yml.tmpl must pin deploy-validate.yml to f6c2969d7f1f4555da3b2cf46ce6a9b364c471b3 # v0.12.0"

# The App token is mandatory in both token-minting workflows.
grep -F "E_APP_TOKEN_MISSING" templates/platform-deploy/workflows/release.yml.tmpl >/dev/null \
  || fail "release.yml.tmpl must assert the App token (E_APP_TOKEN_MISSING)"
grep -F "E_APP_TOKEN_MISSING" templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null \
  || fail "publish.yml.tmpl must assert the App token (E_APP_TOKEN_MISSING)"

# R1-2: the image lock crosses the reusable-workflow boundary as an artifact.
if grep -F "prepare-deploy-inputs" templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null; then
  fail "publish.yml.tmpl must not have a prepare-deploy-inputs job (the reusable workflow downloads the artifact itself)"
fi
grep -F "image-lock-artifact: \${{ needs.resolve-image-lock.outputs.images-lock-artifact }}" \
  templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null \
  || fail "publish.yml.tmpl must pass image-lock-artifact from resolve-image-lock outputs"
if grep -E '^[[:space:]]*image-lock-path:' templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null; then
  fail "publish.yml.tmpl must not pass image-lock-path (image-lock-artifact is the only lock input)"
fi
grep -F "images-lock-\${{ github.run_id }}" templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null \
  || fail "publish.yml.tmpl image-lock artifact name must include github.run_id"

# First-registration payload must emit the full DeployUnitRegistration spec.
for marker in '"owner"' '"namespace"' '"layer"' '"sourceRepository"' '"environments"' '"healthClass"' '"prune"' '"allowedClusterScope"' '"contractPath"'; do
  grep -F "$marker" templates/platform-deploy/workflows/publish.yml.tmpl >/dev/null \
    || fail "publish.yml.tmpl register-service payload must emit $marker"
done

python3 - "$ROOT" <<'PY'
import importlib.util
import pathlib
import sys

if importlib.util.find_spec("yaml") is None:
    print("validate-templates: PyYAML unavailable; skipped platform-deploy workflow structure check")
    raise SystemExit(0)

import yaml

root = pathlib.Path(sys.argv[1])
base = root / "templates/platform-deploy/workflows"

release = yaml.safe_load((base / "release.yml.tmpl").read_text(encoding="utf-8"))
for name, job in release["jobs"].items():
    needs = job.get("needs") or []
    if isinstance(needs, str):
        needs = [needs]
    if "release-please" in needs:
        raise SystemExit(f"release.yml.tmpl job {name} must not depend on release-please "
                         "(tag push triggers publish.yml independently)")

publish = yaml.safe_load((base / "publish.yml.tmpl").read_text(encoding="utf-8"))
jobs = publish["jobs"]
for required in ("publish-image", "resolve-image-lock", "publish-deploy-artifact", "register-service"):
    if required not in jobs:
        raise SystemExit(f"publish.yml.tmpl missing job {required}")
for name, job in jobs.items():
    if "uses" in job and "outputs" in job:
        raise SystemExit(f"publish.yml.tmpl job {name} calls a reusable workflow and must not "
                         "redeclare an outputs: block (consume needs.<job>.outputs directly)")
deploy_with = jobs["publish-deploy-artifact"]["with"]
if "image-lock-artifact" not in deploy_with:
    raise SystemExit("publish.yml.tmpl deploy-artifact call must pass image-lock-artifact")
if "image-lock-path" in deploy_with:
    raise SystemExit("publish.yml.tmpl deploy-artifact call must not pass image-lock-path")
if deploy_with.get("deploy-dir") != "platform":
    raise SystemExit("publish.yml.tmpl deploy-artifact call must pass deploy-dir: platform")
PY

log "checking platform deploy placeholder docs"
python3 - "$ROOT" <<'PY'
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
placeholder_re = re.compile(r"\{\{[a-z0-9_]+\}\}")
placeholders = set()
for path in (root / "templates/platform-deploy").glob("**/*.tmpl"):
    placeholders.update(placeholder_re.findall(path.read_text(encoding="utf-8")))

readme = (root / "templates/platform-deploy/README.md").read_text(encoding="utf-8")
missing = sorted(ph for ph in placeholders if ph not in readme)
if missing:
    raise SystemExit("templates/platform-deploy/README.md missing placeholder docs: "
                     + ", ".join(missing))
PY

log "checking render-local template shell syntax"
bash -n templates/platform-deploy/platform/render-local.sh.tmpl
bash -n templates/platform-deploy/examples/minimal-service/platform/render-local.sh
if command -v shellcheck >/dev/null 2>&1; then
  shellcheck templates/platform-deploy/examples/minimal-service/platform/render-local.sh
else
  log "shellcheck unavailable; skipped shellcheck on render-local.sh"
fi

log "checking minimal-service render-local.sh is the rendered template"
render_platform_deploy_example() {
  sed \
    -e 's/{{service_name}}/minimal-service/g' \
    -e 's/{{service_namespace}}/minimal-service/g' \
    -e 's/{{schema_version}}/0.16.0/g' \
    -e 's|{{context_ref}}|ghcr.io/jorisjonkers-dev/cluster-deploy-context-public@sha256:1111111111111111111111111111111111111111111111111111111111111111|g' \
    -e 's/{{ghcr_owner}}/jorisjonkers-dev/g' \
    -e 's/{{image_alias}}/minimal-service/g' \
    templates/platform-deploy/platform/render-local.sh.tmpl
}
diff <(render_platform_deploy_example) templates/platform-deploy/examples/minimal-service/platform/render-local.sh \
  || fail "examples/minimal-service/platform/render-local.sh drifted from render-local.sh.tmpl; re-render it"

log "checking render-local scorecard behaviour against the minimal-service fixture"
command -v jq >/dev/null 2>&1 || fail "jq is required for the render-local scorecard checks"
scorecard_tmp="$(mktemp -d)"
trap 'rm -rf "$scorecard_tmp"' EXIT

templates/platform-deploy/examples/minimal-service/platform/render-local.sh --help | grep -F "Usage:" >/dev/null \
  || fail "render-local.sh --help must print usage"

(cd templates/platform-deploy/examples/minimal-service \
  && OUT_DIR="$scorecard_tmp/minimal-out" ./platform/render-local.sh --scorecard-only >/dev/null 2>&1) \
  || fail "render-local.sh --scorecard-only must pass for the minimal-service fixture"
diff <(jq -S . "$scorecard_tmp/minimal-out/scorecard.json") \
  <(jq -S . templates/platform-deploy/examples/minimal-service/expected-scorecard.json) \
  || fail "minimal-service scorecard does not match expected-scorecard.json"

mkdir -p "$scorecard_tmp/negative/platform"
cp templates/platform-deploy/examples/minimal-service/platform/render-local.sh "$scorecard_tmp/negative/platform/"
cp templates/platform-deploy/examples/minimal-service/platform/images.lock.json "$scorecard_tmp/negative/platform/"
cat > "$scorecard_tmp/negative/platform/deployment.yml" <<'NEGATIVE'
apiVersion: deployment.jorisjonkers.dev/v2
metadata:
  name: negative-service
spec:
  schemaVersion: "0.16.0"
  namespace: negative-service
  platform:
    layer: apps-core
  workloads:
    - name: negative-service
      image: minimal-service
      routes:
        - host: negative.example.internal
      stateful: true
      health:
        path: /health
        port: 8080
        timeoutClass: stateless
        mandatory: true
      rollbackTargetRetention:
        minimumDays: 90
        acknowledged: true
NEGATIVE
if (cd "$scorecard_tmp/negative" && OUT_DIR="$scorecard_tmp/negative/out" ./platform/render-local.sh --scorecard-only >/dev/null 2>&1); then
  fail "render-local.sh --scorecard-only must fail when routes lack owner/authMode and stateful lacks migrationPolicy"
fi
[[ "$(jq -r '.route_owner_authmode_declared' "$scorecard_tmp/negative/out/scorecard.json")" == "fail" ]] \
  || fail "routes without owner/authMode must set route_owner_authmode_declared=fail"
[[ "$(jq -r '.stateful_policy_declared' "$scorecard_tmp/negative/out/scorecard.json")" == "fail" ]] \
  || fail "stateful workload without migrationPolicy must set stateful_policy_declared=fail"
[[ "$(jq -r '.raw_manifests_guarded' "$scorecard_tmp/negative/out/scorecard.json")" == "not_applicable" ]] \
  || fail "no rawManifests block must keep raw_manifests_guarded=not_applicable"

cp templates/platform-deploy/examples/minimal-service/platform/deployment.yml "$scorecard_tmp/negative/platform/deployment.yml"
printf '{\n  "minimal-service": "ghcr.io/jorisjonkers-dev/minimal-service:latest"\n}\n' \
  > "$scorecard_tmp/negative/platform/images.lock.json"
if (cd "$scorecard_tmp/negative" && OUT_DIR="$scorecard_tmp/negative/out-latest" ./platform/render-local.sh --scorecard-only >/dev/null 2>&1); then
  fail "render-local.sh --scorecard-only must fail for a :latest image ref"
fi
[[ "$(jq -r '.no_latest_images' "$scorecard_tmp/negative/out-latest/scorecard.json")" == "fail" ]] \
  || fail ":latest image ref must set no_latest_images=fail"

log "checking render-local schema-version resolution and digest guard"
render_local_example=templates/platform-deploy/examples/minimal-service/platform/render-local.sh
SCHEMA_VERSION=0.17.0 bash -c "source $render_local_example; resolve_schema_version" 2>&1 \
  | grep -F "Using SCHEMA_VERSION from env: 0.17.0" >/dev/null \
  || fail "SCHEMA_VERSION env must take priority in resolve_schema_version"
mkdir -p "$scorecard_tmp/version/platform" "$scorecard_tmp/version/.platform"
cp "$render_local_example" "$scorecard_tmp/version/platform/"
printf '0.16.1\n' > "$scorecard_tmp/version/.platform/deploy-version"
bash -c "source $scorecard_tmp/version/platform/render-local.sh; resolve_schema_version" 2>&1 \
  | grep -F "Using schema version from .platform/deploy-version: 0.16.1" >/dev/null \
  || fail ".platform/deploy-version must be the second resolution source"
rm -rf "$scorecard_tmp/version/.platform"
bash -c "source $scorecard_tmp/version/platform/render-local.sh; resolve_schema_version" 2>&1 \
  | grep -F "Using baked-in schema version: 0.16.0" >/dev/null \
  || fail "baked-in schema version must be the fallback resolution source"
if bash -c "source $render_local_example; require_digest_ref ghcr.io/example/context:latest" >/dev/null 2>&1; then
  fail "require_digest_ref must reject non-digest context refs"
fi
digest_guard_output="$(bash -c "source $render_local_example; require_digest_ref ghcr.io/example/context:latest" 2>&1 || true)"
grep -F "E_CONTEXT_REF_NOT_PINNED" <<<"$digest_guard_output" >/dev/null \
  || fail "non-digest context ref must fail with E_CONTEXT_REF_NOT_PINNED"
CONTEXT_DIR=/tmp bash -c "source $render_local_example; pull_or_use_local_context" 2>&1 \
  | grep -F "bypasses OCI digest requirement" >/dev/null \
  || fail "--context-dir must bypass the digest requirement with a warning"

log "checking for source-specific values in templates"
forbidden_pattern='esa-blueshell|blueshell|personal-stack|frankfurt-contabo|enschede|167\.86\.79\.203|130\.89\.174\.190|192\.168\.0\.99|assistant-system|knowledge-system|media-system|utility-system|data-system|secret/data/platform|secret/platform|secret/agents|auth-api|assistant-api|knowledge-api|uptime-kuma|stalwart|rabbitmq|valkey|postgres'
if rg -n -i "$forbidden_pattern" templates; then
  fail "source-specific value found in templates"
fi

log "all checks passed"
