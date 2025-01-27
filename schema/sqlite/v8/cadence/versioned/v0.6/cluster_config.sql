CREATE TABLE cluster_config (
  row_type INT NOT NULL,
  version BIGINT NOT NULL,
  --
  timestamp DATETIME(6) NOT NULL,
  data           MEDIUMBLOB NOT NULL,
  data_encoding  VARCHAR(16) NOT NULL,
  PRIMARY KEY (row_type, version)
);
