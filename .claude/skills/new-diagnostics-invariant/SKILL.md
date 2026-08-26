---
name: new-diagnostics-invariant
description: Step-by-step guide for adding a new invariant to Workflow Diagnostics (service/worker/diagnostics). Use when asked to create, extend, or wire up a diagnostics invariant that detects issues or risks in a workflow execution's history.
---

# Adding a Workflow Diagnostics Invariant

Workflow Diagnostics (`service/worker/diagnostics`) reads one workflow execution's history and reports issues, root causes, and runbook links. Each check is an "invariant". Existing ones: `timeout`, `failure`, `retry`, `timeoutrisk`. Read the one closest to yours before writing anything.

## The interface

Defined in `service/worker/diagnostics/invariant/interface.go`:

- `Check(ctx, input)` — scan the history, return issues. The full event list is `input.WorkflowExecutionHistory.GetHistory().GetEvents()`.
- `RootCause(ctx, input)` — explain the issues. Can be a no-op.
- Each issue is an `InvariantCheckResult{IssueID, InvariantType, Reason, Metadata}`. `IssueID` starts at 0 and counts up within one `Check` call — it is not globally unique; `InvariantType` is what tells issues apart. `Metadata` is JSON, built with `invariant.MarshalData(...)`.

Two templates to copy from:
- **Pure history scan** → copy `invariant/retry/`: empty struct, `NewInvariant()` with no arguments, `RootCause` returns an empty non-nil slice with a comment saying the issues explain themselves.
- **Needs live lookups** (e.g. `DescribeTaskList`) → copy `invariant/timeout/`: client passed in via `NewInvariant(Params{Client: ...})`, real `RootCause`. New root-cause categories are `RootCause` consts in `interface.go`, shared by all invariants.

## The new package: `service/worker/diagnostics/invariant/<name>/`

Package name is one lowercase word (`timeout`, `retry`, `timeoutrisk`).

**types.go**
- `<Name>Type` string type + consts + `String()` — these become the `InvariantType` values.
- `IssueType` string type + consts + `String()` — these become the `Reason` values. Each reason is a plain sentence ending with a period. Never build reasons with `fmt.Sprintf` — numbers belong in the metadata.
- Thresholds are package consts, each with a comment saying what it means.
- One metadata struct per check. If checks have different shapes, add a wrapper struct with one pointer field per check and set only the matching field (see `timeoutrisk.TimeoutRiskIssuesMetadata`). If they share a shape, one struct is enough (see `retry.RetryMetadata`).

**<name>.go**
- `type <Name> invariant.Invariant`, an unexported struct, `NewInvariant()`.
- In `Check`: find the started event by scanning for a non-nil `GetWorkflowExecutionStartedEventAttributes()`, and handle it being missing. One flat loop over events. One `issueID` counter, +1 per issue.
- Don't deduplicate by ActivityID — repeated scheduled events are separate invocations, and server-side retries don't write new scheduled events.

**<name>_test.go**
- Plain table-driven `Test__Check`. No testify/suite, no testify mocks — use uber-go/mock if a client is needed (see `timeout_test.go`).
- Build fixtures by hand: `*types.GetWorkflowExecutionHistoryResponse{History: &types.History{Events: ...}}` with explicit event IDs.
- Expected metadata is `json.Marshal` of the struct; expected reasons come from the `IssueType` consts.
- Cover: each check firing alone, values right at each threshold, several issues at once (IssueIDs count up), a healthy workflow, and a history with no started event.

## Gotcha: history shows what the server saved, not what the client asked for

The server rewrites decision attributes before saving them (`validateActivityScheduleAttributes` in `service/history/decision/checker.go`, plus siblings for child workflows and timers). For activities:
- All four timeouts get capped at the workflow's total execution timeout. A "timeout > workflow timeout" check can never fire; a silently capped value shows up as exactly equal to the workflow timeout.
- With a retry policy, ScheduleToStart and ScheduleToClose get raised up to that cap. Equality there is normal, not a mistake.

Before writing any check on configured values, read the validator to see what the server rewrites. Also check the existing invariants so you don't repeat a check (`retry` already flags heartbeat >= StartToClose and expiration < initial interval).

## Wiring checklist (all required)

1. `service/worker/diagnostics/workflow.go`:
   - Add `<Name>s *<name>Diagnostics` to `DiagnosticsWorkflowResult`.
   - Add `<name>Diagnostics{Issues, Runbook}` and `<name>IssuesResult{IssueID, InvariantType, Reason, Metadata}` structs. Add a `RootCause` field only if `RootCause` does real work.
   - Add `retrieve<Name>Issues(checkResult)`: switch on your `InvariantType` consts and unmarshal the metadata (copy `retrieveTimeoutRiskIssues`).
   - In `DiagnosticsWorkflow`, add an `if len(issues) > 0 { ... Runbook: linkTo<Name>Runbook }` block and a field in the final result.
2. `service/worker/diagnostics/activities.go`: add `linkTo<Name>Runbook` (`https://cadenceworkflow.io/docs/workflow-troubleshooting/<topic>/`). The 10-issues-per-invariant cap applies on its own.
3. `service/worker/diagnostics/parent_workflow.go`: add an `issueType<Name>` const and a branch in `getIssueType` (same `-` joining as the others).
4. `cmd/server/cadence/server.go` (~line 306): add `<name>.NewInvariant(...)` to `params.DiagnosticsInvariants`, plus the import.
5. Existing tests: add the invariant to the lists in `workflow_test.go` `SetupTest` and `activities_test.go` `testDiagnosticWorkflow()`. Check first that existing fixtures don't set off your checks; update expectations if they do. Add `Test__retrieve<Name>Issues` to `workflow_test.go`, copying `Test__retrieveTimeoutRiskIssues`.

## Runbook page (cadence-docs repo)

The `linkTo<Name>Runbook` URL has to exist eventually. Write the page in your local cadence-docs checkout as `docs/08-workflow-troubleshooting/<NN>-<topic>.md`, taking the next free number. `01-timeouts.md` on origin/master shows the style (the local checkout can lag — check against origin/master):

- Frontmatter: `layout: default`, `title`, a one-sentence `description`, `keywords`, and `permalink: /docs/workflow-troubleshooting/<topic>` — must match the `linkTo<Name>Runbook` path.
- No H1 in the body (the title comes from frontmatter). Start with a 2-3 sentence intro.
- One `##` section per issue. The heading describes the situation ("Many activities scheduled in quick succession"), not a command. Then a paragraph on the cause, then the fixes: a single fix is a paragraph starting with `Mitigation:`; multiple fixes go under a `Mitigations:` line as a bulleted list, best fix first.
- Internal doc links are relative paths like `/docs/concepts/activities#timeouts` — no domain, no trailing slash. Only external links (GitHub, pkg.go.dev) are absolute.
- Point to built-in solutions first (`workflow.NewBatchFuture` for big fan-outs, Schedules instead of cron) before manual workarounds.
- Describe thresholds loosely ("50 or more within a few seconds") so the page doesn't go stale when the consts are tuned.

The page is a separate PR to cadence-workflow/cadence-docs. Until it merges, say in the server PR that the page doesn't exist yet.

## Verify

```bash
go build ./service/worker/... ./cmd/server/...
go test -race -count=1 ./service/worker/diagnostics/...
make lint          # no codegen needed — Invariant has no generated mock
```

Before the PR: `make pr GEN_DIR=service/worker/diagnostics`. PR title: `feat(diagnostics): ...`.
