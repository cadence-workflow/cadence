# Replication Simulation

This folder contains the replication simulation framework and scenarios.


## Scenario descriptions

- `activeactive`: Active-active basic checks
- `activeactive_cron`: Active-active with cron workflows
- `activeactive_regional_failover`: Active-active with regional failover
- `activepassive_to_activeactive`: Active-passive to active-active migration
- `clusterredirection`: Cluster redirection checks
- `default`: Default scenario
- `reset`: Reset scenario

## Conventions

Scenario files are placed in `testdata/replication_simulation_<scenario>.yaml`.
Scenario file is a YAML file that contains the simulation configuration that is parsed into `ReplicationSimulationConfig` struct.
There should be a corresponding `config/dynamicconfig/replication_simulation_<scenario>.yml` file that contains the dynamic config overrides.

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


## Github Actions

Github Actions are used to run the simulation scenarios as part of the CI/CD pipeline.
Configuration is in `.github/workflows/replication-simulation.yml`.
If you add a new scenario (following the conventions above), also add it to the Github Actions matrix in `.github/workflows/replication-simulation.yml` once the test is passing.
