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
      - name: Check if should skip
        id: should_skip
        uses: actions/github-script@v7
        with:
          script: |
            const prAuthor = context.payload.pull_request.user.login;
            const prTitle = context.payload.pull_request.title;

            // Skip for Dependabot PRs
            if (prAuthor === 'dependabot[bot]') {
              console.log('Skipping check for Dependabot PR');
              core.setOutput('skip', 'true');
              return;
            }

            // Skip for conventional commit types that don't need issues
            const skipPrefixes = /^(chore|docs|ci|style)(\(.+\))?:/;
            if (skipPrefixes.test(prTitle)) {
              console.log(`Skipping check for PR with title: ${prTitle}`);
              core.setOutput('skip', 'true');
              return;
            }

            console.log('Check will run - PR needs linked issue');
            core.setOutput('skip', 'false');

      - name: Check for linked issues
        if: steps.should_skip.outputs.skip != 'true'
        uses: nearform-actions/github-action-check-linked-issues@v1
        continue-on-error: true
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          comment: false
          loose-matching: true
          exclude-labels: skip-issue-check
          custom-body-comment: |
            ## ⚠️ No Linked Issue Found

            This PR does not appear to have a linked issue in its description.

            ### Why link issues?
            Linking issues helps maintain traceability between PRs and the problems they solve, making it easier to:
            - Understand the context and motivation for changes
            - Track feature development and bug fixes
            - Generate accurate release notes

            ### How to link an issue
            Add the issue number to your PR description:
            - `#123`

            You can also use `Closes #123` [to automatically close the issue](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/linking-a-pull-request-to-an-issue#linking-a-pull-request-to-an-issue-using-a-keyword). 

            ### Skipping this check
            
            This check is automatically skipped for minor changes that may not warrant an issue with the conventional commit prefixes of docs, chore, ci, or style. It is still encouraged to add an issue link if it is relevant. 

            If you need to skip the check apply the label `skip-issue-check` to the PR. 

            ---

            **Note:** This check is currently set to warning-only and won't block merging. However, linking issues is strongly encouraged for all substantive changes.

