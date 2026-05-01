ALTER TYPE workflow_execution ADD cron_overlap_policy int;
ALTER TYPE workflow_execution ADD active_cluster_selection_policy blob;
ALTER TYPE workflow_execution ADD active_cluster_selection_policy_encoding text;
ALTER TYPE workflow_execution ADD task_list_kind int;
ALTER TYPE activity_info ADD task_list_kind int;
