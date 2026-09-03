-- Add the single-writer fence to semaphore_tokens.
--
-- range_id is carried on a per-bucket lease row (type = 2), alongside the existing token
-- (type = 0) and owner (type = 1) rows, and is null on both of those. 
ALTER TABLE semaphore_tokens ADD range_id bigint;
