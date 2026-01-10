-- Add cron_schedule field to track cron workflow schedules
ALTER TABLE executions_visibility ADD cron_schedule VARCHAR(255) NULL;

-- Add execution_status field to track workflow execution status (Pending, Started, or close status)
ALTER TABLE executions_visibility ADD execution_status INT NULL;

-- Add scheduled_execution_time field to track the actual scheduled execution time (start_time + first_decision_task_backoff)
ALTER TABLE executions_visibility ADD scheduled_execution_time DATETIME(6) NULL;
