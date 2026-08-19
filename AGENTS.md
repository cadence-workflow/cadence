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
  - Never use yarpcerrors directly in handler logic; instead, create an new Error type in IDL and map to internal ones under common/type/errors.go.

- **Blank Imports (Side-Effect Imports)**:
  - **NEVER** use blank imports (`import _`) in library code under `common/`, `service/`, or any package that gets imported transitively.
  - Blank imports execute `init()` functions with global side effects (driver registration, flag registration, global state mutation).
  - These cause problems: registration conflicts ("driver already registered"), order-of-initialization bugs, implicit dependencies, and issues in custom builds.
  - **Blank imports are ONLY allowed in:**
    - `cmd/**/main.go` — entry points with explicit control over initialization
    - `*_test.go` — test setup (use sparingly, prefer explicit setup)
    - `internal/tools.go` — build tool dependencies pinned in `go.mod`
  - **Examples of prohibited blank imports in library code:**
    - Database drivers: `_ "github.com/lib/pq"`, `_ "github.com/ncruces/go-sqlite3/driver"`
    - Profilers: `_ "net/http/pprof"`
    - Flag packages: `_ "github.com/some/flags"`
    - Any package that registers global state in `init()`
  - **What to do instead:** Move the blank import to `cmd/server/main.go` or the appropriate entry point.
  - **Example:** `common/persistence/sql/sqlplugin/sqlite/db.go` should NOT import drivers. Instead, `cmd/server/main.go` imports both the plugin package AND the driver.

## Architecture Guidelines

**Core Principle**: Cadence must support both the standard open-source build and custom builds from a single codebase. Different builds may use different libraries (e.g., open-source Kafka client vs. company-internal Kafka library). This requires strict separation between core logic and proprietary implementations.

- **Interface-based abstraction**:
  - Core server logic must depend only on interfaces, never on concrete infrastructure implementations.
  - Define clear interface boundaries that isolate core logic from external dependencies.
  - Example: `common/membership/resolver.go` defines a `PeerProvider` interface that abstracts peer discovery, allowing different implementations (e.g., Ringpop from open source, custom peer providers using proprietary libraries) without changing core membership logic.

- **Dependency injection with Fx**:
  - **CRITICAL**: Do NOT add NEW initialization or dependency wiring logic to `cmd/server/cadence/server.go`.
    - The codebase is gradually migrating from manual initialization to Fx-based dependency injection.
    - `cmd/server/cadence/server.go` currently contains substantial initialization logic that is being refactored over time.
    - **New infrastructure dependencies must use Fx modules**, not add to the existing manual initialization.
  - Create Fx modules to wire dependencies and select implementations for each build.
  - Core components should provide Fx modules that accept interface dependencies.
  - Proprietary implementations should provide Fx modules that bind their concrete implementations.
  - Examples of Fx modules: `common/archiver/archiverfx/`, `common/log/logfx/`, `common/metrics/metricsfx/`
  - Wire Fx modules in `cmd/server/cadence/fx.go`, not in `cmd/server/cadence/server.go`

- **Where to place code**:
  - **Core logic** → `service/*/`: business logic, algorithms, coordination — must depend only on interfaces
  - **Common packages** → `common/*`: mix of core utilities, shared interfaces, and implementations
  - **Interfaces** → Define in the package where core logic lives (e.g., `PeerProvider` in `common/membership/hashring.go`)
  - **Proprietary implementations** → Can live in `common/*/` or separate packages, but must implement well-defined interfaces
  - **Fx wiring** → Each implementation provides its own Fx module; core code should not know which implementation is selected

- **When adding new infrastructure dependencies**:
  1. Define an interface in the appropriate package (e.g., `type PeerProvider interface { ... }`)
  2. Have core logic in `service/*/` depend on the interface, not concrete implementations
  3. Provide Fx modules that bind different implementations for different builds
  4. Never import proprietary packages or libraries not available in the open-source build into `service/*/`

- **Red flags** (avoid these patterns):
  - **CRITICAL**: Adding NEW initialization or dependency wiring logic in `cmd/server/cadence/server.go`
    - Small bug fixes to existing code are fine, but NEW infrastructure initialization must use Fx modules
    - See `common/*/fx.go` files for examples of the Fx module pattern
  - Importing packages/libraries that are not available in all builds in `service/*/`
  - Hard-coding infrastructure choices in `service/*/` business logic
  - Adding conditional logic to select implementations in `service/*/` (use interface injection instead)

Following these guidelines allows custom builds to be maintained alongside the open-source build without duplicating or forking core logic.

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

## Generic Rules

See the `.agents/` directory for:
- `go-style.md` - Go formatting and style (Uber style guide)
- `shell-style.md` - Shell scripting conventions

## Workflow Code in Repository

When adding workflow code to tests, examples, or tools:

- See [docs/non-deterministic-error.md](docs/non-deterministic-error.md) for debugging
- See `.cursor/rules/CADENCE-WORKFLOWS.mdc` for detailed patterns
