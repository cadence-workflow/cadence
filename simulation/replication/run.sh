#!/bin/bash

# This script can be used to run replication simulator and check the critical flow via logs
# Scenario specs are located at host/testdata/replication_simulation_${scenario}.yaml
# Dynamic configs are located at config/dynamicconfig/replication_simulation_${scenario}.yml
# The output of the simulation is saved in replication-simulator-output/ folder

# Usage: ./simulation/replication/run.sh <scenario>
# Example command that runs default scenario with a custom dockerfile:
#   ./simulation/replication/run.sh default "" "" ".local"

set -eo pipefail

testCase="${1:-default}"
testCfg="testdata/replication_simulation_$testCase.yaml"
now="$(date '+%Y-%m-%d-%H-%M-%S')"
rerun="${2:-}"
timestamp="${3:-$now}"
dockerFileSuffix="${4:-}"
testName="test-$testCase-$timestamp"
resultFolder="replication-simulator-output"
mkdir -p "$resultFolder"
testSummaryFile="$resultFolder/$testName-summary.txt"

# Prune everything and rebuild images unless rerun is specified
if [ "$rerun" != "rerun" ]; then
  echo "Removing some of the previous containers (if exists) to start fresh"
  SCENARIO=$testCase DOCKERFILE_SUFFIX=$dockerFileSuffix docker compose -f docker/buildkite/docker-compose-local-replication-simulation.yml \
    down cassandra cadence-cluster0 cadence-cluster1 cadence-worker0 cadence-worker1 replication-simulator

  echo "Each simulation run creates multiple new giant container images. Running docker system prune to avoid disk space issues"
  docker system prune -f

  echo "Building test images"
  SCENARIO=$testCase DOCKERFILE_SUFFIX=$dockerFileSuffix docker compose -f docker/buildkite/docker-compose-local-replication-simulation.yml \
    build cadence-cluster0 cadence-cluster1 cadence-worker0 cadence-worker1 replication-simulator
fi

function check_test_failure()
{
  faillog=$(grep 'FAIL: TestReplicationSimulation' -B 10 test.log 2>/dev/null || true)
  timeoutlog=$(grep 'test timed out' test.log 2>/dev/null || true)
  if [ -z "$faillog" ] && [ -z "$timeoutlog" ]; then
    echo "Passed"
  else
    echo 'Test failed!!!'
    echo "Fail log: $faillog"
    echo "Timeout log: $timeoutlog"
    echo "Check test.log file for more details"
    exit 1
  fi
}

trap check_test_failure EXIT

echo "Running the scenario '$testCase' with dockerfile suffix: '$dockerFileSuffix'"
echo "Test name: $testName"

SCENARIO=$testCase DOCKERFILE_SUFFIX=$dockerFileSuffix docker compose \
  -f docker/buildkite/docker-compose-local-replication-simulation.yml \
  run \
  -e REPLICATION_SIMULATION_CONFIG=$testCfg \
  --rm --remove-orphans --service-ports --use-aliases \
  replication-simulator


echo "---- Simulation Summary ----"
cat test.log \
  | sed -n '/Simulation Summary/,/End of Simulation Summary/p' \
  | grep -v "Simulation Summary" \
  | tee -a $testSummaryFile

echo "End of summary" | tee -a $testSummaryFile

printf "\nResults are saved in $testSummaryFile\n"
printf "For further ad-hoc analysis, please check $eventLogsFile via jq queries\n"
printf "Visit http://localhost:3000/ to view Cadence replication grafana dashboard\n"
