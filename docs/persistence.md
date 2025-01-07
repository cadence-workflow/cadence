# Overview
Cadence has a well defined API interface at the persistence layer. Any database that supports multi-row transactions on
a single shard or partition can be made to work with cadence. This includes cassandra, dynamoDB, auroraDB, MySQL,
Postgres and may others. There are currently three supported database implementations at the persistence layer -
cassandra and MySQL/Postgres. This doc shows how to run cadence with cassandra and MySQL(Postgres is mostly the same). It also describes the steps involved
in adding support for a new database at the persistence layer.

# Getting started on mac
## Cassandra
### Start cassandra
```
brew install cassandra
brew services start cassandra
```
### Install cadence schema
```
cd $GOPATH/github.com/uber/cadence
make install-schema
```
> NOTE: See [CONTRIBUTING](/CONTRIBUTING.md) for prerequisite of make command.
>
### Start cadence server
```
cd $GOPATH/github.com/uber/cadence
./cadence-server start --services=frontend,matching,history,worker
```

## MySQL
### Start MySQL server
```
brew install mysql
brew services start mysql
```
### Install cadence schema
```
cd $GOPATH/github.com/uber/cadence
make install-schema-mysql
```
When run tests and CLI command locally, Cadence by default uses a user `uber` with password `uber`, with privileges of creating databases.
You can use the following command to create user(role) and grant access.
In the mysql shell:
```
> CREATE USER 'uber'@'%' IDENTIFIED BY 'uber';
> GRANT ALL PRIVILEGES ON *.* TO 'uber'@'%';
```
### Start cadence server
```
cd $GOPATH/github.com/uber/cadence
cp config/development_mysql.yaml config/development.yaml
./cadence-server start --services=frontend,matching,history,worker
```

## PostgresQL
### Start PostgresQL server
```
brew install postgres
brew services start postgres
```
When run tests and CLI command locally, Cadence by default uses a superuser `postgres` with password `cadence`.
You can use the following command to create user(role) and grant access:
```
$psql postgres
postgres=# CREATE USER postgres WITH PASSWORD 'cadence';
CREATE ROLE
postgres=# ALTER USER postgres WITH SUPERUSER;
ALTER ROLE
```
### Install cadence schema
```
cd $GOPATH/github.com/uber/cadence
make install-schema-postgres
```

### Start cadence server
```
cd $GOPATH/github.com/uber/cadence
cp config/development_postgres.yaml config/development.yaml
./cadence-server start --services=frontend,matching,history,worker
```

# Configuration
## Common to all persistence implementations
There are two major sub-subsystems within cadence that need persistence - cadence-core and visibility. cadence-core is
the workflow engine that uses persistence to store state tied to domains, workflows, workflow histories, task lists
etc. cadence-core powers almost all of the cadence APIs. cadence-core could be further broken down into multiple
subs-systems that have slightly different persistence workload characteristics. But for the purpose of simplicity, we
don't expose these sub-systems today but this may change in future. Visibility is the sub-system that powers workflow
search. This includes APIs such as ListOpenWorkflows and ListClosedWorkflows. Today, it is possible to run a cadence
server with cadence-core backed by one database and cadence-visibility backed by another kind of database.To get the full
feature set of visibility, the recommendation is to use elastic search as the persistence layer. However, it is also possible
to run visibility with limited feature set against Cassandra or MySQL today.  The top level persistence configuration looks
like the following:


```
persistence:
  defaultStore: datastore1    -- Name of the datastore that powers cadence-core
  visibilityStore: datastore2 -- Name of the datastore that powers cadence-visibility
  numHistoryShards: 1024      -- Number of cadence history shards, this limits the scalability of single cadence cluster
  datastores:                 -- Map of datastore-name -> datastore connection params
    datastore1:
      nosql:
         ...
    datastore2:
      sql:
        ...
```

## Note on numHistoryShards
Internally, cadence uses shards to distribute workflow ownership across different hosts. Shards are necessary for the
horizontal scalability of cadence service. The number of shards for a cadence cluster is picked at cluster provisioning
time and cannot be changed after that. One way to think about shards is the following - if you have a cluster with N
shards, then cadence cluster can be of size 1 to N. But beyond N, you won't be able to add more hosts to scale. In future,
we may add support to dynamically split shards but this is not supported as of today. Greater the number of shards,
greater the concurrency and horizontal scalability.

## Cassandra
```
persistence:
  ...
  datastores:
    datastore1:
      nosql:
        pluginName: "cassandra"
        hosts: "127.0.0.1,127.0.0.2"  -- CSV of cassandra hosts to connect to
        User: "user-name"
        Password: "password"
        keyspace: "cadence"           -- Name of the cassandra keyspace
        datacenter: "us-east-1a"      -- Cassandra datacenter filter to limit queries to a single dc (optional)
        maxConns: 2                   -- Number of tcp conns to cassandra server (single sub-system on one host) (optional)
```

## MySQL/Postgres
The default isolation level for MySQL/Postgres is READ-COMMITTED.

Note that for MySQL 5.6 and below only, the isolation level needs to be
specified explicitly in the config via connectAttributes.

```
persistence:
  ...
  datastores:
    datastore1:
      sql:
        pluginName: "mysql"            -- name of the go sql plugin
        databaseName: "cadence"        -- name of the database to connect to
        connectAddr: "127.0.0.1:3306"  -- connection address, could be ip address or domain socket
        connectProtocol: "tcp"         -- connection protocol, tcp or anything that SQL Data Source Name accepts
        user: "uber"
        password: "uber"
        maxConns: 20                   -- max number of connections to sql server from one host (optional)
        maxIdleConns: 20               -- max number of idle conns to sql server from one host (optional)
        maxConnLifetime: "1h"          -- max connection lifetime before it is discarded (optional)
        connectAttributes:             -- custom dsn attributes, map of key-value pairs
          tx_isolation: "READ-COMMITTED"   -- required only for mysql 5.6 and below, optional otherwise
```

## Multiple SQL(MySQL/Postgres) databases
To run Cadence clusters in a much larger scale using SQL database, multiple databases can be used as a sharded SQL database cluster.

Set `useMultipleDatabases` to `true` and specify all databases' user/password/address using `multipleDatabasesConfig`:
```yaml
persistence:
  ...
  datastores:
    datastore1:
      sql:
        pluginName: "mysql"            -- name of the go sql plugin
        connectProtocol: "tcp"         -- connection protocol, tcp or anything that SQL Data Source Name accepts
        maxConnLifetime: "1h"          -- max connection lifetime before it is discarded (optional)
        useMultipleDatabases: true     -- this enabled the multiple SQL databases as sharded SQL cluster
        nShards: 4                     -- the number of shards -- in this mode, it needs to be greater than one and equalt to the length of multipleDatabasesConfig
        multipleDatabasesConfig:       -- each entry will represent a shard of the cluster
        - user: "root"
          password: "cadence"
          connectAddr: "127.0.0.1:3306"
          databaseName: "cadence0"
        - user: "root"
          password: "cadence"
          connectAddr: "127.0.0.1:3306"
          databaseName: "cadence1"
        - user: "root"
          password: "cadence"
          connectAddr: "127.0.0.1:3306"
          databaseName: "cadence2"
        - user: "root"
          password: "cadence"
          connectAddr: "127.0.0.1:3306"
          databaseName: "cadence3"
```


How Cadence implement the sharding:

* Workflow execution and historyShard records are sharded based on historyShardID(which is calculated  `historyShardID =hash(workflowID) % numHistoryShards` ), `dbShardID = historyShardID % numDBShards`
* Workflow History is sharded based on history treeID(a treeID usually is the runID unless it has reset. In case of reset, it will share the same tree as the base run). In that case, `dbShardID = hash(treeID) % numDBShards`
* Workflow tasks(for workflow/activity workers) is sharded based on domainID + tasklistName.  `dbShardID = hash(domainID + tasklistName ) % numDBShards`
* Workflow visibility is  sharded based on domainID like we said above.  `dbShardID = hash(domainID ) % numDBShards`
  * However, due to potential scalability issue, Cadence requires advanced visibility to run with multiple SQL database mode.
* Internal domain records is using single shard, it’s only writing when register/update domain, and read is protected by domainCache  `dbShardID = DefaultShardID(0)`
* Internal queue records is using single shard. Similarly, the read/write is low enough that it’s okay to not sharded. `dbShardID = DefaultShardID(0)`

# Adding support for new database

## For SQL Database
As there are many shared concepts and functionalities in SQL database, we abstracted those common code so that is much easier to implement persistence interfaces with any SQL database. It requires your database supports SQL operations like explicit transaction(with pessimistic locking)

This interface is tied to a specific schema i.e. the way data is laid out across tables and the table
names themselves are fixed. However, you get the flexibility wrt how you store the data within a table (i.e. column names and
types are not fixed). The API interface can be found [here](https://github.com/cadence-workflow/cadence/blob/master/common/persistence/sql/plugins/interfaces.go).
It's basically a CRUD API for every table in the schema. A sample schema definition for mysql that uses this interface
can be found [here](https://github.com/cadence-workflow/cadence/blob/master/schema/mysql/v8/cadence/schema.sql)

Any database that supports this interface can be plugged in with cadence server.
We have implemented Postgres within the repo, and also here is [**an example**](https://github.com/longquanzheng/cadence-extensions/tree/master/cadence-sqlite) to implement any database externally.


## For other Non-SQL Database
For databases that don't support SQL operations like explicit transaction(with pessimistic locking),
Cadence requires at least supporting:
 1. Multi-row single shard conditional write(also called LightWeight transaction by Cassandra terminology)
 2. Strong consistency Read/Write operations

This NoSQL persistence API interface can be found [here](https://github.com/cadence-workflow/cadence/blob/master/common/persistence/nosql/nosqlplugin/interfaces.go).
Currently this is only implemented with Cassandra. DynamoDB and MongoDB are in progress.
