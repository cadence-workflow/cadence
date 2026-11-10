---
title: Blank Imports in Library Code
description: Prevents blank imports (side-effect imports) in library code to avoid registration conflicts and initialization issues
when: PR adds or modifies Go files with blank imports
actions: Detect blank imports in common/*, service/*, and plugin packages; recommend moving to main.go
---

# Blank Imports in Library Code

Blank imports (`import _`) execute `init()` functions with global side effects. When used in library code that gets imported transitively, they cause:

1. **Registration conflicts** - "driver already registered" panics when multiple libraries register the same driver
2. **Order-of-initialization bugs** - unpredictable execution order across packages
3. **Implicit dependencies** - side effects that are hard to track and debug
4. **Custom build issues** - proprietary builds may need different initialization

See `AGENTS.md` Coding Best Practices section for full context.

## Overview

When evaluating a pull request:

1. **Check for blank imports in library code** - `common/**/*.go`, `service/**/*.go`, plugin packages
2. **Verify blank imports are only in allowed locations** - `cmd/**/main.go`, `*_test.go`, `internal/tools.go`
3. **Recommend moving to main.go** when violations are found

## Detection Logic

### Step 1: Identify Go files with blank imports

Scan the PR diff for files matching `**/*.go` (excluding `*_test.go` for now) that contain:

```go
import (
    _ "some/package"
)
```

or

```go
import _ "some/package"
```

Look for the blank identifier `_` followed by a quoted import path.

### Step 2: Categorize by location

For each file with blank imports, determine if it's in:

**Prohibited locations (flag these):**
- `common/**/*.go` - shared library code
- `service/**/*.go` - business logic
- Plugin packages: `common/persistence/sql/sqlplugin/**/*.go`, `common/persistence/nosql/nosqlplugin/**/*.go`
- Any other non-main package under the repository root

**Allowed locations (don't flag):**
- `cmd/**/main.go` - entry points with explicit initialization control
- `internal/tools.go` - build tool dependencies
- `*_test.go` - test files (allow but note in summary if excessive)

### Step 3: Identify common violating patterns

**Common blank imports that violate the rule:**

Database drivers:
- `_ "github.com/lib/pq"` (PostgreSQL)
- `_ "github.com/go-sql-driver/mysql"`
- `_ "github.com/ncruces/go-sqlite3/driver"`
- `_ "github.com/ncruces/go-sqlite3/embed"`
- Any package under `database/sql` driver path

Profilers:
- `_ "net/http/pprof"`

Observability:
- `_ "go.uber.org/automaxprocs"`

Any other package that calls `sql.Register()`, `http.Handle()`, or mutates global state in `init()`.

### Step 4: Check if moved to main.go

If a blank import was removed from library code in this PR, check if it was added to the appropriate `cmd/**/main.go`.

**Good pattern:**
```diff
// common/persistence/sql/sqlplugin/sqlite/db.go
- import _ "github.com/ncruces/go-sqlite3/driver"

// cmd/server/main.go
+ import _ "github.com/ncruces/go-sqlite3/driver"
```

## Report Format

When violations are detected, provide:

1. **List of files and line numbers** with prohibited blank imports
2. **Type of import** (driver, profiler, etc.)
3. **Recommended fix**: where to move the import

### Example Report

> ⚠️ **Blank imports in library code**
>
> **Blank imports with global side effects are prohibited in library code**
>
> The following files contain blank imports that should be moved to entry points:
>
> **common/persistence/sql/sqlplugin/postgres/db.go:15**
> ```go
> import _ "github.com/lib/pq"  // PostgreSQL driver
> ```
> - **Problem:** Driver registration in library code causes "driver already registered" panics when custom builds also import this driver
> - **Fix:** Remove this import and add it to `cmd/server/main.go` instead
>
> **common/metrics/reporter.go:8**
> ```go
> import _ "net/http/pprof"  // profiler registration
> ```
> - **Problem:** Automatically registers pprof handlers on import, affecting all builds
> - **Fix:** Remove this import and add it to `cmd/server/main.go` (or make it opt-in via explicit initialization)
>
> **service/history/engine.go:12**
> ```go
> import _ "go.uber.org/automaxprocs"
> ```
> - **Problem:** Global side effect in business logic package
> - **Fix:** Move to `cmd/server/main.go`
>
> **Why this matters:**
> - Blank imports execute `init()` functions with global side effects
> - Library code should be side-effect free to support multiple builds and prevent conflicts
> - Entry points (`cmd/**/main.go`) have explicit control over initialization order
> - See `AGENTS.md` Coding Best Practices → Blank Imports section
>
> **Allowed locations for blank imports:**
> - ✅ `cmd/**/main.go` - entry points
> - ✅ `internal/tools.go` - build tool dependencies
> - ⚠️ `*_test.go` - test setup (use sparingly)

### Example for tests with many blank imports

> 📊 **Note: Test files with blank imports**
>
> The following test files use blank imports (allowed, but consider explicit setup):
>
> - `service/history/engine_test.go:10` - imports test driver
> - `common/persistence/sql/sql_test.go:15` - imports multiple drivers
>
> Consider: Explicit setup in `TestMain()` or test helpers for better control over initialization.

## Skip Conditions

Skip this rule when **ANY** of these is true:

### 1. No Go files modified
- PR only changes non-Go files (markdown, yaml, proto, etc.)

### 2. Only allowed files modified
- All Go files with blank imports are `cmd/**/main.go`, `internal/tools.go`, or `*_test.go`

### 3. Blank imports being removed
- PR removes blank imports from library code (this is good!)
- Still report if they weren't moved to appropriate location

## Integration with Development Workflow

This rule complements:
- `AGENTS.md` Coding Best Practices - provides the full pattern
- `make lint` - catches some import issues
- Code review - maintainers verify initialization boundaries

## References

- `AGENTS.md` - Coding Best Practices → Blank Imports section
- Go best practice: blank imports should only be in main packages or for tools
- Database driver registration: https://go.dev/doc/database/open-handle
