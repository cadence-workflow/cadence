CREATE TYPE workflow_timer_task_info (
  version             bigint,
  timer_task_type     int,
  task_id             bigint,
  visibility_ts       timestamp
);

ALTER TABLE executions ADD workflow_timer_task_map map<int, frozen<workflow_timer_task_info>>;
