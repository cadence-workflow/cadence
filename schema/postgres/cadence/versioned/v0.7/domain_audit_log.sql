CREATE TABLE domain_audit_log (
  domain_id               BYTEA NOT NULL,
  event_id                BYTEA NOT NULL,
  --
  state_before            BYTEA NOT NULL,
  state_before_encoding   TEXT NOT NULL,
  state_after             BYTEA NOT NULL,
  state_after_encoding    TEXT NOT NULL,
  operation_type          INTEGER NOT NULL,
  created_time            TIMESTAMP NOT NULL,
  last_updated_time       TIMESTAMP NOT NULL,
  identity                TEXT NOT NULL,
  identity_type           TEXT NOT NULL,
  comment                 TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (domain_id, operation_type, created_time, event_id)
);
