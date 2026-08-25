#!/usr/bin/env bash
set -euo pipefail

MAX_AGE_DAYS="${MAX_AGE_DAYS:-14}"

fresh=$(go list -m -u -json all | jq -rs --argjson maxAgeDays "$MAX_AGE_DAYS" '
  .[]
  | select(.Time)
  | ((.Time|fromdateiso8601)) as $t
  | select((now-$t) < ($maxAgeDays*86400))
  | "\(.Path) - published: \(.Time) - eligible: \(($t+$maxAgeDays*86400)|todateiso8601)"
')

if [ -n "$fresh" ]; then
  echo "Identified go modules which are too fresh (less than ${MAX_AGE_DAYS} days):"
  echo "$fresh"
  exit 1
fi
