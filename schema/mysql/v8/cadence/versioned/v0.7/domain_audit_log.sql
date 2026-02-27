CREATE TABLE domain_audit_log (
  domain_id               VARCHAR(255) NOT NULL,
  event_id                VARCHAR(255) NOT NULL,
  --
  state_before            BLOB NOT NULL,
  state_before_encoding   VARCHAR(16) NOT NULL,
  state_after             BLOB NOT NULL,
  state_after_encoding    VARCHAR(16) NOT NULL,
  operation_type          INT NOT NULL,
  created_time            DATETIME(6) NOT NULL,
  last_updated_time       DATETIME(6) NOT NULL,
  identity                VARCHAR(255) NOT NULL,
  identity_type           VARCHAR(255) NOT NULL,
  comment                 VARCHAR(255) NOT NULL DEFAULT '',
  PRIMARY KEY (domain_id, operation_type, created_time, event_id)
);