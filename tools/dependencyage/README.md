# Dependency age check

Dependencyage compares changed `go.mod` files with their merge base and rejects dependency versions published more recently than the configured minimum age. An introduced version is a new requirement, a requirement version bump, or a new versioned replacement target; filesystem replacements are skipped. Unknown versions, including 404 and 410 responses, fall back to a pseudo-version timestamp when available and otherwise fail the run with exit code 2. All other lookup errors also fail closed.

## Local usage

```bash
go run ./cmd/tools/dependencyage --base-ref origin/master
MIN_DEPENDENCY_AGE_DAYS=30 go run ./cmd/tools/dependencyage --base-ref origin/master
```

## Indirect dependencies

Indirect requirements are checked deliberately: they resolve into the build just like direct requirements, and a dependency bump that brings in a too-fresh indirect version is precisely what this gate is intended to catch. If the gate fires on a pull request, wait out the minimum-age window or override it with `MIN_DEPENDENCY_AGE_DAYS`.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | All introduced versions meet the minimum age. |
| 1 | One or more introduced versions are too young. |
| 2 | The check could not classify all introduced versions. |

## Configuration

`MIN_DEPENDENCY_AGE_DAYS` sets the minimum age in days and defaults to 14. `GOPROXY` and `GOPRIVATE` are honored through the Go toolchain.

## Future work

- Add GitHub Actions annotations (`::error::`) and step summary output.
- Add an allowlist for reviewed versions.
- Add ignore patterns.
- Add JSON output.
- Fetch publish times with bounded concurrency (lookups are currently serial, one `go list` per introduced version).
- Replace stderr substring matching in the not-found classification if the toolchain ever exposes structured errors; a wording change would degrade gracefully to a fail-closed error, not a silent pass.
- Extract the check into a standalone reusable GitHub Action.
