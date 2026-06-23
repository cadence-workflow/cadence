---
title: Assign Maintainer Reviewer for External Contributors
description: Automatically assigns a maintainer and posts a welcome comment when a non-maintainer opens a PR
when: Pull request is opened
actions:
  - Assign maintainer as assignee in round-robin rotation
  - Post welcome comment with Slack community details
---

# Auto-assign Maintainer Reviewer

## Overview

When a pull request is opened by someone who is not a project maintainer, assign a maintainer as the PR assignee and post a welcome comment pointing the contributor to the community Slack.

## Detection Logic

### Step 1: Identify the PR author

Get the GitHub username of the person who opened the pull request.

### Step 2: Build the maintainer list from MAINTAINERS.md

Read `MAINTAINERS.md`. Extract all GitHub usernames by parsing the GitHub profile URLs in the format `https://github.com/<username>` — these appear in both the **TSC** and **Maintainers** sections. The full list of maintainers is the union of both sections.

### Step 3: Check if the author is a maintainer

Compare the PR author's username (case-insensitively) against the extracted maintainer list.

**If the author is a maintainer → skip this rule entirely. Take no action.**

### Step 4: Assign a maintainer (for non-maintainer PRs only)

Perform **both** of the following:

1. **Always assign `@natemort`** as an assignee on the pull request.
2. **Assign one additional reviewer** from the rotation list below, selected by taking `(PR number) mod 6` to determine the index (0-based):

   - 0 → @c-warren
   - 1 → @fimanishi
   - 2 → @neil-xie
   - 3 → @zawadzkidiana
   - 4 → @shijiesheng
   - 5 → @abhishekj720

**Important:** You must set the `assignees` field on the pull request, not just the `reviewers` field. Both `@natemort` and the rotation pick must appear as assignees.

### Step 5: Post a welcome comment (for non-maintainer PRs only)

Post the following comment on the pull request, substituting the PR author's GitHub username for `{author}`:

---

Hi @{author}, thanks for your contribution! A maintainer has been assigned to review your PR.

While you wait, consider joining our community on the **CNCF Slack** — the `#cadence-contributors` channel is the best place to ask questions, get help unblocking test runs, and connect directly with maintainers to help accelerate your review.

👉 [Join the CNCF Slack](https://communityinviter.com/apps/cloud-native/cncf) · [Getting started guide](https://cadenceworkflow.io/community/how-to-contribute/getting-started#join-the-community)

---

## Skip Conditions

Skip this rule entirely (no assignment, no comment) when **either** of these is true:

- The PR author's GitHub username appears in `MAINTAINERS.md` (TSC or Maintainers section)
- The pull request was opened by a bot account
