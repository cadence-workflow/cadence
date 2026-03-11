# Development Guidelines

This document contains critical information about working with this codebase.

## Core Rules

- NEVER ever mention a `co-authored-by` or similar aspects. In particular, never mention the tool used to create the commit message or PR.

## Build Commands

```bash
make bins   # build all binaries (includes codegen + lint)
make build  # quick compile check only (no codegen, no tests)
make pr     # pre-PR: tidy → go-generate → fmt → lint
make pr GEN_DIR=service/history  # scoped codegen (faster for single package)
make test   # all unit tests (excludes host/ integration tests)
make test_e2e  # end-to-end integration tests in host/
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

## Coding Best Practices

- **Testing**:
  - Prefer table-tests; plain Go tests for trivially simple cases.
  - Do **not** write new suite-style tests (`testify/suite`) — legacy, maintain only.
  - Do **not** use `github.com/stretchr/testify` mocks — use `github.com/uber-go/mock`.
  - All new tests should be either plain Go tests or table-tests.
  - Round-trip test all mappers: `ToX(FromX(item)) == item`.

- **Types**:
  - Never use IDL code (`.gen/go/` or `.gen/proto/`) directly in service logic.
  - Map to `common/types` or `common/persistence` types via mappers in `common/types/mapper/`.
  - Files in `.gen/` are generated from IDL — do not edit manually.

## Pull Request Guidelines

PRs must follow the template in `.github/pull_request_guidance.md`.

## Development

For database setup, schema installation, and server start options: [CONTRIBUTING.md](CONTRIBUTING.md).
