CREATE TABLE domain_audit_log (
    domain_id uuid,
    event_id uuid, -- event_id is the unique identifier for this change to the domain

    state_before blob, -- state_before stores the domain state before the request
    state_before_encoding text, -- the encoding type used for state_before

    state_after blob, -- state_after stores the domain state after the request
    state_after_encoding text, -- the encoding type used for state_after

    operation_type int, -- operation_type stores the type of operation that was performed. It is deserialized as an enum and can be used to customize the serialization/deserialization strategy. 

    created_time timestamp, -- created_time the time this row was inserted
    last_updated_time timestamp,

    identity text, -- the unique identifier of the user that made the change
    identity_type text, -- identity_type can be used to delineate between service, user, or other identities

    comment text, -- comment can be used when manual updates or creates are performed on the database as an audit trail

    PRIMARY KEY ((domain_id, operation_type), created_time, event_id)
) WITH CLUSTERING ORDER BY (created_time DESC, event_id ASC)
  AND COMPACTION = {
      'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
  };
