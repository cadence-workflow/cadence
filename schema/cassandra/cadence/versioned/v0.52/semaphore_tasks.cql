-- Distributed semaphore: per-bucket FIFO queue of ungranted acquires. Each full bucket
-- (domain_id, semaphore_name, bucket) enqueues acquires as waiters ordered by task_id; the bucket's
-- single owning Matching host drains the head on each release, reusing the tasks range_id fence.
--
-- `type` tells the two row kinds apart (ints match the tasks rowType convention):
--   control row (type = 1): holds range_id, the fence every waiter write CAS-checks (rejects a
--          stale host after an owner handoff).
--   waiter row  (type = 0): one queued acquire.
CREATE TABLE semaphore_tasks (
    domain_id         uuid,
    semaphore_name    text,
    bucket            int,           -- static bucket = f(owner_id); one queue per bucket
    type              int,           -- rowType: 0 = waiter (queued acquire), 1 = control row (range_id fence)
    task_id           bigint,
    range_id          bigint,        -- single-writer fence, carried on the control row
    task              frozen<task>,  -- the queued acquire (reuses the Matching task UDT)
    acquire_deadline  timestamp,     -- the time this waiter gives up if still not granted
                                     -- No deadline means no skip and no TTL
    created_time      timestamp,
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, task_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
