---
title: No License Headers in Source Files
description: Prevents MIT or other per-file license headers from being added to Go files
when: PR adds or modifies Go files
actions: Detect license headers in new or modified Go files; recommend removal
---

# No License Headers in Source Files

The project uses a single top-level `LICENSE` file (Apache 2.0). Per-file license headers are not used and should not be added. Existing MIT license headers are being removed as part of the license migration (see GitHub issue #8483).

## Overview

When evaluating a pull request:

1. **Check for license headers in new Go files** — new files must not include any license header
2. **Check for license headers reintroduced in modified files** — edits must not add a license block
3. **Recommend removal** when violations are found

## Detection Logic

### Step 1: Identify Go files with license headers

Scan the PR diff for **added lines** in `**/*.go` files that contain any of these markers:

- `Permission is hereby granted` (MIT license)
- `Licensed under the Apache License` (Apache boilerplate — the top-level LICENSE file is sufficient)
- `SPDX-License-Identifier` (SPDX header)
- `THE SOFTWARE IS PROVIDED "AS IS"` (MIT warranty disclaimer)

Only flag lines that appear in the **added** side of the diff (lines starting with `+`), not removed lines.

### Step 2: Distinguish headers from legitimate references

Do **not** flag:
- Files under `.gen/` — these are generated from IDL and may carry upstream headers
- Test files that reference license text as test data
- Comments that discuss licensing (e.g., `// This package is licensed under...` in documentation)

Flag only structured license blocks: multi-line comment blocks at the top of a file that contain a full or partial license grant.

## Report Format

When violations are detected, provide:

> **No per-file license headers**
>
> This project does not use per-file license headers. The top-level `LICENSE` file covers all source files.
>
> The following files contain license headers that should be removed:
>
> - `path/to/file.go` — MIT license header detected
>
> **Fix:** Remove the license header block. Do not replace it with an SPDX identifier or Apache boilerplate.

## Skip Conditions

Skip this rule when **ANY** of these is true:

1. No Go files were added or modified
2. All flagged files are under `.gen/` (generated code)
3. The PR is specifically updating the LICENSE file itself
