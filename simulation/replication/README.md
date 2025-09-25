# Replication Simulation

These simulations test replication of workflows across clusters. As replication is handled at the application layer there are many edge case scenarios that lead to conflict resolution when:

- there is a delay in replication
- a domain is failed over from one cluster to another
- active-active is enabled and a customer is making requests to both clusters

There are also sanity checks for the request routing layer. 

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
This naming convention is required by `run.sh` to find test scenarios to run. 

To configure the cadence instances that are running for the test use a dynamic config file at `config/dynamicconfig/replication_simulation_{scenario}.yml`.
Dynamic config can change any feature flag supported by Cadence - these feature flags can be used to hide full features, or to hide test-specific implementations that expose additional data required by your test.

## CI/CD

Scenarios run automatically in GitHub Actions via `.github/workflows/replication-simulation.yml`. Add new scenarios to the matrix once they pass locally.
