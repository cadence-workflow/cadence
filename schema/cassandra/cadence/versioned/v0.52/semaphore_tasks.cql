-- A queued semaphore acquire waiting for a token in a bucket
CREATE TYPE semaphore_task (
    workflow_id  text,
    run_id       uuid,
    hold_id      bigint,  -- identifies this specific hold
    schedule_id  bigint
);

-- Distributed semaphore: per-bucket FIFO queue of ungranted acquires. Each full bucket
-- (domain_id, semaphore_name, bucket) enqueues acquires as waiters ordered by task_id; the bucket's
-- single owning Matching host drains the head on each release, reusing the tasks range_id fence.
--
-- `type` tells the two row kinds apart (ints match the tasks rowType convention):
--   control row (type = 1): holds range_id (the fence every waiter write CAS-checks, rejecting a
--          stale host after an owner handoff) and ack_level (the completed-through read cursor).
--   waiter row  (type = 0): one queued acquire.
CREATE TABLE semaphore_tasks (
    domain_id         uuid,
    semaphore_name    text,
    bucket            int,                    -- static bucket = f(owner_id); one queue per bucket
    type              int,                    -- rowType: 0 = waiter (queued acquire), 1 = control row (range_id fence)
    task_id           bigint,
    range_id          bigint,                 -- single-writer fence, carried on the control row
    ack_level         bigint,                 -- control row: waiters with task_id <= this are granted/expired and
                                              -- range-deleted; the reader reads above it, never scanning acked tombstones
    task              frozen<semaphore_task>, -- the queued acquire's identity (owner_id parts + acquire handle)
    acquire_deadline  timestamp,              -- the time this waiter gives up if still not granted
                                              -- No deadline means no skip and no TTL
    created_time      timestamp,
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, task_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
