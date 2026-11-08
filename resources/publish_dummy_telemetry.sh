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
# Publishes dummy JSON telemetry to both local NATS cells, picking a
# random cell, car, and payload for each message. Local validation only;
# pairs with resources/local_multicell.yml and HOWTO.md.
#
# Usage: resources/publish_dummy_telemetry.sh [count] [interval-seconds]
#   count             number of messages to publish (default 20)
#   interval-seconds  sleep between messages, may be fractional (default 0.2)

set -euo pipefail

COUNT="${1:-20}"
INTERVAL="${2:-0.2}"

CELL_NAMES=("cell-a" "cell-b")
CELL_URLS=("nats://localhost:4222" "nats://localhost:4223")

if ! command -v nats >/dev/null; then
  echo "error: the nats CLI is required (brew install nats-io/nats-tools/nats)" >&2
  exit 1
fi

for ((i = 1; i <= COUNT; i++)); do
  cell=$((RANDOM % 2))
  car="car-$((RANDOM % 5 + 1))"
  speed=$((RANDOM % 130))
  soc=$((RANDOM % 101))
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  subject="telemetry.dummy.${car}"
  payload="$(printf '{"car_id":"%s","source_cell":"%s","seq":%d,"speed_kph":%d,"soc_pct":%d,"ts":"%s"}' \
    "$car" "${CELL_NAMES[$cell]}" "$i" "$speed" "$soc" "$ts")"

  nats -s "${CELL_URLS[$cell]}" pub "$subject" "$payload" 2>/dev/null
  echo "[$i/$COUNT] ${CELL_NAMES[$cell]} $subject $payload"
  sleep "$INTERVAL"
done
