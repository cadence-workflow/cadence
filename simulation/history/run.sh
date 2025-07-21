#!/bin/bash

# This script can be used to run history simulator and check the critical flow via logs
#

set -eo pipefail

testCase="${1:-default}"
testCfg="testdata/history_simulation_$testCase.yaml"
now="$(date '+%Y-%m-%d-%H-%M-%S')"
timestamp="${2:-$now}"
testName="test-$testCase-$timestamp"
resultFolder="history-simulator-output"
mkdir -p "$resultFolder"
eventLogsFile="$resultFolder/$testName-events.json"
testSummaryFile="$resultFolder/$testName-summary.txt"

echo "Building test image"
DOCKERFILE_SUFFIX=$DOCKERFILE_SUFFIX docker compose -f docker/buildkite/docker-compose-local-history-simulation.yml \
  build history-simulator

function check_test_failure()
{
  faillog=$(grep 'FAIL: TestHistorySimulationSuite' -B 10 test.log 2>/dev/null || true)
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
DOCKERFILE_SUFFIX=$DOCKERFILE_SUFFIX docker compose \
  -f docker/buildkite/docker-compose-local-history-simulation.yml \
  run -e HISTORY_SIMULATION_CONFIG=$testCfg --rm --remove-orphans --service-ports --use-aliases \
  history-simulator \
  | grep -a --line-buffered "History New Event" \
  | sed "s/History New Event: //" \
  | jq . > "$eventLogsFile"

echo "---- Simulation Summary ----"

format_counts() {
  event_name="$1"
  header="$2"
  echo "$header:"

  jq -r --arg event "$event_name" '
    select(.EventName == $event) |
    "\(.ShardID)|\(.Payload.task_category)|\(.Payload.task_type)"
  ' "$eventLogsFile" | sort | uniq -c | sort -k2,2n -k3,3 -k4,4 | awk '
  BEGIN { last_shard = "__unset__" }
  {
    count = $1
    sub(/^[ \t]*[0-9]+[ \t]*/, "", $0)
    split($0, parts, "|")
    shard = parts[1]
    category = parts[2]
    type = parts[3]

    if (shard != last_shard) {
      print "ShardID: " shard
      last_shard = shard
    }
    printf "  TaskCategory: %s, TaskType: %s, Count: %d\n", category, type, count
  }'

  echo ""
}

format_counts "Create History Task" "Tasks created" | tee -a "$testSummaryFile"
format_counts "Execute History Task" "Tasks executed" | tee -a "$testSummaryFile"

create_tasks=$(jq -r '
  select(.EventName == "Create History Task") |
    "\(.ShardID)|\(.Payload.task_category)|\(.Payload.task_type)|\(.Payload.task_key.taskID)|\(.Payload.task_key.scheduledTime)"' "$eventLogsFile"
)

execute_tasks=$(jq -r '
  select(.EventName == "Execute History Task") |
    "\(.ShardID)|\(.Payload.task_category)|\(.Payload.task_type)|\(.Payload.task_key.taskID)|\(.Payload.task_key.scheduledTime)"' "$eventLogsFile"
)

missing=$(echo "$create_tasks" | while IFS= read -r line; do
  if ! echo "$execute_tasks" | grep -Fxq "$line"; then
    echo "$line"
  fi
done)

echo "Tasks that were created but not executed:" | tee -a "$testSummaryFile"
# Group and print nicely
echo "$missing" | sort -t '|' -k1,1n -k2,2 -k3,3n -k4,4n | awk -F'|' '
BEGIN { last_key = "__none__"; last_shard = "__none__"}
{
  shard = $1
  category = $2
  type = $3
  taskid = $4
  sched = $5

  key = shard "|" category "|" type
  if (key != last_key) {
    if (shard != last_shard) {
      print "ShardID: " shard
      last_shard = shard
    }
    print "  TaskCategory: " category ", TaskType: " type
    last_key = key
  }
  print "    TaskID: " taskid ", ScheduledTime: " sched
}' | tee -a "$testSummaryFile"


printf "\nResults are saved in %s\n" "$testSummaryFile"
printf "For further ad-hoc analysis, please check %s via jq queries\n" "$eventLogsFile"
printf "Visit http://localhost:3000/ to view Cadence History grafana dashboard\n"
