ARG TARGET=server

# Can be used in case a proxy is necessary
ARG GOPROXY

# Build Cadence binaries
FROM golang:1.23.4-alpine3.21 AS builder

ARG RELEASE_VERSION

RUN apk add --update --no-cache ca-certificates make git curl mercurial unzip bash

WORKDIR /cadence

# Making sure that dependency is not touched
ENV GOFLAGS="-mod=readonly"

# Copy go mod dependencies and try to share the module download cache
COPY go.* ./
COPY cmd/server/go.* ./cmd/server/
COPY common/archiver/gcloud/go.* ./common/archiver/gcloud/
# go.work means this downloads everything, not just the top module
RUN go mod download

COPY . .
RUN rm -fr .bin .build idls

ENV CADENCE_RELEASE_VERSION=$RELEASE_VERSION

# don't do anything fancy, just build.  must be run separately, before building things.
RUN make .just-build
RUN CGO_ENABLED=0 make cadence-cassandra-tool cadence-sql-tool cadence cadence-server cadence-bench cadence-canary


# Download dockerize
FROM alpine:3.18 AS dockerize

# appears to require `docker buildx` or an explicit `--platform` at build time
ARG TARGETARCH

RUN apk add --no-cache openssl ca-certificates && update-ca-certificates

ENV DOCKERIZE_VERSION=v0.9.3
RUN wget https://github.com/jwilder/dockerize/releases/download/$DOCKERIZE_VERSION/dockerize-linux-$TARGETARCH-$DOCKERIZE_VERSION.tar.gz \
    && tar -C /usr/local/bin -xzvf dockerize-linux-$TARGETARCH-$DOCKERIZE_VERSION.tar.gz \
    && rm dockerize-linux-$TARGETARCH-$DOCKERIZE_VERSION.tar.gz \
    && echo "**** fix for host id mapping error ****" \
    && chown root:root /usr/local/bin/dockerize


# Alpine base image
FROM alpine:3.18 AS alpine

RUN apk add --update --no-cache ca-certificates tzdata bash curl

# set up nsswitch.conf for Go's "netgo" implementation
# https://github.com/gliderlabs/docker-alpine/issues/367#issuecomment-424546457
RUN [ -e /etc/nsswitch.conf ] && grep '^hosts: files dns' /etc/nsswitch.conf

SHELL ["/bin/bash", "-c"]


# Cadence server
FROM alpine AS cadence-server

ENV CADENCE_HOME=/etc/cadence
RUN mkdir -p /etc/cadence

COPY --from=dockerize /usr/local/bin/dockerize /usr/local/bin
COPY --from=builder /cadence/cadence-cassandra-tool /usr/local/bin
COPY --from=builder /cadence/cadence-sql-tool /usr/local/bin
COPY --from=builder /cadence/cadence /usr/local/bin
COPY --from=builder /cadence/cadence-server /usr/local/bin
COPY --from=builder /cadence/schema /etc/cadence/schema

COPY docker/entrypoint.sh /docker-entrypoint.sh
COPY config/dynamicconfig /etc/cadence/config/dynamicconfig
COPY config/credentials /etc/cadence/config/credentials
COPY docker/config_template.yaml /etc/cadence/config
COPY docker/start-cadence.sh /start-cadence.sh

WORKDIR /etc/cadence

ENV SERVICES="history,matching,frontend,worker"

EXPOSE 7933 7934 7935 7939
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD /start-cadence.sh


# All-in-one Cadence server (~450mb)
FROM cadence-server AS cadence-auto-setup

RUN apk add --update --no-cache ca-certificates py3-pip mysql-client
RUN pip3 install cqlsh && cqlsh --version

COPY docker/start.sh /start.sh
COPY docker/domain /etc/cadence/domain

CMD /start.sh

# Cadence CLI
FROM alpine AS cadence-cli

COPY --from=builder /cadence/cadence /usr/local/bin

ENTRYPOINT ["cadence"]

# Cadence Canary
FROM alpine AS cadence-canary

COPY --from=builder /cadence/cadence-canary /usr/local/bin
COPY --from=builder /cadence/cadence /usr/local/bin

CMD ["/usr/local/bin/cadence-canary", "--root", "/etc/cadence-canary", "start"]

# Cadence Bench
FROM alpine AS cadence-bench

COPY --from=builder /cadence/cadence-bench /usr/local/bin
COPY --from=builder /cadence/cadence /usr/local/bin

CMD ["/usr/local/bin/cadence-bench", "--root", "/etc/cadence-bench", "start"]

# Final image
FROM cadence-${TARGET}
