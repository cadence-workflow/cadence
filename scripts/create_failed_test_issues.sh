#!/usr/bin/env bash
set -euo pipefail

failed_tests_file="${1:?usage: $0 <failed-tests-file>}"


repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
job_id=$(gh api repos/"${repo}"/actions/runs/"${GITHUB_RUN_ID}"/jobs \
            --jq ".jobs[] | select(.name==\"${GITHUB_JOB}\") | .id")
job_url="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/job/${job_id}"
template=".github/ISSUE_TEMPLATE/test_failure.md"


while IFS= read -r test; do
  [[ -n "$test" ]] || continue

  body=$(sed \
  -e "s|{{TEST_NAME}}|$test|g" \
  -e "s|{{RUN_URL}}|$job_url|g" \
  "$template")

  title="Failed Test: $test"

  matches="$(
    gh issue list \
      --repo "$repo" \
      --state all \
      --search "\"$title\" in:title" \
      --json number,state,title \
      --limit 100
  )"

  open_issue_number="$(
    jq -r --arg title "$title" '
      .[] | select(.title == $title and .state == "OPEN") | .number
    ' <<<"$matches" | head -n1
  )"

  closed_issue_number="$(
    jq -r --arg title "$title" '
      .[] | select(.title == $title and .state != "OPEN") | .number
    ' <<<"$matches" | head -n1
  )"

  if [[ -n "${open_issue_number:-}" ]]; then
    gh issue comment "$open_issue_number" --repo "$repo" --body "$job_url"
  elif [[ -n "${closed_issue_number:-}" ]]; then
    gh issue reopen "$closed_issue_number" --repo "$repo" --comment "$job_url"
  else
    gh issue create --repo "$repo" --title "$title" --body "$body"
  fi
done < "$failed_tests_file"
