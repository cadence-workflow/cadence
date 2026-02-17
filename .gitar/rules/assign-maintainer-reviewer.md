---
title: Assign Maintainer Reviewer for External Contributors
description: Automatically assigns a maintainer to review PRs from non-maintainers
when: Pull request is opened
actions:
  - Assign reviewer in round-robin rotation
---

# Auto-assign Maintainer Reviewer

When a pull request is opened by someone not listed in `.github/CODEOWNERS`, assign @natemort plus one reviewer from the following list in round-robin rotation:

- @c-warren
- @fimanishi 
- @neil-xie
- @zawadzkidiana
- @shijiesheng
- @abhishekj720 

You must modify the assignees field on the Pull Request - being a reviewer is not sufficient.

Do not assign a reviewer if the PR author is already a maintainer listed in CODEOWNERS.
