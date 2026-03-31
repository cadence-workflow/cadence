#!/usr/bin/env bash
set -euo pipefail

failed_tests_file="${1:?usage: $0 <failed-tests-file>}"

repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
job_url="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/job/${GITHUB_JOB}"

while IFS= read -r test; do
  [[ -n "$test" ]] || continue

  body=$(cat <<EOF
GitHub Actions job: ${job_url}

Failed test: \`${test}\`
EOF
)

  matches="$(
    gh issue list \
      --repo "$repo" \
      --state all \
      --search "\"$test\" in:title" \
      --json number,state,title \
      --limit 100
  )"

  open_issue_number="$(
    jq -r --arg title "$test" '
      .[] | select(.title == $title and .state == "OPEN") | .number
    ' <<<"$matches" | head -n1
  )"

  closed_issue_number="$(
    jq -r --arg title "$test" '
      .[] | select(.title == $title and .state != "OPEN") | .number
    ' <<<"$matches" | head -n1
  )"

  if [[ -n "${open_issue_number:-}" ]]; then
    gh issue comment "$open_issue_number" --repo "$repo" --body "$body"
  elif [[ -n "${closed_issue_number:-}" ]]; then
    gh issue reopen "$closed_issue_number" --repo "$repo" --comment "$body"
  else
    gh issue create --repo "$repo" --title "$test" --body "$body"
  fi
done < "$failed_tests_file"
