#!/usr/bin/env bash
# Builds bin/claudemancy. Uses mise's Go if `go` isn't on PATH directly.
#   --if-stale  only build when the binary is missing or older than its sources
set -euo pipefail
cd "$(dirname "$0")"
if [[ ${1:-} == --if-stale && -x bin/claudemancy ]]; then
  stale=0
  for f in src/*.go src/*.json go.mod; do [[ $f -nt bin/claudemancy ]] && stale=1; done
  ((stale)) || exit 0
fi
go=(go)
command -v go >/dev/null && go version >/dev/null 2>&1 || go=(mise exec "go@$(mise ls --installed go --json | jq -r ".[-1].version")" -- go)
"${go[@]}" build -buildvcs=false -trimpath -ldflags='-s -w' -o bin/claudemancy ./src
