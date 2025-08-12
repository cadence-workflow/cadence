# Replication Simulation

This folder contains the replication simulation scenarios.

## Running the simulation

```bash
./simulation/replication/run.sh <scenario>
```

If you have a custom dockerfile such as Dockerfile.local, you can pass it as an argument:

```bash
./simulation/replication/run.sh <scenario> newrun "" ".local"
```

After running the simulation, you can find the test logs in `test.log` file.
Also check the Cadence UI on localhost:8088 to see the workflow executions.

## Scenario descriptions

- `activeactive`: Active-active basic checks
- `activeactive_cron`: Active-active with cron workflows
- `activeactive_regional_failover`: Active-active with regional failover
- `activepassive_to_activeactive`: Active-passive to active-active migration
- `clusterredirection`: Cluster redirection checks
- `default`: Default scenario
- `reset`: Reset scenario

## Github Actions

Github Actions are used to run the simulation scenarios as part of the CI/CD pipeline.

- `.github/workflows/replication-simulation.yml`: Runs the simulation scenarios via GithubActions matrix strategy.
