# Running tests

## Testing the build locally
To try out the build locally, start from the root folder of this repo 
(cadence) and run the following commands.

unit tests:
```bash
docker compose -f docker/github_actions/docker-compose-local.yml build unit-test
```

NOTE: You would expect TestServerStartup to fail here as we don't have a way to install the schema like we do in pipeline.yml 

integration tests:
```bash
docker compose -f docker/github_actions/docker-compose-local.yml build integration-test-cassandra
```

cross DC integration tests:
```bash
docker compose -f docker/github_actions/docker-compose-local.yml build integration-test-ndc-cassandra
```

Run the integration tests:

unit tests:
```bash
docker compose -f docker/github_actions/docker-compose-local.yml run unit-test
```

integration tests:
```bash
docker compose -f docker/github_actions/docker-compose-local.yml run integration-test-cassandra
```

cross DC integration tests:
```bash
docker compose -f docker/github_actions/docker-compose-local.yml run integration-test-ndc-cassandra
```

## Testing the build in Github Actions
Creating a PR against the master branch will trigger the Github Actions
build. Members of the Cadence team can view the build pipeline here on Github.

Eventually this pipeline should be made public. It will need to ignore 
third party PRs for safety reasons.
