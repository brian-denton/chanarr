#!/usr/bin/env bash
# Builds the single-binary Linux release of chanarr on a dev machine:
# frontend first (go:embed picks up internal/webui/dist), then a static
# cross-compile — everything is pure Go (modernc sqlite, go-smb2,
# go-nfs-client), so CGO_ENABLED=0 needs no toolchain beyond Go itself.
#
# Output: dist/chanarr-linux-amd64 (override arch: GOARCH=arm64 ./deploy/build-release.sh)
set -euo pipefail
cd "$(dirname "$0")/.."

export GOARCH="${GOARCH:-amd64}"
export GOOS=linux CGO_ENABLED=0

(cd web && npm install && npm run build)
go build -trimpath -ldflags="-s -w" -o "dist/chanarr-linux-$GOARCH" ./cmd/chanarr

echo "built dist/chanarr-linux-$GOARCH"
