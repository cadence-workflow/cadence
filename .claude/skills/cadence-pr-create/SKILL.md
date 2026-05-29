/---
name: cadence-pr-create
description: Create a new GitHub pull request following Cadence repo conventions — runs CI checks locally (lint, unit tests, integration tests), then creates PR with conventional commit title and structured body.
allowed-tools: Bash(git:*), Bash(gh:*), Bash(make:*), Bash(go test:*), Bash(go build:*), Read, Grep, Glob, AskUserQuestion
argument-hint: [issue_number]
---

## Your task

Create a GitHub pull request from the current branch's changes. Before creating the PR, run the same CI checks that GitHub Actions runs. Follow all Cadence repo conventions.

## Step 1 — Gather context

Run these in parallel to understand the current state:

1. `git status` — check for uncommitted changes
2. `git log --oneline master..HEAD` — see all commits on this branch
3. `git diff master...HEAD --stat` — see which files changed
4. `git diff master...HEAD` — full diff for understanding the changes

If there are uncommitted changes, stage and commit them first (ask the user for a commit message if the intent is unclear). All commits **must** include `--signoff` (`-s`) to add a `Signed-off-by` trailer (DCO requirement).

If the current branch is `master`, create a new branch first:
- If a GitHub issue number is provided, use it in the branch name (e.g., `username/fix-issue-123`)
- Otherwise, pick a descriptive slug (e.g., `username/add-retry-to-shard-takeover`)

## Step 2 — Run CI checks locally

These mirror what the `golangci-lint-validate-code-is-clean` CI job does (`scripts/github_actions/golint.sh`):

1. **IDL submodule check**: `make .idl-status` — verifies the `idls/` submodule points to master
2. **`make pr`**  — runs tidy, go-generate, fmt, lint in sequence
3. **Verify no files changed**: `git status --porcelain` — if any files changed after `make pr`, warn the user and stop. This means generated code or formatting is out of date.
4. **Build check**: `make build` — compile-check all packages and test files
5. **Unit tests**: `make test` — runs all unit tests (excludes `host/` integration tests)

### Handling test failures

- If any check fails, **stop and report the failure**. Do not create the PR.
- Show the failing command and relevant error output.
- Suggest fixes if possible.
- After fixing, re-run all CI checks before proceeding.

## Step 3 — Determine PR metadata

### Title

The PR title **must** follow Conventional Commits format: `<type>(<optional scope>): <description>`

Valid types: `fix`, `feat`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `build`, `revert`

Scope should be the Cadence service or package area when applicable (e.g., `history`, `matching`, `frontend`, `persistence`, `common`).

Examples:
- `feat(history): add retry logic for shard takeover`
- `fix(persistence): handle nil pointer in task list manager`
- `refactor(quotas): generalize LimiterFactory with type params`

### Body

Use the PR template from `.github/pull_request_guidance.md`. Structure the body as:

```
## What changed?
<1-3 sentences: what changed technically. Link the GitHub issue if one exists.>

## Why?
<Context for a future maintainer: how did this work before, what was wrong, why this approach?>

## How did you test it?
<Specific test commands that were run and their results. Include exact commands a reviewer can copy-paste.>
<Always include the commands from Step 2 that were actually run and passed.>

## Potential risks
<API/IDL changes? Schema changes? Feature flag reuse? Performance concerns? Core task processing? If none, say "None — isolated change with no API/schema/flag impact.">

## Release notes
<If user-facing, describe the change. Otherwise "N/A — internal change.">

## Documentation Changes
<Link to docs PR, or describe what needs updating, or "N/A">
```

Guidelines for the body:
- Keep it concise — explain the *why*, not the *what* (the diff shows the what)
- Do not use verbose qualifiers like "with proper descriptions and testing"
- Link GitHub issues using `#<number>` or `https://github.com/cadence-workflow/cadence/issues/<number>`
- The "How did you test it?" section must reflect the actual tests run in Step 2

## Step 4 — Link to a GitHub issue

### If `$ARGUMENTS` contains an issue number
Use that directly — no further lookup needed.

### Otherwise — auto-match against open issues

1. **Fetch open issues**:
   ```bash
     gh issue list --repo cadence-workflow/cadence --state open --limit 100 --json number,title,labels,body
   ```

2. **Evaluate relevance**: Compare the PR's changes (files modified, commit messages, diff content) against each open issue's title, body, and labels. Consider:
   - Keyword overlap between commit messages and issue titles/bodies
   - Whether changed file paths match components mentioned in the issue
   - Whether the PR type (fix, feat, etc.) aligns with the issue type (bug, enhancement, etc.)

3. **Rank and suggest**: Pick the top 1-3 most relevant issues (if any seem related). Present them to the user via AskUserQuestion with options:
   - Each candidate issue as an option, formatted as: `#<number>: <title>` (with a description showing the first ~100 chars of the issue body)
   - A "None of these" option — if selected, ask the user to paste an issue URL/number manually, or skip issue linking entirely

4. **If no issues match**: Tell the user no matching open issues were found, and ask whether they want to enter an issue link manually or skip.

### How to link the issue

Once an issue is confirmed:
- Reference it in the "What changed?" section as `Fixes #<number>` (for bug fixes) or `Relates to #<number>` (for features/refactors)
- If the user chose to skip, omit the issue reference

## Step 5 — Push and create the PR

1. Push the branch: `git push -u origin HEAD`
2. Create the PR:

```bash
gh pr create --title "<title>" --body "$(cat <<'EOF'
<body>
EOF
)"
```

- If the user asks for **draft mode**, add `--draft`
- After creation, print the PR URL

## Important rules

- Never include co-authored-by lines or mention the tool used to create the PR
- If `make pr` hasn't been run and there are generated file changes, warn the user and suggest running `make pr` before creating the PR
- If the diff includes changes to files under `.gen/` or files matching `*_generated.go` / `*_mock.go`, warn that these are generated files and `make go-generate` may need to be run
