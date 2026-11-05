#!/usr/bin/env bash

set -eou pipefail

# basic idea:
# - `go test -list` to print all the test names
# - janky `jq` to filter that output, because -json doesn't differentiate
#   between "what you asked for" vs "the package's results"
#   vs "random in-test output" which is absolutely ridiculous imo
# - iterate through that output and execute it, so we can tell which test
#   produced what output / failed / etc

target="$1"
shift

test_names_orig="$(go test -list '.' -json "$target")"
test_name_lines="$(
  echo "$test_names_orig" \
  | jq 'select(.Action == "output") | select(.Output | startswith("Test"))' --compact-output --raw-output
)"
test_names_and_args="$(
  echo "$test_name_lines" \
  | jq '{"package":(.Package | ltrimstr("github.com/uber/cadence/")), "name":(.Output | rtrimstr("\n"))}' --compact-output --raw-output
)"

while read -r each_test; do
  pkg="$(echo "$each_test" | jq .package --raw-output)"
  tst="'^$(echo "$each_test" | jq .name --raw-output)\$'"
  cmd="go test ${*@Q} -run ${tst} ./${pkg}"
  echo "----------------------------------------------"
  echo "$cmd"
  eval "$cmd"
  echo "----------------------------------------------"
done < <(echo "$test_names_and_args")
