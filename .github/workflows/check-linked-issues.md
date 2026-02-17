name: Check Linked Issues

on:
  pull_request_target:
    types:
      - opened
      - edited
      - reopened
      - synchronize
    branches:
      - master

permissions:
  issues: read
  pull-requests: write

jobs:
  check-linked-issues:
    name: Verify PR has linked issue
    runs-on: ubuntu-latest
    continue-on-error: true

    steps:
      - name: Check for linked issues
        if: steps.should_skip.outputs.skip != 'true'
        uses: nearform-actions/github-action-check-linked-issues@v1
        # TODO: Remove this once action is validated
        continue-on-error: true
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          comment: false
          loose-matching: true
          exclude-labels: skip-issue-check
