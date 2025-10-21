# Developing Cadence

This doc is intended for contributors to Cadence backend. Thanks for considering to contribute ❤️

> 📚 **New to contributing to Cadence?** Check out our [Contributing Guide](https://cadenceworkflow.io/community/how-to-contribute/getting-started) for an overview of the contribution process across all Cadence repositories. This document contains cadence backend specific setup and development instructions.

Once you go through the rest of this doc and get familiar with local development setup, take a look at the list of issues labeled with
[good first issue](https://github.com/cadence-workflow/cadence/labels/good%20first%20issue).
These issues are a great way to start contributing to Cadence. Later when you are more familiar with Cadence, look at issues with
[up-for-grabs](https://github.com/cadence-workflow/cadence/labels/up-for-grabs).

Join our community on the CNCF Slack workspace at [cloud-native.slack.com](https://communityinviter.com/apps/cloud-native/cncf) in the **#cadence-users** channel to reach out and discuss issues with the team.

## Development Environment
Below are the instructions of how to set up a development Environment.

### 1. Building Environment

* Golang. Install on OS X with.
```
brew install go
```
* Make sure set PATH to include bin path of GO so that other executables like thriftrw can be found.
```bash
# check it first
echo $GOPATH
# append to PATH
PATH=$PATH:$GOPATH/bin
# to confirm, run
echo $PATH
```

* Download golang dependencies.
```bash
go mod download
```

After check out and go to the Cadence repo, compile the `cadence` service and helper tools without running test:

```bash
git submodule update --init --recursive

make bins
```

You should be able to get all the binaries of this repo:
* cadence-server: the server binary
* cadence: the CLI binary
* cadence-cassandra-tool: the Cassandra schema tools
* cadence-sql-tool: the SQL schema tools(for now only supports MySQL and Postgres)
* cadence-canary: the canary test binary
* cadence-bench: the benchmark test binary


:warning: Note:

If running into any compiling issue
>1. For proto/thrift errors, run `git submodule update --init --recursive` to fix
>2. Make sure you upgrade to the latest stable version of Golang.
>3. Check if this document is outdated by comparing with the building steps in [Dockerfile](https://github.com/cadence-workflow/cadence/blob/master/Dockerfile)

### 2. Setup Dependency
NOTE: you may skip this section if you want to use SQLite or you have installed the dependencies in any other ways, for example, using homebrew.

Cadence's core data model can be running with different persistence storages, including Cassandra,MySQL and Postgres.
Please refer to [persistence documentation](https://github.com/cadence-workflow/cadence/blob/master/docs/persistence.md) if you want to learn more.
Cadence's visibility data model can be running with either Cassandra/MySQL/Postgres database, or ElasticSearch+Kafka. The latter provides [advanced visibility feature](./docs/visibility-on-elasticsearch.md)

We recommend to use [docker-compose](https://docs.docker.com/compose/) to start those dependencies:

* If you want to start Cassandra dependency, use `./docker/dev/cassandra.yml`:
```
docker compose -f ./docker/dev/cassandra.yml up
```
You will use `CTRL+C` to stop it. Then `docker compose -f ./docker/dev/cassandra.yml down` to clean up the resources.

Or to run in the background
```
docker compose -f ./docker/dev/cassandra.yml up -d
```
Also use `docker compose -f ./docker/dev/cassandra.yml down` to stop and clean up the resources.

* Alternatively, use `./docker/dev/mysql.yml` for MySQL dependency. (MySQL has been updated from 5.7 to 8.0)
* Alternatively, use `./docker/dev/postgres.yml` for PostgreSQL dependency
* Alternatively, use `./docker/dev/cassandra-esv7-kafka.yml` for Cassandra, ElasticSearch(v7) and Kafka/ZooKeeper dependencies
* Alternatively, use `./docker/dev/mysql-esv7-kafka.yml` for MySQL, ElasticSearch(v7) and Kafka/ZooKeeper dependencies
* Alternatively, use `./docker/dev/cassandra-opensearch-kafka.yml` for Cassandra, OpenSearch(compatible with ElasticSearch v7) and Kafka/ZooKeeper dependencies
* Alternatively, use `./docker/dev/mongo-esv7-kafka.yml` for MongoDB, ElasticSearch(v7) and Kafka/ZooKeeper dependencies

### 3. Schema installation
Based on the above dependency setup, you also need to install the schemas.

* If you use SQLite then run `make install-schema-sqlite` to install SQLite schemas
* If you use `cassandra.yml` then run `make install-schema` to install Cassandra schemas
* If you use `cassandra-esv7-kafka.yml` then run `make install-schema && make install-schema-es-v7` to install Cassandra & ElasticSearch schemas
* If you use `cassandra-opensearch-kafka.yml` then run `make install-schema && make install-schema-es-opensearch` to install Cassandra & OpenSearch schemas
* If you use `mysql.yml` then run `install-schema-mysql` to install MySQL schemas
* If you use `postgres.yml` then run `install-schema-postgres` to install Postgres schemas
* `mysql-esv7-kafka.yml` can be used for single MySQL + ElasticSearch or multiple MySQL + ElasticSearch mode
  * for single MySQL: run `install-schema-mysql && make install-schema-es-v7`
  * for multiple MySQL: run `make install-schema-multiple-mysql` which will install schemas for 4 mysql databases and ElasticSearch

:warning: Note:
>If you use `cassandra-esv7-kafka.yml` and start server before `make install-schema-es-v7`, ElasticSearch may create a wrong index on demand.
You will have to delete the wrong index and then run the `make install-schema-es-v7` again. To delete the wrong index:
```
curl -X DELETE "http://127.0.0.1:9200/cadence-visibility-dev"
```

### 4. Run
Once you have done all above, try running the local binaries:


Then you will be able to run a basic local Cadence server for development.

  * If you use SQLite, then run `./cadence-server --zone sqlite start`, which load , which will load `config/development.yaml` + `config/development_sqlite.yaml` as config
  * If you use `cassandra.yml`, then run `./cadence-server start`, which will load `config/development.yaml` as config
  * If you use `mysql.yml` then run `./cadence-server --zone mysql start`, which will load `config/development.yaml` + `config/development_mysql.yaml` as config
  * If you use `postgres.yml` then run `./cadence-server --zone postgres start` , which will load `config/development.yaml` + `config/development_postgres.yaml` as config
  * If you use `cassandra-esv7-kafka.yml` then run `./cadence-server --zone es_v7 start`, which will load `config/development.yaml` + `config/development_es_v7.yaml` as config
  * If you use `cassandra-opensearch-kafka.yml` then run `./cadence-server --zone es_opensearch start` , which will load `config/development.yaml` + `config/development_es_opensearch.yaml` as config
  * If you use `mysql-esv7-kafka.yaml`
    * To run with multiple MySQL : `./cadence-server --zone multiple_mysql start`, which will load `config/development.yaml` + `config/development_multiple_mysql.yaml` as config

Then register a domain:
```
./cadence --do samples-domain domain register
```

### Sample Repo
The sample code is available in the [Sample repo](https://github.com/cadence-workflow/cadence-samples)

Then run a helloworld from [Go Client Sample](https://github.com/cadence-workflow/cadence-samples) or [Java Client Sample](https://github.com/cadence-workflow/cadence-java-samples)

```
make bins
```
will build all the samples.

Then
```
./bin/helloworld -m worker &
```
will start a worker for helloworld workflow.
You will see like:
```
$./bin/helloworld -m worker &
[1] 16520
2021-09-24T21:07:03.242-0700	INFO	common/sample_helper.go:109	Logger created.
2021-09-24T21:07:03.243-0700	DEBUG	common/factory.go:151	Creating RPC dispatcher outbound	{"ServiceName": "cadence-frontend", "HostPort": "127.0.0.1:7933"}
2021-09-24T21:07:03.250-0700	INFO	common/sample_helper.go:161	Domain successfully registered.	{"Domain": "samples-domain"}
2021-09-24T21:07:03.291-0700	INFO	internal/internal_worker.go:833	Started Workflow Worker	{"Domain": "samples-domain", "TaskList": "helloWorldGroup", "WorkerID": "16520@IT-USA-25920@helloWorldGroup"}
2021-09-24T21:07:03.300-0700	INFO	internal/internal_worker.go:858	Started Activity Worker	{"Domain": "samples-domain", "TaskList": "helloWorldGroup", "WorkerID": "16520@IT-USA-25920@helloWorldGroup"}
```
Then
```
./bin/helloworld
```
to start a helloworld workflow.
You will see the result like :
```
$./bin/helloworld
2021-09-24T21:07:06.220-0700	INFO	common/sample_helper.go:109	Logger created.
2021-09-24T21:07:06.220-0700	DEBUG	common/factory.go:151	Creating RPC dispatcher outbound	{"ServiceName": "cadence-frontend", "HostPort": "127.0.0.1:7933"}
2021-09-24T21:07:06.226-0700	INFO	common/sample_helper.go:161	Domain successfully registered.	{"Domain": "samples-domain"}
2021-09-24T21:07:06.272-0700	INFO	common/sample_helper.go:195	Started Workflow	{"WorkflowID": "helloworld_75cf142b-c0de-407e-9115-1d33e9b7551a", "RunID": "98a229b8-8fdd-4d1f-bf41-df00fb06f441"}
2021-09-24T21:07:06.347-0700	INFO	helloworld/helloworld_workflow.go:31	helloworld workflow started	{"Domain": "samples-domain", "TaskList": "helloWorldGroup", "WorkerID": "16520@IT-USA-25920@helloWorldGroup", "WorkflowType": "helloWorldWorkflow", "WorkflowID": "helloworld_75cf142b-c0de-407e-9115-1d33e9b7551a", "RunID": "98a229b8-8fdd-4d1f-bf41-df00fb06f441"}
2021-09-24T21:07:06.347-0700	DEBUG	internal/internal_event_handlers.go:489	ExecuteActivity	{"Domain": "samples-domain", "TaskList": "helloWorldGroup", "WorkerID": "16520@IT-USA-25920@helloWorldGroup", "WorkflowType": "helloWorldWorkflow", "WorkflowID": "helloworld_75cf142b-c0de-407e-9115-1d33e9b7551a", "RunID": "98a229b8-8fdd-4d1f-bf41-df00fb06f441", "ActivityID": "0", "ActivityType": "main.helloWorldActivity"}
2021-09-24T21:07:06.437-0700	INFO	helloworld/helloworld_workflow.go:62	helloworld activity started	{"Domain": "samples-domain", "TaskList": "helloWorldGroup", "WorkerID": "16520@IT-USA-25920@helloWorldGroup", "ActivityID": "0", "ActivityType": "main.helloWorldActivity", "WorkflowType": "helloWorldWorkflow", "WorkflowID": "helloworld_75cf142b-c0de-407e-9115-1d33e9b7551a", "RunID": "98a229b8-8fdd-4d1f-bf41-df00fb06f441"}
2021-09-24T21:07:06.513-0700	INFO	helloworld/helloworld_workflow.go:55	Workflow completed.	{"Domain": "samples-domain", "TaskList": "helloWorldGroup", "WorkerID": "16520@IT-USA-25920@helloWorldGroup", "WorkflowType": "helloWorldWorkflow", "WorkflowID": "helloworld_75cf142b-c0de-407e-9115-1d33e9b7551a", "RunID": "98a229b8-8fdd-4d1f-bf41-df00fb06f441", "Result": "Hello Cadence!"}
```

See [instructions](service/worker/README.md) for setting up replication(XDC).

## Repository layout
A Cadence server cluster is composed of four different services: Frontend, Matching, History and Worker(system).
Here's what's in each top-level directory in this repository:

* **canary/** : The test code that needs to run periodically to ensure Cadence is healthy
* **client/** : Client wrappers to let the four different services talk to each other
* **cmd/** : The main function to build binaries for servers and CLI tools
* **config/** : Sample configuration files
* **docker/** : Code/scripts to build docker images
* **docs/** : Documentation
* **host/** : End-to-end integration tests
* **schema/** : Versioned persistence schema for Cassandra/MySQL/Postgres/ElasticSearch
* **scripts/** : Scripts for CI build
* **tools/** : CLI tools for Cadence workflows and also schema updates for persistence
* **service/** : Contains four sub-folders that dedicated for each of the four services
* **common/** :  Basically contains all the rest of the code in Cadence server, the names of the sub folder are the topics of the packages

If looking at the Git history of the file/package, there should be some engineers focusing on each area/package/component that you are working on.
You will probably get code review from them. Don't hesitate to ask for some early feedback or help on Slack.

## Testing & Debug

To run all the package tests use the below command.
This will run all the tests excluding end-to-end integration test in host/ package):

```bash
make test
```
:warning: Note:
> You will see some test failures because of errors connecting to MySQL/Postgres if only Cassandra is up. This is okay if you don't write any code related to persistence layer.

To run all end-to-end integration tests in **host/** package:
```bash
make test_e2e
```

To debug a specific test case when you see some failure, you can trigger it from an IDE, or use the command
```
go test -v <path> -run <TestSuite> -testify.m <TestSpercificTaskName>
# example:
go test -v github.com/uber/cadence/common/persistence/persistence-tests -run TestVisibilitySamplingSuite -testify.m TestListClosedWorkflowExecutions
```

## IDL Changes

If you make changes in the idls submodule and want to test them locally, you can easily do that by using go mod to use the local idls directory instead of github.com/uber/cadence-idl. Temporarily add the following to the bottom of go.mod:

```replace github.com/uber/cadence-idl => ./idls```

Or alternatively, push your idl changes to a branch in your own fork of idl repo and pull that specific commit to idl submodule. Then apply the go.mod change mentioned above.

### Using IDL Changes

Once your [Pull Request](#pull-requests) has been successfully merged you can update cadence to use the new version:

```bash
# Update the IDLs directory
git submodule foreach git pull origin master
# Update go to use the latest idl package
go get github.com/uber/cadence-idl@latest
```

## Pull Requests
After all the preparation you are about to write code and make a Pull Request for the issue.

### When to open a PR
You have a few options for choosing when to submit:

* You can open a PR with an initial prototype with "Draft" option or with "WIP"(work in progress) in the title. This is useful when want to get some early feedback.

* PR is supposed to be or near production ready. You should have fixed all styling, adding new tests if possible, and verified the change doesn't break any existing tests.

* For small changes where the approach seems obvious, you can open a PR with what you believe to be production-ready or near-production-ready code. As you get more experience with how we develop code, you'll find that more PRs will begin falling into this category.

### Commit Messages And Titles of Pull Requests

Overcommit adds some requirements to your commit messages. At Uber, we follow the
[Chris Beams](http://chris.beams.io/posts/git-commit/) guide to writing git
commit messages. Read it, follow it, learn it, love it.

All commit messages are from the titles of your pull requests. So make sure follow the rules when titling them.
Please don't use very generic titles like "bug fixes".

All PR titles should start with UPPER case.

Examples:

- [Make sync activity retry multiple times before fetch history from remote](https://github.com/cadence-workflow/cadence/pull/1379)
- [Enable archival config per domain](https://github.com/cadence-workflow/cadence/pull/1351)

#### Code Format and Licence headers checking

The project has strict rule about Golang format, import ordering and license declarations. You have to run
```bash
make pr
```
which will take care of formatting for you.


### Code review
We take code reviews very seriously at Cadence. Please don't be deterred if you feel like you've received some hefty feedback. That's completely normal and expected—and, if you're an external contributor, we very much appreciate your contribution!

In this repository in particular, merging a PR means accepting responsibility for maintaining that code for, quite possibly, the lifetime of Cadence. To take on that reponsibility, we need to ensure that meets our strict standards for production-ready code.

No one is expected to write perfect code on the first try. That's why we have code reviews in the first place!

Also, don't be embarrassed when your review points out syntax errors, stray whitespace, typos, and missing docstrings! That's why we have reviews. These properties are meant to guide you in your final scan.

### Addressing feedback
If someone leaves line comments on your PR without leaving a top-level "looks good to me" (LGTM), it means they feel you should address their line comments before merging.

You should respond to all unresolved comments whenever you push a new revision or before you merge.

Also, as you gain confidence in Go, you'll find that some of the nitpicky style feedback you get does not make for obviously better code. Don't be afraid to stick to your guns and push back. Much of coding style is subjective.

### Merging
External contributors: you don't need to worry about this section. We'll merge your PR as soon as you've addressed all review feedback(you will get at least one approval) and pipeline runs are all successful(meaning all tests are passing).
