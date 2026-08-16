#!/usr/bin/env bash
# Builds the Linux release binary and publishes it as a GitHub release —
# the artifact ct/chanarr.sh's installer (and its in-container update
# path) downloads. Requires the gh CLI, authenticated.
#
# Usage: ./deploy/release.sh v0.1.0
set -euo pipefail
cd "$(dirname "$0")/.."

TAG="${1:-}"
[ -n "$TAG" ] || { echo "usage: $0 <tag>  (e.g. $0 v0.1.0)" >&2; exit 1; }

./deploy/build-release.sh

gh release create "$TAG" dist/chanarr-linux-amd64 \
	--title "chanarr $TAG" \
	--generate-notes

echo "published $TAG — the installer's /releases/latest/ URL now serves this binary"
