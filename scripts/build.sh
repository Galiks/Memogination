#!/usr/bin/env bash
# Builds the Memomarium binary for the current platform (dist/memomarium).
# Requires Node.js, npm, and Go on PATH.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 1. Build the frontend (web/dist is embedded into the binary).
(cd "$ROOT/web" && npm ci && npm run build)

# 2. Build the native binary for the current platform.
mkdir -p "$ROOT/dist"
CGO_ENABLED=0 go build -o "$ROOT/dist/memomarium" ./cmd/memomarium

echo "Built dist/memomarium"