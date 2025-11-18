# Development Guidelines

This document contains critical information about working with this codebase.

## Core Development Rules

- NEVER ever mention a `co-authored-by` or similar aspects. In particular, never mention the tool used to create the commit message or PR.

## Coding Best Practices

- **Testing**: 
  - Prefer table-tests unless the change is extremely simple
  - don't use suite-tests, these are legacy and should only be maintained, but all new tests should be either plain Go tests or table-tests
  - Don't use https://github.com/stretchr/testify for any new mocks, these are legacy. Always use https://github.com/uber-go/mock where creating new test
  - Try to round-trip all mappers where possible. Generate symmetric mappers when converting types

- **types**:
  - Never use IDL code directly in service logic, map them to common/types or common/persistence types

## System Architecture

## Core Components

- `common/persistence` contains all persistence layer packages. These are structured with a PersistenceManager for each component and typically have a PersistenceStore which knows how to handle NoSQL and Sql datastore implementations, with various specific database implementations in plugins under these directories
- `common/types` contains the RPC layer internal type representation. This package should have few, if any dependencies and should be the top of the dependency tree. It should have values which represent IDL values and for which there are mappers in `common/types/mapper`
- `services` This is the major services: history, matching, frontend and worker
- `tools/cli` is where the cadence CLI is built
- `idls` is the submodule for building thrift specifically. Protobuf is imported via a go module.

## Development Workflow Commands

- Tests are run, usually via `make test` or `go test` for a specific test
- linting can be performed by `make lint`
- preparing all changes for a PR should be done via `make pr` which re-performs all IDL codegen, linting, formatting etc in order.