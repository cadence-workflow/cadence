# Releaser Tool

A tool for managing semantic versioning releases across multiple Go modules in this repository.

## Quick Start

```bash
# Build from repository root
cd ../../../
make cadence-releaser

# Check current status
./cadence-releaser status

# Iterate during development
./cadence-releaser prerelease  # v1.4.2-prerelease08 → v1.4.2-prerelease09

# Promote to stable
./cadence-releaser release     # v1.4.2-prerelease09 → v1.4.2

# Start new version
./cadence-releaser minor       # v1.4.2 → v1.5.0-prerelease01
```

## Commands

### `status`
Shows current repository state: branch, version, and all modules with their versions.

```bash
./releaser status
```

### `prerelease`
Increments the prerelease number during development.

```bash
./cadence-releaser prerelease
# v1.4.2-prerelease08 → v1.4.2-prerelease09
```

### `release`
Promotes the current prerelease to a stable release.

```bash
./cadence-releaser release
# v1.4.2-prerelease09 → v1.4.2
```

### `minor`
Starts a new minor version cycle (for new features).

```bash
./cadence-releaser minor
# v1.4.2 → v1.5.0-prerelease01
```

### `patch`
Starts a new patch version cycle (for bug fixes).

```bash
./cadence-releaser patch
# v1.4.2 → v1.4.3-prerelease01
```

### `major`
Starts a new major version cycle (for breaking changes).

```bash
./cadence-releaser major
# v1.4.2 → v2.0.0-prerelease01
```

## Global Flags

### `--set-version, -s`
Override automatic version calculation with a specific version.

```bash
./cadence-releaser release --set-version v1.4.3
```

### `--yes`
Skip all confirmation prompts (useful for CI/automation).

```bash
./cadence-releaser prerelease --yes
```

### `--verbose, -i`
Enable verbose output for debugging.

```bash
./cadence-releaser status --verbose
```

## How It Works

1. **Discovers modules** from `go.work` in the repository root
2. **Determines version** based on existing git tags
3. **Plans actions** - creates and pushes tags for all modules
4. **Validates state** - ensures clean working directory, correct branch
5. **Executes** - creates tags locally, then pushes to origin

## Module Discovery

The tool reads `go.work` to discover which modules to tag. All modules listed in `go.work` will receive the same version tag.

Example `go.work`:
```
go 1.24.0

use (
	.
	cmd/server
	common/archiver/gcloud
	common/dynamicconfig/openfeatureprovider/unleash
	common/persistence/sql/sqlplugin/cloudsql-mysql
)
```

## Tag Format

- **Root module**: `v1.4.2`, `v1.4.2-prerelease01`
- **Submodules**: `<module-path>/v1.4.2`, e.g., `cmd/server/v1.4.2`

## Safety Features

- Requires clean git working directory
- Enforces releases only from master branch
- Interactive confirmations before creating/pushing tags
- Detects version conflicts (handles partial tag creation)
- Prevents creating duplicate versions

## Version Conflict Handling

If some tags already exist for a version, the tool will:
1. Detect which tags exist vs which are missing
2. Show you the conflict
3. Offer to create only the missing tags

This allows recovery from partial tag creation or manual fixes.

## Testing

Run the test suite:

```bash
go test ./internal/...
```

## Architecture

- `releaser.go` - CLI interface and command handlers
- `internal/release/` - Core release logic and state management
- `internal/git/` - Git operations wrapper
- `internal/fs/` - Filesystem operations (reads go.work)
- `internal/console/` - User interaction and confirmations
