#!/bin/bash

# exit immediately on failure, or if an undefined variable is used
set -eu

if [[ $BUILDKITE_BRANCH = 'master' ]]
then
    cat .buildkite/pipeline-master.yml
else
    cat .buildkite/pipeline-pull-request.yml
fi

# TODO(bk-cleanup): Remove this file after fully migrated to github actions