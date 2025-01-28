#!/bin/bash

# This script can be used to run replication simulator and check the critical flow via logs
# Scenario specs are located at host/testdata/replication_simulation_${scenario}.yaml
# Dynamic configs are located at config/dynamicconfig/replication_simulation_${scenario}.yml
# The output of the simulation is saved in replication-simulator-output/ folder

# Usage: ./scripts/run_replication_simulator.sh <scenario>

set -eo pipefail

testCase="${1:-default}"
testCfg="testdata/replication_simulation_$testCase.yaml"
now="$(date '+%Y-%m-%d-%H-%M-%S')"
timestamp="${2:-$now}"
testName="test-$testCase-$timestamp"
resultFolder="replication-simulator-output"
mkdir -p "$resultFolder"
eventLogsFile="$resultFolder/$testName-events.json"
testSummaryFile="$resultFolder/$testName-summary.txt"

echo "Removing some of the previous containers (if exists) to start fresh"
docker-compose -f docker/buildkite/docker-compose-local-replication-simulation.yml \
  down cassandra cadence-cluster0 cadence-cluster1 replication-simulator

echo "Each simulation run creates multiple new giant container images. Running docker system prune to avoid disk space issues"
docker system prune -f

echo "Building test images"
docker-compose -f docker/buildkite/docker-compose-local-replication-simulation.yml \
  build cadence-cluster0 cadence-cluster1 replication-simulator

function check_test_failure()
{
  faillog=$(grep 'FAIL: TestReplicationSimulation' -B 10 test.log 2>/dev/null || true)
  if [ -z "$faillog" ]; then
    echo "Passed"
  else
    echo 'Test failed!!!'
    echo "$faillog"
    echo "Check test.log file for more details"
    exit 1
  fi
}

trap check_test_failure EXIT

echo "Running the scenario $testCase"
docker-compose \
  -f docker/buildkite/docker-compose-local-replication-simulation.yml \
  run \
  -e REPLICATION_SIMULATION_CONFIG=$testCfg \
  -e SCENARIO=$testCase \
  --rm --remove-orphans --service-ports --use-aliases \
  replication-simulator \
  | grep -a --line-buffered "Replication New Event" \
  | sed "s/Replication New Event: //" \
  | jq . > "$eventLogsFile"


echo "---- Simulation Summary ----"
cat test.log \
  | sed -n '/Simulation Summary/,/End of Simulation Summary/p' \
  | grep -v "Simulation Summary" \
  | tee -a $testSummaryFile

echo "End of summary" | tee -a $testSummaryFile

printf "\nResults are saved in $testSummaryFile\n"
printf "For further ad-hoc analysis, please check $eventLogsFile via jq queries\n"
printf "Visit http://localhost:3000/ to view Cadence replication grafana dashboard\n"
