# What?

Migrate visibility read/write dynamic configs from the old enable/mode names to store-name configs.


| Old name (pre-v1.2.17)                                                                                                   | New name                                                                                                                                                                                               |
| ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `system.enableReadVisibilityFromES` / `system.enableReadVisibilityFromPinot` (bool: read from advanced visibility or DB) | [`system.readVisibilityStoreName`](https://github.com/cadence-workflow/cadence/blob/v1.2.17/common/dynamicconfig/constants.go#L4593) (which store to read: `db`, `es`, `pinot`, `os`)                  |
| `system.advancedVisibilityWritingMode` (`off` / `on` / `dual`)                                                           | [`system.writeVisibilityStoreName`](https://github.com/cadence-workflow/cadence/blob/v1.2.17/common/dynamicconfig/constants.go#L4530) (which store(s) to write; comma-separated for dual/triple write) |


Also removed: `system.advancedVisibilityMigrationWritingMode`.

Background: [visibility-on-elasticsearch.md](../visibility-on-elasticsearch.md).

# Why?

The old bools only meant "use advanced visibility or not," and the write mode only covered DB ↔ one advanced store. Naming the store(s) supports multiple advanced backends (`es`, `pinot`, `os`) and, more importantly, migration **between** visibility stores: you can dual-write and dual/shadow-read across more than one store (for example `db,es`, `es,pinot`, or `es,os`) without extra dynamic configs.

Introduced in [#6560](https://github.com/cadence-workflow/cadence/pull/6560).

# How?

## 1. Upgrade server

Upgrade Cadence server to **v1.2.17 or newer**. The old keys are removed; remaining `enableReadVisibilityFromES`, `enableReadVisibilityFromPinot`, `advancedVisibilityWritingMode` and `advancedVisibilityMigrationWritingMode` entries are ignored after upgrade.

## 2. Map write config

Translate `system.advancedVisibilityWritingMode` → `system.writeVisibilityStoreName`:


| Old value | New value (ES as advanced store) | Meaning                        |
| --------- | -------------------------------- | ------------------------------ |
| `off`     | `off`                            | Write DB visibility only       |
| `on`      | `es`                             | Write advanced visibility only |
| `dual`    | `db,es`                          | Dual-write DB and ES           |


If advanced visibility is Pinot or OpenSearch, use `pinot` / `os` instead of `es` (for example `db,pinot` or `db,os`).

Use `off`, not `db`, to map the old `off` when an advanced store is still configured. Both write only to DB visibility, but `db` still counts as advanced visibility writing enabled: history keeps generating search-attribute upsert tasks and the worker still starts the indexer. With no advanced store configured, `off` and `db` behave the same.

To write to two advanced stores (for example `es,os` or `es,pinot`), the ES store must also be created; see [section 5](#5-es--opensearch--pinot-migration). Without it, `es` is not available: its writes fall back to DB unless the list already includes `db`, while the other stores still write. For example, `es,os` without the migration config writes to DB and OpenSearch.

`writeVisibilityStoreName` has **no domain filter**. Values are case-insensitive; multiple stores are comma-separated.

Example:

```yaml
# Before
system.advancedVisibilityWritingMode:
  - value: "dual"

# After
system.writeVisibilityStoreName:
  - value: "db,es"
```

## 3. Map read config

Translate `system.enableReadVisibilityFromES` / `system.enableReadVisibilityFromPinot` → `system.readVisibilityStoreName`.

The old bool meant "read from advanced visibility or not." The new value names the store:


| Old value                              | New value | Meaning                         |
| -------------------------------------- | --------- | ------------------------------- |
| `false`                                | `db`      | Read from DB (basic) visibility |
| `true` (enableReadVisibilityFromES)    | `es`      | Read from Elasticsearch         |
| `true` (enableReadVisibilityFromPinot) | `pinot`   | Read from Pinot                 |


Use `os` if advanced visibility is OpenSearch. Domain overrides keep the same shape; only the value type changes from bool to store name.

Example — global DB read, with selected domains on advanced visibility:

```yaml
# Before
system.enableReadVisibilityFromES:
  - value: false
  - value: true
    constraints:
      domainName: "samples-domain-A"

# After
system.readVisibilityStoreName:
  - value: "db"
  - value: "es"
    constraints:
      domainName: "samples-domain-A"
```

Optional shadow (double) read: put a secondary store after a comma. The first store is primary and its result is returned to the client. The second store receives the same query asynchronously; its result is discarded and only errors are logged. Use it to verify that a new store can handle production read traffic before switching reads to it.

```yaml
system.readVisibilityStoreName:
  - value: "es,pinot"   # primary: es, shadow: pinot
```

## 4. Typical zero-downtime DB → ES migration (updated steps)

Same rollout as before, with the new keys:

1. Keep DB visibility configured. Add advanced visibility (`advancedVisibilityStore`) per [visibility-on-elasticsearch.md](../visibility-on-elasticsearch.md).
2. Dual-write, still read from DB:
   ```yaml
   system.writeVisibilityStoreName:
     - value: "db,es"
   system.readVisibilityStoreName:
     - value: "db"
   ```
3. After enough retention / backlog catch-up, flip reads per domain, then globally:
   ```yaml
   system.writeVisibilityStoreName:
     - value: "db,es"
   system.readVisibilityStoreName:
     - value: "es"
   ```
4. Optionally stop writing to DB (`writeVisibilityStoreName: "es"`) and remove DB visibility from static config when no longer needed.

## 5. ES → OpenSearch / Pinot migration

**Note:** Running OpenSearch through the ES visibility store (`advancedVisibilityStore: es-visibility` with `version: "os2"`) is still the ES store. Use store name `es` for read/write — not `os`. The `os` name applies only when `advancedVisibilityStore` is `os-visibility` (the dedicated OpenSearch store used for ES → OS migration below). See [config/development_es_opensearch.yaml](../../config/development_es_opensearch.yaml).

The same keys move visibility between advanced stores. The steps below use OpenSearch; for Pinot, replace `os` with `pinot` and `os-visibility` with `pinot-visibility`. Pinot's datastore block uses `pinot:` instead of `elasticsearch:`.

1. In static config, set `advancedVisibilityStore` to the new store and enable `migration` on it, keeping the existing `es-visibility` datastore. This creates both the ES and the new store. For OpenSearch, also set a distinct `consumerName` on `os-visibility`: otherwise the ES and OS indexers join the same Kafka consumer group, split the topic's partitions, and each store gets only part of the records. Pinot ingests from Kafka directly, so this does not apply to Pinot. See [config/development_es_opensearch_migration.yaml](../../config/development_es_opensearch_migration.yaml) and [config/development_pinot.yaml](../../config/development_pinot.yaml) for examples. The Pinot sample has `migration: enabled: false`; set it to `true` for a migration.
   ```yaml
   persistence:
     advancedVisibilityStore: os-visibility
     datastores:
       es-visibility:
         elasticsearch:
           # existing ES config
       os-visibility:
         elasticsearch:
           # new OpenSearch config
           migration:
             enabled: true
           # Required: must differ from the ES consumer group (default "<es-index>-consumer").
           consumerName: "<topic>-os-consumer"
   ```
2. Dual-write, still read from ES, and shadow-read the new store to check it under production read traffic:
   ```yaml
   system.writeVisibilityStoreName:
     - value: "es,os"
   system.readVisibilityStoreName:
     - value: "es,os"   # primary: es, shadow: os
   ```
3. After enough retention / backlog catch-up, flip the primary per domain, then globally. Keep ES as the shadow if you want to compare in the other direction:
   ```yaml
   system.readVisibilityStoreName:
     - value: "os,es"
   ```
4. Stop writing to ES (`writeVisibilityStoreName: "os"`), set reads to `os`, then remove the `es-visibility` datastore and `migration` from static config.

## 6. Defaults after upgrade

If you do not set the new keys, defaults are:

- `system.writeVisibilityStoreName`: `"es"`
- `system.readVisibilityStoreName`: `"es"`

Clusters with advanced visibility configured that previously relied on `enableReadVisibilityFromES: false` or `advancedVisibilityWritingMode: dual` **must** set the new keys explicitly before or at upgrade, or traffic will shift to ES-only read/write by default.

If no advanced visibility store is configured, the `es` default falls back to DB for both reads and writes, with a warning logged on every call. Set both keys to `db` to avoid the log noise.

# Status

As of v1.2.17, the old keys are removed. Use only `system.readVisibilityStoreName` and `system.writeVisibilityStoreName`.