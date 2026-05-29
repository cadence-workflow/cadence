CREATE TABLE active_cluster_selection_policy (
  shard_id      INT          NOT NULL,
  domain_id     BINARY(16)   NOT NULL,
  workflow_id   VARCHAR(255) NOT NULL,
  run_id        BINARY(16)   NOT NULL,
  --
  data          MEDIUMBLOB   NOT NULL,
  data_encoding VARCHAR(16)  NOT NULL,
  PRIMARY KEY (shard_id, domain_id, workflow_id, run_id)
);
