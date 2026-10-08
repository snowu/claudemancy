#!/usr/bin/env bash
# Builds bin/claudemancy. Uses mise's Go if `go` isn't on PATH directly.
set -euo pipefail
cd "$(dirname "$0")"
go=(go)
command -v go >/dev/null && go version >/dev/null 2>&1 || go=(mise exec "go@$(mise ls --installed go --json | jq -r ".[-1].version")" -- go)
"${go[@]}" build -buildvcs=false -trimpath -ldflags='-s -w' -o bin/claudemancy ./src
