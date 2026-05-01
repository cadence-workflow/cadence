# Simulation Tests

Simulation tests are black box tests that validate complex multi-component scenarios against a Cadence cluster. They enable systematic testing of workflows that would otherwise require manual sanity checks.

## Overview

Each suite of simulation tests:
- instantiates a local docker cluster
- makes requests to the local cluster, based on the test configuration
- analyses the results

### Test Types

- **replication** - Tests the edge-case behaviour of workflows replicated to multiple clusters, e.g during failover, replication lag, or in active-active domains. 
- **matching** - Tests the performance of the cadence-matching service.
- **history** - Tests the behaviour of the history service.

See individual subdirectory READMEs for specific test details.

## Running Simulations

Running the tests requires that the developer has 
- [setup their development environment](../CONTRIBUTING.md#development-environment)
- has docker running

### Quick Start

```bash
# Run a basic replication test
./simulation/replication/run.sh --scenario default

# Run a matching performance test
./simulation/matching/run.sh --scenario throughput

# Run a history analysis test
./simulation/history/run.sh --scenario default
```

### Local Development

Some contributors may require a custom dockerfile to be passed to allow the simulation tests to build locally.
Add your dockerfile to `docker/github_actions/` with a custom suffix, and then pass the `--dockerfile-suffix` paramater to use it:

```bash
# For a file named Dockerfile.local in docker/github_actions:
./simulation/replication/run.sh --scenario default --dockerfile-suffix .local
```

## Adding New Simulations

Any time you are writing code that is complex, touches multiple components, or requires manual testing to sanity check its behaviour it is a candidate for a simulation test. To add a new simulation:

1. Create scenario file: `{type}/testdata/{type}_simulation_{name}.yaml`
2. Add config overrides: `config/dynamicconfig/{type}_simulation_{name}.yml` (if needed)
3. Test locally: `./simulation/{type}/run.sh --scenario {name}`
4. Add to CI matrix in `.github/workflows/{type}-simulation.yml`

### CI/CD

Note: only replication simulations run in GitHub Actions currently. To add a new replication scenario to CI, update the matrix in `.github/workflows/replication-simulation.yml`.
