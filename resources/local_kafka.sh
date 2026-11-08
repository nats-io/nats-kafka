#!/usr/bin/env bash
#
# Copyright 2026 The NATS Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Helper around the dockerized Kafka from resources/local_multicell.yml.
# Local validation only; pairs with HOWTO.md.
#
# Usage:
#   resources/local_kafka.sh list         list all topics
#   resources/local_kafka.sh describe     describe the bridge output topic
#   resources/local_kafka.sh read [N]     read the topic from the beginning;
#                                         with N, exit after N messages,
#                                         without N, follow until Ctrl-C
#
# The topic defaults to the local bridge output topic; override with
# TOPIC=<name> for ad hoc use.

set -euo pipefail

BROKER="localhost:9192"
CONTAINER="nats_kafka_multicell-kafka-1"
TOPIC="${TOPIC:-brand.telemetry}"
BIN="/opt/kafka/bin"

# Allocate a TTY for interactive use so Ctrl-C reaches the consumer.
DOCKER_FLAGS=(-i)
if [ -t 0 ]; then
  DOCKER_FLAGS=(-it)
fi

cmd="${1:-read}"
case "$cmd" in
  list)
    docker exec "$CONTAINER" "$BIN/kafka-topics.sh" \
      --list --bootstrap-server "$BROKER"
    ;;
  describe)
    docker exec "$CONTAINER" "$BIN/kafka-topics.sh" \
      --describe --topic "$TOPIC" --bootstrap-server "$BROKER"
    ;;
  read)
    max_args=()
    if [ -n "${2:-}" ]; then
      max_args=(--max-messages "$2" --timeout-ms 15000)
    fi
    docker exec "${DOCKER_FLAGS[@]}" "$CONTAINER" "$BIN/kafka-console-consumer.sh" \
      --bootstrap-server "$BROKER" --topic "$TOPIC" --from-beginning \
      "${max_args[@]+"${max_args[@]}"}"
    ;;
  *)
    echo "usage: $0 {list|describe|read [N]}" >&2
    exit 1
    ;;
esac
