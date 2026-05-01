# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

You can find a list of previous releases on the [github releases](https://github.com/cadence-workflow/cadence/releases) page.

## [Note]
There's a new opt-in feature for autoscale of tasklist partitions. It's optional but recommended for large scale use cases. Please refer to [tasklist-partition-config.md](https://github.com/cadence-workflow/cadence/blob/master/docs/migration/tasklist-partition-config.md) for additional details on the migration and its rationale.

## [1.3.3] - 2025-08-06
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.3.3) for details

## [1.3.2] - 2025-07-03
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.3.2) for details

## [1.3.1] - 2025-06-11
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.3.1) for details

## [1.3.0] - 2025-05-14
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.3.0) for details

## [1.2.18] - 2025-04-03
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.18) for details

## [1.2.17] - 2025-03-05
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.17) for details

## [1.2.16] - 2025-02-19
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.16) for details

## [1.2.15] - 2025-01-22
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.15) for details

## [1.2.14] - 2024-11-13
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.14) for details

## [1.2.13] - 2024-09-25
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.13) for details

## [1.2.12] - 2024-08-19
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.12) for details

## [1.2.11] - 2024-07-10
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.11) for details

## [1.2.10] - 2024-06-04
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.10) for details

## [1.2.9] - 2024-05-01
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.9) for details

## [1.2.8] - 2024-03-26
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.8) for details

## [1.2.7] - 2024-02-09
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.2.7) for details
### Upgrade notes
Cadence repo now has multiple submodules,
the split and explanation in [PR](https://github.com/cadence-workflow/cadence/pull/5609).

In principle, "plugins" are "optional" and we should not be forcing all optional dependencies on all users of any of Cadence.
Splitting dependencies into choose-your-own-adventure submodules is simply good library design for the ecosystem, and it's something we should be doing more of.

## [1.2.6] - 2023-12-14
### Added
- Added range query support for Pinot json index (#5426)
- Implemented GetTaskListSize method at persistence layer (#5442, #5447)
- Added a framework for the Task validator service (#5446)
- Added nit comments describing the Update workflow cycle (#5432)
- Added log user query param (#5437)
- Added CODEOWNERS file (#5453)
- Added a function to evict all elements older than the cache TTL (#5464)

### Fixed
- Fixed workflow replication for reset workflow (#5412)
- Fixed visibility mode for admin when use Pinot visibility (#5441)
- Fixed workflow started metric (#5443)
- Fixed timer-fixer, unfortunately broken in 1.2.5 (#5433)
- Fixed confusing comment in matching handler (#5450)

### Changed
- Cassandra version is changed from 3.11 to 4.1.3 (#5461)
  - If your machine already has ubercadence/server:master-auto-setup image then you need to repull so it works with latest docker-compose*.yml files
- Move dynamic ratelimiter to its own file (#5451)
- Create and use a limiter struct instead of just passing a function (#5454)
- Dynamic ratelimiter factories (#5455)
- Update github action for image publishing to released (#5460)
- Update matching to emit metric for tasklist backlog size (#5448)
- Change variable name from SecondsSinceEpoch into EventTimeMs (#5463)

### Removed
- Get rid of noisy task adding failure log in matching service (#5445)

## [1.2.5] - 2023-11-01

### Caution:

Prefer 1.2.4 or 1.2.6 if you have enabled the timer fixer.
These were broken by #5361, and fixed by #5433.

By default this fixer is _disabled_, so the version has not been retracted.

If you have already upgraded to 1.2.5, downgrading or using 1.2.6 should restore the timer fixer.
Or apply #5433 as a local patch.

### Added
- Scanner / Fixer changes (#5361)
  - Stale-workflow detection and cleanup added to shardscanner, disabled by default.
  - New dynamic config to better control scanner and fixer, particularly for concrete executions.
  - Documentation about how scanner/fixer work and how to control them, see [the scanner readme.md](service/worker/scanner/README.md)
    - This also includes example config to enable the new fixer.
- MigrationChecker interface to expose migration CLI (#5424)
- Added Pinot as new visibility store option (#5201)
  - Added pinot visibility triple manager to provide options to write to both ES and Pinot.
  - Added pinotVisibilityStore and pinotClient to support CRUD operations for Pinot.
  - Added pinot integration test to set up Pinot test cluster and test Pinot functionality.

### Fixed
- Fix CreateWorkflowModeContinueAsNew for SQL (#5413)
- Fix CLI count&list workflows error message (#5417)
- Hotfix for async matching for isolation-group redirection (#5423)
- Fix closeStatus for --format flag (#5422)

### Upgrade notes
- Any "concrete execution" or "timers" fixers run on upgrade may be missing the new config and those activities will have no invariants for a single run.  Later runs will work normally. (#5361)

## [1.2.4] - 2023-09-27
### Added
Implement config store for MySQL (#5403)
Implement config store for PostgresSQL (#5405)

### Fixed
Remove database check for config store tests (#5401)
Fix persistence tests setup (#5402)
Retract v1.2.3 (#5406)

## [1.2.3] - 2023-09-15
### Added
Expose workflow history size and count to client (#5392)

### Fixed
[cadence-cli] fix typo in input flag for parallelism (#5397)

### Changed
Update config store client to support SQL database (#5395)
Scaffold config store for sql plugins (#5396)
Improve poller detection for isolation (#5399)

## [1.2.2] - 2023-08-29
### Added
Added a update workflow execution count metric for RI (#5386)

### Fixed
Fixed nil pointer issue in domain migration command rendering (#5378)

### Changed
Pass partition config and isolation group to history/matching even if isolation is disabled (#5385)

## [1.2.1] - 2023-08-24
### Added
Added guardrail to on scalable tasklist number of write partitions (#5331)
Added Opensearch2 client with bulk API shared between clients (#5241)
Added Filters method to dynamic config Key (#5346)
Added domain migration validator command (#5369, #5374)
Added docker-compose with http API enabled (#5358)
Added support to enable TLS for HTTP inbounds (#5381)

### Fixed

### Changed
Updates to partition config middleware (#5334)
Allow to configure HTTP settings using template (#5329)
Cleanup the unused watchdog code (#5096)
Upgrade yarpc to v1.70.3 (#5341)
upgrade mysql (#5345)
change mysql schema folder v57 to v8 (#5356)
Remove duplicated line (#5328)
Fill IsCron for proto of WorkflowExecutionInfo (#5366)
Update idls and ensure thrift fields roundtrip (#5365)
Go version bump (#5367)
Update Dockerfile with a proper Go version and bump alpine version (#5371)
StartWorkflowExecution: validate RequestID before calling history (#5359)
Update list of available frontend HTTP endpoints (#5338)
Set a minimum timeout for async task dispatch based on rate limit (#5382)
Validate search-attribute-key so keys are fine in advanced search (#5340)

## [1.2.0] - 2023-08-24
### Added
Added more logs around unsupported version for consistent query (#5287)
Added datasource and dashboards to Grafana in Docker (#5207)
Added metric for isolation task matching (#5288)
Added local build instructions to Readme (#5299)
Added examples for reset-batch and batch commands in help usage section (#5302)
Record current worker Identity on Pending activity tasks (#5307)
Support invoking RPC using HTTP and JSON (#5305)
Added Worklfow start metric (#5289)
Added count of workflows indicator when running the listall command and waiting it to complete (#5309)
Added option to pass consistency level in the cassandra schema version validation (#5327)

### Fixed
Fixed rendering for isolation-groups in the CLI (#5285)
Fixed SearchAfter usage (#5311)
Fixed garbage collection logic for matching tasks (#5355)
Do not make poller crash if isolation group is invalid (#5372)

### Changed
Removed unused metric (#5292)
Show error message if requesting workflow does not exist (#5300)
Parse JWT flags when creating a context (#5296)
Set ReplicatorCacheCapacity to 0 (#5301)
Show explicit message if command is not found (#5298)
IDL update to include new field in PendingActivityInfo (#5306)
Set 12.4 version for postgres containers (#5326)
Extract EventColorFunction from ColorEvent (#5321)
Update consistency level for cassandra visibility (#5330)
Improve async-matching performance for isolation (#5363)

## [1.1.0] - 2023-08-24
### Added
Added metrics for delete workflow execution on a shard level (#5126)
Added overall persistence count for shardid (#5134)
Added Scanner to purge deprecated domain workflows (#5125)
Added domain status check in taskfilter (#5140)
Added usage for InactiveDomain Invariant (#5144, #5213)
Added request body into Attributes for auditing purpose with PII (#5151)
Added remaining persistence metrics that goes to a shard #5142
Added consistent query pershard metric (#5143, #5170)
Added logging with workflow/domain tags (#5159)
Added ShardID to valid attributes (#5161)
Added Pinot docker files, table config and schema (#5163)
Added Canary TLS support (#5086)
Added a small test to catch issues with deadlocks (#5171)
Added large workflow hot shard detection (#5166)
Added thin ES clients (#5162, #5217)
Added generic ES query building utilities (#5168)
Added Physical sharding for NoSQL-based persistence (#5187)
Added tasklist traffic metrics for non-sticky and non-forwarded tasklists (#5218, #5235)
Added default value for shard_id and update_time in mysql (#5246)
Added default value for shard_id and update_time in postgres (#5259)
Added Hostname tag into metrics and other services (#5245)
Added Domain name validation (#5250)
Added isolation-group types (#5260)
Added Dynamic-config type (#5261)
Added isolation groups to persistence (#5270)
Added helper function to store/retrieve partition config from context (#5195)
Added domain handler changes for isolation group (#5274)
Added isolation-group and partition libraries (#5262)
Added resource implmentation (#5278)
Added Admin API for zonal-isolation drain commands (#5282)
Added cli operations for the zonal-isolation drains (#5283)
Added metric for isolation task matching (#5288)
Added some more coverage to isolation-group state handler (#5304)

### Fixed
Removed circular dependencies between matching components (#5111)
Fixed InsertTasks query for Cassandra (#5119)
Fixed prometheus frontend label inconsistencies (#5087)
Fixed samples documentation (#5088)
Fixed type validation in configstore DC client value updating (#5110)
Fixed the config-store handling for not-found errors (#5203)
Fixed docker (#5244)
Fixed thrift mapper for DomainConfiguration (#5268)
Fixed panic while parsing workflows with timeouts (#5267)
Fixed sticky tasklist isolation (#5308)
Fixed SearchAfter usage (#5311)
Fixed isolation groups domain drains (#5315)

### Changed
Indexer: refactor ES processor (#5100)
Moved sample logger into persistence metric client for cleaness (#5129)
ES: do not set _type when using Bulk API for v7 client (#5104)
ES: single interface for different ES/OpenSearch versions (#5158)
ES: reduce code duplication (#5137)
Updated README.md (#5064, #5251)
Set poll interval for filebased dynamic config if not set (#5160)
Upgraded Golang base image to 1.18 to remediate CVEs (#5035)
Refactor matching integration test (#5182)
Merge Activity and Decision matching tests (#5186)
Update idls version (#5200)
Allow registering search attributes without Advance Visibility enabled (#5185)
Scaffold config store for SQL (#5239)
Bench: possibility to pass frontend address using env (#5113)
Remove tcheck dependency (#5247)
Update internal type to adopt idl change (#5253)
Update persistence layer to adopt idl update for isolation (#5254)
Update history to persist partition config (#5257)
Upgrades IDL to include isolation-groups (#5258)
Make ESClient fields public (#5269)
Remove unneeded file (#5276)
Updated frontend to adopt draining isolation groups (#5225) (#5281)
Updated matching to support tasklist isolation (#5280)
Bumping version to 1.1.0 (#5284)
Improvements on tasklist isolation (#5291)
Task processing panic handling (#5294)
Update ClusterNameForFailoverVersion to return error (#5293)
Feature/isolation group library logging improvements (#5295)
Increase number of forward tokens for isolation tasklists (#5310)
Seperate token pools for tasklists with isolation (#5314)
Disable isolation for sticky tasklist (#5319)
Change default value of AsyncTaskDispatchTimeout (#5320)

## [1.0.0] - 2023-04-26
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v1.0.0)

## [0.23.1] - 2021-11-18
See [Release Note](https://github.com/cadence-workflow/cadence/releases/tag/v0.23.1)

## [0.21.3] - 2021-07-17
### Added
- Added GRPC support. Cadence server will accept requests on both TChannel and GRPC. With dynamic config flag `system.enableGRPCOutbound` it will also switch to GRPC communication internally between server components.

### Fixed
- This change contains breaking change on user config. The masterClusterName config key is deprecated and is replaced with primaryClusterName key. (#4185)

### Changed
- Bump CLI version to v0.19.0
- Change `--connect-attributes` in `cadence-sql-tool` from URL encoding to the format of k1=v1,k2=v2...
- Change `--domain_data` in `cadence domain update/register` from the format of k1:v1,k2:v2... to the format of k1=v1,k2=v2...
- Deprecate local domains. All new domains should be created as global domains.
- Server will deep merge configuration files when loading. No need to copy the whole config section, specify only those fields that needs overriding. (#4165)

## [0.18.0] - 2021-01-22

## [0.16.1] - 2021-01-21

## [0.17.0] - 2021-01-13

## [0.16.0] - 2020-12-10
