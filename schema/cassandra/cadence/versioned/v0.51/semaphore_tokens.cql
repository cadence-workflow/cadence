-- Distributed semaphore token ownership, with the reverse index inline.
--
-- One partition per bucket (domain_id, semaphore_name, bucket), written by the single Matching
-- host that owns it. `type` gives the partition two row shapes over the same columns: a token
-- row keyed by token_id ("who holds this slot?") and an owner row keyed by owner_id ("which
-- slot does this hold own?"). A key column that does not apply to a shape carries a sentinel,
-- since a key cannot be null; the other columns are simply left unset.
--
-- Grant and release write both rows in one atomic batch, so the two cannot diverge.
CREATE TABLE semaphore_tokens (
    domain_id       uuid,
    semaphore_name  text,
    bucket          int,        -- static bucket = f(owner_id)
    type            int,        -- rowType {token, owner}: forward row vs reverse-index row
    token_id        int,        -- token row: slot id [1,size]; owner row: TOKEN_NONE sentinel
    owner_id        text,       -- owner row: the holder's identity; token row: OWNER_NONE sentinel
    holder          text,       -- token row: current holder or FREE (LWT-guarded); owner row: unset
    held_token      int,        -- owner row: the token this owner holds; token row: unset
    updated_time    timestamp,
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, token_id, owner_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
