---
title: Restrict GitHub Actions Workflow Changes to Maintainers
description: Flags PRs from non-maintainers that modify GitHub Actions workflow files and asks them to be removed
when: PR is opened or updated
actions: Detect changes to .github/workflows/ from non-maintainers and request removal
---

# Restrict GitHub Actions Workflow Changes to Maintainers

GitHub Actions workflow files control CI/CD execution and can be a vector for supply-chain attacks (e.g. exfiltrating secrets, running malicious code on runners). Only repository maintainers should modify them.

## Overview

When evaluating a pull request:

1. **Identify the PR author**
2. **Check if any file under `.github/workflows/` is modified**
3. **Check if the author is a maintainer** (listed in `.github/CODEOWNERS`, directly or as a member of a listed team)
4. **If a non-maintainer modifies workflow files, request removal**

## Detection Logic

### Step 1: Get the list of changed files

Inspect the PR diff for any files matching:
- `.github/workflows/**` (any file inside the workflows directory)

If no workflow files are modified, skip this rule.

### Step 2: Check author against CODEOWNERS

Read `.github/CODEOWNERS` from the **base branch**, not from the PR head, so that a PR cannot add its own author. Extract the owners from the global `*` ownership line. An owner is either:
- a user, in the format `@<username>`
- a team, in the format `@<org>/<team-slug>` (for example `@cadence-workflow/cadence-dev`)

The PR author is a maintainer if **either** is true:
- The author's username is one of the user owners (case-insensitive)
- The author is a member of one of the team owners. Check the membership with the GitHub API: `GET /orgs/<org>/teams/<team-slug>/memberships/<username>`. The author is a member only when the response has `"state": "active"`. A `404` response means that the author is not a member.

If the PR author is a maintainer, the change is allowed. Skip and approve.

If you cannot read the team membership (for example, the API returns `403`), do not post the removal comment. Instead, post a short comment that says the membership check failed and that a maintainer must review the workflow changes.

### Step 3: Report when a non-maintainer touches workflow files

When a non-maintainer PR modifies `.github/workflows/`, post a comment asking the contributor to:

1. **Remove the workflow changes from this PR** and resubmit the rest of the change without them, AND
2. **Open a separate issue** describing the workflow change they want made, so a maintainer can implement it in a dedicated PR

Include the list of workflow files that were modified.

## Skip Conditions

The rule is automatically skipped when **ANY** of these is true:

### 1. PR author is a maintainer
- Listed in `.github/CODEOWNERS` on the base branch, directly or as a member of a listed team

### 2. No workflow files changed
- Diff does not touch `.github/workflows/`

## Example Report

> ⚠️ **Workflow changes require maintainer authorship**
>
> This PR modifies the following GitHub Actions workflow file(s):
> - `.github/workflows/ci-checks.yml`
>
> Changes to `.github/workflows/` can only be made by repository maintainers, since they control CI execution and have access to repo secrets.
>
> **Please:**
> 1. Revert the workflow changes from this PR so the rest can be reviewed.
> 2. Open a [new issue](https://github.com/cadence-workflow/cadence/issues/new) describing the workflow change you'd like, and a maintainer will pick it up in a separate PR.

## Integration Notes

This rule is complementary to other Gitar rules:
- **PR description quality**: Checks content structure
- **Issue linking**: Checks for issue reference
- **Workflow restriction**: Gates a security-sensitive path

All rules run independently; this one only fires when workflow files are touched.
