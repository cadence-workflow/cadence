#!/usr/bin/env bash
set -euo pipefail

MAX_AGE_DAYS="${MAX_AGE_DAYS:-14}"
EXEMPT_PREFIXES="${EXEMPT_PREFIXES:-github.com/uber/cadence-idl,github.com/cadence-workflow}"

exempt_json=$(jq -cn --arg s "$EXEMPT_PREFIXES" '$s | split(",") | map(select(length > 0))')

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
failed=0

while IFS= read -r -d '' gomod; do
  moddir="$(dirname "$gomod")"
  modrel="${moddir#"$repo_root"/}"
  [ "$moddir" = "$repo_root" ] && modrel="."

  fresh=$(cd "$moddir" && go list -m -json all | jq -rs --argjson maxAgeDays "$MAX_AGE_DAYS" --argjson exempt "$exempt_json" '
    def is_exempt($path): any($exempt[]; . as $p | $path | startswith($p));
    .[]
    | select(.Time)
    | select(is_exempt(.Path) | not)
    | (try (.Time|fromdateiso8601) catch null) as $t
    | select($t != null)
    | select((now-$t) < ($maxAgeDays*86400))
    | "\(.Path) - published: \(.Time) - eligible: \(($t+$maxAgeDays*86400)|todateiso8601)"
  ')

  if [ -n "$fresh" ]; then
    echo "[$modrel] Identified go modules which are too fresh (less than ${MAX_AGE_DAYS} days):"
    echo "$fresh"
    failed=1
  fi
done < <(find "$repo_root" -name go.mod -not -path "$repo_root/idls/*" -print0)

exit "$failed"
