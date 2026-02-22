---
title: GitHub Issue Linking Requirement
description: Ensures PRs link to cadence-workflow org issues with smart exemptions
when: PR is opened or updated
actions: Check for issue link and report requirement status
---

# GitHub Issue Linking Requirement

This rule ensures that pull requests link to GitHub issues from the cadence-workflow organization, providing better tracking, context, and historical linkage between code changes and their motivations.

## Overview

When evaluating a pull request:

1. **Check skip conditions first** - Some PRs are exempt from this requirement
2. **Search for issue links** - Look for references in the PR description body
3. **Validate organization** - Ensure links point to cadence-workflow org
4. **Report status** - Approve silently if valid link found, or provide helpful feedback if missing

## Skip Conditions

The issue linking requirement is automatically skipped when **ANY** of these conditions are true:

### 1. Small Changes (< 50 lines)
- Count total lines changed: additions + deletions
- PRs with fewer than 50 total lines changed are exempt
- Example: Quick typo fixes, minor documentation updates

### 2. Maintenance Commits
PRs with titles starting with these conventional commit prefixes are exempt:
- `docs:` - Documentation changes
- `chore:` - Maintenance tasks, dependency updates
- `ci:` - CI/CD configuration changes
- `style:` - Code formatting or style changes
- `revert:` - Pull Request Reverts

Examples:
- ✅ Skip: `docs: Update installation instructions`
- ✅ Skip: `chore: Bump dependency versions`
- ✅ Skip: `ci: Add new workflow for release`
- ✅ Skip: `style: Apply consistent formatting`
- ❌ Don't skip: `feat: Add new authentication method`
- ❌ Don't skip: `fix: Resolve memory leak in worker`

### 3. Bot-Authored PRs
- PRs where the author username ends with `[bot]`
- Examples: `dependabot[bot]`, `renovate[bot]`, `github-actions[bot]`

## Issue Link Detection

Search the **PR description body only** for issue references in these formats:

### Accepted Formats

1. **Same repository short format:**
   - `#123`
   - Example: "Fixes #456"
   - Example: "Related to #789"

2. **Cross-repository format:**
   - `cadence-workflow/other-repo#123`
   - Example: "Addresses cadence-workflow/web#45"
   - Must reference the `cadence-workflow` organization

3. **Full URL format:**
   - `https://github.com/cadence-workflow/cadence/issues/123`
   - `https://github.com/cadence-workflow/other-repo/issues/456`
   - Must be from the `cadence-workflow` organization

### Not Accepted

- ❌ Issues from other organizations: `other-org/repo#123`
- ❌ Issue links in code comments or file content
- ❌ Issue links in commit messages (must be in PR description)

## Organization Validation

**CRITICAL**: All issue links must reference the `cadence-workflow` organization.

How to validate:
- Short format (`#123`) is implicitly within cadence-workflow if the PR is in a cadence-workflow repo
- Cross-repo format must explicitly include `cadence-workflow/`
- Full URLs must contain `github.com/cadence-workflow/`

Examples:
- ✅ Valid: `#123` (in cadence-workflow repo)
- ✅ Valid: `cadence-workflow/web#45`
- ✅ Valid: `https://github.com/cadence-workflow/cadence/issues/789`
- ❌ Invalid: `external-org/repo#123`
- ❌ Invalid: `https://github.com/other-org/repo/issues/123`

## Validation Logic

### Step 1: Check Skip Conditions

```
1. Get PR diff stats
2. Calculate total_lines = additions + deletions
3. If total_lines < 50, skip and approve
4. Get PR title
5. If title starts with "docs:", "chore:", "ci:", or "style:", skip and approve
6. Get PR author username
7. If author ends with "[bot]", skip and approve
```

### Step 2: Search for Issue Links

```
1. Get PR description body
2. Search for patterns:
   - #\d+
   - cadence-workflow/[\w-]+#\d+
   - https://github.com/cadence-workflow/[\w-]+/issues/\d+
3. For each match, extract organization name
4. Validate organization is "cadence-workflow"
```

### Step 3: Report Status

Report as part of all rules. 

## Edge Cases

### Multiple Issue Links
- If the PR description contains multiple valid issue links, the rule passes
- Use the first valid link found for reporting purposes

### Issue Links in Code
- Issue links in code comments or file content do not count
- Only links in the PR description body are evaluated

### Mixed Formats
- A PR can reference issues in multiple formats
- The rule passes if ANY format contains a valid cadence-workflow issue link

### Case Sensitivity
- Organization name matching is case-insensitive
- `cadence-workflow`, `Cadence-Workflow`, and `CADENCE-WORKFLOW` are all valid

## Integration Notes

This rule is complementary to the existing `pr-description-quality.md` rule:
- **Description quality**: Checks content structure and completeness
- **Issue linking**: Checks for specific requirement (issue reference)
- Both rules can run on the same PR without conflicts
- Both provide guidance without blocking PRs

