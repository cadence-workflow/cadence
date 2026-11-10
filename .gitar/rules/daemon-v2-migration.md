---
title: Migrate from common.Daemon to common.DaemonV2
description: Encourages migration to context-aware DaemonV2 when touching Daemon-related code
when: PR modifies files that use common.Daemon
actions: Detect common.Daemon usage and suggest migrating to common.DaemonV2
---

# Migrate from common.Daemon to common.DaemonV2

The `common.Daemon` interface is deprecated in favor of `common.DaemonV2`, which provides context-aware lifecycle management for graceful shutdown coordination.

## Overview

When a pull request modifies files that use `common.Daemon`, encourage the author to migrate to `common.DaemonV2` as part of their changes.

**Key differences:**

```go
// Deprecated: common.Daemon
type Daemon interface {
    Start()
    Stop()
}

// Preferred: common.DaemonV2
type DaemonV2 interface {
    Start(context.Context) error
    Stop(context.Context) error
}
```

**Benefits of DaemonV2:**
- Context-aware: supports cancellation and deadlines
- Error handling: can return errors during startup/shutdown
- Graceful shutdown: coordinates shutdown across components

## Detection Logic

### Step 1: Check if PR modifies files that use common.Daemon

Inspect the PR diff for modified files that contain:
- `common.Daemon` in interface definitions (e.g., embedded in other interfaces)
- Type assertions to `common.Daemon` (e.g., `var _ common.Daemon = (*MyStruct)(nil)`)
- Struct fields or function parameters of type `common.Daemon`
- Calls to `Start()` or `Stop()` on daemon instances

If no such usages are found in the modified files, skip this rule.

### Step 2: Determine migration scope

**Recommend migration when:**
- The PR is already modifying the daemon implementation (e.g., changing how Start/Stop work)
- The PR is adding new daemon-related functionality
- The PR is refactoring the component that uses the daemon

**Don't require migration when:**
- The PR is a trivial bug fix unrelated to lifecycle management
- The PR only touches code that uses the daemon but doesn't modify the daemon itself
- The migration would significantly expand the scope of the PR

### Step 3: Provide migration guidance

When recommending migration, provide:

1. **What to change:**
   - Interface definitions: replace `common.Daemon` with `common.DaemonV2`
   - Implementation signatures: update `Start()` to `Start(context.Context) error` and `Stop()` to `Stop(context.Context) error`
   - Callers: pass context and handle returned errors

2. **Migration pattern:**

```go
// Before
type MyInterface interface {
    common.Daemon
    // other methods...
}

type MyService struct {
    // fields...
}

func (s *MyService) Start() {
    // initialization logic
}

func (s *MyService) Stop() {
    // cleanup logic
}

// After
type MyInterface interface {
    common.DaemonV2
    // other methods...
}

type MyService struct {
    // fields...
}

func (s *MyService) Start(ctx context.Context) error {
    // initialization logic
    // use ctx for cancellation, return errors on failure
    return nil
}

func (s *MyService) Stop(ctx context.Context) error {
    // cleanup logic
    // use ctx for graceful shutdown timeout, return errors on failure
    return nil
}
```

3. **Example reference:**
   - Point to `common/membership/hashring.go` which uses `common.DaemonV2`
   - The `PeerProvider` interface embeds `common.DaemonV2` (line 57)

## Report Format

When migration is recommended, provide a clear, actionable message:

### Example Report

> 💡 **Opportunistic refactor: Migrate to common.DaemonV2**
>
> This PR modifies code that uses the deprecated `common.Daemon` interface. Consider migrating to `common.DaemonV2` while you're here.
>
> **Files using common.Daemon in this PR:**
> - `service/history/queuev2/interface.go:36` - interface embeds `common.Daemon`
> - `service/history/queuev2/virtual_queue.go:51` - struct implements `common.Daemon`
>
> **Why migrate:**
> - `common.Daemon` is deprecated (see `common/daemon.go:40`)
> - `common.DaemonV2` provides context-aware lifecycle management
> - Enables graceful shutdown coordination and error handling
>
> **Migration pattern:**
> ```go
> // Change interface
> - common.Daemon
> + common.DaemonV2
> 
> // Update implementation
> - func (q *Queue) Start() {
> + func (q *Queue) Start(ctx context.Context) error {
>       // use ctx for cancellation
> +     return nil
>   }
> 
> - func (q *Queue) Stop() {
> + func (q *Queue) Stop(ctx context.Context) error {
>       // use ctx for graceful shutdown timeout
> +     return nil
>   }
> ```
>
> **Example:** See `common/membership/hashring.go` for a completed migration.
>
> **Note:** This is an opportunistic suggestion. If it significantly expands the scope of your PR, it's fine to defer to a follow-up.

## Skip Conditions

Skip this rule when **ANY** of these is true:

### 1. No common.Daemon usage in modified files
- PR doesn't touch any files that use `common.Daemon`

### 2. Changes are to test files only
- `*_test.go` files can be migrated separately

### 3. The file is already using DaemonV2
- If the file already uses `common.DaemonV2`, no suggestion needed

### 4. Migration would derail the PR
- Use judgment: if the PR is a focused bug fix and migration would double the scope, suggest but don't insist

## Integration with Development Workflow

This rule is **advisory, not blocking**:
- Suggests migration when the timing is opportune
- Acknowledges that migration may be out of scope for some PRs
- Encourages incremental progress toward deprecating `common.Daemon`

## References

- `common/daemon.go` - Daemon and DaemonV2 interface definitions
- `common/membership/hashring.go` - Example of PeerProvider using DaemonV2
