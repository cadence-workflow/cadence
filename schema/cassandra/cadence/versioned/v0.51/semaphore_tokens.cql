-- Distributed semaphore: per-token ownership with an inline reverse index.
--
-- Each bucket (domain_id, semaphore_name, bucket) is its own Cassandra partition, written by a
-- single owning Matching host. A partition holds TWO kinds of row, told apart by `type`, and the
-- four columns token_id / owner_id / holder / held_token mean different things in each kind:
--
--   type = token  (forward index, keyed by token_id): "who holds slot token_id?"
--       token_id   = slot id in [1, size]                    <- the key
--       holder     = current holder's owner_id, or FREE      (LWT-guarded on grant/release)
--       owner_id   = OWNER_NONE sentinel                     (not used by this kind)
--       held_token = TOKEN_NONE sentinel                     (not used by this kind)
--
--   type = owner  (reverse index, keyed by owner_id): "which slot does this hold own?"
--       owner_id   = the hold's identity, length-prefixed workflow_id:run_id:hold_id  <- the key
--       held_token = the token_id this owner currently holds
--       token_id   = TOKEN_NONE sentinel                     (not used by this kind)
--       holder     = OWNER_NONE sentinel                     (not used by this kind)
--
-- The roles flip between the two kinds: a token row keys on token_id and names its holder; an owner
-- row keys on owner_id and names its held_token. Grant and release write the token row and its
-- matching owner row in ONE atomic batch on the same partition, so the forward and reverse views
-- can never diverge.
CREATE TABLE semaphore_tokens (
    domain_id       uuid,       -- domain that owns the semaphore
    semaphore_name  text,       -- user-chosen semaphore name
    bucket          int,        -- static bucket = f(owner_id); one Cassandra partition per bucket
    type            int,        -- rowKind {token, owner}: forward row vs reverse-index row
    token_id        int,        -- token row: slot id [1,size]; owner row: TOKEN_NONE sentinel
    owner_id        text,       -- owner row: the holder's identity; token row: OWNER_NONE sentinel
    holder          text,       -- token row: current holder or FREE (LWT-guarded); owner row: OWNER_NONE sentinel
    held_token      int,        -- owner row: the token this owner holds; token row: TOKEN_NONE sentinel
    updated_time    timestamp,  -- set on grant and on release (held-since / free-since)
    PRIMARY KEY ((domain_id, semaphore_name, bucket), type, token_id, owner_id)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
