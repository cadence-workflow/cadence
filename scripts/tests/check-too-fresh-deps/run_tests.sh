#!/usr/bin/env bash
set -uo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$script_dir/../../check-too-fresh-deps.sh"

# Fixed reference time so test case "ages" (baked into each case's "input"
# below) never drift with wall-clock time. All "Time" fields in "input" are
# hand-computed relative to this epoch, in the same UTC ISO8601 format as
# "Time": now=1700000000 == 2023-11-14T22:13:20Z.
now=1700000000

# Table-driven test cases. "input" is a go-list-style JSON stream (newline
# separated JSON objects, matching `go list -m -json all` output).
# "wantOutput"/"wantNotOutput" are matched as a (possibly multi-line)
# substring of the full script output, so they can assert on the complete
# per-module detail block (and which section - blocking vs exempted - it
# landed in), not just that a module path appears somewhere.
cases='[
  {
    "name": "no fresh deps -> passes with no output",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/old\",\"Version\":\"v1.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 0,
    "wantOutput": "",
    "wantNotOutput": ""
  },
  {
    "name": "one blocking fresh dep -> fails, full detail block printed",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/bar\",\"Version\":\"v1.2.3\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "Dependencies published within 14 days (blocking):\n\nmodule: github.com/foo/bar\nversion: v1.2.3\npublished: 2023-11-09T22:13:20Z\ndays_since_published: 5\ndays_until_eligible: 9",
    "wantNotOutput": ""
  },
  {
    "name": "exempted fresh dep -> passes, full detail block printed as exempted",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 0,
    "wantOutput": "Exempted dependencies published within 14 days (not blocking):\n\nmodule: github.com/cadence-workflow/somepkg\nversion: v0.5.0\npublished: 2023-11-09T22:13:20Z\ndays_since_published: 5\ndays_until_eligible: 9",
    "wantNotOutput": ""
  },
  {
    "name": "mixed blocking and exempt -> fails, blocking block printed",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/bar\",\"Version\":\"v1.2.3\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-04T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "Dependencies published within 14 days (blocking):\n\nmodule: github.com/foo/bar\nversion: v1.2.3\npublished: 2023-11-09T22:13:20Z\ndays_since_published: 5\ndays_until_eligible: 9",
    "wantNotOutput": ""
  },
  {
    "name": "mixed blocking and exempt -> exempt block also printed",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/bar\",\"Version\":\"v1.2.3\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-04T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "Exempted dependencies published within 14 days (not blocking):\n\nmodule: github.com/cadence-workflow/somepkg\nversion: v0.5.0\npublished: 2023-11-04T22:13:20Z\ndays_since_published: 10\ndays_until_eligible: 4",
    "wantNotOutput": ""
  },
  {
    "name": "days override: below default threshold -> passes with no output",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/baz\",\"Version\":\"v3.1.0\",\"Time\":\"2023-10-25T22:13:20Z\"}",
    "args": [],
    "wantExit": 0,
    "wantOutput": "",
    "wantNotOutput": "github.com/foo/baz"
  },
  {
    "name": "days override: raised threshold catches it -> fails, full detail block printed",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/baz\",\"Version\":\"v3.1.0\",\"Time\":\"2023-10-25T22:13:20Z\"}",
    "args": ["--days", "30"],
    "wantExit": 1,
    "wantOutput": "Dependencies published within 30 days (blocking):\n\nmodule: github.com/foo/baz\nversion: v3.1.0\npublished: 2023-10-25T22:13:20Z\ndays_since_published: 20\ndays_until_eligible: 10",
    "wantNotOutput": ""
  },
  {
    "name": "exempt-prefix override: default prefix does not match -> fails, blocking block printed",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/acme/pkg1\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/other/pkg2\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "Dependencies published within 14 days (blocking):\n\nmodule: github.com/acme/pkg1\nversion: v1.0.0\npublished: 2023-11-09T22:13:20Z\ndays_since_published: 5\ndays_until_eligible: 9",
    "wantNotOutput": ""
  },
  {
    "name": "exempt-prefix override: custom prefixes exempt both -> passes, exempted block printed",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/acme/pkg1\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/other/pkg2\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}",
    "args": ["--exempt-prefix", "github.com/acme/,github.com/other/"],
    "wantExit": 0,
    "wantOutput": "Exempted dependencies published within 14 days (not blocking):\n\nmodule: github.com/acme/pkg1\nversion: v1.0.0\npublished: 2023-11-09T22:13:20Z\ndays_since_published: 5\ndays_until_eligible: 9",
    "wantNotOutput": ""
  },
  {
    "name": "empty --exempt-prefix disables exemptions -> blocking block printed",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": ["--exempt-prefix", ""],
    "wantExit": 1,
    "wantOutput": "Dependencies published within 14 days (blocking):\n\nmodule: github.com/cadence-workflow/somepkg\nversion: v0.5.0\npublished: 2023-11-09T22:13:20Z\ndays_since_published: 5\ndays_until_eligible: 9",
    "wantNotOutput": ""
  },
  {
    "name": "malformed input -> fails without hanging",
    "input": "this is not json at all",
    "args": [],
    "wantExit": "nonzero",
    "wantOutput": "",
    "wantNotOutput": ""
  }
]'

failures=0

while IFS= read -r case_row; do
	name="$(jq -r '.name' <<<"$case_row")"
	input="$(jq -r '.input' <<<"$case_row")"
	want_exit="$(jq -r '.wantExit' <<<"$case_row")"
	want_output="$(jq -r '.wantOutput' <<<"$case_row")"
	want_not_output="$(jq -r '.wantNotOutput' <<<"$case_row")"

	args=()
	while IFS= read -r arg; do
		args+=("$arg")
	done < <(jq -r '.args[]?' <<<"$case_row")

	output="$("$script" --now "$now" --input "$input" "${args[@]}" 2>&1)"
	actual_exit=$?

	ok=1
	if [[ "$want_exit" == "nonzero" ]]; then
		if [[ "$actual_exit" -eq 0 ]]; then
			echo "FAIL $name: expected non-zero exit, got 0"
			ok=0
		fi
	elif [[ "$actual_exit" -ne "$want_exit" ]]; then
		echo "FAIL $name: expected exit $want_exit, got $actual_exit"
		ok=0
	fi
	# bash substring match (not grep) so multi-line blocks compare as one unit.
	if [[ -n "$want_output" && "$output" != *"$want_output"* ]]; then
		echo "FAIL $name: expected output to contain:"
		echo "$want_output" | sed 's/^/    want> /'
		ok=0
	fi
	if [[ -n "$want_not_output" && "$output" == *"$want_not_output"* ]]; then
		echo "FAIL $name: expected output NOT to contain '$want_not_output'"
		ok=0
	fi

	if [[ "$ok" -eq 1 ]]; then
		echo "PASS $name"
	else
		echo "$output" | sed 's/^/    got>  /'
		failures=$((failures + 1))
	fi
done < <(jq -c '.[]' <<<"$cases")

if [[ "$failures" -gt 0 ]]; then
	echo "$failures test case(s) failed"
	exit 1
fi

echo "all test cases passed"
