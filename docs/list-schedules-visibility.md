# ListSchedules visibility behavior

`ListSchedules` is implemented entirely on the server by reading **workflow visibility** for the per-schedule long-running workflows (`WorkflowType = cadence-scheduler`, workflow ID prefix `cadence-scheduler:`). There is no separate schedule index table.

## Default path (most clusters)

When dynamic config `frontend.disableListVisibilityByFilter` is **false** for the domain (the default), `ListSchedules` calls **`ListOpenWorkflowExecutions` with a workflow-type filter** for `cadence-scheduler`. This uses the same visibility APIs as other “list open by type” operations and works on **Cassandra / SQL** visibility backends that do **not** support `ListWorkflowExecutions` with a custom query string.

- Pagination tokens are those returned by `ListOpenWorkflowExecutions`.
- Rows are filtered to workflow IDs that start with `cadence-scheduler:` so stray visibility rows are ignored.

## Advanced path (filtered list APIs disabled)

When `frontend.disableListVisibilityByFilter` is **true** for the domain, filtered list visibility calls are blocked for security/operational reasons. In that mode `ListSchedules` falls back to **`ListWorkflowExecutions` with the visibility query**  
`WorkflowType = 'cadence-scheduler' and CloseTime = missing`. That requires **advanced visibility** (e.g. Elasticsearch or Pinot with list-by-query support).

## Enrichment fields (cron, paused, target workflow type)

`ScheduleListEntry` fields beyond `schedule_id` come from **search attributes** upserted by the scheduler workflow (`CadenceScheduleState`, `CadenceScheduleCron`, `CadenceScheduleWorkflowType`). Until the scheduler’s first decision task runs after `CreateSchedule`, those attributes may be missing; the API then returns defaults (not paused, empty cron / type).

## Operational checklist if the list looks empty

1. **Scheduler worker** is running for the domain (`EnableScheduler` worker dynamic config) so schedule workflows exist and visibility records are written.
2. **Visibility store** is configured for the domain and the dual-write path is healthy.
3. If you use **`disableListVisibilityByFilter = true`**, confirm advanced visibility supports the list-by-query API above.
4. **Deleted** schedules close the scheduler workflow; they no longer appear in open listings.

## Cross-cluster (XDC)

Schedule read handlers resolve visibility on the local frontend process (they do not go through the same client redirection stack as some other RPCs). Passive clusters may show stale list results until XDC-aware routing exists for schedule reads.
