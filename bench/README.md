Cadence Bench Tests
===================

This README describes how to set up Cadence bench, different types of bench loads, and how to start the load.

Setup
-----------
### Cadence server

Bench suite is running against a Cadence server/cluster. See [documentation](https://cadenceworkflow.io/docs/operation-guide/setup/) for Cadence server cluster setup.

Note that only the Basic bench test don't require Advanced Visibility. 
 
Other advanced bench tests requires Cadence server with Advanced Visibility. 

For local env you can run it through:
- Docker: Instructions for running Cadence server through docker can be found in `docker/README.md`. Either `docker-compose-es-v7.yml` or `docker-compose-es.yml` can be used to start the server.
- Build from source: Please check [CONTRIBUTING](/CONTRIBUTING.md) for how to build and run Cadence server from source. Please also make sure Kafka and ElasticSearch are running before starting the server with `./cadence-server --zone es start`. If ElasticSearch v7 is used, change the value for `--zone` flag to `es_v7`.

See more [documentation here](https://cadenceworkflow.io/docs/concepts/search-workflows/).

### Bench Workers
:warning: NOTE: Starting this bench worker will not automatically start a bench test. Next two sections will cover how to start and configure it.

Different ways of start the bench workers:

#### 1. Use docker image `ubercadence/cadence-bench:master`

You can [pre-built docker-compose file](../docker/docker-compose-bench.yml) to run against local server
In the `docker/` directory, run:
```
docker compose -f docker-compose-bench.yml up
```
You can modify [the bench worker config](../docker/config/bench/development.yaml) to run against a prod server cluster. 

Or may run it with Kubernetes, for [example](https://github.com/longquanzheng/cadence-lab/blob/master/eks/bench-deployment.yaml). 


NOTE: Similar to server/CLI images, the `master` image will be built and published automatically by Github on every commit onto the `master` branch.
To use a different image than `master` tag.  See [docker hub](https://hub.docker.com/repository/docker/ubercadence/cadence-bench) for all the images.     

#### 2.  Build & Run the binary 

In the project root, build cadence bench binary:
   ```
   make cadence-bench
   ```

Then start bench worker:
   ```
   ./cadence-bench start
   ```
By default, it will load [the configuration in `config/bench/development.yaml`](../config/bench/development.yaml). 
Run `./cadence-bench -h` for details to understand the start options of how to change the loading directory if needed. 

Worker Configurations
----------------------
Bench workers configuration contains two parts:
- **Bench**: this part controls the client side, including the bench service name, which domains bench workers are responsible for and how many taskLists each domain should use.
```yaml 
bench:
  name: "cadence-bench" # bench name
  domains: ["cadence-bench", "cadence-bench-sync", "cadence-bench-batch"] # it will start workers on all those domains(also try to register if not exists) 
  numTaskLists: 3 # it will start workers listening on cadence-bench-tl-0, cadence-bench-tl-1,  cadence-bench-tl-2
``` 
1. Bench workers will only poll from task lists whose name start with `cadence-bench-tl-`. If in the configuration, `numTaskLists` is specified to be 2, then workers will only listen to `cadence-bench-tl-0` and `cadence-bench-tl-1`. So make sure you use a valid task list name when starting the bench load.
2. When starting bench workers, it will try to register a **local domain with archival feature disabled** for each domain name listed in the configuration, if not already exists. If your want to test the performance of global domains and/or archival feature, please register the domains first before starting the worker.

- **Cadence**: this control how bench worker should talk to Cadence server, which includes the server's service name and address.
```yaml
cadence:
  service: "cadence-frontend" # frontend service name
  host: "127.0.0.1:7833" # frontend address
  #metrics: ... # optional detailed client side metrics like workflow latency  
```
- **Metrics**: metrics configuration. Similar to server metric emitter, only M3/Statsd/Prometheus is supported. 
- **Log**: logging configuration.  Similar to server logging configuration. 


Bench Load Types
-----------
This section briefly describes the purpose of each bench load and provides a sample command for running the load. Detailed descriptions for each test's configuration can be found in `bench/lib/config.go`

Please note that all load configurations in `config/bench` is for only local development and illustration purpose, it does not reflect the actual capability of Cadence server.

### Basic
:warning: NOTE: This is the only bench test which doesn't require advanced visibility feature on the server. Make sure you set `useBasicVisibilityValidation` to true if run with basic(db) visibility.  
Also basicVisibilityValidation requires only one test load run in the same domain. This is because of the limitation of basic visibility now allow using workflowType and status filters at the same time.  

As the name suggests, this load tests the basic case of load testing. 
You will start a `launchWorkflow` which will execute some `launchActivities` to start `stressWorkflows`. Then the stressWorkflows running activities in sequential/parallel.
Once all stressWorkflows are started, launchWorkflow will wait stressWorkflows timeout + buffer time(default to 5 mins) before checking the status of all test workflows. 

Two criteria must be met to pass the verification:
1. No open workflows(this means server may lose some tasks and not able to close the stressWorkflows)
2. Failed/timeouted workflows <= threshold(totalLaunchCount * failureThreshold )

The basic load can also be run in "panic" mode by setting `"panicStressWorkflow": true,` to test if server can handle large number of panic workflows (which can be caused by a bad worker deployment).

Sample configuration can be found in `config/bench/basic.json` and `config/ben/basic_panic.json`. To start the test, a sample command can be
```
cadence --do <domain> wf start --tl cadence-bench-tl-0 --wt basic-load-test-workflow --dt 30 --et 3600 --if config/bench/basic.json
```

`<domain>` needs to be one of the domains in bench config (by default  ./config/bench/development.yaml), e.g. `cadence-bench`. 

Then wait for the bench test result. 
```
$cadence --do cadence-bench wf ob -w a2813321-a1bd-40c6-934f-88ad0ded6037
Progress:
  1, 2021-08-20T11:49:14-07:00, WorkflowExecutionStarted
  2, 2021-08-20T11:49:14-07:00, DecisionTaskScheduled
...
...
  20, 2021-08-20T11:59:24-07:00, DecisionTaskStarted
  21, 2021-08-20T11:59:24-07:00, DecisionTaskCompleted
  22, 2021-08-20T11:59:24-07:00, WorkflowExecutionCompleted

Result:
  Run Time: 26 seconds
  Status: COMPLETED
  Output: "TEST PASSED. Details report: timeoutCount: 0, failedCount: 0, openCount:0, launchCount: 100, maxThreshold:1"

```

The output/error result shows whether the test passes with detailed report.

Configuration of basic load type. The config is passed as the launch workflow input parameter using a JSON file. 
 
```yaml
# configuration for launch workflow
useBasicVisibilityValidation:   use basic(db based) visibility to verify the stress workflows, default false which requires advanced visibility on the server
totalLaunchCount	: total number of stressWorkflows that started by the launchWorkflow
waitTimeBufferInSeconds : buffer time in addition of ExecutionStartToCloseTimeoutInSeconds to wait for stressWorkflows before verification, default 300(5 minutes)
routineCount	: number of in-parallel launch activities that started by launchWorkflow, to start the stressWorkflows
failureThreshold	: the threshold of failed stressWorkflow for deciding whether or not the whole testSuite failed.
maxLauncherActivityRetryCount   : the max retry on launcher activity to start stress workflows, default: 5
contextTimeoutInSeconds	: RPC timeout inside activities(e.g. starting a stressWorkflow) default 3s

# configuration for stress workflow
executionStartToCloseTimeoutInSeconds	: StartToCloseTimeout of stressWorkflow, default 5m
chainSequence	: number of steps in the stressWorkflow
concurrentCount	: number of in-parallel activity(dummy activity only echo data) in a step of the stressWorkflow
payloadSizeBytes	: payloadSize of echo data in the dummy activity
minCadenceSleepInSeconds	: control sleep time between two steps in the stressWorkflow, actual sleep time = random(min,max), default: 0
maxCadenceSleepInSeconds	: control sleep time between two steps in the stressWorkflow, actual sleep time = random(min,max), default: 0
panicStressWorkflow	: if true, stressWorkflow will always panic, default false
``` 

### Cancellation
The load tests the StartWorkflowExecution and CancelWorkflowExecution sync API, and validates the number of cancelled workflows and if there's any open workflow.

Sample configuration can be found in `config/bench/cancellation.json` and it can be started with
```
cadence --do <domain> wf start --tl cadence-bench-tl-0 --wt cancellation-load-test-workflow --dt 30 --et 3600 --if config/bench/cancellation.json 
```

### Signal
The load tests the SignalWorkflowExecution and SignalWithStartWorkflowExecution sync API, and validates the latency of signaling, the number of successfully completed workflows and if there's any open workflow.

Sample configuration can be found in `config/bench/signal.json` and it can be started with
```
cadence --do cadence-bench wf start --tl cadence-bench-tl-0 --wt signal-load-test-workflow --dt 30 --et 3600 --if config/bench/signal.json  
```


### Concurrent Execution
The purpose of this load is to test when a workflow schedules a large number of activities or child workflows in a single decision batch, whether server can properly throttle the processing of this workflow without affecting the execution of workflows in other domains. It will also check if the delayed period is within limit or not and fail the test if it takes too long.

A typical usage will be run this load and another load for testing sync APIs (for example, basic, cancellation or signal) in two different test suites/domains (so that they are run in parallel in two domains). Apply proper task processing throttling configuration to the domain that is running the concurrent execution test and see if tests in the other domain can still pass or not. 

Sample configuration can be found in `config/bench/concurrent_execution.json` and it can be started with
```
cadence --do <domain> wf start --tl cadence-bench-tl-0 --wt concurrent-execution-test-workflow --dt 30 --et 3600 --if config/bench/concurrent_execution.json
```

### Timer
This load tests if Cadence server can properly handle the case when one domain fires a large number of timers in a short period of time. Ideally timer from that domain should be throttled and delayed without affecting workflows in other domains. It will also check if the delayed period is within limit or not and fail the test if the timer latency is too high.

Typical usage is the same as the concurrent execution load above. Run it in parallel with another sync API test and see if the other test can pass or not.

Sample configuration can be found in `config/bench/timer.json` and it can be started with
```
cadence --do <domain> wf start --tl cadence-bench-tl-0 --wt timer-load-test-workflow --dt 30 --et 3600 --if config/bench/timer.json 
```

### Cron: Run all the workloads as a TestSuite

:warning: NOTE: This requires a search attribute named `Passed` as boolean type. This search attribute should have been added to the [ES schema](/schema/elasticsearch). 
make sure the dynamic config also have [this search attribute (`frontend.validSearchAttributes`)](/config/dynamicconfig/development_es.yaml), so that Cadence server can recognize it.
* Validate `Passed` has been successfully added in the dynamic config:
   ```
   cadence cluster get-search-attr
   ```
   
`Cron` itself is not a test. It is responsible for running all other tests in parallel or sequential according a cron schedule. 

Tests in `Cron` are divided to into multiple test suites. Tests in different test suites will be run in parallel, while tests within a test suite will be run in a random sequential order. Different test suites can also be run in different domains, which provides a way for testing the multi-tenant performance of Cadence server. 

On the completion of each test, `Cron` will be signaled with the result of the test, which can be queried through:
```
cadence --do <domain> wf query --wid <workflowID of the Cron workflow> --qt test-results
```
This command will show the result of all completed tests.

When all tests complete, `Cron` will update the value of the `Passed` search attribute accordingly. `Passed` will be set to `true` only when all tests have passed, and `false` otherwise. Since the last event for cron workflow is always WorkflowContinuedAsNew, this search attribute can be used to tell whether one run of `Cron` is successful or not. You can see the search attribute value by adding `--psa` flag to workflow list commands when listing `Cron` runs.

A sample cron configuration is in `config/bench/cron.json`, and it can be started with
```
cadence --do <domain> wf start --tl cadence-bench-tl-0 --wt cron-test-workflow --dt 30 --et 7200 --if config/bench/cron.json
```
