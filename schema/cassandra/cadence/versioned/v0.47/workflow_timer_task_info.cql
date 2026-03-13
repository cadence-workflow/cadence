-- Add blob-encoded workflow timer tasks fields
ALTER TABLE executions ADD workflow_timer_tasks blob;
ALTER TABLE executions ADD workflow_timer_tasks_encoding text;
