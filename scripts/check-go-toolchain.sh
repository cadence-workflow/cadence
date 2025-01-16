#!/usr/bin/env bash

set -eo pipefail

fail=0
bad() {
  echo -e "$@" >/dev/stderr
  fail=1
}

if [[ $V == 1 ]]; then
  set -x
fi

target="${1#go}"
root="$(git rev-parse --show-toplevel)"

# this SHOULD match the dependencies in the goversion-lint check to avoid skipping it

# check dockerfiles
while read file; do
  # find "FROM golang:1.22.3-alpine3.18 ..." lines
  line="$(grep -i 'from golang:' "$file")"
  # remove "from golang:" prefix
  version="${line#*golang:}"
  # remove "-alpine..." suffix
  version="${version%-*}"
  # and make sure it matches
  if [[ "$version" != "$target" ]]; then
    bad "Wrong Go version in file $file:\n\t$line"
  fi
done < <( find "$root" -name Dockerfile )

declare -a codecov_files=( "$root/.github/workflows/codecov-on-pr.yml" "$root/.github/workflows/codecov-on-master.yml" );

for codecov_file in "${codecov_files[@]}"; do
  # check workflows
  codecov_file="$root/.github/workflows/codecov-on-pr.yml"
  codecov_line="$(grep 'go-version:' "$codecov_file")"
  codecov_version="${codecov_line#*go-version: }"
  if [[ "$codecov_version" != "$target" ]]; then
    bad "Wrong Go version in file $codecov_file:\n\t$codecov_line"
  fi
done

if [[ $fail == 1 ]]; then
  bad "Makefile pins Go to go${target}, Dockerfiles and GitHub workflows should too."
  bad "Non-matching versions lead to pointless double-downloading to get the correct version."
  exit 1
fi
