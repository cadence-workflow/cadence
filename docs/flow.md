# Flow

## Workflow processing flow

Below diagram demonstrates how a workflow is processed by Cadence workers and backend components.
Note that this is a partial diagram that only highlights selected components and their interactions. There is more happening behind the scenes.

[comment]: <> (To visualize mermaid flowchart below, install Mermaid plugin for your IDE. Works in github out of the box)

```mermaid
flowchart TD
    User((Domain X Admin)) -->|"0. Deploy worker with config: domain, tasklist(s)"| Worker[[\nDomain X Worker\n\n]]
    ExternalInitiator(("Domain X\nAnother Service/CLI")) -->|"1. Start/Signal Workflow"| Frontend
    Worker -->|2. Poll tasks for domain: X, tasklist: tl1| Frontend[[\nCadence Frontend\n\n]]
    Frontend -->|3. Forward Poll request| Matching[[\nCadence Matching\n\n]]
    Matching -->|4. Poll| TaskListManager(Tasklist Manager for tl1)
    TaskListManager -->|5. Long Poll| WaitUntilTaskFound
    WaitUntilTaskFound --> |6. Return task to worker| Worker
    Worker -->|7. Generate decisions\n& respond| Frontend
    Frontend -->|8. Respond decisions| History[[\nCadence History\n\n]]
    History -->|9. Mutable state update| ExecutionsStore[(Executions Store)]
    History -->|10.a. Notify| HistoryQueue(History Queues - per shard)
    HistoryQueue -->|Periodic task fetch| ExecutionsStore
    HistoryQueue -->|11.b. Execute transfer task| TransferTaskExecutor(Transfer Task Executor)
    HistoryQueue -->|11.a. Execute timer task| TimerTaskExecutor(Timer Task Executor)
    HistoryQueue -->|Periodic per-shard offset save| ShardsStore[(Shards Store)]
    TransferTaskExecutor -->|12.b. Add task| Matching
    TimerTaskExecutor -->|12.a. Add task| Matching
    Matching -->|13. Add task| TaskListManager
    TaskListManager -->|14. Add task| SyncMatch{Check sync match}
    SyncMatch -->|15.b. Found poller - Do sync match| SyncMatched
    SyncMatch -->|15.a. No pollers - Save task| TasksStore[(Tasks Store)]
    TaskListManager -->|Periodic task fetch| TasksStore

```

### Cassandra Notes:

In order to leverage [Cassandra Light Weight Transactions](https://www.yugabyte.com/blog/apache-cassandra-lightweight-transactions-secondary-indexes-tunable-consistency/), Cadence stores multiple types of records in the same `executions` table. ([ref](https://github.com/cadence-workflow/cadence/blob/51758676ce9d9609e736c64f94dc387ed2c75b7c/schema/cassandra/cadence/schema.cql#L338))
- **Shards Store**: Shard records are stored in `executions` table with type=0.
- **Executions Store**: Execution records are stored in `executions` table with type=1
- **Tasks Store**: Task records are stored in `executions` table with type={2,3,4,5}
