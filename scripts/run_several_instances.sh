#!/bin/bash

# This script can be used to run several instances of the different cadence services (matching, history, frontend, etc)
#

set -eo pipefail

ctrl_c() {
  echo "Killing all the services"
  pkill -9 -P $$
}

trap ctrl_c SIGINT

# Start the services used in ringpop discovery on the default ports
./cadence-server start &

# Start two more instances of the frontend service on different ports
FRONTEND_PORT=10001 FRONTEND_PORT_GRPC=10101 FRONTEND_PORT_PPROF=10201 \
  ./cadence-server --env development start --services frontend &
FRONTEND_PORT=10002 FRONTEND_PORT_GRPC=10102 FRONTEND_PORT_PPROF=10202 \
  ./cadence-server --env development start --services frontend &

# Start two more instances of the matching service on different ports
MATCHING_PORT=11001 MATCHING_PORT_GRPC=11101 MATCHING_PORT_PPROF=11201 \
  ./cadence-server --env development start --services matching &
MATCHING_PORT=11002 MATCHING_PORT_GRPC=11102 MATCHING_PORT_PPROF=11202 \
  ./cadence-server --env development start --services matching &

# Start two more instances of the history service on different ports
HISTORY_PORT=12001 HISTORY_PORT_GRPC=12101 HISTORY_PORT_PPROF=12201 \
  ./cadence-server --env development start --services history &
HISTORY_PORT=12002 HISTORY_PORT_GRPC=12102 HISTORY_PORT_PPROF=12202 \
  ./cadence-server --env development start --services history &

wait
