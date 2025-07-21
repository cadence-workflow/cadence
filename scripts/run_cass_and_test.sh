#!/bin/bash

set -eo pipefail

function finish {
  echo "shut down containers"
  docker compose -f docker/docker-compose.yml down;
}
trap finish EXIT

# shut down containers
docker compose -f docker/docker-compose.yml down;

# run cassandra & cadence container
# we need cadence here because it handles db setup
docker compose -f docker/docker-compose.yml up -d cassandra cadence;

status=""
while [[ "$status" != "running" ]]; do
  echo "waiting for containers to be healthy. status: $status"
  sleep 5
  # checking cadence container is running is sufficient because it depends on health status of cassandra in docker-compose.yml
  status="$(docker inspect docker-cadence-1 -f '{{ .State.Status }}')"
done;

echo "containers are healthy. sleeping for 10 seconds so cadence container can finish db setup"
sleep 10
echo "running tests"

# run the tests with cassandra
CASSANDRA=1 make test | tee test.log;
