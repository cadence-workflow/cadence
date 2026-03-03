FROM golang:1.23.4-bullseye

# Tried to set Python to ignore warnings due to the instructions at this link:
# https://github.com/yaml/pyyaml/wiki/PyYAML-yaml.load(input)-Deprecation
# But this causes all the pip installs to fail, so don't do this:
# ENV PYTHONWARNINGS=ignore::yaml.YAMLLoadWarning
# ENV PYTHONWARNINGS=ignore

RUN apt-get update && apt-get install -y --no-install-recommends \
      curl \
      gettext-base \
      libyaml-dev \
      python3-pip \
      python-setuptools \
      time \
      unzip \
      ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN pip3 install cqlsh && cqlsh --version

# verbose test output from `make`, can be disabled with V=0
ENV V=0

# allow git-status and similar to work
RUN git config --global --add safe.directory /cadence

# https://github.com/docker-library/golang/blob/c1baf037d71331eb0b8d4c70cff4c29cf124c5e0/1.4/Dockerfile
RUN mkdir -p /cadence
WORKDIR /cadence

# Copy go mod dependencies and try to share the module download cache
COPY go.* /cadence
COPY internal/tools/go.* /cadence/internal/tools/
COPY cmd/server/go.* /cadence/cmd/server/
COPY common/archiver/gcloud/go.* /cadence/common/archiver/gcloud/
# go.work means this downloads everything, not just the top module
RUN go mod download
