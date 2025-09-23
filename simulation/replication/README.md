# Replication Simulation

Tests cross-cluster replication flows, failover scenarios, and cluster redirection. Validates critical replication behavior across multiple clusters.

## Purpose

Runs replication simulator tests to validate critical replication flows. Tests complex cross-cluster replication scenarios including failover, active-active configurations, and cluster redirection.

## Quick Run

```bash
# Basic test
./simulation/replication/run.sh --scenario default

# Active-active test
./simulation/replication/run.sh --scenario activeactive

# With custom dockerfile
./simulation/replication/run.sh --scenario default --dockerfile-suffix .local

# Rerun without rebuilding
./simulation/replication/run.sh --scenario default --rerun
```

### Results

Results are output to the following files:
- **Test logs**: `test.log` contains the summary of the test run
- **Summary**: `replication-simulator-output/test-{scenario}-{timestamp}-summary.txt` contains a summary of the test run 

You can also use the [Cadence UI](http://localhost:8088) to debug the workflows that ran during your test. 

To further debug, you can query for logs against the running docker containers.

## Configuration

Scenarios are written in `testdata/replication_simulation_{scenario}.yaml`. 

To configure the cadence instances that are running for the test use a dynamic config file at `config/dynamicconfig/replication_simulation_{scenario}.yml`.
Dynamic config can change any feature flag supported by Cadence - these feature flags can be used to hide full features, or to hide test-specific implementations that expose additional data required by your test.

## Available Scenarios

- `default` - Tests workflow behaviour during a failover
- `activeactive` - Tests that active-active allows workflows to be created in multiple clusters
- `activeactive_cron` - Active-active with cron workflows
- `activeactive_regional_failover` - Tests active-active behaviour during a failover
- `activeactive_regional_failover_start_same_wfid` - Tests that the same wfid cannot be created in multiple regions
- `activeactive_regional_failover_start_same_wfid_2` - Variant of same workflow ID test
- `activepassive_to_activeactive` - Validates behaviour of workflows in a domain converted from active-passive to active-active
- `clusterredirection` - Ensures redirection of RPCs behaves as expected
- `reset` - Tests the behaviour of workflows when failovers and reset commands 

## CI/CD

Scenarios run automatically in GitHub Actions via `.github/workflows/replication-simulation.yml`. Add new scenarios to the matrix once they pass locally.
