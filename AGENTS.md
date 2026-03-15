# Development Guidelines

This document contains critical development guidelines for working with this codebase.

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

## System Architecture

### Core Components

- `common/persistence` contains all persistence layer packages. These are structured with a PersistenceManager for each component and typically have a PersistenceStore which knows how to handle NoSQL and Sql datastore implementations, with various specific database implementations in plugins under these directories
- `common/types` contains the RPC layer internal type representation. This package should have few, if any dependencies and should be the top of the dependency tree. It should have values which represent IDL values and for which there are mappers in `common/types/mapper`
- `services` - Major services: history, matching, frontend, worker
- `tools/cli` - Cadence CLI
- `idls` - Submodule for Thrift codegen (Protobuf via go module)

## Development Workflow

### Commands

- Tests: `make test` or `go test` for specific tests
- Linting: `make lint`
- Preparing for PR: `make pr` (re-runs IDL codegen, linting, formatting)
- Build: `go build ...` or `make build`
- Format: `make fmt`

**Test Guidelines:**
- Tests in `<filename>_test.go`
- Run with `go test /path/to/changes` during development
- Only run `make test` at the end for final sanity check (very slow)

## Generic Rules 

See the `.agents/` directory for:
- `go-style.md` - Go formatting and style (Uber style guide)
- `shell-style.md` - Shell scripting conventions

## Workflow Code in Repository

When adding workflow code to tests, examples, or tools:

- See [docs/non-deterministic-error.md](docs/non-deterministic-error.md) for debugging
- See `.cursor/rules/CADENCE-WORKFLOWS.mdc` for detailed patterns
