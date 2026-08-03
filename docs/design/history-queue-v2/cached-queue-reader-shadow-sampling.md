# CachedQueueReader: periodic shadow sampling in `enabled` mode

## Problem Statement

`CachedQueueReader` has three modes: `disabled`, `shadow`, and `enabled`. In `shadow`
mode, every `GetTask` call is served from the cache internally but always
double-checked against the database, with mismatches logged and counted via
`CachedQueueShadowMismatchCounter`. In `enabled` mode, `GetTask` is served
purely from the cache — there is no ongoing comparison against the database.

Once a shard is promoted from `shadow` to `enabled`, operators lose all
visibility into whether the cache is still agreeing with the database. A
regression in cache logic that passes unit and integration tests could
silently diverge from the database on a production or staging shard, and
nothing would catch it until a workflow-visible symptom (a skipped or
duplicated timer, for example) surfaced much later.

## Solution

Add a low-overhead, continuous health check to `enabled` mode: on a steady
wall-clock cadence (independent of request volume), one `GetTask` call per
shard is diverted through the same shadow comparison logic already used by
`shadow` mode, instead of being served purely from the cache. This reuses the
existing shadow-mismatch metric and log lines, giving operators an ongoing
regression signal — and the ability to alert on it — for shards already
running in `enabled` mode, without paying the cost of comparing every
request.

## User Stories

1. As an on-call engineer, I want `enabled`-mode shards to keep being checked against the database periodically, so that a cache regression is caught by an alert instead of by a customer-visible symptom.
2. As an on-call engineer, I want the periodic check's cadence to be independent of a shard's request volume, so that a busy shard doesn't get checked (and doesn't incur DB read amplification) much more often than a quiet shard.
3. As an on-call engineer, I want the periodic check to reuse the existing shadow-mismatch metric, so that I can build one alert that covers both shadow-mode rollouts and steady-state enabled-mode monitoring.
4. As an operator, I want to be able to disable the periodic check via dynamic config, so that I can turn it off if it turns out to be too expensive or noisy in a given environment.
5. As an operator, I want the periodic check's interval to be configurable per shard via existing dynamic config mechanisms, so that I can tune the check frequency without a code change.
6. As a developer debugging a mismatch report, I want a distinct log line marking "this GetTask call was diverted into a shadow sample check," so that I can tell a sampled comparison apart from a request that would otherwise have been served straight from cache.
7. As a maintainer, I want the periodic-check code to be clearly marked as temporary scaffolding tied to `enabled` mode not yet being the default, so that a future cleanup pass knows it can be removed once `CachedQueueReader` is enabled by default and shadow-mode rollouts are no longer needed.
8. As a developer, I want the periodic check to apply only when the mode is `enabled` (not `shadow`, which already checks every request, and not `disabled`), so that the two mechanisms don't overlap or double-count.
9. As a developer, I want the periodic check to leave `LookAHead` untouched, so that the scope of this change stays limited to `GetTask`, which is the read path the health check is meant to validate.
10. As a developer, I want the check's "last sampled" state to live per `cachedQueueReader` instance (i.e., per shard), so that sampling cadence is naturally per-shard without needing cross-shard coordination.
11. As a developer, I want concurrent `GetTask` calls racing on the same due-check window to result in exactly one shadow sample, so that a burst of concurrent requests doesn't turn "one check per interval" into "N checks per interval."
12. As a developer, I want the "last sampled" clock to reset whenever a check is attempted — regardless of whether the database call underlying it succeeds or fails — so that a struggling database doesn't get hammered by rapid retries disguised as health checks.
13. As a developer, I want the very first `GetTask` call after a shard starts (or after mode switches to `enabled`) to be eligible for an immediate sample, so that a newly-promoted shard doesn't wait a full interval before its first health check.

## Implementation Decisions

- **New dynamic config**: `TimerProcessorCachedQueueReaderShadowSampleInterval`, a `DurationPropertyFn`, default `5 * time.Minute`. A value `<= 0` disables periodic sampling entirely. Follows the naming and wiring pattern of the existing `TimerProcessorCachedQueueReader*` family (e.g. `TimerProcessorCachePrefetchTriggerWindow`): added to the dynamic property constants, exposed on the history service `Config` struct, and passed into `cachedQueueReaderOptions` alongside the existing `Mode`, `MaxSize`, etc.
- **Scope**: applies only to `GetTask`, only when the reader's mode is `enabled` (as reported by the existing `isEnabled()` helper). Does not apply in `shadow` mode (already compares every request) or `disabled` mode. Does not apply to `LookAHead`.
- **Sampling mechanism**: a time-interval gate, not a request counter. `cachedQueueReader` tracks the unix-nano timestamp of the last shadow sample in an `atomic.Int64` field. On each cache-served `GetTask` call in `enabled` mode, if `now - lastSampleTime >= interval` (and `interval > 0`), the call attempts to claim the sample via a compare-and-swap on that field. Only the caller whose CAS succeeds performs the sample for that window; all other concurrent callers in the same window are treated as ordinary cache hits.
- **Reset semantics**: the CAS unconditionally advances the "last sampled" timestamp to `now` at the moment the sample is claimed — before the underlying database call is made — so the interval is measured from attempt-to-attempt, not success-to-success. A failing or slow database call does not cause rapid re-triggering.
- **Initial state**: the "last sampled" field starts at its zero value, so the first `GetTask` call after the reader starts (or after mode transitions into `enabled`) is immediately eligible, without waiting a full interval.
- **Sampled-call behavior**: when a call claims the sample, it is routed through the existing shadow-comparison path (the same logic `shadow` mode uses for every request: query the database, compare against the cache snapshot via the existing mismatch-bucketing logic, and return the database's response to the caller for that one call). No new comparison logic is introduced — this reuses the shadow path as-is. The `CachedQueueShadowMismatchCounter` metric and existing mismatch log lines fire exactly as they do in `shadow` mode, since it is the same code path.
- **Mode re-validation**: no additional mode re-check is needed at the point the sample is claimed — the surrounding `GetTask` control flow already guarantees this branch is only reached when the mode is `enabled` at read time, and this decision was confirmed as sufficient (no defensive re-read of mode once already routed to the shadow path).
- **New log line**: an `Info`-level log, message `"shadow sample check"`, emitted directly in `GetTask` immediately before diverting the call into the shadow path (not inside the shared shadow-comparison helper, so it's visibly distinct from the mismatch-report logging that already happens there). The log line is preceded by a code comment explaining that this is a temporary continuous-health-check mechanism for `enabled` mode, with a `TODO` noting it can be removed once `CachedQueueReader` is enabled by default (i.e., once `shadow`-mode rollouts, and the need for this ongoing regression signal, are no longer relevant).

## Testing Decisions

- Tests should exercise only externally observable behavior of `cachedQueueReader.GetTask`: what is returned to the caller, which metrics/log lines fire, and how many times the underlying base reader is invoked — not internal field values directly, except where a field is the simplest way to control a fake clock for deterministic timing assertions.
- Prior art: the existing shadow-mode test coverage for `GetTask` (covering current-range mismatches, previous-range mismatches, and stale-shard-owner suppression via a hand-rolled `metrics.Scope` test double) is the direct template for asserting the sampled path's mismatch/metric behavior, since it exercises the same underlying comparison logic.
- New cases to add for the periodic-sampling behavior, using the existing fake time source used elsewhere in this test suite:
  - First `GetTask` call after start/mode-switch-to-`enabled` triggers a sample immediately (base reader invoked, shadow log line present).
  - A subsequent call within the configured interval does not trigger another sample (cache-only response, no extra base reader invocation).
  - A call after the interval has elapsed (advancing the fake clock) triggers another sample.
  - Setting the interval to `0` or a negative value disables sampling entirely, even across a long elapsed time.
  - The `"shadow sample check"` log line is present exactly on sampled calls and absent on ordinary cache hits.
  - A mismatch surfaced via the sampled path increments `CachedQueueShadowMismatchCounter` with the same tagging behavior already verified for `shadow` mode.
- Add a corresponding default-value assertion for the new dynamic config entry in the history service config test suite, matching how other `TimerProcessorCachedQueueReader*` properties are asserted.

## Out of Scope

- Any change to `LookAHead` behavior or its shadow/disabled handling.
- Any change to `shadow` mode's existing per-request comparison behavior.
- A new or separate metric for the sampled path — it reuses `CachedQueueShadowMismatchCounter` as-is.
- Shard-level or percentage-of-shards sampling strategies (considered and explicitly rejected in favor of the time-interval gate).
- Probabilistic (random-chance) sampling (considered and explicitly rejected in favor of a deterministic time-interval gate).
- Alert configuration itself (dashboards, alert thresholds/routing) — this spec only covers emitting the signal the alert would consume.

## Further Notes

- This mechanism is explicitly intended as temporary scaffolding for the period during which `CachedQueueReader` is being rolled out and is not yet the default. The code comment and TODO next to the new log line are meant to make this discoverable during a future cleanup once `enabled` mode is the unconditional default and `shadow` mode (and this periodic variant of it) can be retired.
- Because the sampled call incurs a synchronous database round-trip on the request path (identical to what already happens for every request in `shadow` mode), the default 5-minute interval was chosen to bound this to roughly once per shard per five minutes — negligible additional load compared to a full shadow-mode rollout.
