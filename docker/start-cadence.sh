#!/bin/bash

set -ex

# If CADENCE_CONFIG_FILE is set, copy it to docker.yaml and use the config directory
# Otherwise, use the template to generate docker.yaml
if [ -n "$CADENCE_CONFIG_FILE" ]; then
    echo "Using custom config file: $CADENCE_CONFIG_FILE"
    # Copy custom config to docker.yaml so cadence-server can find it
    cp "$CADENCE_CONFIG_FILE" /etc/cadence/config/docker.yaml
else
    echo "Generating config from template"
    CONFIG_TEMPLATE_PATH="${CONFIG_TEMPLATE_PATH:-/etc/cadence/config_template.yaml}"
    dockerize -template "$CONFIG_TEMPLATE_PATH:/etc/cadence/config/docker.yaml"
fi

exec cadence-server --root "$CADENCE_HOME" --env docker --config config start --services="$SERVICES"

