# Development Guidelines

This document contains critical information about working with this codebase.

## Core Rules

- NEVER ever mention a `co-authored-by` or similar aspects. In particular, never mention the tool used to create the commit message or PR.

## Development Commands

```bash
make bins   # build all binaries
make build  # compile-check all packages + test files (no codegen, no test execution)
make pr     # pre-PR: tidy → go-generate → fmt → lint
make pr GEN_DIR=service/history  # scoped codegen (faster for single package)
make test   # all unit tests (excludes host/ integration tests)
make test_e2e  # end-to-end integration tests in host/ (requires Docker dependencies running)
make lint   # lint only
make go-generate  # regenerate mocks, enums, wrapper files

go test -race -run TestFoo ./path/to/pkg/...  # run a specific test
```

## Gotchas

- **`idls/` submodule**: run `git submodule update --init --recursive` after checkout or all codegen and build targets fail silently.
- **Generated files**: `*_generated.go` and `*_mock.go` are produced by `make go-generate`.
  Edit the source `.tmpl` or interface file, then regenerate — never edit generated files directly.
- **`make pr` is required before every PR**: it runs tidy → go-generate → fmt → lint in sequence.
  If CI shows unexpected diffs in generated files, you forgot `make pr`. Prefer to use `make pr GEN_DIR=<package>` for faster iteration.
- **Go workspace gotcha**: `go build ./...` and `go test ./...` only cover the root module.
  Use `make bins` and `make test` for full coverage. Use `make tidy` (not `go mod tidy`).
- **IDL local testing**: To test local IDL changes before pushing, add `replace github.com/uber/cadence-idl => ./idls` to the bottom of `go.mod`. Remove before committing.
- **Submodule drift**: `git submodule update --init --recursive` fixes not just post-checkout failures but also mid-development build errors after upstream IDL changes.
- **SQLite for quick local dev**: No Docker required. Run `make install-schema-sqlite` then `./cadence-server --zone sqlite start` for the fastest path to a running local server.

## Coding Best Practices

- **Testing**:
  - Prefer table-tests; plain Go tests for trivially simple cases.
  - Do **not** write new suite-style tests (`testify/suite`) — legacy, maintain only.
  - Do **not** use `github.com/stretchr/testify` mocks — use `github.com/uber-go/mock`.
  - All new tests should be either plain Go tests or table-tests.
  - Round-trip test all mappers: `ToX(FromX(item)) == item`. Fuzz-test mappers following the pattern in `common/types/mapper/proto/api_test.go` and `schedule_test.go`.

- **Types**:
  - Never use IDL code (`.gen/go/` or `.gen/proto/`) directly in service logic.
  - Map to `common/types` or `common/persistence` types via mappers in `common/types/mapper/`.
  - Files in `.gen/` are generated from IDL — do not edit manually.

## Pull Request Guidelines

PRs must follow the template in `.github/pull_request_guidance.md`.

PR titles must use Conventional Commits format: `<type>(<optional scope>): <description>`.
Valid types are defined in `.github/workflows/semantic-pr.yml`.
Example: `feat(history): add retry logic for shard takeover`.

## Development

For database setup, schema installation, and server start options: [CONTRIBUTING.md](CONTRIBUTING.md).

## Repository layout
A Cadence server cluster is composed of four different services: Frontend, Matching, History and Worker(system).
Here's what's in each top-level directory in this repository:

* **bench/** : Benchmark and load test suite for stress-testing a Cadence cluster
* **canary/** : The test code that needs to run periodically to ensure Cadence is healthy
* **client/** : Client wrappers to let the four different services talk to each other
* **cmd/** : The main function to build binaries for servers and CLI tools
* **common/** : Basically contains all the rest of the code in Cadence server, the names of the sub folder are the topics of the packages
* **config/** : Sample configuration files
* **docker/** : Code/scripts to build docker images
* **docs/** : Documentation
* **environment/** : Test helpers for reading environment variables (DB host/port, etc.) used by integration tests
* **host/** : End-to-end integration tests
* **idls/** : Git submodule with Thrift and Protobuf IDL definitions; source for all generated code under `.gen/`
* **internal/** : Internal Go tool dependencies (blank imports to pin codegen tools in `go.mod`)
* **proto/** : Protobuf definitions for internal persistence blobs and public Cadence APIs
* **schema/** : Versioned persistence schema for Cassandra/MySQL/Postgres/ElasticSearch
* **scripts/** : Scripts for CI build
* **service/** : Contains four sub-folders dedicated to each of the four services (frontend, history, matching, worker)
* **simulation/** : Black-box simulation tests that spin up a local Docker cluster and validate complex multi-component scenarios
* **tools/** : CLI tools for Cadence workflows and also schema updates for persistence