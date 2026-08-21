#!/usr/bin/env bash
set -uo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failures=0
found=0

for test_script in "$script_dir"/*/run_tests.sh; do
	[[ -f "$test_script" ]] || continue
	found=$((found + 1))

	suite="$(basename "$(dirname "$test_script")")"
	echo "=== $suite ==="
	if ! "$test_script"; then
		failures=$((failures + 1))
	fi
	echo
done

if [[ "$found" -eq 0 ]]; then
	echo "no test suites found under $script_dir/*/run_tests.sh" >&2
	exit 1
fi

if [[ "$failures" -gt 0 ]]; then
	echo "$failures test suite(s) failed"
	exit 1
fi

echo "all script test suites passed"
