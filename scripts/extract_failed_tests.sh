#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <go-test-log>" >&2
  exit 2
fi

log_file="$1"
if [[ ! -f "$log_file" ]]; then
  echo "log file not found: $log_file" >&2
  exit 2
fi

tmp_all="$(mktemp)"
tmp_leaf="$(mktemp)"
trap 'rm -f "$tmp_all" "$tmp_leaf"' EXIT

# Extract every failing test name exactly as Go prints it.
# This captures both top-level tests and subtests, e.g.
#   --- FAIL: TestFoo (0.01s)
#   --- FAIL: TestFoo/TestBar (0.00s)
perl -ne 'print "$1\n" if /^\s*--- FAIL: (.+) \(\d+(?:\.\d+)?s\)$/;' "$log_file" \
  | sort -u > "$tmp_all"

# Drop parent tests when a failing subtest exists underneath them.
# Example: keep TestFoo/TestBar, omit TestFoo.
awk '
  {
    tests[NR] = $0
  }
  END {
    for (i = 1; i <= NR; i++) {
      parent = tests[i] "/"
      has_child = 0
      for (j = 1; j <= NR; j++) {
        if (i != j && index(tests[j], parent) == 1) {
          has_child = 1
          break
        }
      }
      if (!has_child) {
        print tests[i]
      }
    }
  }
' "$tmp_all" | sort -u > "$tmp_leaf"

cat "$tmp_leaf"
