#!/bin/bash

set -ex

# This script can be used to locally build all the images. Do NOT push these images. Dockerhub registry should only be updated via github workflow.

docker build . -f Dockerfile -t ubercadence/server:master --build-arg TARGET=server
docker build . -f Dockerfile -t ubercadence/server:master-auto-setup --build-arg TARGET=auto-setup
docker build . -f Dockerfile -t ubercadence/cli:master --build-arg TARGET=cli
docker build . -f Dockerfile -t ubercadence/cadence-bench:master --build-arg TARGET=bench
docker build . -f Dockerfile -t ubercadence/cadence-canary:master --build-arg TARGET=canary
