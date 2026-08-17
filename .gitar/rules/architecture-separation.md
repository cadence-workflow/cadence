---
title: Architecture Separation - Core Logic Abstraction
description: Prevents initialization logic from accumulating in cmd/server and ensures service/* depends only on interfaces
when: PR modifies files in cmd/server or service/*/
actions: Detect initialization logic in cmd/server, proprietary imports in service/*, and recommend Fx-based abstraction
---

# Architecture Separation - Core Logic Abstraction

Cadence must support both the standard open-source build and custom builds from a single codebase. Different builds may use different libraries (e.g., open-source Kafka client vs. company-internal Kafka library). To enable this, core service logic must depend only on interfaces, never on concrete infrastructure implementations.

See `AGENTS.md` Architecture Guidelines section for full context.

## Overview

When evaluating a pull request:

1. **Check `cmd/server` for new initialization logic** - server initialization and dependency wiring should use Fx modules, not accumulate in cmd/server
2. **Check `service/*/` for hard-coded infrastructure dependencies** - concrete types from infrastructure packages instead of interfaces
3. **Check `service/*/` for conditional environment logic** - if/switch statements selecting implementations based on environment
4. **Recommend interface-based abstraction** when violations are found

## Detection Logic

### Step 1: Check for new initialization logic in cmd/server

**CRITICAL**: Prevent ADDITIONAL initialization and dependency wiring logic from accumulating in `cmd/server`.

**Context**: The codebase is gradually migrating from manual initialization in `cmd/server/cadence/server.go` to Fx-based dependency injection. The existing `startService()` method contains substantial initialization logic that is being refactored over time. **New infrastructure dependencies should NOT add to this accumulation** - they should use Fx modules instead.

Inspect the PR diff for changes to `cmd/server/**/*.go` that add:

**Red flags (flag these as violations):**
- Adding NEW infrastructure client initialization that wasn't there before
  - Examples: new Kafka producers/consumers, new metrics backends, new peer discovery clients, new database clients
- Adding NEW conditional logic to select between different implementations
  - Examples: `if config.UseCustomKafka { ... } else { ... }` selecting infrastructure based on config
- Significant expansion of `startService()` or related initialization functions (e.g., adding 50+ lines)
- Creating new private helper functions in `cmd/server` specifically for infrastructure setup

**Allowable (don't flag these):**
- Small bug fixes or parameter adjustments to existing initialization code (< 10 lines changed)
- Refactoring existing initialization code to make it cleaner without adding new infrastructure types
- **Moving initialization code OUT of cmd/server into Fx modules** (this is progress toward the goal!)
- Changes to `cmd/server/cadence/fx.go` that wire Fx modules (this is the desired pattern)

**When new initialization logic is detected, recommend:**
1. Check if similar infrastructure already has an Fx module (look for `fx.go` files in `common/*` packages)
2. If yes: use the existing Fx module pattern instead of manual initialization
3. If no: create a new Fx module
   - Define the interface in the appropriate package (e.g., `common/messaging/interfaces.go`)
   - Create an Fx module file (e.g., `common/messaging/kafka/fx.go`) that provides the implementation
   - Wire it in `cmd/server/cadence/fx.go` instead of `cmd/server/cadence/server.go`
   - See existing examples: `common/archiver/archiverfx/`, `common/log/logfx/`, `common/metrics/metricsfx/`

### Step 2: Identify changes to service/* packages

Inspect the PR diff for any files matching:
- `service/**/*.go` (any Go file under service directories)

If no service files are modified, skip the service/* checks below.

### Step 3: Check for hard-coded infrastructure types in service/*

Look for:
- Variable declarations or struct fields with concrete infrastructure types (e.g., `*kafka.Producer`, `*ringpop.Ring`)
- Direct instantiation of infrastructure clients (`kafka.NewProducer(...)`, `NewCustomMetricsClient(...)`)
- Type assertions to concrete infrastructure types

**Expected pattern:**
```go
// Good: depends on interface
type MyService struct {
    peerProvider membership.PeerProvider  // interface
    kafkaClient  messaging.Client         // interface
}

// Bad: depends on concrete type
type MyService struct {
    ringpop *ringpop.Ring                 // concrete implementation
    kafka   *sarama.AsyncProducer         // concrete implementation
}
```

### Step 4: Check for environment-based conditionals in service/*

Look for:
- Conditional logic that selects implementations based on environment flags or build tags
- Switch/if statements choosing between different infrastructure providers

**Expected pattern:**
```go
// Bad: conditional logic in service/*
if config.Environment == "production" {
    client = NewProdKafkaClient()
} else {
    client = NewDevKafkaClient()
}

// Good: dependency injected via Fx
func NewMyService(kafkaClient messaging.Client) *MyService {
    // implementation selection happens at Fx wiring time
}
```

## Report Format

When violations are detected, provide:

1. **List of files and line numbers** with violations
2. **Type of violation** (proprietary import / hard-coded type / conditional logic)
3. **Recommended fix**: how to abstract behind an interface

### Example Report

> ⚠️ **Architecture violation: New initialization logic added to cmd/server**
>
> **CRITICAL: Do not add more initialization and dependency wiring logic inside cmd/server**
>
> **cmd/server/cadence/server.go:400-425**
> - Adds NEW Pinot client initialization (25 lines)
> - Adds conditional logic to select between Pinot implementations based on config
> 
> **Recommendation:**
> The codebase is migrating away from manual initialization in `cmd/server/cadence/server.go`. Instead of adding to the existing `startService()` function:
> 1. Create an Fx module for Pinot client initialization (e.g., `common/pinot/fx.go`)
> 2. Define the interface if one doesn't exist (e.g., `common/pinot/interfaces.go`)
> 3. Wire it through `cmd/server/cadence/fx.go` instead of `server.go`
> 
> **Examples to follow:**
> - `common/archiver/archiverfx/` - Fx module for archiver setup
> - `common/log/logfx/` - Fx module for logger initialization
> - `common/metrics/metricsfx/` - Fx module for metrics client
>
> **Why this matters:**
> - Over time, `cmd/server/cadence/server.go` has grown to 800+ lines of initialization logic
> - This accumulation causes open-source and custom builds to diverge
> - New infrastructure should use Fx modules to prevent further tech debt accumulation
>
> ---
>
> The following changes in `service/*/` also violate the interface-based abstraction principle:
>
> **service/history/engine.go:120**
> - Hard-coded concrete type `*ringpop.Ring` in struct field
> - Recommendation: Use the existing `membership.PeerProvider` interface instead
>
> **service/matching/handler.go:85**
> - Adds conditional logic: `if config.UseCustomQueue { ... } else { ... }`
> - Recommendation: Accept an interface dependency and let Fx wire the implementation based on config
>
> **Why this matters:**
> - Cadence supports both open-source builds and custom builds with different libraries
> - Core logic in `service/*/` must depend only on interfaces to remain portable
> - See `AGENTS.md` Architecture Guidelines for the full pattern

## Skip Conditions

Skip this rule when **ANY** of these is true:

### 1. No cmd/server or service/* files modified
- PR only changes `common/*`, `tools/*`, `schema/*`, tests, or other packages

### 2. Changes are to test files
- `*_test.go` files can use concrete implementations for testing

### 3. Changes are to Fx modules (for service/* only)
- Files named `*_fx.go` or `module.go` are expected to wire concrete implementations
- Note: `cmd/server` should NOT grow new initialization logic even in separate files

## Integration with Development Workflow

This rule complements:
- `AGENTS.md` Architecture Guidelines - provides the full pattern
- Local development - developers should check imports before `make pr`
- Code review - maintainers verify abstraction boundaries

## References

- `AGENTS.md` - Architecture Guidelines section
- `common/membership/hashring.go` - Example of interface definition pattern (PeerProvider interface at line 57)
- Fx documentation - https://uber-go.github.io/fx/
