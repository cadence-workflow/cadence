#!/usr/bin/env bash
set -uo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixtures_dir="$script_dir/fixtures"
script="$script_dir/../../check-too-fresh-deps.sh"

# fixed reference time so fixture ages never drift with wall-clock time
now=1700000000

failures=0

# args: name, fixture, expected_exit, expect_substring, unexpect_substring, [extra script args...]
run_case() {
	local name="$1" fixture="$2" expected_exit="$3" expect="$4" unexpect="$5"
	shift 5
	local input output actual_exit

	input="$(cat "$fixtures_dir/$fixture")"
	output="$("$script" --now "$now" --input "$input" "$@" 2>&1)"
	actual_exit=$?

	local ok=1
	if [[ "$expected_exit" == "nonzero" ]]; then
		if [[ "$actual_exit" -eq 0 ]]; then
			echo "FAIL $name: expected non-zero exit, got 0"
			ok=0
		fi
	elif [[ "$actual_exit" -ne "$expected_exit" ]]; then
		echo "FAIL $name: expected exit $expected_exit, got $actual_exit"
		ok=0
	fi
	if [[ -n "$expect" ]] && ! grep -qF -- "$expect" <<<"$output"; then
		echo "FAIL $name: expected output to contain '$expect'"
		ok=0
	fi
	if [[ -n "$unexpect" ]] && grep -qF -- "$unexpect" <<<"$output"; then
		echo "FAIL $name: expected output NOT to contain '$unexpect'"
		ok=0
	fi

	if [[ "$ok" -eq 1 ]]; then
		echo "PASS $name"
	else
		echo "$output" | sed 's/^/    | /'
		failures=$((failures + 1))
	fi
}

run_case "no fresh deps -> passes" \
	"no_fresh_deps.json" 0 "" ""

run_case "one blocking fresh dep -> fails" \
	"one_blocking.json" 1 "module: github.com/foo/bar" ""

run_case "exempted fresh dep -> passes but reported" \
	"one_exempt.json" 0 "module: github.com/cadence-workflow/somepkg" ""

run_case "mixed blocking and exempt -> fails, both reported" \
	"mixed.json" 1 "module: github.com/foo/bar" ""
run_case "mixed blocking and exempt -> exempt section reported" \
	"mixed.json" 1 "module: github.com/cadence-workflow/somepkg" ""

run_case "days override: below default threshold -> passes" \
	"days_override.json" 0 "" "module: github.com/foo/baz"
run_case "days override: raised threshold catches it -> fails" \
	"days_override.json" 1 "module: github.com/foo/baz" "" \
	--days 30

run_case "exempt-prefix override: default prefix does not match -> fails" \
	"exempt_prefix_override.json" 1 "module: github.com/acme/pkg1" ""
run_case "exempt-prefix override: custom prefixes exempt both -> passes" \
	"exempt_prefix_override.json" 0 "module: github.com/acme/pkg1" "" \
	--exempt-prefix "github.com/acme/,github.com/other/"

run_case "empty --exempt-prefix disables exemptions" \
	"one_exempt.json" 1 "module: github.com/cadence-workflow/somepkg" "" \
	--exempt-prefix ""

run_case "malformed input -> fails without hanging" \
	"malformed.json" nonzero "" ""

if [[ "$failures" -gt 0 ]]; then
	echo "$failures test case(s) failed"
	exit 1
fi

echo "all test cases passed"
