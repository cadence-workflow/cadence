-- Add cron_schedule field to track cron workflow schedules
ALTER TABLE open_executions ADD cron_schedule text;
ALTER TABLE closed_executions ADD cron_schedule text;
ALTER TABLE closed_executions_v2 ADD cron_schedule text;

-- Add execution_status field to track workflow execution status (Pending, Started, or close status)
ALTER TABLE open_executions ADD execution_status int;
ALTER TABLE closed_executions ADD execution_status int;
ALTER TABLE closed_executions_v2 ADD execution_status int;

-- Add scheduled_execution_time field to track the actual scheduled execution time (start_time + first_decision_task_backoff)
ALTER TABLE open_executions ADD scheduled_execution_time timestamp;
ALTER TABLE closed_executions ADD scheduled_execution_time timestamp;
ALTER TABLE closed_executions_v2 ADD scheduled_execution_time timestamp;
