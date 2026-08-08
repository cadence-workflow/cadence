#!/bin/bash

set -eo pipefail

# add verbose env var for debugging
test -n "$V" && set -x

MODE=$1

if [ "$MODE" != "LDFLAG" ] && [ "$MODE" != "ECHO" ]; then
    echo "Usage: $0 <LDFLAG|ECHO>"
    exit 1
fi

GIT_REVISION=$(git log -1 --format=%cI-%h 2>/dev/null || echo unknown) # use commit date time and hash: e.g. 2021-07-27 19:36:53 -0700-40c5f1896, doc: https://git-scm.com/docs/pretty-formats
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)
BUILD_DATE=$(date '+%F-%T') # outputs something in this format 2017-08-21-18:58:45
BUILD_TS_UNIX=$(date '+%s') # second since epoch
BASE_PACKAGE=github.com/uber/cadence/common/metrics
if [ -z ${CADENCE_RELEASE_VERSION} ]; then
  # If not set CADENCE_RELEASE_VERSION, then use the most recent tag.
  RELEASE_VERSION=$(git describe --tags --abbrev=0 --dirty 2>/dev/null || echo unknown)
else
  # If passing a CADENCE_RELEASE_VERSION explicitly, then use it
  RELEASE_VERSION=${CADENCE_RELEASE_VERSION}
fi

if [ "$MODE" = "LDFLAG" ]; then
  LD_FLAGS="-X ${BASE_PACKAGE}.Revision=${GIT_REVISION} \
  -X ${BASE_PACKAGE}.Branch=${GIT_BRANCH}               \
  -X ${BASE_PACKAGE}.ReleaseVersion=${RELEASE_VERSION}             \
  -X ${BASE_PACKAGE}.BuildDate=${BUILD_DATE}            \
  -X ${BASE_PACKAGE}.BuildTimeUnix=${BUILD_TS_UNIX}"

  echo $LD_FLAGS
fi

if [ "$MODE" = "ECHO" ]; then
  cat <<EOF
BASE_PACKAGE=${BASE_PACKAGE}
GIT_REVISION=${GIT_REVISION}
GIT_BRANCH=${GIT_BRANCH}
GIT_VERSION=${GIT_VERSION}
BUILD_DATE=${BUILD_DATE}
BUILD_TS_UNIX=${BUILD_TS_UNIX}
EOF
fi
