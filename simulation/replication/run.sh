#!/bin/bash

# Cadence Replication Simulation Test Script
#
# Usage:
#   ./simulation/replication/run.sh [OPTIONS]
#
# Examples:
#   # Run default scenario
#   ./simulation/replication/run.sh --scenario default
#
#   # Run specific scenario with custom dockerfile
#   ./simulation/replication/run.sh --scenario activeactive --dockerfile-suffix .local
#
#   # Rerun without rebuilding images
#   ./simulation/replication/run.sh --scenario default --rerun
#
#   # Run with custom timestamp
#   ./simulation/replication/run.sh --scenario activeactive --timestamp 2024-01-15-10-30-00
#
# See simulation/replication/README.md for scenario details and output information.

set -eo pipefail

show_help() {
    cat << EOF
Cadence Replication Simulation Test Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -s, --scenario SCENARIO          Test scenario to run (required)
                                   Corresponds to testdata/replication_simulation_SCENARIO.yaml

    -r, --rerun                     Skip rebuilding images and reuse existing containers
                                   Useful for faster reruns during development

    -t, --timestamp TIMESTAMP       Custom timestamp for test naming (default: current time)
                                   Format: YYYY-MM-DD-HH-MM-SS

    -d, --dockerfile-suffix SUFFIX  Dockerfile suffix for custom builds (default: empty)
                                   Example: .local for Dockerfile.local

    -h, --help                      Show this help message

EXAMPLES:
    # Run default scenario
    $0 --scenario default

    # Run specific scenario with custom dockerfile
    $0 --scenario activeactive --dockerfile-suffix .local

    # Rerun without rebuilding images
    $0 --scenario default --rerun

    # Run with all options
    $0 --scenario activeactive --rerun --timestamp 2024-01-15-10-30-00 --dockerfile-suffix .local

EOF
}

# Default values
testCase=""
rerun=""
timestamp=""
dockerFileSuffix=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -s|--scenario)
            testCase="$2"
            shift 2
            ;;
        -r|--rerun)
            rerun="rerun"
            shift
            ;;
        -t|--timestamp)
            timestamp="$2"
            shift 2
            ;;
        -d|--dockerfile-suffix)
            dockerFileSuffix="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        --)
            shift
            break
            ;;
        -*)
            echo "Unknown option: $1" >&2
            echo "Use --help for usage information." >&2
            exit 1
            ;;
        *)
            echo "Unexpected positional argument: $1" >&2
            echo "Use --scenario to specify the test scenario." >&2
            echo "Use --help for usage information." >&2
            exit 1
            ;;
    esac
done

# Require scenario parameter
if [[ -z "$testCase" ]]; then
    echo "Error: --scenario parameter is required" >&2
    echo "" >&2
    show_help
    exit 1
fi

# Set default timestamp if not provided
if [[ -z "$timestamp" ]]; then
    timestamp="$(date '+%Y-%m-%d-%H-%M-%S')"
fi

testCfg="testdata/replication_simulation_$testCase.yaml"
testName="test-$testCase-$timestamp"
resultFolder="replication-simulator-output"
mkdir -p "$resultFolder"
testSummaryFile="$resultFolder/$testName-summary.txt"

# Prune everything and rebuild images unless rerun is specified
if [ "$rerun" != "rerun" ]; then
  echo "Removing some of the previous containers (if exists) to start fresh"
  SCENARIO=$testCase DOCKERFILE_SUFFIX=$dockerFileSuffix docker compose -f docker/github_actions/docker-compose-local-replication-simulation.yml \
    down cassandra cadence-cluster0 cadence-cluster1 cadence-worker0 cadence-worker1 replication-simulator

  echo "Each simulation run creates multiple new giant container images. Running docker system prune to avoid disk space issues"
  docker system prune -f

  echo "Building test images"
  SCENARIO=$testCase DOCKERFILE_SUFFIX=$dockerFileSuffix docker compose -f docker/github_actions/docker-compose-local-replication-simulation.yml \
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
  -f docker/github_actions/docker-compose-local-replication-simulation.yml \
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
