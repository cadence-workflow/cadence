-- Add column to track timer tasks associated with a workflow execution
-- for cleanup when the workflow is deleted at retention time.
-- Each element is a (visibilityTimestamp, taskID) tuple.
ALTER TABLE executions ADD workflow_timer_tasks set<frozen<tuple<timestamp, bigint>>>;
