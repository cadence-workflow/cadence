-- A queued semaphore acquire waiting for a token in a bucket
CREATE TYPE semaphore_task (
    workflow_id  text,
    run_id       uuid,
    hold_id      bigint  -- identifies this hold
);

-- Distributed semaphore: per-bucket FIFO queue of ungranted acquires, ordered by task_id.
-- Each bucket (domain_id, semaphore_name, bucket) is owned by a single Matching host that
-- drains the head on each release, reusing the tasks range_id fence.
CREATE TABLE semaphore_tasks (
    domain_id         uuid,
    semaphore_name    text,
    bucket            int,                    -- static bucket = f(owner_id); one queue per bucket
    type              int,                    -- rowType: 0 = waiter (queued acquire), 1 = control row (range_id fence)
    task_id           bigint,
    range_id          bigint,                 -- single-writer fence, carried on the control row
    ack_level         bigint,                 -- control row: waiters with task_id <= this are granted/expired and
                                              -- range-deleted; the reader reads above it, never scanning acked tombstones
    task              frozen<semaphore_task>,
    acquire_deadline  timestamp,              -- the time this waiter gives up if still not granted
                                              -- No deadline means no skip and no TTL
    created_time      timestamp,
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, task_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
