# Async API

## Overview

Triggering too many workflows at the same time may overload the underlying storage system. It's typical the Cadence operator to set up quotas for these APIs via `frontend.rps` dynamic config. This means the caller service has to backoff and retry if the request is rejected by the Cadence server.

When you have millions of workflows to trigger, you may want to consider using the Async APIs. More specifically,
- `StartWorkflowExecutionAsync`
- `SignalWithStartWorkflowExecutionAsync`

These APIs are designed to be more efficient than the regular APIs. They don't wait for the workflow to be started or signaled. Instead, they queue a message to underlying queue system and return. The queue system supported currently is Kafka. The Cadence server (workers service) will poll the queue and process the messages.

## Caveats

- Global availability: Regional failovers will work as is for Global Cadence Domains. The async workflow requests will start on the active side once failed over. Previously enqueued requests will be forwarded to active side by Cadence in an idempotent way. If the regions are disconnected or the previously active region is fully down then the leftover messages in the queue will be processed once the region is healthy again.
- Idempotency: To avoid any edge cases and achieve full idempotency, Cadence dedupes requests based on request id. Request id is not exposed from our fat clients used internally so if you have additional retries on top of what client library already performs then you might send duplicate requests. See workflowid reuse policy to get around duplicate request problems.
- Workflow id reuse policy: If you enqueue duplicate requests with same workflow id and choose a reuse policy that causes failure to start, the failure will be discarded. Since duplicate delivery is given with queue systems (at least once) avoid using WorkflowIDReusePolicyTerminateIfRunning .
- Run id: Async APIs accept the same input parameters as their corresponding sync versions but do NOT return run id. Based on our discussions with multiple Cadence users, run id is discarded almost all the time so by switching to Async APIs you are getting pretty much the same semantics.
- Request size and rate limits: Async APIs can support higher rate limits than the regular APIs. The default rate limit is 10k rps. You can adjust the rate limit via `frontend.asyncrps` dynamic config. Your kafka topic might be the bottleneck so you can adjust the topic configuration accordingly.
- Delays: Async API requests are queued and consumed by Cadence backend. There can be some unexpected delays in this flow due to high number of messages/bytes etc. Basically your workflows don't start immediately and the delay depends on various factors.

## How to use

This section walks through how to use the Async APIs on a local Cadence cluster.


1. **Start a local Cadence cluster with async workflow queue enabled.**
```
docker compose -f docker/docker-compose-async-wf-kafka.yml up
```

This will start Cadence server, Cadence UI, Kafka, Cassandra, Prometheus and Grafana containers.
Notice the environment variables in the docker compose file:
- `ASYNC_WF_KAFKA_QUEUE_ENABLED`: This is set to `true` to enable the async workflow queue.
- `ASYNC_WF_KAFKA_QUEUE_TOPIC`: This is the name of the Kafka topic to use for the async workflow queue.
- `KAFKA_SEEDS`: This is the list of Kafka brokers to use for the async workflow queue.
- `KAFKA_PORT`: This is the port to use for the Kafka brokers.

When Cadence server container starts, it will materialize the ./docker/config_template.yaml file and populate `asyncWorkflowQueues` section with the following content:

```
asyncWorkflowQueues:
  queue1:
    type: "kafka"
    config:
      connection:
        brokers:
          - kafka:9092
      topic: async-wf-topic1
```

Now the Cadence server recognizes `queue1` as a valid async workflow queue. However, it doesn't interact with Kafka yet.

Note: It may take a minute for the containers to be ready. Check their status via `docker ps` command and once all are running, you can proceed to the next step.

2. **Register a domain (or skip this step if you already have a domain)**

```
docker run -t --rm --network host ubercadence/cli:master \
    --address localhost:7933 \
    --domain test-domain \
    domain register
```

3. **Enable async APIs for the domain**

Configure `test-domain` to use `queue1` as the async workflow queue.
```
docker run -t --rm --network host ubercadence/cli:master \
    --address localhost:7933 \
    --domain test-domain \
    admin async-wf-queue update \
    --json "{\"PredefinedQueueName\":\"queue1\", \"Enabled\": true}"
```

Note: If you get "Domain update too frequent." error, you can try to wait for a minute and run the command again.

Validate the new configuration:
```
docker run -t --rm --network host ubercadence/cli:master \
    --address localhost:7933 \
    --domain test-domain \
    admin async-wf-queue get
```

Cadence server will start a consumer to poll from the kafka topic within a minute. If more than one domain is using the same queue, the consumer will be shared across all domains.


4. **Start a workflow asynchronously**

The `test-domain` is now ready to accept async workflow requests. Update your worker (or modify samples code) to use one of the async APIs:
- [StartWorkflowExecutionAsync](https://github.com/cadence-workflow/cadence-idl/blob/0e56e57909d9fa738eaa8d7a9561ea16acdf51e4/proto/uber/cadence/api/v1/service_workflow.proto#L49)
- [SignalWithStartWorkflowExecutionAsync](https://github.com/cadence-workflow/cadence-idl/blob/0e56e57909d9fa738eaa8d7a9561ea16acdf51e4/proto/uber/cadence/api/v1/service_workflow.proto#L63)
