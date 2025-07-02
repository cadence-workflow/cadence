#!/bin/sh

set -ex

verify_no_files_changed () {
  # intentionally capture stderr, so status-errors are also PR-failing.
  # in particular this catches "dubious ownership" failures, which otherwise
  # do not fail this check and the $() hides the exit code.
  NUM_FILES_CHANGED=$(git status --porcelain 2>&1 | wc -l)
  if [ "${NUM_FILES_CHANGED}" -ne 0 ]; then
    printf "There file changes after applying your diff and performing a build.\n"
    printf "Please run this command and commit the changes:\n"
    printf "\tmake tidy && make go-generate && make fmt && make lint\n"
    git status --porcelain
    git --no-pager diff
    exit 1
  fi
}

# Run the fast checks first, to fail quickly.
make tidy
make fmt
make lint

verify_no_files_changed

# Run go-generate after the fast checks, to avoid unnecessary waiting
# We need to run fmt and lint after, as the generated files
# may change the output of these commands.
make go-generate
make fmt
make lint

verify_no_files_changed

# TODO(bk-cleanup): Remove this file after fully migrated to github actions