## Using the SQL schema tool

This package contains the tooling for cadence sql operations. The tooling itself is agnostic of the storage engine behind
the sql interface. So, this same tool can be used against, say, OracleDB and MySQLDB

## For localhost development
```
SQL_USER=$USERNAME SQL_PASSWORD=$PASSWD make install-schema-mysql
```
> NOTE: See [CONTRIBUTING](/CONTRIBUTING.md) for prerequisite of make command.

## For production

### Get the SQL Schema tool
* Use brew to install CLI: `brew install cadence-workflow` which includes `cadence-sql-tool`
  * The schema files are located at `/usr/local/etc/cadence/schema/`.
  * Follow the [instructions](https://github.com/cadence-workflow/cadence/discussions/4457) if you need to install older versions of schema tools via homebrew.
 However, easier way is to use new versions of schema tools with old versions of schemas.
 All you need is to check out the older version of schemas from this repo. Run `git checkout v0.21.3` to get the v0.21.3 schemas in [the schema folder](/schema).
* Or build yourself, with `make cadence-sql-tool`. See [CONTRIBUTING](/CONTRIBUTING.md) for prerequisite of make command.

> Note: The binaries can also be found in the `ubercadence/server` docker images

### Do one time database creation and schema setup for a new cluster
- All command below are taking MySQL as example. For postgres, simply use with "--plugin postgres"

```
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql create-database --db cadence
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql create-database --db cadence_visibility
```

```
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence setup-schema -v 0.0 -- this sets up just the schema version tables with initial version of 0.0
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence update-schema -d ./schema/mysql/v8/cadence/versioned -- upgrades your schema to the latest version

./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence_visibility setup-schema -v 0.0 -- this sets up just the schema version tables with initial version of 0.0 for visibility
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence_visibility update-schema -d ./schema/mysql/v8/visibility/versioned  -- upgrades your schema to the latest version for visibility
```

### Update schema as part of a release
You can only upgrade to a new version after the initial setup done above.

```
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence update-schema -d ./schema/mysql/v8/cadence/versioned -v x.x --dryrun -- executes a dryrun of upgrade to version x.x
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence update-schema -d ./schema/mysql/v8/cadence/versioned -v x.x    -- actually executes the upgrade to version x.x

./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence_visibility update-schema -d ./schema/mysql/v8/visibility/versioned -v x.x --dryrun -- executes a dryrun of upgrade to version x.x
./cadence-sql-tool --ep $SQL_HOST_ADDR -p $port --plugin mysql --db cadence_visibility update-schema -d ./schema/mysql/v8/visibility/versioned -v x.x    -- actually executes the upgrade to version x.x
```
