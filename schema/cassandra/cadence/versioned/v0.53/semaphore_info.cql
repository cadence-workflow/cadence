-- Semaphore hold in progress mutable state
CREATE TYPE semaphore_info (
  version                   bigint,
  initiated_id              bigint,
  semaphore_name            text,
  owner_id                  text,
  token_id                  int,
  acquire_deadline          timestamp,
);

ALTER TABLE executions ADD semaphore_map map<bigint, frozen<semaphore_info>>;
