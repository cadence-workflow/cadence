ALTER TYPE workflow_execution ADD import_source_workflow_id text;
ALTER TYPE workflow_execution ADD import_source_run_id uuid;
ALTER TYPE workflow_execution ADD import_timestamp timestamp;
ALTER TYPE workflow_execution ADD import_in_progress boolean;
