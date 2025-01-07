# Cadence
[![Build Status](https://badge.buildkite.com/159887afd42000f11126f85237317d4090de97b26c287ebc40.svg?theme=github&branch=master)](https://buildkite.com/uberopensource/cadence-server)
[![Coverage](https://codecov.io/gh/cadence-workflow/cadence/graph/badge.svg?token=7SD244ImNF)](https://codecov.io/gh/cadence-workflow/cadence)
[![Slack Status](https://img.shields.io/badge/slack-join_chat-white.svg?logo=slack&style=social)](http://t.uber.com/cadence-slack)

[![Github release](https://img.shields.io/github/v/release/cadence-workflow/cadence.svg)](https://github.com/cadence-workflow/cadence/releases)
[![License](https://img.shields.io/github/license/cadence-workflow/cadence.svg)](http://www.apache.org/licenses/LICENSE-2.0)

[![GitHub stars](https://img.shields.io/github/stars/cadence-workflow/cadence.svg?style=social&label=Star&maxAge=2592000)](https://github.com/cadence-workflow/cadence/stargazers/)
[![GitHub forks](https://img.shields.io/github/forks/cadence-workflow/cadence.svg?style=social&label=Fork&maxAge=2592000)](https://github.com/cadence-workflow/cadence/network/)


This repo contains the source code of the Cadence server and other tooling including CLI, schema tools, bench and canary.

You can implement your workflows with one of our client libraries.
The [Go](https://github.com/cadence-workflow/cadence-go-client) and [Java](https://github.com/cadence-workflow/cadence-java-client) libraries are officially maintained by the Cadence team,
while the [Python](https://github.com/firdaus/cadence-python) and [Ruby](https://github.com/coinbase/cadence-ruby) client libraries are developed by the community.

You can also use [iWF](https://github.com/indeedeng/iwf) as a DSL framework on top of Cadence.

See Maxim's talk at [Data@Scale Conference](https://atscaleconference.com/videos/cadence-microservice-architecture-beyond-requestreply) for an architectural overview of Cadence.

Visit [cadenceworkflow.io](https://cadenceworkflow.io) to learn more about Cadence. Join us in [Cadence Documentation](https://github.com/cadence-workflow/Cadence-Docs) project. Feel free to raise an Issue or Pull Request there.

### Community
* [Github Discussion](https://github.com/cadence-workflow/cadence/discussions)
  * Best for Q&A, support/help, general discusion, and annoucement
* [StackOverflow](https://stackoverflow.com/questions/tagged/cadence-workflow)
  * Best for Q&A and general discusion
* [Github Issues](https://github.com/cadence-workflow/cadence/issues)
  * Best for reporting bugs and feature requests
* [Slack](http://t.uber.com/cadence-slack)
  * Best for contributing/development discussion

## Getting Started

### Start the cadence-server

To run Cadence services locally, we highly recommend that you use [Cadence service docker](docker/README.md) to run the service.
You can also follow the [instructions](./CONTRIBUTING.md) to build and run it.

Please visit our [documentation](https://cadenceworkflow.io/docs/operation-guide/) site for production/cluster setup.

### Run the Samples

Try out the sample recipes for [Go](https://github.com/cadence-workflow/cadence-samples) or [Java](https://github.com/cadence-workflow/cadence-java-samples) to get started.

### Use [Cadence CLI](https://cadenceworkflow.io/docs/cli/)

Cadence CLI can be used to operate workflows, tasklist, domain and even the clusters.

You can use the following ways to install Cadence CLI:
* Use brew to install CLI: `brew install cadence-workflow`
  * Follow the [instructions](https://github.com/cadence-workflow/cadence/discussions/4457) if you need to install older versions of CLI via homebrew. Usually this is only needed when you are running a server of a too old version.
* Use docker image for CLI: `docker run --rm ubercadence/cli:<releaseVersion>`  or `docker run --rm ubercadence/cli:master ` . Be sure to update your image when you want to try new features: `docker pull ubercadence/cli:master `
* Build the CLI binary yourself, check out the repo and run `make cadence` to build all tools. See [CONTRIBUTING](CONTRIBUTING.md) for prerequisite of make command.
* Build the CLI image yourself, see [instructions](docker/README.md#diy-building-an-image-for-any-tag-or-branch)

Cadence CLI is a powerful tool. The commands are organized by **tabs**. E.g. `workflow`->`batch`->`start`, or `admin`->`workflow`->`describe`.

Please read the [documentation](https://cadenceworkflow.io/docs/cli/#documentation) and always try out `--help` on any tab to learn & explore.

### Use Cadence Web

Try out [Cadence Web UI](https://github.com/cadence-workflow/cadence-web) to view your workflows on Cadence.
(This is already available at localhost:8088 if you run Cadence with docker compose)


## Contributing

We'd love your help in making Cadence great. Please review our [contribution guide](CONTRIBUTING.md).

If you'd like to propose a new feature, first join the [Slack channel](http://t.uber.com/cadence-slack) to start a discussion and check if there are existing design discussions. Also peruse our [design docs](docs/design/index.md) in case a feature has been designed but not yet implemented. Once you're sure the proposal is not covered elsewhere, please follow our [proposal instructions](PROPOSALS.md).

## Other binaries in this repo

#### Bench/stress test workflow tools
See [bench documentation](./bench/README.md).

#### Periodical feature health check workflow tools(aka Canary)
See [canary documentation](./canary/README.md).

#### Schema tools for SQL and Cassandra
The tools are for [manual setup or upgrading database schema](docs/persistence.md)

  * If server runs with Cassandra, Use [Cadence Cassandra tool](tools/cassandra/README.md)
  * If server runs with SQL database, Use [Cadence SQL tool](tools/sql/README.md)

The easiest way to get the schema tool is via homebrew.

`brew install cadence-workflow` also includes `cadence-sql-tool` and `cadence-cassandra-tool`.
 * The schema files are located at `/usr/local/etc/cadence/schema/`.
 * To upgrade, make sure you remove the old ElasticSearch schema first: `mv /usr/local/etc/cadence/schema/elasticsearch /usr/local/etc/cadence/schema/elasticsearch.old && brew upgrade cadence-workflow`. Otherwise ElasticSearch schemas may not be able to get updated.
 * Follow the [instructions](https://github.com/cadence-workflow/cadence/discussions/4457) if you need to install older versions of schema tools via homebrew.
 However, easier way is to use new versions of schema tools with old versions of schemas.
 All you need is to check out the older version of schemas from this repo. Run `git checkout v0.21.3` to get the v0.21.3 schemas in [the schema folder](/schema).


## Stargazers over time
[![Stargazers over time](https://starchart.cc/uber/cadence.svg?variant=adaptive)](https://starchart.cc/uber/cadence)


## License

MIT License, please see [LICENSE](https://github.com/cadence-workflow/cadence/blob/master/LICENSE) for details.
