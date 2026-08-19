-- Distributed semaphore: per-token ownership with an inline reverse index.
--
-- Each bucket (domain_id, semaphore_name, bucket) is one Cassandra partition, written by its single
-- owning Matching host. `type` splits the partition into two row kinds that reuse the same four
-- columns (token_id / owner_id / holder / held_token) for opposite roles:
--
--   token row (type = token): forward index keyed by token_id — "who holds this slot?"
--       token_id = slot id in [1, size] (the key); holder = holder's owner_id, or FREE (LWT-guarded).
--       owner_id, held_token = unused sentinels (OWNER_NONE, TOKEN_NONE).
--
--   owner row (type = owner): reverse index keyed by owner_id — "which slot does this hold own?"
--       owner_id = hold identity, length-prefixed workflow_id:run_id:hold_id (the key); held_token = its token_id.
--       token_id, holder = unused sentinels (TOKEN_NONE, OWNER_NONE).
--
-- Grant and release write the paired token row and owner row in ONE atomic batch on the partition,
-- so the forward and reverse views can never diverge.
CREATE TABLE semaphore_tokens (
    domain_id       uuid,
    semaphore_name  text,
    bucket          int,        -- static bucket = f(owner_id)
    type            int,        -- rowKind {token, owner}: forward row vs reverse-index row
    token_id        int,        -- token row: slot id [1,size]; owner row: TOKEN_NONE sentinel
    owner_id        text,       -- owner row: the holder's identity; token row: OWNER_NONE sentinel
    holder          text,       -- token row: current holder or FREE (LWT-guarded); owner row: OWNER_NONE sentinel
    held_token      int,        -- owner row: the token this owner holds; token row: TOKEN_NONE sentinel
    updated_time    timestamp,
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, token_id, owner_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
