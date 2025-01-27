CREATE INDEX by_close_time_by_status ON executions_visibility (domain_id, close_time DESC, run_id, close_status);
