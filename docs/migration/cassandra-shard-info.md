# What?

The [`shard`](https://github.com/cadence-workflow/cadence/blob/v1.3.3/schema/cassandra/cadence/schema.cql#L1) Cassandra data type and [`shard`](https://github.com/cadence-workflow/cadence/blob/v1.3.3/schema/cassandra/cadence/schema.cql#L368) column in Cassandra [`executions`](https://github.com/cadence-workflow/cadence/blob/v1.3.3/schema/cassandra/cadence/schema.cql#L357) table is deprecated in favor of [`data`](https://github.com/cadence-workflow/cadence/blob/v1.3.3/schema/cassandra/cadence/schema.cql#L366C3-L366C7) and [`data_encoding`](https://github.com/cadence-workflow/cadence/blob/v1.3.3/schema/cassandra/cadence/schema.cql#L367) field.

# Why?

The process to introduce new field to existing data type is quite tedious because we need to update the corresponding Cassandra data type.

# How?
1. Update server to v1.3.1 or a newer version
2. Set this dynamic configuration value to true
    - [`history.readNoSQLShardFromDataBlob`](https://github.com/cadence-workflow/cadence/blob/v1.3.3/common/dynamicconfig/dynamicproperties/constants.go#L4578C18-L4578C52)
3. Check the metrics with these tags:
    - operation: getshard
    - name: nosql_shard_store_read_from_original_column OR nosql_shard_store_read_from_data_blob

    These metrics are only emitted during service start time, and after migration you should only see metrics with `name:nosql_shard_store_read_from_data_blob` tag.

# Status
Starting from 1.3.4, the default value of [`history.readNoSQLShardFromDataBlob`](https://github.com/cadence-workflow/cadence/blob/v1.3.3/common/dynamicconfig/dynamicproperties/constants.go#L4578C18-L4578C52) is set to `true`. And we're planning to remove this dynamic config in later version.
