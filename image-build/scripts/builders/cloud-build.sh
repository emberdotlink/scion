#!/bin/bash
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# cloud-build builder for the scion image build orchestrator.
#
# Operates in target mode: instead of looping per image, the orchestrator
# calls builder_run_target once per invocation and this builder hands the
# whole target off to Google Cloud Build via `gcloud builds submit`. Caching
# and per-image step ordering are concerns of the static cloudbuild-*.yaml
# files that ship alongside this script.

BUILDER_MODE="target"

builder_check() {
  if ! command -v gcloud >/dev/null 2>&1; then
    echo "Error: 'gcloud' not found in PATH."
    echo "Install Google Cloud SDK before using --builder cloud-build."
    return 1
  fi
}

builder_prepare() {
  :
}

# cloud_build_config_for_target <target>
# Echoes the absolute path to the cloudbuild-*.yaml that implements the
# given target. Returns nonzero if no mapping exists.
cloud_build_config_for_target() {
  local target="$1"
  local file
  case "${target}" in
    common)     file="cloudbuild-common.yaml" ;;
    all)        file="cloudbuild.yaml" ;;
    core-base)  file="cloudbuild-core-base.yaml" ;;
    scion-base) file="cloudbuild-scion-base.yaml" ;;
    harnesses)  file="cloudbuild-harnesses.yaml" ;;
    hub)        file="cloudbuild-hub.yaml" ;;
    *)
      echo "cloud-build: no cloudbuild-*.yaml mapping for target '${target}'" >&2
      return 1
      ;;
  esac
  local path="${IMAGE_BUILD_DIR}/${file}"
  if [[ ! -f "${path}" ]]; then
    echo "cloud-build: expected config file does not exist: ${path}" >&2
    echo "(target table and cloudbuild-*.yaml files are out of sync)" >&2
    return 1
  fi
  echo "${path}"
}

# builder_run_target <target> <registry> <tag> <push>
#
# Submits the target to Cloud Build. <push> is ignored — the YAMLs always
# push. <registry> and <tag> become _REGISTRY and _TAG substitutions.
builder_run_target() {
  local target="$1"
  local registry="$2"
  local tag="$3"
  # local push="$4"  # ignored: cloud-build YAMLs always push

  local config
  config="$(cloud_build_config_for_target "${target}")" || return 1

  local project="${GCLOUD_PROJECT:-}"
  if [[ -z "${project}" ]]; then
    project="$(gcloud config get-value project 2>/dev/null)" || true
  fi
  if [[ -z "${project}" ]]; then
    echo "Error: could not determine GCP project." >&2
    echo "Set \$GCLOUD_PROJECT or run 'gcloud config set project <project>'." >&2
    return 1
  fi

  # SHORT_SHA / COMMIT_SHA are computed once by the orchestrator. Fall back
  # to "unknown" when the working tree is not a git repo so the substitution
  # is always populated (Cloud Build rejects unset substitutions).
  local short_sha="${SHORT_SHA:-unknown}"
  local commit_sha="${COMMIT_SHA:-unknown}"

  local subs="_TAG=${tag},_SHORT_SHA=${short_sha},_COMMIT_SHA=${commit_sha}"
  if [[ -n "${registry}" ]]; then
    subs="${subs},_REGISTRY=${registry}"
  fi

  echo "==> [cloud-build] submitting target '${target}' via ${config}"
  local -a cmd=(
    gcloud builds submit --async
    --project="${project}"
    --substitutions="${subs}"
    --config="${config}"
    "${REPO_ROOT}"
  )

  if [[ "${DRY_RUN:-false}" == "true" ]]; then
    printf '[dry-run]'
    printf ' %q' "${cmd[@]}"
    printf '\n'
    return 0
  fi

  "${cmd[@]}"

  echo ""
  echo "Build submitted. View progress at:"
  echo "  https://console.cloud.google.com/cloud-build/builds?project=${project}"
}

# Per-image entry point is unused for target-mode builders, but define a
# stub that errors loudly if the orchestrator ever calls it by mistake.
builder_build() {
  echo "cloud-build: builder_build called on a target-mode builder (orchestrator bug)" >&2
  return 1
}

builder_finalize() {
  :
}
