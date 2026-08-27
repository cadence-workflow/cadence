-- Distributed semaphore: which run holds which token slot.
--
-- Each bucket (domain_id, semaphore_name, bucket) is one partition, written by the single
-- Matching host that owns it. `type` splits it into two row kinds sharing the same four columns:
--
--   token row (type = 0), keyed by token_id: "who holds this slot?"
--       token_id = the slot id in [1, size]; holder = the owner_id holding it, or FREE
--       owner_id = OWNER_NONE; held_token = unset
--
--   owner row (type = 1), keyed by owner_id: "which slot does this hold own?"
--       owner_id = length-prefixed workflow_id:run_id:hold_id; held_token = its token_id
--       token_id = TOKEN_NONE; holder = unset
--
-- FREE, OWNER_NONE, and TOKEN_NONE are reserved values, not nulls. They are defined in
-- nosqlplugin/cassandra/semaphore_tokens.go.
--
-- Grant and release write both rows in one atomic batch, so the two views cannot diverge.
CREATE TABLE semaphore_tokens (
    domain_id       uuid,
    semaphore_name  text,
    bucket          int,        -- static bucket = f(owner_id)
    type            int,        -- 0 = token row, 1 = owner row
    token_id        int,        -- token row: slot id [1,size]; owner row: TOKEN_NONE sentinel
    owner_id        text,       -- owner row: the holder's identity; token row: OWNER_NONE sentinel
    holder          text,       -- token row: current holder or FREE (LWT-guarded); owner row: unset
    held_token      int,        -- owner row: the token this owner holds; token row: unset
    updated_time    timestamp,
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, token_id, owner_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
