-- Distributed semaphore: per-bucket FIFO waiter queue for ungranted acquires.
--
-- When a bucket (domain_id, semaphore_name, bucket) has no free token, an acquire cannot be
-- granted immediately and is enqueued here as a waiter, ordered by task_id (arrival). The
-- bucket's single owning Matching host drains the head of this queue on each release. The
-- framing is "a semaphore is a special task type", so this reuses Matching's durable task
-- machinery: the range_id single-writer fence and the background task reader.
--
-- A partition holds TWO kinds of row, told apart by `type`:
--   type = TaskList (control row): carries range_id, the single-writer fence. Every waiter
--          write CAS-checks range_id, so a stale host (after an owner handoff) is rejected.
--   type = Task     (waiter row):  one queued, ungranted acquire, ordered by task_id.
--
-- Removal is an explicit delete at grant; a waiter past its acquire_deadline is skipped by the
-- reader and reaped by a per-row TTL (set at write time to acquire_deadline plus a margin). The
-- TTL is never table-level: that would also expire the control row and lose the fence.
CREATE TABLE semaphore_tasks (
    domain_id         uuid,          -- domain that owns the semaphore
    semaphore_name    text,          -- user-chosen semaphore name
    bucket            int,           -- static bucket = f(owner_id); one queue per bucket
    type              int,           -- rowType {Task, TaskList}: waiter row vs control row
    task_id           bigint,        -- FIFO order within the bucket (arrival)
    range_id          bigint,        -- single-writer fence, carried on the control row
    task              frozen<task>,  -- the queued acquire (reuses the Matching task UDT)
    acquire_deadline  timestamp,     -- waitTimeout; reader skips rows past it, per-row TTL is a backstop
    created_time      timestamp,
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, task_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
