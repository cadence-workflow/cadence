# What?

This document writes the steps we need to execute to migrate the number of task list partitions configuration from dynamic configuration to the database. For background knowledge about task list partition, please read this [doc](../scalable_tasklist.md).

# Why?

We're doing this migration because we want to programmatically update the number of task list partitions. Not all implementations of dynamic configuration dependencies support update operation.

# How?
1. Check the existing number of partitions for the task list you want to migrate:
   - [matching.numTasklistReadPartitions](https://github.com/cadence-workflow/cadence/blob/v1.2.13/common/dynamicconfig/constants.go#L3350)
   - [matching.numTasklistWritePartitions](https://github.com/cadence-workflow/cadence/blob/v1.2.13/common/dynamicconfig/constants.go#L3344)

2. Run the following ClI commands to update the number of partitions of the task list you want to migrate and make sure that task list type parameter is not missing in your commands:
```
cadence admin tasklist update-partition -h
```
To get the number of partitions from database, use the following CLI command:
```
cadence admin tasklist describe -h
```
3. Set this dynamic configuration value to true for the task list you want to migrate:
  - [matching.enableGetNumberOfPartitionsFromCache](https://github.com/cadence-workflow/cadence/blob/v1.2.15-prerelease02/common/dynamicconfig/constants.go#L4008)

4. Repeat the steps for all task lists. However, you can skip the steps if the number of partitions of the task list is 1.

5. You can enable adaptive task list scaler for the task list. Set [matching.enableAdaptiveScaler](https://github.com/cadence-workflow/cadence/blob/v1.2.17/common/dynamicconfig/constants.go#L4012) to true for the task list.

# Status
As of v1.2.17, the default value of [matching.enableGetNumberOfPartitionsFromCache](https://github.com/cadence-workflow/cadence/blob/v1.2.17/common/dynamicconfig/constants.go#L4004) is still false.

# Plan
We're planning to change the default value of [matching.enableGetNumberOfPartitionsFromCache](https://github.com/cadence-workflow/cadence/blob/v1.2.17/common/dynamicconfig/constants.go#L4004) to `true` in v1.2.18.
We're planning to deprecate [matching.numTasklistReadPartitions](https://github.com/cadence-workflow/cadence/blob/v1.2.13/common/dynamicconfig/constants.go#L3350) and [matching.numTasklistWritePartitions](https://github.com/cadence-workflow/cadence/blob/v1.2.13/common/dynamicconfig/constants.go#L3344), but we haven't decided when to do it. Please be prepared for the migration.
