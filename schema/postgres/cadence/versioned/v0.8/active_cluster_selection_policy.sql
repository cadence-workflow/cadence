CREATE TABLE active_cluster_selection_policy (
  shard_id      INT          NOT NULL,
  domain_id     BYTEA        NOT NULL,
  workflow_id   TEXT         NOT NULL,
  run_id        BYTEA        NOT NULL,
  --
  data          BYTEA        NOT NULL,
  data_encoding VARCHAR(16)  NOT NULL,
  PRIMARY KEY (shard_id, domain_id, workflow_id, run_id)
);
