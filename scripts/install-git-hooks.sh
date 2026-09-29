#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
hooks_dir="${repo_root}/.git/hooks"
template_dir="${repo_root}/templates/push-protection/hooks"

install -m 0755 "${template_dir}/pre-push" "${hooks_dir}/pre-push"
install -m 0755 "${template_dir}/commit-msg" "${hooks_dir}/commit-msg"

echo "Installed private-repo hooks into ${hooks_dir}"
