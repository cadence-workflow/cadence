### How to run Cadence locally with replication


This is a guide for using a local laptop instance and running three Cadence
clusters, all cross-replicted. They will (for the purposes of demonstration) be
entirely independentent, but poll one another for workflows for replication and
write to their own keyspaces independently.

It's a good way to work through how Cadence's domains are configured with
replication, how failovers work and so forth.

This is a somewhat more advanced tutorial, and so it's recommended to ensure
that you have a basic familiarity with Cadence and it's concepts first before
trying this.

#### Prerequisites

- Docker
- The ability to perform a local go build

#### Getting started

All commands are expected to be run from the repo's base path.

1. Ensure there's no existing containers running or preexisting state. You'll be
   creating a new keyspace and a new docker-container *only* for Cassandra, and
   so this can conflict in a few ways with other cassandra instances for
   fully-containerized cadence. Be sure to stop and/or remove them before
   running this.

2. Ensure you build the `cadence-server` binaries by using `make bins`.
   This will build the main application and produce a binary build of the local
   instance of cadence in the repo's base path.

3. Start docker with the XDC configuration: `docker compose  -f docker/dev/cassandra.yml up -d`

4. setup the schema for each of the instances with `until make
   install-schema-xdc; do sleep 1; done` (since Cassandra can take a while to
   setup, loop this check until it completes).

5. start (in tmux or terminal windows) each of the Cadence instances
  - `./cadence-server --config config --zone xdc --env development_xdc_cluster0 start`
  - `./cadence-server --config config --zone xdc --env development_xdc_cluster1 start`
  - `./cadence-server --config config --zone xdc --env development_xdc_cluster2 start`

6. Register a global domain that has multiple clusters enabled using the cadence
   CLI, so that it's worklows will be replicated across clusters: 
   `./cadence --env development --domain test-domain domain register --gd true --clusters cluster0,cluster1`

7. Start and test workflows as needed and failover the domain to make another
   cluster active with a command such as this: `./cadence --env development --domain test-domain domain update --ac cluster1`

