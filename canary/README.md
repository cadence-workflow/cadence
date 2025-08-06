# Periodical feature health check workflow tools(aka Canary)

This README describes how to set up Cadence canary, different types of canary test cases, and how to start the canary.

Setup
-----------
## Prerequisite: Cadence server

Canary test suite is running against a Cadence server/cluster. See [documentation](https://cadenceworkflow.io/docs/operation-guide/setup/) for Cadence server cluster setup.

Note that some tests require features like [Advanced Visibility]((https://cadenceworkflow.io/docs/concepts/search-workflows/).) and [History Archival](https://cadenceworkflow.io/docs/concepts/archival/).

For local server env you can run it through:
- Docker: Instructions for running Cadence server through docker can be found in `docker/README.md`. Either `docker-compose-es-v7.yml` or `docker-compose-es.yml` can be used to start the server.
- Build from source: Please check [CONTRIBUTING](/CONTRIBUTING.md) for how to build and run Cadence server from source. Please also make sure Kafka and ElasticSearch are running before starting the server with `./cadence-server --zone es start`. If ElasticSearch v7 is used, change the value for `--zone` flag to `es_v7`.

## Run canary

Different ways of start the canary:

### 1. Use docker image `ubercadence/cadence-canary:master`

You can [pre-built docker-compose file](../docker/docker-compose-canary.yml) to run against local server
In the `docker/` directory, run:
```
docker compose -f docker-compose-canary.yml up
```

This will start the canary worker and also the cron canary.

You can modify [the canary worker config](../docker/config/canary/development.yaml) to run against a prod server cluster:
* Use a different mode to start canary worker only for testing
* Update the config to use Thrift/gRPC for communication
* Use a different image than `master` tag.  See [docker hub](https://hub.docker.com/repository/docker/ubercadence/cadence-canary) for all the images.
Similar to server/CLI images, the `master` image will be built and published automatically by Github on every commit onto the `master` branch.

### 2.  Build & Run

In the project root, build cadence canary binary:
 ```
 make cadence-canary
 ```

Then start canary worker & cron:
 ```
 ./cadence-canary start
 ```
This is essentially the same as
```
 ./cadence-canary start -mode all
 ```

By default, it will load [the configuration in `config/canary/development.yaml`](../config/canary/development.yaml).
Run `./cadence-canary -h` for details to understand the start options of how to change the loading directory if needed.

To start the worker only for manual testing certain cases:
 ```
  ./cadence-canary start -mode worker
  ```

### 3. Monitoring

In production, it's recommended to monitor the result of this canary. You can use [the workflow success metric](https://github.com/cadence-workflow/cadence/blob/9336ed963ca1b5e0df7206312aa5236433e04fd9/service/history/execution/context_util.go#L138)
emitted by cadence history service `workflow_success`. To monitor all the canary test cases, use `workflowType` of `workflow.sanity`.

Configurations
----------------------
Canary workers configuration contains two parts:
- **Canary**: this part controls which domains canary workers are responsible for what tests the sanity workflow will exclude.
```yaml
canary:
  domains: ["cadence-canary"] # it will start workers on all those domains(also try to register if not exists)
  excludes: ["workflow.searchAttributes", "workflow.batch", "workflow.archival.visibility", "workflow.archival.history"] # it will exclude the three test cases. If archival is not enabled, you should exclude "workflow.archival.visibility" and"workflow.archival.history". If advanced visibility is not enabled, you should exclude "workflow.searchAttributes" and "workflow.batch". Otherwise canary will fail on those test cases.
  cron:
    cronSchedule: "@every 30s" #the schedule of cron canary, default to "@every 30s"
    cronExecutionTimeout: 18m #the timeout of each run of the cron execution, default to 18 minutes
    startJobTimeout: 9m #the timeout of each run of the sanity test suite, default to 9 minutes
```
An exception here is `HistoryArchival` and `VisibilityArchival` test cases will always use `canary-archival-domain` domain.

- **Cadence**: this control how canary worker should talk to Cadence server, which includes the server's service name and address.
```yaml
cadence:
  service: "cadence-frontend" # frontend service name
  address: "127.0.0.1:7833" # frontend address
  #host: "127.0.0.1:7933" # replace address with host if using Thrift for compatibility
  #tlsCaFile: "path/to/file" # give file path to TLS CA file if TLS is enabled on the Cadence server
  #metrics: ... # optional detailed client side metrics like workflow latency. But for monitoring, simply use server side metrics `workflow_success` is enough.
```
- **Metrics**: metrics configuration. Similar to server metric emitter, only M3/Statsd/Prometheus is supported.
- **Log**: logging configuration.  Similar to server logging configuration.

Canary Test Cases & Starter
----------------------

### Cron Canary (periodically running the Sanity/starter suite)

The Cron workflow is not a test case. It's a top-level workflow to kick off the Sanity suite(described below) periodically.
To start the cron canary:
```
 ./cadence-canary start -mode cronCanary
 ```

For local development, you can also start the cron canary workflows along with the worker:
```
 ./cadence-canary start -m all
 ```

The Cron Schedule is from the Configuration.
However, changing the schedule requires you manually terminate the existing cron workflow to take into effect.
It can be [improved](https://github.com/cadence-workflow/cadence/issues/4469) in the future.

The workflowID is fixed: `"cadence.canary.cron"`

### Sanity suite (Starter for all test cases)
The sanity workflow is test suite workflow. It will kick off a bunch of childWorkflows for all the test to verify that Cadence server is operating correctly.

An error result of the sanity workflow indicates at least one of the test case fails.

You can start the sanity workflow as one-off run:
```
cadence --do <the domain you configured> workflow start --tl canary-task-queue --et 1200 --wt workflow.sanity -i 0
```

Or using the Cron Canary mentioned above to manage it.


Then observe the progress:
```
cadence --do cadence-canary workflow ob -w <...workflowID form the start command output>
```

NOTE 1:
* tasklist(tl) is fixed to `canary-task-queue`
* execution timeout(et) is recommended to 20 minutes(`1200` seconds) but you can adjust it
* the only required input is the scheduled unix timestamp, and `0` will uses the workflow starting time


NOTE 2: This is the workflow that you should monitor for alerting.
You can use [the workflow success metric](https://github.com/cadence-workflow/cadence/blob/9336ed963ca1b5e0df7206312aa5236433e04fd9/service/history/execution/context_util.go#L138)
emitted by cadence history service `workflow_success`. To monitor all the canary test cases use `workflowType` of `workflow.sanity`.


NOTE 3: This is [the list of the test cases](./sanity.go) that it will start all supported test cases by default if no excludes are configured.
You can find [the workflow names of the tests cases in this file](./const.go) if you want to manually start certain test cases.


### Echo
Echo workflow tests the very basic workflow functionality. It executes an activity to return some output and verifies it as the workflow result.

To manually start an `Echo` test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.echo -i 0
```
Then observe the progress:
```
cadence --do cadence-canary workflow ob -w <...workflowID form the start command output>
```

You can use these command for all other test cases listed below.

### Signal
Signal workflow tests the signal feature.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.signal -i 0
```

### Visibility
Visibility workflow tests the basic visibility feature. No advanced visibility needed, but advanced visibility should also support it.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.visibility -i 0
```

### SearchAttributes
SearchAttributes workflow tests the advanced visibility feature. Make sure advanced visibility feature is configured on the server. Otherwise, it should be excluded from the sanity test suite/case.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.searchAttributes -i 0
```

### ConcurrentExec
ConcurrentExec workflow tests executing activities concurrently.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.concurrent-execution -i 0
```

### Query
Query workflow tests the Query feature.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.query -i 0
```

### Timeout
Timeout workflow make sure the activity timeout is enforced.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.timeout -i 0
```

### LocalActivity
LocalActivity workflow tests the local activity feature.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.localactivity -i 0
```

### Cancellation
Cancellation workflowt tests cancellation feature.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.cancellation -i 0
```

### Retry
Retry workflow tests activity retry policy.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.retry -i 0
```

### Reset
Reset workflow tests reset feature.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.reset -i 0
```

### HistoryArchival
HistoryArchival tests history archival feature. Make sure history archival feature is configured on the server. Otherwise, it should be excluded from the sanity test suite/case.

This test case always uses `canary-archival-domain` domain.

To manually start one run of this test case:
```
cadence --do canary-archival-domain workflow start --tl canary-task-queue --et 10 --wt workflow.timeout -i 0
```

### VisibilityArchival
VisibilityArchival tests visibility archival feature. Make sure visibility feature is configured on the server. Otherwise, it should be excluded from the sanity test suite/case.

This test case always uses `canary-archival-domain` domain.

To manually start one run of this test case:
```
cadence --do canary-archival-domain workflow start --tl canary-task-queue --et 10 --wt workflow.timeout -i 0
```

### Batch
Batch workflow tests the batch job feature. Make sure advanced visibility feature is configured on the server. Otherwise, it should be excluded from the sanity test suite/case.

To manually start one run of this test case:
```
cadence --do <> workflow start --tl canary-task-queue --et 10 --wt workflow.batch -i 0
```
