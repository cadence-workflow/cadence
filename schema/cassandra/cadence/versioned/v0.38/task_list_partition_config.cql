CREATE TYPE task_list_partition_config (
  version              bigint,
  num_read_partitions  int,
  num_write_partitions int
);

ALTER TYPE task_list ADD adaptive_partition_config frozen<task_list_partition_config>;
