#!/bin/bash

set -ex

# Default values
CASSANDRA_SEEDS="${CASSANDRA_SEEDS:-cassandra}"
CASSANDRA_USER="${CASSANDRA_USER:-cassandra}"
CASSANDRA_PASSWORD="${CASSANDRA_PASSWORD:-cassandra}"
CASSANDRA_PROTO_VERSION="${CASSANDRA_PROTO_VERSION:-4}"
REPLICATION_FACTOR="${RF:-1}"

# Keyspaces for multiclusters setup
PRIMARY_KEYSPACE="${PRIMARY_KEYSPACE:-cadence_primary}"
PRIMARY_VISIBILITY_KEYSPACE="${PRIMARY_VISIBILITY_KEYSPACE:-cadence_visibility_primary}"
SECONDARY_KEYSPACE="${SECONDARY_KEYSPACE:-cadence_secondary}"
SECONDARY_VISIBILITY_KEYSPACE="${SECONDARY_VISIBILITY_KEYSPACE:-cadence_visibility_secondary}"

# Schema directories
SCHEMA_DIR="/etc/cadence/schema/cassandra/cadence/versioned"
VISIBILITY_SCHEMA_DIR="/etc/cadence/schema/cassandra/visibility/versioned"

wait_for_cassandra() {
    echo "Waiting for Cassandra to be ready..."
    server=$(echo $CASSANDRA_SEEDS | awk -F ',' '{print $1}')
    
    # Wait for Cassandra to be fully ready with timeout
    echo "Waiting for Cassandra to be fully ready..."
    timeout=300  # 5 minutes timeout
    counter=0
    
    while [[ $counter -lt $timeout ]]; do
        if cadence-cassandra-tool --ep $server --u $CASSANDRA_USER --pw $CASSANDRA_PASSWORD --pv $CASSANDRA_PROTO_VERSION create -k test_keyspace --rf 1 2>/dev/null; then
            echo 'cassandra started and ready'
            return 0
        fi
        echo "waiting for cassandra to be fully ready (attempt $((counter/5 + 1)))"
        sleep 5
        counter=$((counter + 5))
    done
    
    echo "Error: Cassandra did not become ready within $timeout seconds"
    exit 1
}

setup_keyspace_schema() {
    local keyspace=$1
    local visibility_keyspace=$2
    
    echo "Setting up schema for keyspace: $keyspace"
    
    # Create keyspace
    cadence-cassandra-tool --ep $CASSANDRA_SEEDS create -k $keyspace --rf $REPLICATION_FACTOR
    
    # Setup schema versioning
    cadence-cassandra-tool --ep $CASSANDRA_SEEDS -k $keyspace setup-schema -v 0.0
    
    # Update to latest schema
    cadence-cassandra-tool --ep $CASSANDRA_SEEDS -k $keyspace update-schema -d $SCHEMA_DIR
    
    echo "Setting up visibility schema for keyspace: $visibility_keyspace"
    
    # Create visibility keyspace
    cadence-cassandra-tool --ep $CASSANDRA_SEEDS create -k $visibility_keyspace --rf $REPLICATION_FACTOR
    
    # Setup visibility schema versioning
    cadence-cassandra-tool --ep $CASSANDRA_SEEDS -k $visibility_keyspace setup-schema -v 0.0
    
    # Update visibility to latest schema
    cadence-cassandra-tool --ep $CASSANDRA_SEEDS -k $visibility_keyspace update-schema -d $VISIBILITY_SCHEMA_DIR
}

setup_primary_cluster() {
    echo "Setting up primary cluster schema..."
    setup_keyspace_schema $PRIMARY_KEYSPACE $PRIMARY_VISIBILITY_KEYSPACE
    
    # Note: Domain registration will be handled by the Cadence services when they start
    echo "Primary cluster schema setup completed. Domain registration will be handled by Cadence services."
}

setup_secondary_cluster() {
    echo "Setting up secondary cluster schema..."
    setup_keyspace_schema $SECONDARY_KEYSPACE $SECONDARY_VISIBILITY_KEYSPACE
    
    # Note: Domain registration will be handled by the Cadence services when they start
    echo "Secondary cluster schema setup completed. Domain registration will be handled by Cadence services."
}

main() {
    echo "Starting multiclusters schema setup..."
    
    # Wait for Cassandra to be ready
    wait_for_cassandra
    
    # Setup both clusters
    setup_primary_cluster
    setup_secondary_cluster
    
    echo "Multiclusters schema setup completed successfully!"
}

# Run the main function
main "$@" 