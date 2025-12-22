#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$ROOT_DIR/docker"
RUN_DIR="$ROOT_DIR/.run"
mkdir -p "$RUN_DIR"

ETCD_CMD="${ETCD_CMD:-etcd}"
ETCD_LOG="${ETCD_LOG:-$RUN_DIR/etcd.log}"
ETCD_PIDFILE="$RUN_DIR/etcd.pid"

CADENCE_LOG="${CADENCE_LOG:-$RUN_DIR/cadence-server.prometheus.log}"
CADENCE_PIDFILE="$RUN_DIR/cadence-server.prometheus.pid"

CANARY_ENDPOINT="${CANARY_ENDPOINT:-127.0.0.1:7943}"
CANARY_CSV="${CANARY_CSV:-$ROOT_DIR/tasklist1000_fixed.csv}"
CANARY_NAMESPACE="${CANARY_NAMESPACE:-shard-distributor-replay}"
CANARY_EXECUTORS="${CANARY_EXECUTORS:-9}"

die() { echo "ERROR: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }

stop_pidfile() {
  local pidfile="$1"
  [[ -f "$pidfile" ]] || return 0
  local pid
  pid="$(cat "$pidfile" 2>/dev/null || true)"
  [[ -n "${pid:-}" ]] || { rm -f "$pidfile"; return 0; }

  if kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid" >/dev/null 2>&1 || true
    sleep 1
    kill -0 "$pid" >/dev/null 2>&1 && kill -9 "$pid" >/dev/null 2>&1 || true
  fi
  rm -f "$pidfile" || true
}

kill_leftovers() {
  echo "==> Killing leftovers from prior runs (best-effort)"
  local repo
  repo="$(cd "$ROOT_DIR" && pwd -P)"

  stop_pidfile "$CADENCE_PIDFILE"
  stop_pidfile "$ETCD_PIDFILE"

  # Scope kills to binaries launched from this repo path
  pkill -f "$repo/.*cadence-server( |$)" 2>/dev/null || true
  pkill -f "$repo/.*sharddistributor-canary( |$)" 2>/dev/null || true
  sleep 1
  pkill -9 -f "$repo/.*cadence-server( |$)" 2>/dev/null || true
  pkill -9 -f "$repo/.*sharddistributor-canary( |$)" 2>/dev/null || true
}

start_etcd_detached() {
  echo "==> Starting etcd (detached) if not already running"
  if pgrep -x etcd >/dev/null 2>&1; then
    echo "etcd already running; leaving it as-is"
    return 0
  fi

  need "$ETCD_CMD"
  : > "$ETCD_LOG"

  # Detach so it survives script exit / Ctrl+C and keeps logging
  nohup $ETCD_CMD >>"$ETCD_LOG" 2>&1 &
  echo $! > "$ETCD_PIDFILE"

  echo "etcd pid: $(cat "$ETCD_PIDFILE")"
  echo "etcd log: $ETCD_LOG"
}

restart_prom_graf() {
  echo "==> Restarting prometheus + grafana via docker compose"
  cd "$DOCKER_DIR"

  local DC
  if docker compose version >/dev/null 2>&1; then
    DC="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    DC="docker-compose"
  else
    die "Neither 'docker compose' nor 'docker-compose' found"
  fi

  $DC stop prometheus grafana
  $DC rm -f -v prometheus grafana
  $DC up -d prometheus grafana
}

start_cadence_server_detached() {
  echo "==> Building cadence + cadence-server"
  cd "$ROOT_DIR"
  make cadence
  make cadence-server

  echo "==> Starting cadence-server (detached; keeps running)"
  : > "$CADENCE_LOG"

  # Detach so it survives script exit / Ctrl+C and keeps logging
  nohup ./cadence-server --zone prometheus start --services shard-distributor >>"$CADENCE_LOG" 2>&1 &
  echo $! > "$CADENCE_PIDFILE"

  echo "cadence-server pid: $(cat "$CADENCE_PIDFILE")"
  echo "cadence-server log: $CADENCE_LOG"
}

start_canary_foreground() {
  echo "==> Building sharddistributor-canary"
  cd "$ROOT_DIR"
  make sharddistributor-canary

  echo "==> Starting sharddistributor-canary (foreground; Ctrl+C stops only this)"
  exec ./sharddistributor-canary start \
    --endpoint "$CANARY_ENDPOINT" \
    --replay-csv "$CANARY_CSV" \
    --replay-namespace "$CANARY_NAMESPACE" \
    --replay-num-fixed-executors "$CANARY_EXECUTORS"
}

cmd="${1:-start}"
case "$cmd" in
  start)
    need docker
    need make
    need pkill
    need pgrep
    need nohup

    kill_leftovers
    start_etcd_detached
    restart_prom_graf
    start_cadence_server_detached
    start_canary_foreground
    ;;

  stop)
    need pkill
    echo "==> Stopping canary + cadence-server + etcd (only if started via pidfiles)"
    stop_pidfile "$CADENCE_PIDFILE"
    stop_pidfile "$ETCD_PIDFILE"
    # Also best-effort stop canary by name (scoped by repo path)
    repo="$(cd "$ROOT_DIR" && pwd -P)"
    pkill -f "$repo/.*sharddistributor-canary( |$)" 2>/dev/null || true
    ;;

  *)
    echo "Usage: $0 {start|stop}" >&2
    exit 2
    ;;
esac
