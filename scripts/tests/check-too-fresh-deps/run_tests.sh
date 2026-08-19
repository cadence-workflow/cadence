#!/usr/bin/env bash
set -uo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$script_dir/../../check-too-fresh-deps.sh"

# Fixed reference time so test case "ages" (baked into each case's "input"
# below) never drift with wall-clock time. All "Time" fields in "input" are
# hand-computed relative to this epoch.
now=1700000000

# Table-driven test cases. "input" is a go-list-style JSON stream (newline
# separated JSON objects, matching `go list -m -json all` output).
cases='[
  {
    "name": "no fresh deps -> passes",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/old\",\"Version\":\"v1.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 0,
    "wantOutput": "",
    "wantNotOutput": ""
  },
  {
    "name": "one blocking fresh dep -> fails",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/bar\",\"Version\":\"v1.2.3\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "module: github.com/foo/bar",
    "wantNotOutput": ""
  },
  {
    "name": "exempted fresh dep -> passes but reported",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 0,
    "wantOutput": "module: github.com/cadence-workflow/somepkg",
    "wantNotOutput": ""
  },
  {
    "name": "mixed blocking and exempt -> fails, blocking reported",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/bar\",\"Version\":\"v1.2.3\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-04T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "module: github.com/foo/bar",
    "wantNotOutput": ""
  },
  {
    "name": "mixed blocking and exempt -> exempt section also reported",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/bar\",\"Version\":\"v1.2.3\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-04T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "module: github.com/cadence-workflow/somepkg",
    "wantNotOutput": ""
  },
  {
    "name": "days override: below default threshold -> passes",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/baz\",\"Version\":\"v3.1.0\",\"Time\":\"2023-10-25T22:13:20Z\"}",
    "args": [],
    "wantExit": 0,
    "wantOutput": "",
    "wantNotOutput": "module: github.com/foo/baz"
  },
  {
    "name": "days override: raised threshold catches it -> fails",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/foo/baz\",\"Version\":\"v3.1.0\",\"Time\":\"2023-10-25T22:13:20Z\"}",
    "args": ["--days", "30"],
    "wantExit": 1,
    "wantOutput": "module: github.com/foo/baz",
    "wantNotOutput": ""
  },
  {
    "name": "exempt-prefix override: default prefix does not match -> fails",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/acme/pkg1\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/other/pkg2\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}",
    "args": [],
    "wantExit": 1,
    "wantOutput": "module: github.com/acme/pkg1",
    "wantNotOutput": ""
  },
  {
    "name": "exempt-prefix override: custom prefixes exempt both -> passes",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/acme/pkg1\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/other/pkg2\",\"Version\":\"v1.0.0\",\"Time\":\"2023-11-09T22:13:20Z\"}",
    "args": ["--exempt-prefix", "github.com/acme/,github.com/other/"],
    "wantExit": 0,
    "wantOutput": "module: github.com/acme/pkg1",
    "wantNotOutput": ""
  },
  {
    "name": "empty --exempt-prefix disables exemptions",
    "input": "{\"Path\":\"github.com/uber/cadence\",\"Main\":true}\n{\"Path\":\"github.com/cadence-workflow/somepkg\",\"Version\":\"v0.5.0\",\"Time\":\"2023-11-09T22:13:20Z\"}\n{\"Path\":\"github.com/bar/ancient\",\"Version\":\"v2.0.0\",\"Time\":\"2022-10-10T22:13:20Z\"}",
    "args": ["--exempt-prefix", ""],
    "wantExit": 1,
    "wantOutput": "module: github.com/cadence-workflow/somepkg",
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
	if [[ -n "$want_output" ]] && ! grep -qF -- "$want_output" <<<"$output"; then
		echo "FAIL $name: expected output to contain '$want_output'"
		ok=0
	fi
	if [[ -n "$want_not_output" ]] && grep -qF -- "$want_not_output" <<<"$output"; then
		echo "FAIL $name: expected output NOT to contain '$want_not_output'"
		ok=0
	fi

	if [[ "$ok" -eq 1 ]]; then
		echo "PASS $name"
	else
		echo "$output" | sed 's/^/    | /'
		failures=$((failures + 1))
	fi
done < <(jq -c '.[]' <<<"$cases")

if [[ "$failures" -gt 0 ]]; then
	echo "$failures test case(s) failed"
	exit 1
fi

echo "all test cases passed"
