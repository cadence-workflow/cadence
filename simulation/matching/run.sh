#!/bin/bash

# Cadence Matching Simulation Test Script
#
# Usage:
#   ./simulation/matching/run.sh [OPTIONS]
#
# Examples:
#   # Run default scenario
#   ./simulation/matching/run.sh --scenario default
#
#   # Run specific scenario
#   ./simulation/matching/run.sh --scenario throughput
#
#   # Run with custom timestamp
#   ./simulation/matching/run.sh --scenario default --timestamp 2024-01-15-10-30-00
#
#   # Run with custom dockerfile
#   ./simulation/matching/run.sh --scenario throughput --dockerfile-suffix .local
#
# TODO: Add README with more information on how to analyse the test results, as well as more detailed information on the comparison tests.  

set -eo pipefail

show_help() {
    cat << EOF
Cadence Matching Simulation Test Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -s, --scenario SCENARIO      Test scenario to run (required)
                                Corresponds to testdata/matching_simulation_SCENARIO.yaml

    -t, --timestamp TIMESTAMP   Custom timestamp for test naming (default: current time)
                                Format: YYYY-MM-DD-HH-MM-SS

    -d, --dockerfile-suffix SUFFIX  Dockerfile suffix for custom builds (default: empty)
                                   Example: .local for Dockerfile.local

    -h, --help                  Show this help message

EXAMPLES:
    # Run default scenario
    $0 --scenario default

    # Run specific scenario
    $0 --scenario throughput

    # Run with custom timestamp
    $0 --scenario default --timestamp 2024-01-15-10-30-00

    # Run with custom dockerfile
    $0 --scenario throughput --dockerfile-suffix .local

EOF
}

# Default values
testCase=""
timestamp=""
dockerFileSuffix=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -s|--scenario)
            testCase="$2"
            shift 2
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

testCfg="testdata/matching_simulation_$testCase.yaml"
testName="test-$testCase-$timestamp"
resultFolder="matching-simulator-output"
mkdir -p "$resultFolder"
eventLogsFile="$resultFolder/$testName-events.json"
testSummaryFile="$resultFolder/$testName-summary.txt"

echo "Building test image"
DOCKERFILE_SUFFIX=$dockerFileSuffix docker compose -f docker/github_actions/docker-compose-local-matching-simulation.yml \
  build matching-simulator

function check_test_failure()
{
  faillog=$(grep 'FAIL: TestMatchingSimulationSuite' -B 10 test.log 2>/dev/null || true)
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

echo "Running the test $testCase"
DOCKERFILE_SUFFIX=$dockerFileSuffix docker compose \
  -f docker/github_actions/docker-compose-local-matching-simulation.yml \
  run -e MATCHING_SIMULATION_CONFIG=$testCfg --rm --remove-orphans --service-ports --use-aliases \
  matching-simulator \
  | grep -a --line-buffered "Matching New Event" \
  | sed "s/Matching New Event: //" \
  | jq . > "$eventLogsFile"

echo "---- Simulation Summary ----"
cat test.log \
  | sed -n '/Simulation Summary/,/End of Simulation Summary/p' \
  | grep -v "Simulation Summary" \
  | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq .Payload.Latency | awk '{s+=$0}END{print s/NR}')
echo "Avg Task latency (ms): $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq .Payload.Latency | sort -n | awk '{a[NR]=$0}END{print a[int(NR*0.50)]}')
echo "P50 Task latency (ms): $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq .Payload.Latency | sort -n | awk '{a[NR]=$0}END{print a[int(NR*0.75)]}')
echo "P75 Task latency (ms): $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq .Payload.Latency | sort -n | awk '{a[NR]=$0}END{print a[int(NR*0.95)]}')
echo "P95 Task latency (ms): $tmp" | tee -a $testSummaryFile


tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq .Payload.Latency | sort -n | awk '{a[NR]=$0}END{print a[int(NR*0.99)]}')
echo "P99 Task latency (ms): $tmp" | tee -a $testSummaryFile


tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq .Payload.Latency | sort -n | tail -n 1)
echo "Max Task latency (ms): $tmp" | tee -a $testSummaryFile


tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq -s 'map(if .Payload.IsolationGroup == .PartitionConfig."original-isolation-group" then 1 else 0 end) | add / length')
echo "Task Containment: $tmp" | tee -a $testSummaryFile


tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Worker Polls that returned a task: $tmp" | tee -a $testSummaryFile


tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task" and .Payload.TaskIsForwarded == true)' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Worker Polls that returned a forwarded task: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returned no tasks")' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Worker Polls that returned NO task: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Matcher Falling Back to Non-Local Polling")' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Worker Polls that falled back to non-local polling: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Attempting to Forward Poll")' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Poll forward attempts: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Forwarded Poll returned task")' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Forwarded poll returned task: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Task Written to DB")' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Tasks Written to DB: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Attempting to Forward Task")' \
  | jq '{ScheduleID,TaskListName,EventName,Payload}' \
  | jq -c '.' | wc -l)
echo "Task forward attempts: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName | contains("Matched Task"))' \
  | jq -c 'select((.Payload.SyncMatched == true) and (.Payload.TaskIsForwarded == true))' \
  | jq '{ScheduleID,TaskListName}' \
  | jq -c '.' | wc -l)
echo "Sync matches - task is forwarded: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName | contains("Matched Task"))' \
  | jq -c 'select((.Payload.SyncMatched == true) and (.Payload.TaskIsForwarded == false))' \
  | jq '{ScheduleID,TaskListName}' \
  | jq -c '.' | wc -l)
echo "Sync matches - task is not forwarded: $tmp" | tee -a $testSummaryFile


echo "Per tasklist sync matches:" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "SyncMatched so not persisted")' \
  | jq '.TaskListName' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile


tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Could not SyncMatched Forwarded Task so not persisted")' \
  | jq '{ScheduleID,TaskListName}' \
  | jq -c '.' | wc -l)
echo "Forwarded Task failed to sync match: $tmp" | tee -a $testSummaryFile

tmp=$(cat "$eventLogsFile" \
  | jq -c 'select(.EventName | contains("Matched Task"))' \
  | jq -c 'select(.Payload.SyncMatched != true)' \
  | jq '{ScheduleID,TaskListName,Payload}' \
  | jq -c '.' | wc -l)
echo "Async matches: $tmp" | tee -a $testSummaryFile

echo "Matched tasks per tasklist:" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName | contains("Matched Task"))' \
  | jq '.TaskListName' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "AddDecisionTask request per tasklist (excluding forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received AddDecisionTask" and .Payload.RequestForwardedFrom == "")' \
  | jq '.TaskListName' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "AddDecisionTask request per tasklist (forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received AddDecisionTask" and .Payload.RequestForwardedFrom != "")' \
  | jq '.TaskListName' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile


echo "PollForDecisionTask request per tasklist (excluding forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received PollForDecisionTask" and .Payload.RequestForwardedFrom == "")' \
  | jq '.TaskListName' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile


echo "PollForDecisionTask request per tasklist (forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received PollForDecisionTask" and .Payload.RequestForwardedFrom != "")' \
  | jq '.TaskListName' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "AddDecisionTask request per isolation group (excluding forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received AddDecisionTask" and .Payload.RequestForwardedFrom == "")' \
  | jq '.PartitionConfig."original-isolation-group" // .PartitionConfig."isolation-group"' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "AddDecisionTask request per isolation group (forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received AddDecisionTask" and .Payload.RequestForwardedFrom != "")' \
  | jq '.PartitionConfig."original-isolation-group" // .PartitionConfig."isolation-group"' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "PollForDecisionTask request per isolation group  (excluding forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received PollForDecisionTask" and .Payload.RequestForwardedFrom == "")' \
  | jq '.Payload.IsolationGroup' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "PollForDecisionTask request per isolation group  (forwarded):" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "Received PollForDecisionTask" and .Payload.RequestForwardedFrom != "")' \
  | jq '.Payload.IsolationGroup' \
  | jq -c '.' | sort -n | uniq -c | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "Latency per isolation group:" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq -s 'group_by(.PartitionConfig."original-isolation-group")[] | {"IsolationGroup": .[0].PartitionConfig."original-isolation-group",
   "Avg": (map(.Payload.Latency | tonumber) | add / length), "Median": (map(.Payload.Latency | tonumber) | sort | .[length/2]), "Max":(map(.Payload.Latency | tonumber) | max) }'\
  | jq -s 'sort_by(.IsolationGroup)[]'\
  | jq -r '[.IsolationGroup, .Median, .Avg, .Max] | @tsv' \
  | sort -n | column -t  | sed -e 's/^/     /' | tee -a $testSummaryFile


echo "Latency per isolation group and task list:" | tee -a $testSummaryFile
cat "$eventLogsFile" \
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq -s 'group_by(.PartitionConfig."original-isolation-group", .TaskListName)[] | {"IsolationGroup": .[0].PartitionConfig."original-isolation-group", "TaskListName": .[0].TaskListName,
   "Avg": (map(.Payload.Latency | tonumber) | add / length), "Median": (map(.Payload.Latency | tonumber) | sort | .[length/2]), "Max":(map(.Payload.Latency | tonumber) | max) }'\
  | jq -s 'sort_by(.TaskListName, .IsolationGroup)[]'\
  | jq -r '[.TaskListName, .IsolationGroup, .Median, .Avg, .Max] | @tsv' \
  | sort -n | column -t  | sed -e 's/^/     /' | tee -a $testSummaryFile

echo "Task Containment per isolation group and task list:" | tee -a $testSummaryFile
cat "$eventLogsFile"\
  | jq -c 'select(.EventName == "PollForDecisionTask returning task")' \
  | jq -s 'group_by(.PartitionConfig."original-isolation-group", (.Payload.RequestForwardedFrom // .TaskListName))[] | {"IsolationGroup": .[0].PartitionConfig."original-isolation-group", "TaskListName": (.[0].Payload.RequestForwardedFrom // .[0].TaskListName),
   "Containment": (map(if .Payload.IsolationGroup == .PartitionConfig."original-isolation-group" then 1 else 0 end) | add / length) }'\
  | jq -s 'sort_by(.TaskListName, .IsolationGroup)[]'\
  | jq -r '[.TaskListName, .IsolationGroup, .Containment] | @tsv' \
  | sort -n | column -t  | sed -e 's/^/     /' | tee -a $testSummaryFile


echo "End of summary" | tee -a $testSummaryFile

printf "\nResults are saved in $testSummaryFile\n"
printf "For further ad-hoc analysis, please check $eventLogsFile via jq queries\n"
printf "Visit http://localhost:3000/ to view Cadence Matching grafana dashboard\n"
