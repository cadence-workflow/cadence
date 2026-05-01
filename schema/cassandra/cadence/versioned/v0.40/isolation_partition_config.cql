CREATE TYPE task_list_partition (
                                    isolation_groups set<text>
                                );

ALTER TYPE task_list_partition_config ADD read_partitions map<int, frozen<task_list_partition>>;
ALTER TYPE task_list_partition_config ADD write_partitions map<int, frozen<task_list_partition>>;