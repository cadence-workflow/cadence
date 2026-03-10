# Cadence Development Agent Configuration

This document contains development guidelines and agent configurations for the Cadence codebase.
It is read by both Claude Code and Cursor.

---

## Core Development Rules

### NEVER mention in commits

- Never ever mention `co-authored-by` or similar attribution
- Never mention the tool used to create commit messages or PRs

### Testing

- Prefer table-tests unless the change is extremely simple
- Don't use suite-tests (legacy) - only maintain existing ones
- All new tests should be plain Go tests or table-tests
- Don't use https://github.com/stretchr/testify for new mocks (legacy)
- Always use https://github.com/uber-go/mock for new tests
- Try to round-trip all mappers where possible - generate symmetric mappers

### Types

- Never use IDL code directly in service logic
- Map them to `common/types` or `common/persistence` types

---

## System Architecture

### Core Components

- `common/persistence` - All persistence layer packages
  - Structured with PersistenceManager per component
  - PersistenceStore handles NoSQL and SQL datastore implementations
  - Database-specific implementations in plugins under these directories

- `common/types` - RPC layer internal type representation
  - Should have few dependencies (top of dependency tree)
  - Contains values representing IDL values
  - Mappers in `common/types/mapper`

- `services` - Major services: history, matching, frontend, worker

- `tools/cli` - Cadence CLI

- `idls` - Submodule for Thrift codegen (Protobuf via go module)

---

## Development Workflow

### Commands

- Tests: `make test` or `go test` for specific tests
- Linting: `make lint`
- Preparing for PR: `make pr` (re-runs IDL codegen, linting, formatting)
- Build: `go build ...` or `make build`
- Format: `make fmt`

### Workflow Process

1. Write code
2. Write tests for code
3. Build code
4. Test code
5. Format code

**Test Guidelines:**
- Tests in `<filename>_test.go`
- Run with `go test /path/to/changes` during development
- Only run `make test` at the end for final sanity check (very slow)

---

## Generic Rules (Tool-Agnostic)

See `.agents/` directory for:
- `go-style.md` - Go formatting and style (Uber style guide)
- `development-workflow.md` - Development process
- `shell-style.md` - Shell scripting conventions

---

## Tool-Specific Extensions

### Cursor

See `.cursor/rules/` for:
- `CADENCE-WORKFLOWS.mdc` - Workflow code style checking

### Claude Code

See `.claude/agents/` for:
- `simulation-test-runner.md` - Simulation test execution specialist

---

## Workflow Code in Repository

When adding workflow code to tests, examples, or tools:

- Follow determinism rules:
  - Use `workflow.Now(ctx)` not `time.Now()`
  - Use `workflow.Sleep(ctx, d)` not `time.Sleep(d)`
  - Use `workflow.Go(ctx, func)` not `go func()`
  - Use `workflow.NewChannel(ctx)` not `make(chan T)`
  - Use `workflow.NewSelector(ctx)` not `select {}`
  - Sort map keys before iteration
  - Use `workflow.GetVersion()` when modifying workflow logic

- See [docs/non-deterministic-error.md](docs/non-deterministic-error.md) for debugging
- See `.cursor/rules/CADENCE-WORKFLOWS.mdc` for detailed patterns

---

*This is the canonical configuration. CLAUDE.md symlinks to this file.*
