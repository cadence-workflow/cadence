#!/bin/bash

set -euo pipefail

readonly compose_file="docker/docker-compose-sqlite.yml"
readonly domain="sqlite-auto-setup-test"

if docker compose version >/dev/null 2>&1; then
    compose=(docker compose)
else
    compose=(docker-compose)
fi

cleanup() {
    "${compose[@]}" -f "$compose_file" down --volumes
}
trap cleanup EXIT

up_arguments=(--detach)
if [ -z "${CADENCE_IMAGE:-}" ]; then
    up_arguments=(--build "${up_arguments[@]}")
fi
"${compose[@]}" -f "$compose_file" up "${up_arguments[@]}"

ready=false
for _ in $(seq 1 90); do
    if "${compose[@]}" -f "$compose_file" exec -T cadence \
        cadence --address cadence:7933 --context_timeout 5 \
        admin domain list >/dev/null 2>&1; then
        ready=true
        break
    fi
    sleep 2
done

if [ "$ready" != true ]; then
    echo "Cadence did not become ready."
    "${compose[@]}" -f "$compose_file" logs cadence
    exit 1
fi

"${compose[@]}" -f "$compose_file" exec -T cadence \
    test -s /tmp/cadence.db
"${compose[@]}" -f "$compose_file" exec -T cadence \
    test -s /tmp/cadence_visibility.db

"${compose[@]}" -f "$compose_file" exec -T cadence \
    cadence --address cadence:7933 --context_timeout 5 admin domain list
"${compose[@]}" -f "$compose_file" exec -T cadence \
    cadence --address cadence:7933 --context_timeout 5 \
    --domain "$domain" domain register --global_domain false
"${compose[@]}" -f "$compose_file" exec -T cadence \
    cadence --address cadence:7933 --context_timeout 5 \
    --domain "$domain" domain describe
