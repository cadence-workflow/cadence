CREATE TABLE domain_audit_log (
  domain_id               UUID NOT NULL,
  event_id                UUID NOT NULL,
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
  comment                 TEXT NOT NULL,
  PRIMARY KEY (domain_id, operation_type, created_time, event_id)
);
