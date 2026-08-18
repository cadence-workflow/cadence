# Dependency age check

Dependencyage compares changed `go.mod` files with their merge base and rejects dependency versions published more recently than the configured minimum age. An introduced version is a new requirement, a requirement version bump, or a new versioned replacement target; filesystem replacements are skipped. Unknown versions, including 404 and 410 responses, fall back to a pseudo-version timestamp when available, while all other lookup errors fail closed.

## Local usage

```bash
go run ./tools/dependencyage/cmd/dependencyage --base-ref origin/master
MIN_DEPENDENCY_AGE_DAYS=30 go run ./tools/dependencyage/cmd/dependencyage --base-ref origin/master
```

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
