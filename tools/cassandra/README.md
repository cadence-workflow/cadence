## Using the cassandra schema tool
This package contains the tooling for cadence cassandra operations.

## For localhost development
```
make install-schema
```
> NOTE: See [CONTRIBUTING](/CONTRIBUTING.md) for prerequisite of make command.

## For production

### Get the Cassandra Schema tool
* Use brew to install CLI: `brew install cadence-workflow` which includes `cadence-cassandra-tool`
  * The schema files are located at `/usr/local/etc/cadence/schema/`.
  * Follow the [instructions](https://github.com/cadence-workflow/cadence/discussions/4457) if you need to install older versions of schema tools via homebrew.
 However, easier way is to use new versions of schema tools with old versions of schemas.
 All you need is to check out the older version of schemas from this repo. Run `git checkout v0.21.3` to get the v0.21.3 schemas in [the schema folder](/schema).
* Or build yourself, with `make cadence-cassandra-tool`. See [CONTRIBUTING](/CONTRIBUTING.md) for prerequisite of make command.

> Note: The binaries can also be found in the `ubercadence/server` docker images

### Do one time database creation and schema setup for a new cluster
This uses Cassandra's SimpleStratagey for replication. For production, we recommend using a replication factor of 3 with NetworkTopologyStrategy.

```
cadence-cassandra-tool --ep $CASSANDRA_SEEDS create -k $KEYSPACE --rf $RF
```

See https://www.ecyrd.com/cassandracalculator for an easy way to determine how many nodes and what replication factor you will want to use.  Note that Cadence by default uses `Quorum` for read and write consistency.

```
./cadence-cassandra-tool -ep 127.0.0.1 -k cadence setup-schema -v 0.0 # this sets up just the schema version tables with initial version of 0.0
./cadence-cassandra-tool -ep 127.0.0.1 -k cadence update-schema -d ./schema/cassandra/cadence/versioned # upgrades your schema to the latest version

./cadence-cassandra-tool -ep 127.0.0.1 -k cadence_visibility setup-schema -v 0.0 # this sets up just the schema version tables with initial version of 0.0 for visibility
./cadence-cassandra-tool -ep 127.0.0.1 -k cadence_visibility update-schema -d ./schema/cassandra/visibility/versioned # upgrades your schema to the latest version for visibility
```

### Update schema as part of a release
You can only upgrade to a new version after the initial setup done above.

```
./cadence-cassandra-tool -ep 127.0.0.1 -k cadence update-schema -d ./schema/cassandra/cadence/versioned -v x.xx -dryrun # executes a dryrun of upgrade to version x.xx
./cadence-cassandra-tool -ep 127.0.0.1 -k cadence update-schema -d ./schema/cassandra/cadence/versioned -v x.xx # actually executes the upgrade to version x.xx

./cadence-cassandra-tool -ep 127.0.0.1 -k cadence_visibility update-schema -d ./schema/cassandra/visibility/versioned -v x.xx --dryrun # executes a dryrun of upgrade to version x.xx
./cadence-cassandra-tool -ep 127.0.0.1 -k cadence_visibility update-schema -d ./schema/cassandra/visibility/versioned -v x.xx  # actually executes the upgrade to version x.xx
```
