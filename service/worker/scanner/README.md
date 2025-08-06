# A brief* overview of Scanner and Fixer and how to control them

First and foremost: most code in this folder is disabled by default dynamic config,
and it should be considered beta-like quality in general.  Understand what you are
enabling before enabling it, and run it at your own risk.  It is shared primarily so
others can learn from and leverage what we have already encountered, if they end up
needing it or something similar.

**Any _actually recommended_ fixing processes will be explicitly called out in release
notes or similar.**  This document is **not** recommending any, merely describing.

There are a variety of problems with the Scanner and Fixer related code, so
please do not take this document as a sign that it is a structure we want to _keep_.
It has just been confusing enough that it took substantial time to understand,
and now some of that effort is written down to save people the full effort in
the future.

## What is this folder for

This folder as a whole contains a variety of data-cleanup workflows.

Stuff like:
- Find old, unnecessary data and delete it.
- Find data caused by old bugs and fix it.
- Clean up and remove abandoned tasklists so they do not continue taking up space.

As a general rule, these all scan the full database (for one kind of data), check
some data, and clean it up if necessary.  
*How their code does that* varies quite a bit, however.

E.g. the "history scavenger" finds old branches of history that lost their CAS race
to update the workflow's official history.  It is not possible to guarantee that
these are cleaned up while a workflow runs, because any cleanup could have failed.  
Instead, the history scavenger periodically walks through the whole database and
looks for these orphaned history branches, and deletes them.

The most complex of these processes are based around `Scanner` and `Fixer`, and
so **this readme is almost exclusively written for them**.  
For others, just read their code, it's probably faster than reading any doc about
their code.

## Basic structure of `Scanner` and `Fixer` workflows

- "Invariants" define `Check` and `Fix` methods that make sure our invariants hold,
  and fixes them if they did not for some reason.
  - These are in the `common/reconciliation/invariant` folder, e.g.
    [concreteExecutionExists.go](../../../common/reconciliation/invariant/concreteExecutionExists.go)
    checks (`Check`) that a current execution record points to a concrete record
    that exists.  If it does not, it deletes the current record (the `Fix`).
  - Some of these have a paired "invariant collection" which is currently a 1:1
    relationship, and is used elsewhere to refer to the invariants by a name where
    it isn't unique per data-type (e.g. timer only has one).
  - These are often bundled together in an `InvariantManager`, which runs multiple and
    aggregates the results.
  - Invariants are _almost_ exclusively used by Scanner and Fixer.  One is also used
    as part of replication processing, in [ndc/history_resender.go](../../../common/ndc/history_resender.go).
    That use is essentially completely unrelated to Scanner or Fixer, and is ignored for this doc.
- [Scanner](shardscanner/scanner.go) runs only the `Check`, and pushes all failing checks to the blobstore.
  - It gets its core data through an Iterator, whose implementation depends on your datastore.
  - On each Iterator item, it runs through its list of Invariants (via an `InvariantManager`).
  - Per item, the aggregated result from the InvariantManager is collected, and pushed to a blob-store. 
- [Fixer](shardscanner/fixer.go) runs only `Fix`, on the downloaded results from the most-recent Scanner.
  - Structurally it's extremely similar to Scanner, but it calls `Fix` instead, and
    it gets its data from a different Iterator (this one iterates over the scanner
    results in your blobstore).
  - _All_ configured invariants run, not just the ones that failed `Check` previously.
  - Because only `Fix` is called, invariants should likely `Check` first internally.
- ^ The workflows that run Scanner and Fixer are started at service startup _if enabled_,
  and are run as continuous every-minute crons that essentially never expire.
  - Each scanner / fixer type uses its own tasklist, and these workers are only
    started if enabled.
    - This effectively means that enabling one works immediately after a service start,
      but disabling _only pauses_, which may be undesirable if they are resumed after
      a lengthy delay.
    - Because these are crons, only the original start arguments are retained, not new
      ones if changes are made.
    - **Manually cancel or terminate the cron workflows if you change relevant fields,
      and (re)start a worker to start the new versions.**
  - Each workflow that runs processes *only one data-source*, primarily via its Iterator.
    - This means all invariants within a scanner/fixer run handle the same type of data.
- The bulk of all ^ this is plugged together by `*shardscanner.ScannerConfig` instances,
  which contain everything that customizes each particular _kind_ of scanner/fixer,
  which is stored in / retrieved from the context by the workflow's type as a key.
  - E.g. see the [concrete_execution.go config](executions/concrete_execution.go).
  - The workflow type (registered function name) is in there, as are the start args,
    some high level dynamic config to control the scanner/fixer workflows (enabled,
    concurrency, etc), and "hooks" for both scanner and fixer.
  - The workflows largely do not care about this config, they just run the same
    activities each time and let the activities figure out what to do.
  - Activities get their config and other dependencies from [Get{Scanner,Fixer}Context](shardscanner/types.go)),
    which contain this config.
- Hooks (fields in the `ScannerConfig`) are where much of the non-workflow behavior comes from.
  - E.g. the concrete scanner hooks bundle up a "manager" (InvariantManager), "iterator"
    (walks the datasource and yields individual items to check), and "custom config"
    (dynamic config to control which invariants are enabled) as part of the `create*Hooks`
    funcs in [concrete_exeuction.go](executions/concrete_execution.go).
- And so, ultimately the workflows essentially take this config and create a Scanner or Fixer
  out of them (in activities), as you can see in e.g. [the scanner workflow](shardscanner/scanner_workflow.go).
  - It reads some config through an activity (which gets its per-workflow-type context).
    - This is used to decide parallelism / pass additional args to the scan activities.
  - The `scanShardActivity` (in [shardscanner/activities.go](shardscanner/activities.go)
    essentially iterates over shards, and runs `scanShard` on each one, which creates
    a `NewScanner` that gets its config and behavior from args / environment / hooks.
  - Fixer is **very** similar, except it also queries the previous Scanner run to get the
    list of blobstore files that it needs to process. 

Last but not least: there are other workflows in this folder which _do not_ follow these patterns!
- E.g. the [tasklist scanner and history scavenger](workflow.go), and [CheckDataCorruptionWorkflow](data_corruption_workflow.go).
- These are MUCH more localized in behavior and simpler, so they are not covered in this document.
  Just read the code :)
- Their workflows are still started in the main entry-point, [scanner.go](scanner.go).
  See the `Start` function, most are pretty easily found in there.

Hopefully that made sense.

## Config

Scanners and fixers are generally disabled by default, as they can consume
substantial resources and may delete or modify data to correct issues.  
Because of this, you generally need to modify your dynamic config to run them.

At the time of writing, you can enable these with config like the following.

Enable scanner workflows (these are per data source / per record type, like
"concrete executions" and "timers"):
```yaml
worker.executionsScannerEnabled:
  - value: true        # default false
worker.currentExecutionsScannerEnabled:
  - value: true        # default false
worker.timersScannerEnabled:
  - value: true        # default false
worker.historyScannerEnabled:
  - value: true        # default false
worker.taskListScannerEnabled:
  - value: true        # default true, only used on sql stores
```
Enable scanner invariants (currently each one only supports one data source /
record type, but there may be multiple invariants for the data source):
```yaml
# concretes
worker.executionsScannerInvariantCollectionStale:
  - value: true         # default false
worker.executionsScannerInvariantCollectionMutableState:
  - value: true         # default true
worker.executionsScannerInvariantCollectionHistory:
  - value: true         # default true

# timer invariant is implied as there is only one.
# to enable it, enable the workflow.

# currents, NONE OF THESE WORK because of type mismatch
worker.currentExecutionsScannerInvariantCollectionHistory:
  - value: true         # default true
worker.currentExecutionsInvariantCollectionMutableState:
  - value: true         # default true
```
Enable fixer workflows (also one per type):
```yaml
worker.concreteExecutionFixerEnabled:
  - value: true       # default false
worker.currentExecutionFixerEnabled:
  - value: true       # default false
worker.timersFixerEnabled:
  - value: true       # default false
```
Enable fixer to run on a domain (required to do anything to a domain's data,
which also means nothing will be fixed without this):
```yaml
worker.currentExecutionFixerDomainAllow:
  - value: true         # default false
    constraints: {domainName: "your-domain"}  # for example, or have no constraints to enable for all domains
worker.concreteExecutionFixerDomainAllow:
  - value: true         # default false
worker.timersFixerDomainAllow:
  - value: true         # default false
```
Enable fixer invariants:
```yaml
# concretes
worker.executionsFixerInvariantCollectionStale:
  - value: true         # default false
worker.executionsFixerInvariantCollectionMutableState:
  - value: true         # default true
worker.executionsFixerInvariantCollectionHistory:
  - value: true         # default true

# timer invariant is enabled if timer-fixer is enabled, as there is only one

# current execution fixer has never worked and does not currently support dynamic config
```

## Verifying locally

There are a few ways to run local clusters and make changes and test things out,
but this is the way I did things when reading and changing this code:

1. Run the default docker-compose cluster ([docker-compose.yml](../../../docker/docker-compose.yml))
2. Make your config/code/etc changes locally for scanner / fixer.
3. `make cadence-server` to ensure it builds
4. `./cadence-server start --services worker` to start up a worker that will
   connect to your docker compose cluster.
5. Browse history via the web UI, usually http://localhost:8088/domains/cadence-system/workflows

The default docker compose setup starts a worker instance, but due to the default
dynamic config setup where all but `worker.taskListScannerEnabled` are disabled,
the in-docker worker will not run (most) scanner/fixer tasklists and will not steal
any tasks from a local worker.

So you can often simply run it without any changes, start up your local (customized)
worker service outside of docker, and everything will Just Work™.

This way you can leverage the normal docker compose yaml files with minimal effort,
use the web UI to observe the results, and rapidly change/rebuild/rerun/debug/etc
without needing to deal with docker.

If you **do** need to debug the tasklist scanner, I would recommend making a custom build,
and modifying the docker compose file to use your build.  Details on that are below,
but they **are not necessary** for other scanners/fixers.

### Docker compose changes (tasklist scanner only)

There are a few ways to achieve this, but I like modifying the `docker-compose*.yaml`
file to use a custom local build, and just changing its dynamic config to disable
the tasklist scanner.

To do that, see the [docker/README.md file](../../../docker/README.md) for instructions.
Personally I prefer making a unique auto-setup tag so it does not replace any non-customized
docker compose runs in the future.  E.g.:
```yaml
services:
  # ...
  cadence:
    image: ubercadence/server:my-auto-setup  # use your new tag
    # ...
    environment:
      # note this env var, it is the file you need to change
      - "DYNAMIC_CONFIG_FILE_PATH=config/dynamicconfig/development.yaml"
```
And just make a build after changing the dynamic config file.  This will copy the file
into the docker image, and any local changes won't affect the running container.
```yaml
worker.taskListScannerEnabled:
  - value: false        # default true, only used on sql stores
```
Just set ^ this, and make sure the others are not explicitly enabled as they are
disabled by default, and you're likely done.

### Other things to change outside docker

- Config, as running the server locally will generally use the `config/dynamicconfig/development.yaml`
  file, so you likely want to change that one to enable your code.
- Data, to give your invariant / scanner something to notice
  - Running some workflows and then deleting / modifying the data by hand in the
    database is fairly simple.
- Your invariants, to prevent prematurely purging any manual data changes you made.
  - I'm fond of this trick:
    ```go
    func (i *invariant) Check(...) {
        x := true    // go vet doesn't currently complain
        if x {       // about dead code with this.  handy!
            return CheckResult{Failed} // fake failure, so all records go to fixer
        }
        // ... the rest of your normal code, unchanged
    }
    func (i *invariant) Fix(...) {
        // print the fix rather than doing it, so the next runs try too.
        // or do the `if x {` trick here too
    }
    ```
- Your IDE, to launch with only the worker, as other services are not necessary
  and it will just slow down startup/shutdown.
  - Just make sure you have `start --services worker` in its start-args.

### Running it all and checking the results

Add some breakpoints or print statements, run it and see what happens :)

If Scanner found anything interesting, you should now have a `/tmp/blobstore` folder with
files like `{uuid()}_0.corrupted`.  These uuids are _random_, and the `_0.corrupted` suffix
marks them as page 0 (out of N), and that they refer to corrupted entries.  You'll have
one uuid per Scanner shard (configurable, for concurrency) that found issues, and multiple
pages per shard if the results exceeded the paging size limit.

If Fixer found anything, `/tmp/blobstore` should now have `{uuid()}_0.skipped` and/or
`{uuid()}_0.fixed` files.  These uuids are _also random_, and do not refer to the Scanner
file that their data came from, and the uuid and paging patterns are the same as Scanner.

You may also have `*.failed` files, following the same pattern.  Similarly, these are cases
where an invariant returned a Failure result (they can come from scanner _or_ fixer).
Only `*.corrupted` files from scanner will be processed by fixer, however.

Note that `*.failed` files can contain invariant results of _all_ statuses, as the status
of a record _trends towards_ "failed", and only the final status is recorded.
For details, see the behavior in the [InvariantManger](../../../common/reconciliation/invariant/invariantManager.go).

You can also see the results in the scanner and fixer workflows in the UI.  In particular:
- Scanner:
  - Each scanner type has a unique ID, like `concreteExecutionsScannerWFID = "cadence-sys-executions-scanner"`.
  - Check the activities to see how many corruptions were found per shard
  - Query its `aggregate_report` (works in UI, others require arguments and you currently
    need to use the CLI) to get overall counts.
  - The activities return results with UUIDs and page numbers.
    These are the UUIDs and page ranges for files in `/tmp/blobstore`, so you can look up
    specific detailed results.
    - Otherwise tbh I grab a known workflow ID and grep the most-recent batch of files for it.
- Fixer:
  - Check the recent fixer workflows for fix results.
    - If it has completed already, it is likely _not_ the most-recent or the currently running one.
      Check older ones until you find some with more than ~ a dozen events, as those are no-ops.
  - The activities accept UUIDs and page ranges from scanner (these match the return values in
    scanner, and refer to the scan-result files in `/tmp/blobstore`), and return the same kind of
    structure (new random UUIDs, new page ranges, referring to new files in `/tmp/blobstore`).
    - Again, I would also recommend just grepping for known IDs that should have been processed.

If you are not printing or debugging whatever info you are looking for, check the
contents of these files to verify they're doing what you expect!

#### An example working scanner/fixer setup, visible in /tmp/blobstore files

This is the new "stale" invariant working in its concrete execution scanner -> the
concrete execution fixer also working, with faked results to simplify testing.
I also ran all the other concrete invariants because I was curious.

I made a change like this:
```go
func (c *staleWorkflowCheck) Check(
	ctx context.Context,
	execution interface{},
) CheckResult {
	x := true
	if x {
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   c.Name(),
			Info:            "fake corrupt",
		}
	}
	_, result := c.check(ctx, execution)
	return result
}
```
so the `Check` call would always fail, and the `Fix` call would run the real check.

And added dynamic config like this:
```yaml
# enable these invariants
worker.executionsScannerInvariantCollectionStale:
  - value: true         # default false
worker.executionsScannerInvariantCollectionMutableState:
  - value: true         # default true
worker.executionsScannerInvariantCollectionHistory:
  - value: true         # default true
worker.executionsFixerInvariantCollectionStale:
  - value: true         # default false
worker.executionsFixerInvariantCollectionMutableState:
  - value: true         # default true
worker.executionsFixerInvariantCollectionHistory:
  - value: true         # default true

# these invariants are all run by the concrete execution scanner/fixer
worker.executionsScannerEnabled:  # note slightly different name
  - value: true         # default false
worker.concreteExecutionFixerEnabled:
  - value: true         # default false
worker.concreteExecutionFixerDomainAllow:
  - value: true         # default false
```

After a scanner and fixer run, `/tmp/blobstore` contained `*.corrupted` and `*.skipped` files.
The `*.corrupted` files contained data like this:
```json
{
  "Input": {
    "Execution": { ... },
    "Result": {
      "CheckResultType": "corrupted",
      "DeterminingInvariantType": "stale_workflow",
      "CheckResults": [
        {
          "CheckResultType": "healthy",
          "InvariantName": "history_exists",
          "Info": "",
          "InfoDetails": ""
        },
        {
          "CheckResultType": "healthy",
          "InvariantName": "open_current_execution",
          "Info": "",
          "InfoDetails": ""
        },
        {
          "CheckResultType": "corrupted",
          "InvariantName": "stale_workflow",
          "Info": "fake corrupt",
          "InfoDetails": ""
        }
      ]
    }
  },
  "Result": {
    "FixResultType": "skipped",
    "DeterminingInvariantName": null,
    "FixResults": null
  }
}
```
You can see the two healthy invariants, and the one I faked.

When fixer then ran I got this in a `*.skipped` file:
```json
{
  "Execution": { ... },
  "Input": {
    "Execution": { ... },
    "Result": {
      "CheckResultType": "corrupted",
      "DeterminingInvariantType": "stale_workflow",
      "CheckResults": [
        {
          "CheckResultType": "healthy",
          "InvariantName": "history_exists",
          "Info": "",
          "InfoDetails": ""
        },
        {
          "CheckResultType": "healthy",
          "InvariantName": "open_current_execution",
          "Info": "",
          "InfoDetails": ""
        },
        {
          "CheckResultType": "corrupted",
          "InvariantName": "stale_workflow",
          "Info": "fake corrupt",
          "InfoDetails": ""
        }
      ]
    }
  },
  "Result": {
    "FixResultType": "skipped",
    "DeterminingInvariantName": null,
    "FixResults": [
      {
        "FixResultType": "skipped",
        "InvariantName": "history_exists",
        "CheckResult": {
          "CheckResultType": "healthy",
          "InvariantName": "history_exists",
          "Info": "",
          "InfoDetails": ""
        },
        "Info": "skipped fix because execution was healthy",
        "InfoDetails": ""
      },
      {
        "FixResultType": "skipped",
        "InvariantName": "open_current_execution",
        "CheckResult": {
          "CheckResultType": "healthy",
          "InvariantName": "open_current_execution",
          "Info": "",
          "InfoDetails": ""
        },
        "Info": "skipped fix because execution was healthy",
        "InfoDetails": ""
      },
      {
        "FixResultType": "skipped",
        "InvariantName": "stale_workflow",
        "CheckResult": {
          "CheckResultType": "",
          "InvariantName": "",
          "Info": "",
          "InfoDetails": ""
        },
        "Info": "no need to fix: completed workflow still within retention + 10-day buffer",
        "InfoDetails": "completed workflow still within retention + 10-day buffer, closed 2023-09-20 20:26:04.924876012 -0500 CDT and allowed to exist until 2023-10-07"
      }
    ]
  }
}
```
Notice that _all three_ invariants ran in the fixer (all three were enabled), and
all three fixes were skipped because they did not find any problems.

If I had also faked the stale workflow invariant `Fix`, you would see a `FixResultType`
of "fixed" on that invariant in fixer, and a file named `*.fixed` rather than `*.skipped`.

#### An example bad scanner/fixer setup

This is the current-execution scanner working -> the current-execution fixer misbehaving
and using the wrong type, and creating `*.failed` files, as of commit `eb55629d`.

I faked the current execution invariant to always say "corrupt" in `Check`, and panic in `Fix`,
and enabled the current execution scanner and fixer in dynamic config, and ran the worker.

First, the scanner run creates `*.corrupted` files with entries like this:
```json
{
  "Input": {
    "Execution": { ... },
    "Result": {
      "CheckResultType": "corrupted",
      "DeterminingInvariantType": "concrete_execution_exists",
      "CheckResults": [
        {
          "CheckResultType": "corrupted",
          "InvariantName": "concrete_execution_exists",
          "Info": "execution is open without having concrete execution",
          "InfoDetails": "concrete execution not found. WorkflowId: e905c98f-108a-4191-9ef2-ca07a1361f9c, RunId: 6bc5386b-c043-4eb1-a332-c3bb7b5188f0"
        }
      ]
    }
  },
  "Result": {
    "FixResultType": "skipped",
    "DeterminingInvariantName": null,
    "FixResults": null
  }
}
```
This shows that it identified my "always corrupt" results correctly, in the
[concrete_execution_exists invariant](../../../common/reconciliation/invariant/concreteExecutionExists.go).

This is later consumed by the current execution fixer, which produces `*.failed` files with
contents like this:
```json
{
  "Execution": { ... },
  "Input": {
    "Execution": { ... },
    "Result": {
      "CheckResultType": "corrupted",
      "DeterminingInvariantType": "concrete_execution_exists",
      "CheckResults": [
        {
          "CheckResultType": "corrupted",
          "InvariantName": "concrete_execution_exists",
          "Info": "execution is open without having concrete execution",
          "InfoDetails": "concrete execution not found. WorkflowId: e905c98f-108a-4191-9ef2-ca07a1361f9c, RunId: 6bc5386b-c043-4eb1-a332-c3bb7b5188f0"
        }
      ]
    }
  },
  "Result": {
    "FixResultType": "failed",
    "DeterminingInvariantName": "history_exists",
    "FixResults": [
      {
        "FixResultType": "failed",
        "InvariantName": "history_exists",
        "CheckResult": {
          "CheckResultType": "failed",
          "InvariantName": "history_exists",
          "Info": "failed to check: expected concrete execution",
          "InfoDetails": ""
        },
        "Info": "failed fix because check failed",
        "InfoDetails": ""
      },
      {
        "FixResultType": "failed",
        "InvariantName": "open_current_execution",
        "CheckResult": {
          "CheckResultType": "failed",
          "InvariantName": "open_current_execution",
          "Info": "failed to check: expected concrete execution",
          "InfoDetails": ""
        },
        "Info": "failed fix because check failed",
        "InfoDetails": ""
      }
    ]
  }
}
```
You can see scanner's data as the input-result, and _completely different invariants_
running and failing as the fixer result.

## Thoughts and prayers



Most notably:
- The current implementation is extremely difficult to extend externally, as the
  code depends heavily on constants that cannot be added to or changed.
  - Changing this likely requires rewriting a significant amount of the code, but
    does seem worth doing.
  - Future versions really should try to fix this.  Custom database plugins may
    have unique problems and need unique scan/fix tools, and those are unlikely
    to be open-source friendly and useful to run on _every_ database.
- The current-execution _fixer_ has never been run successfully anywhere.
  Beware drawing any conclusions from its code or config.
  - The scanner appears to work, and the invariant seems correct, but the fixer
    was written to use the _concrete_ execution invariants, and that has caused
    it to always fail when run.
  - This might be fixed and (optionally) enabled or deleted in the future.
- Current, concrete, and timer scan/fix code is _very similar_ but not identical.
  - Maybe this is good, maybe not, but it can be confusing.  Read carefully.
- Config keys, configurability, all things config-like vary widely between scan/fix implementations.
  - Copy/paste, don't guess.  And verify it before calling it "done".
- The current "scan everything, then fix everything" structure is problematically slow
  on large clusters, and quite resource-wasteful due to doing everything at least 2x.
  - Generally speaking it's probably not a bad thing to slow it down and re-check, but
    large clusters can take _weeks_ before fixer starts.  That's horrible for verifying
    fixes, or addressing any issues quickly.
- Queries feel like an odd way to pass data from scanner to fixer.
  - I _suspect_ they were done to avoid returning too much data, violating a blob-size
    limit constraint.  This... might be valid?  The data it actually uses is saved by
    fixer's querying activity though, so it is still bound by that limit.
  - Due to results being spread between many activities and many queries, overviews
    are hard to get.  We could instead push it all to a new blobstore file, for human use.
- There's a lot of indirection in general for unknown reasons, and that makes control-flow
  extremely hard to figure out.
  - Possibly it was intended for more flexibility than it has seen, possibly it would
    simply benefit from some modernizing (instance fields rather than background
    activity context), possibly it's a functional-like lament about the lack of
    generics in Go when it was written, I'm really not sure.

Overall it seems like it probably deserves a rewrite, though some of the parts are
pretty clearly reusable (invariants, iterators, etc).

---

[*]: may not actually be brief
