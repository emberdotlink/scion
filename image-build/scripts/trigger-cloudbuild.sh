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

# Deprecation shim for the old trigger-cloudbuild.sh entry point.
#
# Forwards to `build-images.sh --builder cloud-build`. The old positional
# `target` argument and the `--project` / `--registry` flags are translated.
#
# This shim will be removed in a future release. New code and CI should
# call `build-images.sh --builder cloud-build` directly.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PROJECT=""
REGISTRY=""
TARGET=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --project)  PROJECT="$2"; shift 2 ;;
    --registry) REGISTRY="$2"; shift 2 ;;
    -h|--help)
      cat <<EOF
$(basename "$0") is deprecated. Use:

  image-build/scripts/build-images.sh --builder cloud-build --registry <registry> [--target <target>]

Old usage (still supported by this shim):
  $(basename "$0") [--project <project>] [--registry <registry>] [target]
EOF
      exit 0
      ;;
    -*) echo "Unknown option: $1" >&2; exit 1 ;;
    *)  TARGET="$1"; shift ;;
  esac
done

TARGET="${TARGET:-common}"

echo "Note: trigger-cloudbuild.sh is deprecated."
echo "      Forwarding to: build-images.sh --builder cloud-build --target ${TARGET}"
echo ""

# --project is no longer a flag; the cloud-build builder reads $GCLOUD_PROJECT
# or `gcloud config get-value project`. Translate by exporting the env var
# for the duration of this invocation.
if [[ -n "${PROJECT}" ]]; then
  export GCLOUD_PROJECT="${PROJECT}"
fi

# --registry is required by build-images.sh. If the caller didn't pass one,
# fall back to the same Artifact Registry path the old script implicitly
# resolved at GCB substitution time.
if [[ -z "${REGISTRY}" ]]; then
  resolved_project="${GCLOUD_PROJECT:-}"
  if [[ -z "${resolved_project}" ]]; then
    resolved_project="$(gcloud config get-value project 2>/dev/null)" || true
  fi
  if [[ -z "${resolved_project}" ]]; then
    echo "Error: could not determine GCP project for default --registry." >&2
    echo "Pass --registry <path> or set GCLOUD_PROJECT / 'gcloud config set project'." >&2
    exit 1
  fi
  REGISTRY="us-central1-docker.pkg.dev/${resolved_project}/public-docker"
fi

exec "${SCRIPT_DIR}/build-images.sh" \
  --builder cloud-build \
  --registry "${REGISTRY}" \
  --target "${TARGET}"
